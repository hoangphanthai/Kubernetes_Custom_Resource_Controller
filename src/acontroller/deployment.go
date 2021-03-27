package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
	"acontroller/src/acontroller/api/types/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

func createExternalSecret(appName string, exSecretMap map[string]string) error {
	showMessage(" -> Creating External Secret")

	secret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: appName + "-externalsecret",
		},
		StringData: exSecretMap,
	}

	_, err := secretsClient.Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		showMessage(err.Error())
	}
	return err
}

func getEvnMap(url string) (map[string]string, error) {

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr, Timeout: 3 * time.Second}

	response, err := client.Get(url)
	if err != nil {
		return nil, err
	}

	var objmap map[string]interface{}
	bodyBytes, _ := ioutil.ReadAll(response.Body)

	err = json.Unmarshal(bodyBytes, &objmap)
	if err != nil {
		return nil, err
	}

	mapString := make(map[string]string)
	for key, value := range objmap {
		strKey := fmt.Sprintf("%v", key)
		strValue := fmt.Sprintf("%v", value)
		mapString[strKey] = strValue
	}

	defer response.Body.Close()

	return mapString, nil
}

func getExtSecretStatus(newApp *v1alpha1.Application) bool {
	// Get the previous version of secret
	oneSecret, errExSecret := secretsClient.Get(context.TODO(), newApp.Name+"-externalsecret", metav1.GetOptions{})
	extSecretIsValid := (errExSecret == nil)

	url := newApp.Spec.Externalsecret.URL
	exSecretMap, err := getEvnMap(url)

	if err == nil {
		if !extSecretIsValid {
			// Previous external secret is not existing -> create new from given URL
			err3 := createExternalSecret(newApp.Name, exSecretMap)
			if err3 != nil {
				showMessage("An error occurs when creating external secret in K8s")
				showMessage(err3.Error())
				extSecretIsValid = false
			} else {
				extSecretIsValid = true
				//showMessage(" -> New external secret is created successfully!")
			}
		} else {
			// External secret exists -> update it
			oneSecret.StringData = exSecretMap
			retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				_, updateErr := secretsClient.Update(context.TODO(), oneSecret, metav1.UpdateOptions{})
				return updateErr
			})

			if retryErr != nil {
				showMessage(retryErr.Error())
				showMessage("---Previous version of external secret remains")
			} else {
				showMessage("---External secret updated successfully...")
			}

			// Either old secret or the updated one is valid
			extSecretIsValid = true
		}
	} else {
		if len(url) > 0 {
			showMessage("The external secret (via URL) is changed or requested to reload, but invalid or inaccessible!")
		}
	}
	return extSecretIsValid
}

func createDeployment(newApp *v1alpha1.Application) {

	extSecretIsValid := getExtSecretStatus(newApp)

	// Check existence of database's configmap or secret on k8s
	_, err := configMapsClient.Get(context.TODO(), newApp.Spec.Database.Spec.Configmap.Name, metav1.GetOptions{})
	configmapExists := (err == nil)

	_, err = secretsClient.Get(context.TODO(), newApp.Spec.Database.Spec.Secret.Name, metav1.GetOptions{})
	secretExists := (err == nil)

	containerList := newApp.Spec.Template.Spec.Containers

	// 2.3 External secret is valid and created
	if newApp.Spec.Database.Enable {
		for i := 0; i < len(containerList); i++ {
			if configmapExists && secretExists {
				// Both database configmap and secret exists
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
			if extSecretIsValid {
				containerList[i].VolumeMounts = []apiv1.VolumeMount{{Name: "externalsecret", MountPath: "/external-secret"}}
			}
		}
	} else {
		for i := 0; i < len(containerList); i++ {
			if extSecretIsValid {
				containerList[i].VolumeMounts = []apiv1.VolumeMount{{Name: "externalsecret", MountPath: "/external-secret"}}
			}
		}
	}

	repNo := int32(newApp.Spec.Replicas)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: newApp.Name + "-deployment",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &repNo,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "trial",
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "trial",
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{},
					Volumes:    []apiv1.Volume{},
				},
			},
		},
	}

	if extSecretIsValid {
		deployment.Spec.Template.Spec.Volumes = []apiv1.Volume{
			{
				Name: "externalsecret",
				VolumeSource: apiv1.VolumeSource{
					Secret: &apiv1.SecretVolumeSource{
						SecretName: newApp.Name + "-externalsecret",
					},
				},
			}}
	}

	deployment.Spec.Template.Spec.Containers = containerList

	showMessage(" -> Creating Deployment")
	_, err = deploymentsClient.Create(context.TODO(), deployment, metav1.CreateOptions{})

	if err != nil {
		showMessage(err.Error())
	}
}
