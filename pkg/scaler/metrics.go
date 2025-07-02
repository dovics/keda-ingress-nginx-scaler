package scaler

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/dovics/keda-ingress-nginx-scaler/pkg/utils"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"k8s.io/klog/v2"
)

type CounterCache struct {
	name     string
	internal time.Duration
	period   time.Duration

	addrs  []string
	addrCh chan []string

	parser expfmt.TextParser

	cacheSize int
	cache     map[string]*utils.Ring[*model.Sample]
	mu        sync.RWMutex

	indexFunc func(model.Metric) string
}

func NewCounterCache(name string, internal time.Duration, period time.Duration, addrCh chan []string) *CounterCache {
	cacheSize := int(period / internal)
	if period%internal != 0 {
		klog.Warningf("period %s should be a multiple of internal %s", period, internal)
		cacheSize += 1
	}

	return &CounterCache{
		name:     name,
		internal: internal,
		period:   period,
		addrCh:   addrCh,
		parser:   expfmt.TextParser{},

		cacheSize: cacheSize,

		cache: make(map[string]*utils.Ring[*model.Sample]),
	}
}

func (c *CounterCache) SetIndexFunc(f func(model.Metric) string) {
	c.indexFunc = f
}

func (c *CounterCache) Run() {
	ticker := time.NewTicker(c.period)
	klog.Infof("Starting counter cache for %s with period %s", c.name, c.period)
	for {
		select {
		case <-ticker.C:
			for _, addr := range c.addrs {
				klog.V(6).Infof("Fetching metrics from %s", addr)
				data, err := c.FetchMetrics(addr)
				if err != nil {
					continue
				}

				c.mu.Lock()
				for name, samples := range data {
					r, ok := c.cache[name]
					if !ok {
						r = utils.NewRing[*model.Sample](int(c.period / c.internal))
						c.cache[name] = r
					}

					r.Enqueue(samples)
				}
				c.mu.Unlock()
			}
		case addrs, ok := <-c.addrCh:
			if !ok {
				klog.Warning("CounterCache channel closed")
				return
			}

			c.addrs = addrs
		}
	}
}

func (c *CounterCache) FetchMetrics(url string) (map[string]*model.Sample, error) {
	resp, err := http.Get(url)
	if err != nil {
		klog.Errorf("Failed to fetch metrics from %s: %v", url, err)
		return nil, err
	}
	defer resp.Body.Close()

	metricFamilies, err := c.parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		klog.Errorf("Failed to parse metrics from %s: %v", url, err)
		return nil, err
	}

	samples := make(map[string]*model.Sample)
	for name, mf := range metricFamilies {
		if name != c.name {
			continue
		}

		for _, m := range mf.Metric {
			var labels model.Metric = model.Metric{
				model.MetricNameLabel: model.LabelValue(name),
			}
			for _, lp := range m.Label {
				labels[model.LabelName(lp.GetName())] = model.LabelValue(lp.GetValue())
			}

			// Extract value based on type
			var value float64
			switch mf.GetType() {
			case io_prometheus_client.MetricType_COUNTER:
				value = m.GetCounter().GetValue()
			case io_prometheus_client.MetricType_GAUGE:
				value = m.GetGauge().GetValue()
			case io_prometheus_client.MetricType_UNTYPED:
				value = m.GetUntyped().GetValue()
			}

			var index string
			if c.indexFunc != nil {
				index = c.indexFunc(labels)
			} else {
				index = labels.String()
			}

			samples[index] = &model.Sample{
				Metric:    labels,
				Value:     model.SampleValue(value),
				Timestamp: model.TimeFromUnixNano(m.GetTimestampMs() * 1000000),
			}
		}
	}

	return samples, nil
}

func (c *CounterCache) GetLatest(index string) (*model.Sample, error) {
	if c.cache[index] == nil {
		return nil, fmt.Errorf("index %s not found", index)
	}
	return c.cache[index].GetLatest(), nil
}

func (c *CounterCache) GetBefore(index string, beforeTime time.Duration) (*model.Sample, error) {
	if beforeTime > c.period {
		return nil, fmt.Errorf("beforeTime %s is greater than period %s", beforeTime, c.period)
	}

	if c.cache[index] == nil {
		return nil, fmt.Errorf("index %s not found", index)
	}

	before := beforeTime / c.internal
	if before%c.internal != 0 {
		klog.Warningf("beforeTime %s is not a multiple of internal %s", beforeTime, c.internal)
	}

	c.mu.RLock()
	result := c.cache[index].GetBefore(int(before))
	c.mu.RUnlock()

	return result, nil
}

func (c *CounterCache) IsActive(index string, beforeTime time.Duration) bool {
	// TODO: use beforeTime to check if the cache is active
	if c.cache[index] == nil {
		return false
	}

	c.mu.RLock()
	result := c.cache[index].IsFill()
	c.mu.RUnlock()

	return result
}
