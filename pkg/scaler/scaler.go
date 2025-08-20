package scaler

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	pb "github.com/dovics/keda-ingress-nginx-scaler/pkg/api"
	"github.com/dovics/keda-ingress-nginx-scaler/pkg/utils"
	"github.com/prometheus/common/model"
)

type IngressNginxScalerMetadata struct {
	namespace string
	name      string

	ingressName      string
	ingressClass     string
	ingressClassGlob string

	period time.Duration
	qps    int64
}

func (s *IngressNginxScaler) parseIngressNginxScalerMetadata(ctx context.Context, scaledObject *pb.ScaledObjectRef) (*IngressNginxScalerMetadata, error) {
	metadata := &IngressNginxScalerMetadata{
		namespace: scaledObject.Namespace,
		name:      scaledObject.Name,
	}

	ingressName, ok := scaledObject.ScalerMetadata["ingressName"]
	if !ok || ingressName == "" {
		klog.Errorf("scalerobject %s/%s ingressName must be specified", scaledObject.Namespace, scaledObject.Name)
		return nil, status.Error(codes.InvalidArgument, "ingressName must be specified and not empty")
	}
	metadata.ingressName = ingressName

	periodStr, ok := scaledObject.ScalerMetadata["period"]
	if !ok || periodStr == "" {
		klog.Errorf("scalerobject %s/%s period must be specified", scaledObject.Namespace, scaledObject.Name)
		return nil, status.Error(codes.InvalidArgument, "period must be specified")
	}

	period, err := time.ParseDuration(periodStr)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	metadata.period = period

	qpsStr, ok := scaledObject.ScalerMetadata["qps"]
	if !ok || qpsStr == "" {
		klog.Errorf("scalerobject %s/%s qps must be specified", scaledObject.Namespace, scaledObject.Name)
		return nil, status.Error(codes.InvalidArgument, "qps must be specified")
	}

	qps, err := strconv.ParseInt(qpsStr, 10, 64)
	if err != nil {
		klog.Errorf("scalerobject %s/%s qps %s is not an integer", scaledObject.Namespace, scaledObject.Name, qpsStr)
		return nil, status.Error(codes.InvalidArgument, "qps must be an integer")
	}
	metadata.qps = qps

	ingressClass := scaledObject.ScalerMetadata["ingressClass"]
	if ingressClass == "" {
		ingress, err := s.clientset.NetworkingV1().Ingresses(metadata.namespace).Get(ctx, ingressName, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("scalerobject %s/%s get ingress err: %v", metadata.namespace, metadata.name, err)
			return nil, status.Error(codes.Internal, err.Error())
		}

		if ingress.Spec.IngressClassName != nil {
			ingressClass = *ingress.Spec.IngressClassName
		} else {
			ingressClass = EmptyIngressClass
		}
	}
	metadata.ingressClass = ingressClass
	metadata.ingressClassGlob = fmt.Sprintf("%s-*", ingressClass)

	return metadata, nil
}

const (
	MetricsName       string = "nginx_ingress_controller_requests"
	EmptyIngressClass string = "nginx_ingress_empty"
)

type IngressNginxScaler struct {
	clientset kubernetes.Interface
	watcher   utils.MetricsAddrWatcher

	cacheDuration time.Duration
	interval      time.Duration
	metricsCache  map[string]*utils.CounterCache
}

func NewIngressNginxScaler(clientset kubernetes.Interface, watcher utils.MetricsAddrWatcher, interval time.Duration, cacheDuration time.Duration) *IngressNginxScaler {
	return &IngressNginxScaler{
		clientset:     clientset,
		watcher:       watcher,
		interval:      interval,
		cacheDuration: cacheDuration,
		metricsCache:  make(map[string]*utils.CounterCache),
	}
}

func (s *IngressNginxScaler) getMetricsCache(globString string) *utils.CounterCache {
	if cache, ok := s.metricsCache[globString]; ok {
		return cache
	}

	watchCh := s.watcher.WatchByGlob(globString)
	cache := utils.NewCounterCache(MetricsName, s.interval, s.cacheDuration, watchCh)
	cache.SetIndexFunc(func(labels model.Metric) string {
		return string(labels["ingress"])
	})
	s.metricsCache[globString] = cache

	go cache.Run()
	return cache
}

