package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// Config holds the configuration for the TPS benchmark
type Config struct {
	TargetURL    string
	TargetTPS    int
	Duration     time.Duration
	Concurrency  int
	ClientTimeout time.Duration
}

// Metrics holds the benchmark metrics
type Metrics struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	TotalLatency       time.Duration
}

func main() {
	var config Config
	var help bool

	flag.StringVar(&config.TargetURL, "url", "http://localhost:8080", "Target URL to send requests to")
	flag.IntVar(&config.TargetTPS, "tps", 100, "Target requests per second")
	flag.DurationVar(&config.Duration, "duration", 30*time.Second, "Test duration")
	flag.IntVar(&config.Concurrency, "concurrency", 10, "Number of concurrent workers")
	flag.DurationVar(&config.ClientTimeout, "timeout", 30*time.Second, "HTTP client timeout")
	flag.BoolVar(&help, "help", false, "Show help message")

	flag.Parse()

	if help {
		flag.Usage()
		return
	}

	if config.TargetURL == "" {
		log.Fatal("Target URL is required")
	}

	if config.TargetTPS <= 0 {
		log.Fatal("Target TPS must be positive")
	}

	fmt.Printf("Starting TPS benchmark:\n")
	fmt.Printf("  Target URL: %s\n", config.TargetURL)
	fmt.Printf("  Target TPS: %d\n", config.TargetTPS)
	fmt.Printf("  Duration: %v\n", config.Duration)
	fmt.Printf("  Concurrency: %d\n", config.Concurrency)
	fmt.Printf("  Timeout: %v\n", config.ClientTimeout)
	fmt.Println()

	metrics := runBenchmark(config)
	printMetrics(metrics, config)
}

func runBenchmark(config Config) *Metrics {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: config.ClientTimeout,
	}

	// Create metrics collector
	metrics := &Metrics{}
	var metricsMutex sync.Mutex

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), config.Duration)
	defer cancel()

	// Calculate interval between requests to achieve target TPS
	requestInterval := time.Duration(int64(time.Second) / int64(config.TargetTPS))

	// Create workers
	var wg sync.WaitGroup
	requestsChan := make(chan time.Time, config.TargetTPS*2)

	// Start worker goroutines
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case requestTime, ok := <-requestsChan:
					if !ok {
						return
					}

					startTime := time.Now()
					resp, err := client.Get(config.TargetURL)
					requestLatency := time.Since(startTime)

					metricsMutex.Lock()
					metrics.TotalRequests++
					if err != nil {
						metrics.FailedRequests++
						log.Printf("Request failed: %v", err)
					} else {
						metrics.SuccessfulRequests++
						metrics.TotalLatency += requestLatency
						resp.Body.Close()
					}
					metricsMutex.Unlock()

					// Sleep to maintain consistent request timing
					sleepTime := requestTime.Add(requestInterval).Sub(time.Now())
					if sleepTime > 0 {
						time.Sleep(sleepTime)
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// Start ticker to send requests at fixed intervals
	ticker := time.NewTicker(requestInterval)
	defer ticker.Stop()

	// Send requests for the specified duration
	go func() {
		for {
			select {
			case t := <-ticker.C:
				select {
				case requestsChan <- t:
				case <-ctx.Done():
					close(requestsChan)
					return
				}
			case <-ctx.Done():
				close(requestsChan)
				return
			}
		}
	}()

	// Wait for test to complete
	<-ctx.Done()
	wg.Wait()

	return metrics
}

func printMetrics(metrics *Metrics, config Config) {
	fmt.Println("\n=== Benchmark Results ===")
	fmt.Printf("Total Requests: %d\n", metrics.TotalRequests)
	fmt.Printf("Successful Requests: %d\n", metrics.SuccessfulRequests)
	fmt.Printf("Failed Requests: %d\n", metrics.FailedRequests)
	fmt.Printf("Actual TPS: %.2f\n", float64(metrics.TotalRequests)/config.Duration.Seconds())

	if metrics.SuccessfulRequests > 0 {
		avgLatency := metrics.TotalLatency / time.Duration(metrics.SuccessfulRequests)
		fmt.Printf("Average Latency: %v\n", avgLatency)
	}

	if metrics.TotalRequests > 0 {
		successRate := float64(metrics.SuccessfulRequests) / float64(metrics.TotalRequests) * 100
		fmt.Printf("Success Rate: %.2f%%\n", successRate)
	}
}