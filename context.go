package webhook_core

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	DefaultKubeConfigPath = os.Getenv("HOME") + "/.kube/config"
)

var initConfigOnce sync.Once
var kubeconfig string
var config *rest.Config
var clientset *kubernetes.Clientset

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", DefaultKubeConfigPath, "path to kubeconfig file")
}

func initializeConfig() {
	var err error
	config, err = rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(fmt.Sprintf("Error in reading kubeconfig: %v", err))
		}
	}

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(fmt.Sprintf("Error in create kubernetes client: %v", err))
	}
}

// GetRESTConfig get REST configuration that created from loading kubeconfig
func GetRESTConfig() *rest.Config {
	initConfigOnce.Do(initializeConfig)
	return config
}

// GetClientset get Clientset object from global context
func GetClientset() *kubernetes.Clientset {
	initConfigOnce.Do(initializeConfig)
	return clientset
}

// GetNamespaceContext get information about namespace
func GetNamespaceContext(name string, options metav1.GetOptions, ctx context.Context) (*corev1.Namespace, error) {
	return GetClientset().CoreV1().Namespaces().Get(ctx, name, options)
}

// GetNamespace get information about namespace
func GetNamespace(name string, options metav1.GetOptions) (*corev1.Namespace, error) {
	return GetNamespaceContext(name, options, context.Background())
}

// GetPodContext get information about a POD
func GetPodContext(ns string, name string, options metav1.GetOptions, ctx context.Context) (*corev1.Pod, error) {
	return GetClientset().CoreV1().Pods(ns).Get(ctx, name, options)
}

// GetPod get information about a POD
func GetPod(ns string, name string, options metav1.GetOptions) (*corev1.Pod, error) {
	return GetPodContext(ns, name, options, context.Background())
}
