package main

import (
	"log"

	"github.com/spf13/pflag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/dovics/keda-ingress-nginx-scaler/pkg/scaler"
	"github.com/dovics/keda-ingress-nginx-scaler/pkg/server"
)

func main() {
	var port int
	var labelSelector string
	var kubeconfig string
	pflag.IntVar(&port, "port", 9443, "Port number to serve webhooks. Defaults to 9443")
	pflag.StringVar(&labelSelector, "label-selector", "", "Label selector to filter events. Defaults to empty string")
	pflag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file. Defaults to in-cluster config")

	pflag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig) // Fixed here
	if err != nil {
		log.Fatalf("Failed to build config from kubeconfig: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create clientset: %v", err)
	}

	server := server.NewServer(port)
	cache := scaler.NewMetricsAddrCache(clientset, labelSelector,
		scaler.GetIngressIdentity, scaler.GetIngressMetricsAddr)

	stopCh := make(chan struct{})
	defer close(stopCh)
	go cache.Run(stopCh)

	scaler := scaler.NewIngressNginxScaler(clientset, cache)
	klog.Info("Starting scaler server")
	if err := server.Start(scaler); err != nil {
		log.Fatal(err)
	}
}
