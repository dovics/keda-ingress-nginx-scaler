package scaler

import (
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type MetricsAddrCache struct {
	podInformer cache.SharedIndexInformer

	cache   map[string]string
	watchCh map[string]chan []string

	getIdentity    func(obj *corev1.Pod) (string, error)
	getMetricsAddr func(obj *corev1.Pod) (string, error)
}

func NewMetricsAddrCache(clientset *kubernetes.Clientset, labelSelector string, getIdentity, getMetricsAddr func(obj *corev1.Pod) (string, error)) *MetricsAddrCache {
	factory := informers.NewSharedInformerFactoryWithOptions(
		clientset,
		time.Minute,
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = labelSelector
		}),
	)

	podInformer := factory.Core().V1().Pods().Informer()

	watchCh := make(map[string]chan []string)
	return &MetricsAddrCache{
		cache:          make(map[string]string),
		podInformer:    podInformer,
		getIdentity:    getIdentity,
		getMetricsAddr: getMetricsAddr,
		watchCh:        watchCh,
	}
}

func (c *MetricsAddrCache) Run(stopCh <-chan struct{}) {
	c.podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			if !IsReady(pod) {
				return
			}

			c.AddPod(pod)
		},
		UpdateFunc: func(_, newObj interface{}) {
			pod := newObj.(*corev1.Pod)
			if !IsReady(pod) {
				return
			}

			c.AddPod(pod)
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			c.RemovePod(pod)
		},
	})

	klog.V(2).Info("Starting pod informer in cache")
	c.podInformer.Run(stopCh)
}

func (c *MetricsAddrCache) AddPod(pod *corev1.Pod) {
	identity, err := c.getIdentity(pod)
	if err != nil {
		klog.Errorf("Failed to get identity for pod %s/%s: %v", pod.Namespace, pod.Name, err)
		return
	}

	addr, err := c.getMetricsAddr(pod)
	if err != nil {
		klog.Errorf("Failed to get address for pod %s/%s: %v", pod.Namespace, pod.Name, err)
		return
	}

	klog.V(4).Infof("Adding pod %s/%s with identity %s and address %s", pod.Namespace, pod.Name, identity, addr)
	c.cache[identity] = addr

	c.triggerWatch(identity)
}

func (c *MetricsAddrCache) RemovePod(pod *corev1.Pod) {
	identity, err := c.getIdentity(pod)
	if err != nil {
		klog.Errorf("Failed to get identity for pod %s/%s: %v", pod.Namespace, pod.Name, err)
		return
	}

	klog.V(4).Infof("Removing pod %s/%s from cache", pod.Namespace, pod.Name)
	delete(c.cache, identity)

	c.triggerWatch(identity)
}

func IsReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}

func (c *MetricsAddrCache) GetByGlob(glob string) []string {
	result := []string{}
	for key, addr := range c.cache {
		if match, err := filepath.Match(glob, key); match && err == nil {
			result = append(result, addr)
		}
	}

	return result
}

func (c *MetricsAddrCache) WatchByGlob(glob string) chan []string {
	ch := make(chan []string)
	c.watchCh[glob] = ch
	return ch
}

func (c *MetricsAddrCache) StopWatchByGlob(glob string) {
	ch, ok := c.watchCh[glob]
	if !ok {
		return
	}

	close(ch)
	delete(c.watchCh, glob)
}

func (c *MetricsAddrCache) triggerWatch(identity string) {
	for glob, ch := range c.watchCh {
		if match, err := filepath.Match(glob, identity); match && err == nil {
			ch <- c.GetByGlob(glob)
		}
	}
}
