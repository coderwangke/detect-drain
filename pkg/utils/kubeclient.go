package utils

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"time"
)

type KubeCient struct {
	KubeConfigPath string
	ClientSet *kubernetes.Clientset
}

func NewKubeClient(kubeConfigPath string) (*KubeCient, error) {
	client := &KubeCient{
		KubeConfigPath: kubeConfigPath,
	}

	err := client.build()
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (c *KubeCient) build() error {
	config, err := clientcmd.BuildConfigFromFlags("", c.KubeConfigPath)
	if err != nil {
		klog.Errorf("Fail to build config from flags: %v", err)
		return err
	}

	config.Timeout = time.Second * 10

	c.ClientSet, err = kubernetes.NewForConfig(config)
	if err != nil {
		klog.Errorf("Fail to create clientSet: %v", err)
		return err
	}

	return nil
}