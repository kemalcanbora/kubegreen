package controller

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type ContextController struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
}

func (c *ContextController) GetConfig() *rest.Config {
	return c.config
}

func NewContextController() (*ContextController, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &ContextController{
		clientset: clientset,
		config:    config,
	}, nil
}

func (c *ContextController) GetContexts() ([]string, error) {
	configAccess := clientcmd.NewDefaultPathOptions()
	config, err := configAccess.GetStartingConfig()
	if err != nil {
		return nil, err
	}

	contexts := make([]string, 0, len(config.Contexts))
	for name := range config.Contexts {
		contexts = append(contexts, name)
	}
	return contexts, nil
}

func (c *ContextController) SwitchContext(name string) error {
	configAccess := clientcmd.NewDefaultPathOptions()
	config, err := configAccess.GetStartingConfig()
	if err != nil {
		return err
	}

	config.CurrentContext = name
	return clientcmd.ModifyConfig(configAccess, *config, true)
}

func (c *ContextController) GetClientset() *kubernetes.Clientset {
	return c.clientset
}