func (s *IngressNginxScaler) IsActive(ctx context.Context, scaledObject *pb.ScaledObjectRef) (*pb.IsActiveResponse, error) {
	klog.V(6).Infof("IsActive called, scaledObject: %s/%s", scaledObject.Namespace, scaledObject.Name)
	metadata, err := s.parseIngressNginxScalerMetadata(ctx, scaledObject)
	if err != nil {
		return nil, err
	}

	cache := s.getMetricsCache(metadata.ingressClassGlob)

	if !cache.IsActive(metadata.ingressName, metadata.period) {
		return &pb.IsActiveResponse{
			Result: false,
		}, nil
	}

	return &pb.IsActiveResponse{
		Result: true,
	}, nil
}
func (s *IngressNginxScaler) StreamIsActive(scaledObject *pb.ScaledObjectRef, epsServer pb.ExternalScaler_StreamIsActiveServer) error {
	klog.V(6).Infof("StreamIsActive called, scaledObject: %s/%s", scaledObject.Namespace, scaledObject.Name)
	metadata, err := s.parseIngressNginxScalerMetadata(epsServer.Context(), scaledObject)
	if err != nil {
		return err
	}

	for {
		select {
		case <-epsServer.Context().Done():
			// call cancelled
			return nil
		case <-time.Tick(time.Minute):
			cache := s.getMetricsCache(metadata.ingressClassGlob)
			result := cache.IsActive(metadata.ingressClassGlob, metadata.period)

			if err = epsServer.Send(&pb.IsActiveResponse{
				Result: result,
			}); err != nil {
				klog.Errorf("scalerobject %s/%s send isActiveResponse err: %v", scaledObject.Namespace, scaledObject.Name, err)
			}
		}
	}
}

func (s *IngressNginxScaler) GetMetricSpec(ctx context.Context, scaledObject *pb.ScaledObjectRef) (*pb.GetMetricSpecResponse, error) {
	klog.V(6).Infof("GetMetricSpec called, scaledObject: %s/%s", scaledObject.Namespace, scaledObject.Name)

	metadata, err := s.parseIngressNginxScalerMetadata(ctx, scaledObject)
	if err != nil {
		return nil, err
	}

	_ = s.getMetricsCache(metadata.ingressClassGlob)

	return &pb.GetMetricSpecResponse{
		MetricSpecs: []*pb.MetricSpec{{
			MetricName: "ingress-nginx-qps",
			TargetSize: metadata.qps,
		}},
	}, nil
}

func (s *IngressNginxScaler) GetMetrics(ctx context.Context, metricRequest *pb.GetMetricsRequest) (*pb.GetMetricsResponse, error) {
	scaledObject := metricRequest.ScaledObjectRef
	klog.V(6).Infof("GetMetrics called, scaledObject: %s/%s", scaledObject.Namespace, scaledObject.Name)

	metadata, err := s.parseIngressNginxScalerMetadata(ctx, scaledObject)
	if err != nil {
		return nil, err
	}

	cache := s.getMetricsCache(metadata.ingressClassGlob)

	latest, err := cache.GetLatest(metadata.ingressName)
	if err != nil {
		klog.Errorf("scalerobject %s/%s get latest metrics cache err: %v", scaledObject.Namespace, scaledObject.Name, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	before, err := cache.GetBefore(metadata.ingressName, metadata.period)
	if err != nil {
		klog.Errorf("scalerobject %s/%s get before metrics cache err: %v", scaledObject.Namespace, scaledObject.Name, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	qps := (latest - before) / metadata.period.Seconds()
	klog.V(5).Infof("scalerobject %s/%s qps: %f, latest: %f, before: %f", scaledObject.Namespace, scaledObject.Name, qps, latest, before)
	return &pb.GetMetricsResponse{
		MetricValues: []*pb.MetricValue{{
			MetricName:       "ingress-nginx-qps",
			MetricValueFloat: qps,
		}},
	}, nil
}
