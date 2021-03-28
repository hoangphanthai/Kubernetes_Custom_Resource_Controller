package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"appscontroller/api/types/v1alpha1"
	clientV1alpha1 "appscontroller/clientset/v1alpha1"
)

var kubeconfig *string
var config *rest.Config
var clientSet1 *kubernetes.Clientset
var clientSet2 *clientV1alpha1.ExampleV1Alpha1Client

// var storageClassName string = "jiva-pods-in-openebs-ns"
// var storageClassName string = "manual"
var workingNspace string

func main() {

	initClient()

	// List namespaces
	/* nsList, err := clientSet1.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	fmt.Printf("There are currently %d namespaces:\n", len(nsList.Items))
	for _, ns := range nsList.Items {
		fmt.Printf("- %s\n", ns.Name)
	} */

	// Select namespace the controller will work on
	/* fmt.Printf("\nSelect a namespace (or Enter for default namespace): ")
	workingNspace = strReader() */

	workingNspace = "default"
	fmt.Println("The controller is working on [default] namespace!")

	_, err := clientSet2.Applications(workingNspace).List(metav1.ListOptions{})
	if err != nil {
		fmt.Printf(err.Error())
		fmt.Printf("\nCRD Application is not found on K8s, please apply 1 and relaunch the controller")
	} else {
		watchResources()
		//store := WatchResources(clientSet)
	}
}

func initClient() {
	var err error

	// test the in-cluster config
	_, err = rest.InClusterConfig()
	if err != nil {
		// if not running on K8s
		//fmt.Println("NOT running on K8s")
		//fmt.Println(err.Error())
		home := homedir.HomeDir()
		if home != "" {
			kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		} else {
			kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
		}
		flag.Parse()

		config, err2 := clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err2 != nil {
			fmt.Println(err2.Error())
		} else {
			v1alpha1.AddToScheme(scheme.Scheme)

			clientSet1, err = kubernetes.NewForConfig(config)
			if err != nil {
				panic(err)
			}

			clientSet2, err = clientV1alpha1.NewForConfig(config)
			if err != nil {
				panic(err)
			}
		}
	} else {
		// fmt.Println("Running on K8s")
		// creates the in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			fmt.Println(err.Error())
		}
		v1alpha1.AddToScheme(scheme.Scheme)

		clientSet1, err = kubernetes.NewForConfig(config)
		if err != nil {
			panic(err)
		}

		clientSet2, err = clientV1alpha1.NewForConfig(config)
		if err != nil {
			panic(err)
		}
	}
}

// strReader is to read and return a string from the keyboard
func strReader() string {
	reader := bufio.NewReader(os.Stdin)
	ns, _ := reader.ReadString('\n')
	ns = strings.TrimSuffix(ns, "\n")
	if len(ns) == 0 {
		ns = "default"
	}
	return ns
}
