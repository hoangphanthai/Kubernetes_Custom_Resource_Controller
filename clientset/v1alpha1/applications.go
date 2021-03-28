package clientV1alpha1

import (
	"context"

	"github.com/hoangphanthai/Kubernetes_Custom_Resource_Controller/api/types/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

// ApplicationInterface is
type ApplicationInterface interface {
	List(opts metav1.ListOptions) (*v1alpha1.ApplicationList, error)
	Get(name string, options metav1.GetOptions) (*v1alpha1.Application, error)
	Create(*v1alpha1.Application) (*v1alpha1.Application, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	Update(application *v1alpha1.Application, opts metav1.UpdateOptions) (result *v1alpha1.Application, err error)
	// ...
}

// applicationClient is
type applicationClient struct {
	restClient rest.Interface
	ns         string
}

func (c *applicationClient) List(opts metav1.ListOptions) (*v1alpha1.ApplicationList, error) {
	result := v1alpha1.ApplicationList{}
	err := c.restClient.
		Get().
		Namespace(c.ns).
		Resource("applications").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(context.Background()).
		Into(&result)

	return &result, err

}

func (c *applicationClient) Get(name string, opts metav1.GetOptions) (*v1alpha1.Application, error) {
	result := v1alpha1.Application{}
	err := c.restClient.
		Get().
		Namespace(c.ns).
		Resource("applications").
		Name(name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(context.Background()).
		Into(&result)

	return &result, err
}

func (c *applicationClient) Update(application *v1alpha1.Application, opts metav1.UpdateOptions) (result *v1alpha1.Application, err error) {
	result = &v1alpha1.Application{}
	err = c.restClient.Put().
		Namespace(c.ns).
		Resource("applications").
		Name(application.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(application).
		Do(context.Background()).
		Into(result)
	return
}

func (c *applicationClient) Create(application *v1alpha1.Application) (*v1alpha1.Application, error) {
	result := v1alpha1.Application{}
	err := c.restClient.
		Post().
		Namespace(c.ns).
		Resource("applications").
		Body(application).
		Do(context.Background()).
		Into(&result)

	return &result, err
}

func (c *applicationClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.restClient.
		Get().
		Namespace(c.ns).
		Resource("applications").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch(context.Background())
}
