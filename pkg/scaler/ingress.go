package scaler

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

func GetIngressIdentity(pod *corev1.Pod) (string, error) {
	if !IsIngressController(pod) {
		return "", errors.New("pod is not an ingress controller")
	}

	return fmt.Sprintf("%s-%s", GetIngressClass(pod), pod.GetName()), nil
}

func GetIngressMetricsAddr(pod *corev1.Pod) (string, error) {
	return fmt.Sprintf("http://%s:%d/metrics", pod.Status.PodIP, GetHealthzPortForIngressController(pod)), nil
}

func IsIngressController(pod *corev1.Pod) bool {
	if len(pod.Spec.Containers) != 1 {
		return false
	}

	container := pod.Spec.Containers[0]
	if container.Name == "controller" &&
		container.Args[0] == "/nginx-ingress-controller" {
		return true
	}

	return false
}

func GetIngressClass(pod *corev1.Pod) string {
	for _, container := range pod.Spec.Containers {
		if container.Name != "controller" {
			continue
		}

		for _, arg := range container.Args {
			if strings.HasPrefix(arg, "--ingress-class=") {
				return strings.TrimPrefix(arg, "--ingress-class=")
			}
		}
	}

	klog.V(4).Infof("Ingress controller pod %s does not have an ingress-class argument, use empty as default", pod.Name)
	return ""
}

const DefaultHealthzPort = 10254

func GetHealthzPortForIngressController(pod *corev1.Pod) int {
	for _, container := range pod.Spec.Containers {
		if container.Name != "controller" {
			continue
		}

		for _, arg := range container.Args {
			if after, ok := strings.CutPrefix(arg, "--healthz-port="); ok {
				port, err := strconv.Atoi(after)
				if err != nil {
					klog.Errorf("Failed to parse healthz port from container args: %v", err)
					return DefaultHealthzPort
				}

				return port
			}
		}
	}

	klog.V(4).Infof("Using default healthz port: %d", DefaultHealthzPort)
	return DefaultHealthzPort
}
