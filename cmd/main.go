package main

import (
	"flag"
	"time"

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
	var interval time.Duration
	var cacheDuration time.Duration

	flag.IntVar(&port, "port", 9443, "Port number to serve webhooks. Defaults to 9443")
	// flag.StringVar(&labelSelector, "label-selector", "", "Label selector to filter events. Defaults to empty string")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file. Defaults to in-cluster config")
	flag.DurationVar(&interval, "interval", 10*time.Second, "Interval to fetch metrics. Defaults to 10 seconds")
	flag.DurationVar(&cacheDuration, "cache-duration", 5*time.Minute, "Duration to cache metrics. Defaults to 5 minutes")

	// Initialize klog flags
	klog.InitFlags(nil)

	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig) // Fixed here
	if err != nil {
		klog.Fatalf("Failed to build config from kubeconfig: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to create clientset: %v", err)
	}

	server := server.NewServer(port)
	cache := scaler.NewMetricsAddrCache(clientset, labelSelector,
		scaler.GetIngressIdentity, scaler.GetIngressMetricsAddr)

	stopCh := make(chan struct{})
	defer close(stopCh)
	go cache.Run(stopCh)

	scaler := scaler.NewIngressNginxScaler(clientset, cache, interval, cacheDuration)
	klog.V(2).Info("Starting scaler server")
	if err := server.Start(scaler); err != nil {
		klog.Fatal(err)
	}
}
