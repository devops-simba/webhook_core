package webhook_core

import (
	"context"
	"flag"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var kubeconfig string
var masterURL string
var config *restclient.Config
var clientset *kubernetes.Clientset

func init() {
	home := os.Getenv("HOME")
	defaultPath := filepath.Join(home, ".kube", "config")
	flag.StringVar(&kubeconfig, "kubeconfig", defaultPath, "path to kubeconfig file")
	flag.StringVar(&masterURL, "masterURL", "", "master URL for kubernetes")
}

// InitializeGlobalContext initialize global context.
// DO NOT call this function, it will be called by server after `flag.Parse()`
func InitializeGlobalContext() error {
	var err error
	config, err = clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		return err
	}

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	return nil
}

// GetRESTConfig get REST configuration that created from loading kubeconfig
func GetRESTConfig() *restclient.Config { return config }

// GetClientset get Clientset object from global context
func GetClientset() *kubernetes.Clientset { return clientset }

// GetNamespace get information about namespace
func GetNamespace(name string, options metav1.GetOptions) (*corev1.Namespace, error) {
	return clientset.CoreV1().Namespaces().Get(context.TODO(), name, options)
}

// GetPod get information about a POD
func GetPod(ns string, name string, options metav1.GetOptions) (*corev1.Pod, error) {
	return clientset.CoreV1().Pods(ns).Get(context.TODO(), name, options)
}
