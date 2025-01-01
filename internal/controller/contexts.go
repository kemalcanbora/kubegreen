package controller

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type ContextController struct {
	configAccess clientcmd.ConfigAccess
	clientset    *kubernetes.Clientset
}

func NewContextController() (*ContextController, error) {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename())
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &ContextController{
		configAccess: clientcmd.NewDefaultClientConfigLoadingRules(),
		clientset:    clientset,
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
