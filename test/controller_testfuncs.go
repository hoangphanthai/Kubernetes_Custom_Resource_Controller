package test

import (
	"github.com/hoangphanthai/Kubernetes_Custom_Resource_Controller/api/types/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var storageClassName string = "jiva-pods-in-openebs-ns"

func testCreateDeployment(newApp *v1alpha1.Application, extSecretIsValid, configmapExists, secretExists bool) appsv1.Deployment {

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

	return *deployment
}

func testCreateStatefulset(newApp *v1alpha1.Application, configmapExists, secretExists bool) appsv1.StatefulSet {
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
	return *statefulset

}
