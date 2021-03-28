package test

import (
	"appscontroller/api/types/v1alpha1"
	"fmt"
	"io/ioutil"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
	apiv1 "k8s.io/api/core/v1"
)

func TestDeploymentCreation(t *testing.T) {
	var app1 v1alpha1.Application

	yamlFile, err := ioutil.ReadFile("test3.yaml")
	if err != nil {
		fmt.Println(err.Error())
	}
	err = yaml.Unmarshal(yamlFile, &app1)
	if err != nil {
		fmt.Println(err.Error())
	}
	app1.Name = "test3"
	app1.Spec.Database.Spec.Configmap.Name = "configmap"
	app1.Spec.Database.Spec.Secret.Name = "secret"

	gotDep := testCreateDeployment(&app1, true, true, true)

	containersMatch := reflect.DeepEqual(gotDep.Spec.Template.Spec.Containers, app1.Spec.Template.Spec.Containers)
	if !containersMatch {
		t.Errorf("Containers list in application and deployment are not identical")
	}
}

func TestStatefulsetCreation(t *testing.T) {
	var app1 v1alpha1.Application

	yamlFile, err := ioutil.ReadFile("test3.yaml")
	if err != nil {
		fmt.Println(err.Error())
	}
	err = yaml.Unmarshal(yamlFile, &app1)
	if err != nil {
		fmt.Println(err.Error())
	}
	app1.Name = "test3"
	app1.Spec.Database.Spec.Configmap.Name = "configmap"
	app1.Spec.Database.Spec.Secret.Name = "secret"

	containerListWithNoConfigAndSecret := []apiv1.Container{
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

	gotSflset := testCreateStatefulset(&app1, false, false)

	containersMatch := reflect.DeepEqual(gotSflset.Spec.Template.Spec.Containers, containerListWithNoConfigAndSecret)
	if !containersMatch {
		t.Errorf("Containers list in application and deployment are not identical")
	}

}
