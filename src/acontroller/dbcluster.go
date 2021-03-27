package main

import (
	"context"
	"fmt"
	"acontroller/src/acontroller/api/types/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func enableDBCluster(app *v1alpha1.Application) {
	fmt.Println()
	showMessage("->Enabling Postgres Cluster")
	createConfigmap(app)
	createSecret(app)

	if !configmapSecretIsValid(app.Spec.Database.Spec.Configmap.Name, app.Spec.Database.Spec.Secret.Name) {
		validateDBSecret(app)
	}

	createService(app)
	createStatefulset(app)
}

func disableDBCluster(app *v1alpha1.Application) {
	fmt.Println()
	showMessage("<-Disabling Postgres Cluster")

	deletePolicy := metav1.DeletePropagationForeground

	// Statefulset deleting
	err := statefulsetsClient.Delete(context.TODO(), app.Name+"-statefulset", metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy})
	if err != nil {
		//fmt.Printf("Database was disabled in %s application \n", app.Name)
	} else {
		showMessage(" <- Deleting Statefulset")
	}

	// Service deleting
	err = servicesClient.Delete(context.TODO(), app.Name+"-service", metav1.DeleteOptions{})
	if err != nil {
		//showMessage(err.Error())
	} else {
		showMessage(" <- Deleting Service")
	}

	// Secret deleting
	err = secretsClient.Delete(context.TODO(), app.Spec.Database.Spec.Secret.Name, metav1.DeleteOptions{})
	if err != nil {
		//showMessage(err.Error())
	} else {
		showMessage(" <- Deleting Secret")
	}

	// Configmap deleting
	err = configMapsClient.Delete(context.TODO(), app.Spec.Database.Spec.Configmap.Name, metav1.DeleteOptions{})
	if err != nil {
		//showMessage(err.Error())
	} else {
		showMessage(" <- Deleting Configmap")
	}

	// Secret deleting
	err = secretsClient.Delete(context.TODO(), app.Name+"-dbdefaultsecret", metav1.DeleteOptions{})
	if err != nil {
		//showMessage(err.Error())
	} else {
		showMessage(" <- Deleting Default Secret")
	}

}

func validateDBSecret(app *v1alpha1.Application) {
	showMessage("    -> Could not find the key POSTGRES_PASSWORD with non-empty value in both database's configmap or secret!")

	// Check if exists a secret then append that key to the secret, else create new secret called "dbsecret" containing the key POSTGRES_PASSWORD with its value being "default"
	oneSecret, errExSecret := secretsClient.Get(context.TODO(), app.Spec.Database.Spec.Secret.Name, metav1.GetOptions{})
	if errExSecret == nil {
		// Add key POSTGRES_PASSWORD and it default value to the current secret
		if oneSecret.StringData != nil {
			oneSecret.StringData["POSTGRES_PASSWORD"] = "password"
		} else {
			oneSecret.StringData = map[string]string{}
			oneSecret.StringData["POSTGRES_PASSWORD"] = "password"
		}
		_, err := secretsClient.Update(context.TODO(), oneSecret, metav1.UpdateOptions{})
		if err != nil {
			showMessage(err.Error())
		} else {
			showMessage("    -> Default POSTGRES_PASSWORD key and value are added to existing database's secret")
		}
	} else {
		showMessage("    -> Creating database's default secret")
		secret := apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: app.Name + "-dbdefaultsecret",
			},
			StringData: map[string]string{
				"POSTGRES_PASSWORD": "password",
			},
		}
		_, err1 := secretsClient.Create(context.TODO(), &secret, metav1.CreateOptions{})
		if err1 != nil {
			showMessage(err1.Error())
		} else {
			// Add default secret to the current application because other function will use it
			app.Spec.Database.Spec.Secret = secret
			_, er := clientSet2.Applications(workingNspace).Update(app, metav1.UpdateOptions{})
			if er != nil {
				showMessage(er.Error())
			}
		}
	}
}

func configmapSecretIsValid(confName, secretName string) bool {

	// Check whether a "POSTGRES_PASSWORD" key and non-empty value are presented in the database's configmap/secret
	config, errCfg := configMapsClient.Get(context.TODO(), confName, metav1.GetOptions{})
	if errCfg == nil {
		for key, value := range config.Data {
			if key == "POSTGRES_PASSWORD" && len(value) > 0 {
				return true
			}
		}
	}

	secret, errScr := secretsClient.Get(context.TODO(), secretName, metav1.GetOptions{})
	if errScr == nil {
		for key, value := range secret.StringData {
			if key == "POSTGRES_PASSWORD" && len(value) > 0 {
				return true
			}
		}
		for key, value := range secret.Data {
			if key == "POSTGRES_PASSWORD" && len(value) > 0 {
				return true
			}
		}
	}

	return false
}

