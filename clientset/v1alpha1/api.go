//package v1alpha1
package clientV1alpha1

import (
	"github.com/hoangphanthai/Kubernetes_Custom_Resource_Controller/api/types/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

// ExampleV1Alpha1Interface is
type ExampleV1Alpha1Interface interface {
	Applications(namespace string) ApplicationInterface
}

// ExampleV1Alpha1Client is
type ExampleV1Alpha1Client struct {
	restClient rest.Interface
}

// NewForConfig is
func NewForConfig(c *rest.Config) (*ExampleV1Alpha1Client, error) {
	config := *c
	config.ContentConfig.GroupVersion = &schema.GroupVersion{Group: v1alpha1.GroupName, Version: v1alpha1.GroupVersion}
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()

	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	return &ExampleV1Alpha1Client{restClient: client}, nil
}

// Applications is
func (c *ExampleV1Alpha1Client) Applications(namespace string) ApplicationInterface {
	return &applicationClient{
		restClient: c.restClient,
		ns:         namespace,
	}
}
