package v1alpha1

//go:generate controller-gen object paths=/Users/delivion/Dev/Go_K8s/src/appscontroller/api/types/v1alpha1/application.go
import (
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Extsecretspec is
type Extsecretspec struct {
	URL    string `json:"url"`
	Reload bool   `json:"reload"`
}

// Dbspec is
type Dbspec struct {
	Dbname      string          `json:"dbname"`
	Disksize    string          `json:"disksize"`
	Clustersize int32           `json:"clustersize"`
	Configmap   apiv1.ConfigMap `json:"configmap"`
	Secret      apiv1.Secret    `json:"secret"`
}

// Dbsetting is
type Dbsetting struct {
	Enable bool   `json:"enable"`
	Spec   Dbspec `json:"spec"`
}

// DeploymentStatus is
type DeploymentStatus struct {
	Status appsv1.DeploymentStatus `json:"status"`
}

// Spec is
type Spec struct {
	Spec apiv1.PodSpec `json:"spec"`
}

// ApplicationSpec is
type ApplicationSpec struct {
	Replicas         int32            `json:"replicas"`
	Database         Dbsetting        `json:"database"`
	Externalsecret   Extsecretspec    `json:"externalsecret"`
	Template         Spec             `json:"template"`
	Deploymentstatus DeploymentStatus `json:"deploymentstatus"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ApplicationSpec `json:"spec"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Application `json:"items"`
}