func createConfigmap(newApp *v1alpha1.Application) {
	showMessage(" -> Creating Configmap")

	if len(newApp.Spec.Database.Spec.Configmap.Data) > 0 {
		// Create Configmap
		configMap := newApp.Spec.Database.Spec.Configmap
		_, err := configMapsClient.Create(context.TODO(), &configMap, metav1.CreateOptions{})
		if err != nil {
			showMessage(err.Error())
		}
	}
}

func createSecret(newApp *v1alpha1.Application) {
	showMessage(" -> Creating Secret")

	if len(newApp.Spec.Database.Spec.Secret.StringData) > 0 {
		// Create Secret
		secret := newApp.Spec.Database.Spec.Secret
		_, err := secretsClient.Create(context.TODO(), &secret, metav1.CreateOptions{})
		if err != nil {
			showMessage(err.Error())
		}
	}
}

func createService(newApp *v1alpha1.Application) {
	showMessage(" -> Creating Service")

	service := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: newApp.Name + "-service",
		},
		Spec: apiv1.ServiceSpec{
			Selector: map[string]string{
				"app": "postgres",
			},
			Type: "ClusterIP",
			Ports: []apiv1.ServicePort{
				{
					Port: 5432,
					Name: "postgres",
				},
			},
		},
	}

	_, err := servicesClient.Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		showMessage(err.Error())
	}

}

func createStatefulset(newApp *v1alpha1.Application) {
	showMessage(" -> Creating Statefulset")

	repNo := newApp.Spec.Database.Spec.Clustersize
	containerList := []apiv1.Container{
		{
			Name:            "postgres",
			Image:           "postgres:13",
			ImagePullPolicy: "IfNotPresent",
			Ports: []apiv1.ContainerPort{{
				ContainerPort: int32(5432),
				Name:          "postgredb",
			}},
		},
	}

	_, err := configMapsClient.Get(context.TODO(), newApp.Spec.Database.Spec.Configmap.Name, metav1.GetOptions{})
	configmapExists := (err == nil)
	_, err = secretsClient.Get(context.TODO(), newApp.Spec.Database.Spec.Secret.Name, metav1.GetOptions{})
	secretExists := (err == nil)

	for i := 0; i < len(containerList); i++ {
		if configmapExists && secretExists {
			// Both database configmap and secret exist
			containerList[i].EnvFrom = []apiv1.EnvFromSource{
				{ConfigMapRef: &apiv1.ConfigMapEnvSource{LocalObjectReference: apiv1.LocalObjectReference{Name: newApp.Spec.Database.Spec.Configmap.Name}}},
				{SecretRef: &apiv1.SecretEnvSource{LocalObjectReference: apiv1.LocalObjectReference{Name: newApp.Spec.Database.Spec.Secret.Name}}},
			}
		} else {
			// Either database configmap or secret exists
			if configmapExists {
				containerList[i].EnvFrom = []apiv1.EnvFromSource{
					{ConfigMapRef: &apiv1.ConfigMapEnvSource{LocalObjectReference: apiv1.LocalObjectReference{Name: newApp.Spec.Database.Spec.Configmap.Name}}},
				}
			}
			if secretExists {
				containerList[i].EnvFrom = []apiv1.EnvFromSource{
					{SecretRef: &apiv1.SecretEnvSource{LocalObjectReference: apiv1.LocalObjectReference{Name: newApp.Spec.Database.Spec.Secret.Name}}},
				}
			}
		}
	}

	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: newApp.Name + "-statefulset",
		},

		Spec: appsv1.StatefulSetSpec{
			Replicas: &repNo,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "postgres",
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "postgres",
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{},
				},
			},
			VolumeClaimTemplates: []apiv1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "postgredb",
				},
				Spec: apiv1.PersistentVolumeClaimSpec{
					AccessModes: []apiv1.PersistentVolumeAccessMode{
						"ReadWriteOnce"},
					StorageClassName: &storageClassName,
					Resources: apiv1.ResourceRequirements{
						Requests: apiv1.ResourceList{
							"storage": resource.MustParse(newApp.Spec.Database.Spec.Disksize),
						},
					},
				},
			},
			},
		},
	}

	statefulset.Spec.Template.Spec.Containers = containerList
	_, err = statefulsetsClient.Create(context.TODO(), statefulset, metav1.CreateOptions{})
	if err != nil {
		showMessage(err.Error())
	}

}
