package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"
)

var benchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Run synthetic load test",
	Long:  `Run a benchmark test against the Argus server to measure performance.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		duration, _ := cmd.Flags().GetDuration("duration")
		concurrency, _ := cmd.Flags().GetInt("concurrency")
		target, _ := cmd.Flags().GetString("target")

		fmt.Printf("Argus Benchmark\n")
		fmt.Printf("================\n")
		fmt.Printf("Target: %s\n", target)
		fmt.Printf("Duration: %s\n", duration)
		fmt.Printf("Concurrency: %d\n", concurrency)
		fmt.Println()

		return runBenchmark(target, duration, concurrency)
	},
}

type benchmarkMetrics struct {
	totalRequests   int64
	successRequests int64
	failedRequests  int64
	totalLatency     int64
	minLatency       int64
	maxLatency       int64
}

func runBenchmark(target string, duration time.Duration, concurrency int) error {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var metrics benchmarkMetrics
	metrics.minLatency = 1<<63 - 1

	var wg sync.WaitGroup
	fmt.Printf("Starting benchmark...\n\n")

	startTime := time.Now()

	// Launch worker goroutines
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			benchmarkWorker(ctx, target, &metrics)
		}()
	}

	// Monitor progress
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			select {
			case <-ctx.Done():
				return
			default:
				total := atomic.LoadInt64(&metrics.totalRequests)
				success := atomic.LoadInt64(&metrics.successRequests)
				failed := atomic.LoadInt64(&metrics.failedRequests)
				elapsed := time.Since(startTime)
				rps := float64(total) / elapsed.Seconds()

				fmt.Printf("[%v] Total: %d, Success: %d, Failed: %d, RPS: %.2f\n",
					elapsed.Round(time.Second), total, success, failed, rps)
			}
		}
	}()

	wg.Wait()

	// Print results
	elapsedTime := time.Since(startTime)
	totalRequests := atomic.LoadInt64(&metrics.totalRequests)
	successRequests := atomic.LoadInt64(&metrics.successRequests)
	failedRequests := atomic.LoadInt64(&metrics.failedRequests)

	fmt.Println("\n════════════════════════════════════════")
	fmt.Println("Benchmark Results")
	fmt.Println("════════════════════════════════════════")
	fmt.Printf("Total requests:     %d\n", totalRequests)
	fmt.Printf("Successful:         %d\n", successRequests)
	fmt.Printf("Failed:             %d\n", failedRequests)
	fmt.Printf("Duration:           %v\n", elapsedTime.Round(time.Millisecond))

	if totalRequests > 0 {
		rps := float64(totalRequests) / elapsedTime.Seconds()
		fmt.Printf("Requests/sec:       %.2f\n", rps)

		avgLatency := time.Duration(atomic.LoadInt64(&metrics.totalLatency) / totalRequests)
		minLatency := time.Duration(atomic.LoadInt64(&metrics.minLatency))
		maxLatency := time.Duration(atomic.LoadInt64(&metrics.maxLatency))

		fmt.Printf("Latency (avg):      %v\n", avgLatency)
		fmt.Printf("Latency (min):      %v\n", minLatency)
		fmt.Printf("Latency (max):      %v\n", maxLatency)

		if totalRequests > 0 {
			successRate := float64(successRequests) / float64(totalRequests) * 100
			fmt.Printf("Success rate:       %.2f%%\n", successRate)
		}
	}

	return nil
}

func benchmarkWorker(ctx context.Context, target string, metrics *benchmarkMetrics) {
	client := &http.Client{Timeout: 30 * time.Second}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Generate random path
		paths := []string{"/api/v1/signals", "/api/v1/traces", "/api/v1/health", "/api/v1/rules"}
		path := paths[rand.Intn(len(paths))]

		url := fmt.Sprintf("http://%s%s", target, path)

		startTime := time.Now()
		resp, err := client.Get(url)
		latency := time.Since(startTime)

		atomic.AddInt64(&metrics.totalRequests, 1)
		atomic.AddInt64(&metrics.totalLatency, latency.Nanoseconds())

		if err != nil {
			atomic.AddInt64(&metrics.failedRequests, 1)
		} else {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				atomic.AddInt64(&metrics.successRequests, 1)
			} else {
				atomic.AddInt64(&metrics.failedRequests, 1)
			}
		}

		// Update min/max latency
		currentMin := atomic.LoadInt64(&metrics.minLatency)
		latencyNs := latency.Nanoseconds()
		if latencyNs < currentMin {
			atomic.StoreInt64(&metrics.minLatency, latencyNs)
		}

		currentMax := atomic.LoadInt64(&metrics.maxLatency)
		if latencyNs > currentMax {
			atomic.StoreInt64(&metrics.maxLatency, latencyNs)
		}
	}
}

func init() {
	rootCmd.AddCommand(benchmarkCmd)

	benchmarkCmd.Flags().DurationP("duration", "d", 60*time.Second, "Duration of benchmark")
	benchmarkCmd.Flags().IntP("concurrency", "c", 10, "Number of concurrent workers")
	benchmarkCmd.Flags().String("target", "localhost:9090", "Target server (host:port)")
}
