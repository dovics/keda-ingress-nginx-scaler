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
	"github.com/prometheus/common/model"
)

const MetricsName string = "nginx_ingress_controller_requests"

type IngressNginxScaler struct {
	clientset kubernetes.Interface
	addrCache *MetricsAddrCache

	metricsCache map[string]*CounterCache
}

func NewIngressNginxScaler(clientset kubernetes.Interface, cache *MetricsAddrCache) *IngressNginxScaler {
	return &IngressNginxScaler{
		clientset:    clientset,
		addrCache:    cache,
		metricsCache: make(map[string]*CounterCache),
	}
}

func (s *IngressNginxScaler) parseGlobFromScaledObject(ctx context.Context, scaledObject *pb.ScaledObjectRef) (string, error) {
	ingressName := scaledObject.ScalerMetadata["ingressName"]
	ingressClass := scaledObject.ScalerMetadata["ingressClass"]
	if ingressClass == "" {
		ingress, err := s.clientset.NetworkingV1().Ingresses(scaledObject.Namespace).Get(ctx, ingressName, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("scalerobject %s/%s get ingress err: %v", scaledObject.Namespace, scaledObject.Name, err)
			return "", err
		}

		if ingress.Spec.IngressClassName != nil {
			ingressClass = *ingress.Spec.IngressClassName
		}
	}

	return fmt.Sprintf("%s-*", ingressClass), nil
}

func (s *IngressNginxScaler) getMetricsCache(globString string) *CounterCache {
	if cache, ok := s.metricsCache[globString]; ok {
		return cache
	}

	watchCh := s.addrCache.WatchByGlob(globString)
	cache := NewCounterCache(MetricsName, 10*time.Second, 10*time.Minute, watchCh)
	cache.SetIndexFunc(func(labels model.Metric) string {
		return string(labels["ingress"])
	})
	s.metricsCache[globString] = cache

	go cache.Run()
	return cache
}

func (s *IngressNginxScaler) IsActive(ctx context.Context, scaledObject *pb.ScaledObjectRef) (*pb.IsActiveResponse, error) {
	klog.V(6).Infof("IsActive called, scaledObject: %s/%s", scaledObject.Namespace, scaledObject.Name)
	ingressName := scaledObject.ScalerMetadata["ingressName"]
	if len(ingressName) == 0 {
		klog.Errorf("scalerobject %s/%s ingressName must be specified", scaledObject.Namespace, scaledObject.Name)
		return nil, status.Error(codes.InvalidArgument, "ingressName must be specified")
	}

	period, err := time.ParseDuration(scaledObject.ScalerMetadata["period"])
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	globString, err := s.parseGlobFromScaledObject(ctx, scaledObject)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	cache := s.getMetricsCache(globString)

	if !cache.IsActive(ingressName, period) {
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
	ingressName := scaledObject.ScalerMetadata["ingressName"]
	if len(ingressName) == 0 {
		klog.Errorf("scalerobject %s/%s ingressName must be specified", scaledObject.Namespace, scaledObject.Name)
		return status.Error(codes.InvalidArgument, "ingressName must be specified")
	}

	period, err := time.ParseDuration(scaledObject.ScalerMetadata["period"])
	if err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	for {
		select {
		case <-epsServer.Context().Done():
			// call cancelled
			return nil
		case <-time.Tick(time.Minute):
			globString, err := s.parseGlobFromScaledObject(epsServer.Context(), scaledObject)
			if err != nil {
				return status.Error(codes.Internal, err.Error())
			}

			cache := s.getMetricsCache(globString)
			result := cache.IsActive(globString, period)

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

	globString, err := s.parseGlobFromScaledObject(ctx, scaledObject)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	_ = s.getMetricsCache(globString)

	qpsValue := scaledObject.ScalerMetadata["qps"]
	qps, err := strconv.Atoi(qpsValue)
	if err != nil {
		klog.Errorf("scalerobject %s/%s qps %v is not an integer", scaledObject.Namespace, scaledObject.Name, qpsValue)
		return nil, status.Error(codes.InvalidArgument, "qps must be an integer")
	}

	return &pb.GetMetricSpecResponse{
		MetricSpecs: []*pb.MetricSpec{{
			MetricName: "ingress-nginx-qps",
			TargetSize: int64(qps),
		}},
	}, nil
}

func (s *IngressNginxScaler) GetMetrics(ctx context.Context, metricRequest *pb.GetMetricsRequest) (*pb.GetMetricsResponse, error) {
	scaledObject := metricRequest.ScaledObjectRef
	klog.V(6).Infof("GetMetrics called, scaledObject: %s/%s", scaledObject.Namespace, scaledObject.Name)

	ingressName := scaledObject.ScalerMetadata["ingressName"]
	if len(ingressName) == 0 {
		klog.Errorf("scalerobject %s/%s ingressName must be specified", scaledObject.Namespace, scaledObject.Name)
		return nil, status.Error(codes.InvalidArgument, "ingressName must be specified")
	}

	globString, err := s.parseGlobFromScaledObject(ctx, scaledObject)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	periodString := scaledObject.ScalerMetadata["period"]
	period, err := time.ParseDuration(periodString)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	cache := s.getMetricsCache(globString)

	latest, err := cache.GetLatest(ingressName)
	if err != nil {
		klog.Errorf("scalerobject %s/%s get latest metrics cache err: %v", scaledObject.Namespace, scaledObject.Name, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	before, err := cache.GetBefore(ingressName, period)
	if err != nil {
		klog.Errorf("scalerobject %s/%s get before metrics cache err: %v", scaledObject.Namespace, scaledObject.Name, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	qps := (latest - before) / period.Seconds()
	return &pb.GetMetricsResponse{
		MetricValues: []*pb.MetricValue{{
			MetricName:       "ingress-nginx-qps",
			MetricValueFloat: qps,
		}},
	}, nil
}
