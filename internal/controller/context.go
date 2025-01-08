package controller

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
)

type ContextController struct {
	clientset  *kubernetes.Clientset
	config     *rest.Config
	mclientset *metrics.Clientset
}

func NewContextController() (*ContextController, error) {
	home := os.Getenv("HOME")
	kubeconfig := filepath.Join(home, ".kube", "config")

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	mclientset, err := metrics.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &ContextController{
		clientset:  clientset,
		config:     config,
		mclientset: mclientset,
	}, nil
}

func (c *ContextController) GetClientset() *kubernetes.Clientset {
	return c.clientset
}

func (c *ContextController) GetConfig() *rest.Config {
	return c.config
}

func (c *ContextController) GetMetricsClientset() *metrics.Clientset {
	return c.mclientset
}

func (c *ContextController) GetContexts() ([]string, error) {
	config, err := clientcmd.LoadFromFile(filepath.Join(os.Getenv("HOME"), ".kube", "config"))
	if err != nil {
		return nil, err
	}

	var contexts []string
	for name := range config.Contexts {
		contexts = append(contexts, name)
	}
	return contexts, nil
}

func (c *ContextController) SwitchContext(context string) error {
	configPath := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	config, err := clientcmd.LoadFromFile(configPath)
	if err != nil {
		return err
	}

	config.CurrentContext = context
	return clientcmd.WriteToFile(*config, configPath)
}
