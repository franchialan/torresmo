package benchmark

import (
	"context"
	"fmt"
	"math"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Result holds benchmark results.
type Result struct {
	Target     string
	Type       string // "redis", "postgres", "http"
	Duration   time.Duration
	TotalOps   int64
	SuccessOps int64
	FailedOps  int64
	OpsPerSec  float64
	Latencies  LatencyStats
	Details    map[string]string
}

// LatencyStats contains percentile latency measurements.
type LatencyStats struct {
	Min time.Duration
	Max time.Duration
	Avg time.Duration
	P50 time.Duration
	P95 time.Duration
	P99 time.Duration
}

// Options configures a benchmark run.
type Options struct {
	Target      string // host:port or URL
	Type        string // "redis", "postgres", "http"
	Duration    time.Duration
	Concurrency int
	// HTTP-specific
	Path   string
	Method string
}

// BenchmarkRedis runs a simple Redis benchmark using raw TCP.
func BenchmarkRedis(ctx context.Context, opts Options) (*Result, error) {
	if opts.Duration == 0 {
		opts.Duration = 10 * time.Second
	}
	if opts.Concurrency == 0 {
		opts.Concurrency = 10
	}

	result := &Result{
		Target:  opts.Target,
		Type:    "redis",
		Details: make(map[string]string),
	}

	var totalOps, successOps, failedOps atomic.Int64
	var latencies []time.Duration
	var mu sync.Mutex

	start := time.Now()
	deadline := start.Add(opts.Duration)

	var wg sync.WaitGroup
	for i := 0; i < opts.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			conn, err := net.DialTimeout("tcp", opts.Target, 5*time.Second)
			if err != nil {
				return
			}
			defer conn.Close()

			buf := make([]byte, 256)

			for time.Now().Before(deadline) {
				select {
				case <-ctx.Done():
					return
				default:
				}

				totalOps.Add(1)
				opStart := time.Now()

				// PING command in RESP protocol
				_, err := conn.Write([]byte("*1\r\n$4\r\nPING\r\n"))
				if err != nil {
					failedOps.Add(1)
					continue
				}

				conn.SetReadDeadline(time.Now().Add(2 * time.Second))
				_, err = conn.Read(buf)
				if err != nil {
					failedOps.Add(1)
					continue
				}

				lat := time.Since(opStart)
				successOps.Add(1)

				mu.Lock()
				latencies = append(latencies, lat)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	result.Duration = time.Since(start)
	result.TotalOps = totalOps.Load()
	result.SuccessOps = successOps.Load()
	result.FailedOps = failedOps.Load()
	result.OpsPerSec = float64(result.SuccessOps) / result.Duration.Seconds()
	result.Latencies = calculateLatencies(latencies)
	result.Details["command"] = "PING"
	result.Details["concurrency"] = fmt.Sprintf("%d", opts.Concurrency)

	return result, nil
}

// BenchmarkPostgres runs a simple Postgres benchmark using raw TCP.
// Since we don't add a postgres driver dependency, it measures connection throughput.
func BenchmarkPostgres(ctx context.Context, opts Options) (*Result, error) {
	if opts.Duration == 0 {
		opts.Duration = 10 * time.Second
	}
	if opts.Concurrency == 0 {
		opts.Concurrency = 5
	}

	result := &Result{
		Target:  opts.Target,
		Type:    "postgres",
		Details: make(map[string]string),
	}

	var totalOps, successOps, failedOps atomic.Int64
	var latencies []time.Duration
	var mu sync.Mutex

	start := time.Now()
	deadline := start.Add(opts.Duration)

	var wg sync.WaitGroup
	for i := 0; i < opts.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(deadline) {
				select {
				case <-ctx.Done():
					return
				default:
				}

				totalOps.Add(1)
				opStart := time.Now()

				conn, err := net.DialTimeout("tcp", opts.Target, 2*time.Second)
				if err != nil {
					failedOps.Add(1)
					continue
				}
				conn.Close()

				lat := time.Since(opStart)
				successOps.Add(1)

				mu.Lock()
				latencies = append(latencies, lat)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	result.Duration = time.Since(start)
	result.TotalOps = totalOps.Load()
	result.SuccessOps = successOps.Load()
	result.FailedOps = failedOps.Load()
	result.OpsPerSec = float64(result.SuccessOps) / result.Duration.Seconds()
	result.Latencies = calculateLatencies(latencies)
	result.Details["type"] = "connection"
	result.Details["concurrency"] = fmt.Sprintf("%d", opts.Concurrency)

	return result, nil
}

// BenchmarkHTTP runs an HTTP benchmark.
func BenchmarkHTTP(ctx context.Context, opts Options) (*Result, error) {
	if opts.Duration == 0 {
		opts.Duration = 10 * time.Second
	}
	if opts.Concurrency == 0 {
		opts.Concurrency = 10
	}
	if opts.Method == "" {
		opts.Method = "GET"
	}
	if opts.Path == "" {
		opts.Path = "/"
	}

	url := fmt.Sprintf("http://%s%s", opts.Target, opts.Path)

	result := &Result{
		Target:  opts.Target,
		Type:    "http",
		Details: make(map[string]string),
	}

	var totalOps, successOps, failedOps atomic.Int64
	statusCodes := make(map[int]*atomic.Int64)
	var statusMu sync.Mutex
	var latencies []time.Duration
	var mu sync.Mutex

	client := &http.Client{Timeout: 5 * time.Second}

	start := time.Now()
	deadline := start.Add(opts.Duration)

	var wg sync.WaitGroup
	for i := 0; i < opts.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(deadline) {
				select {
				case <-ctx.Done():
					return
				default:
				}

				totalOps.Add(1)
				opStart := time.Now()

				req, _ := http.NewRequestWithContext(ctx, opts.Method, url, nil)
				resp, err := client.Do(req)

				lat := time.Since(opStart)

				if err != nil {
					failedOps.Add(1)
					continue
				}
				resp.Body.Close()

				successOps.Add(1)

				// Track status codes
				statusMu.Lock()
				counter, ok := statusCodes[resp.StatusCode]
				if !ok {
					counter = &atomic.Int64{}
					statusCodes[resp.StatusCode] = counter
				}
				statusMu.Unlock()
				counter.Add(1)

				mu.Lock()
				latencies = append(latencies, lat)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	result.Duration = time.Since(start)
	result.TotalOps = totalOps.Load()
	result.SuccessOps = successOps.Load()
	result.FailedOps = failedOps.Load()
	result.OpsPerSec = float64(result.SuccessOps) / result.Duration.Seconds()
	result.Latencies = calculateLatencies(latencies)
	result.Details["url"] = url
	result.Details["method"] = opts.Method
	result.Details["concurrency"] = fmt.Sprintf("%d", opts.Concurrency)

	// Format status codes
	var codes []string
	for code, count := range statusCodes {
		codes = append(codes, fmt.Sprintf("%d: %d", code, count.Load()))
	}
	sort.Strings(codes)
	result.Details["status_codes"] = strings.Join(codes, ", ")

	return result, nil
}

func calculateLatencies(latencies []time.Duration) LatencyStats {
	if len(latencies) == 0 {
		return LatencyStats{}
	}

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	var total time.Duration
	for _, l := range latencies {
		total += l
	}

	n := len(latencies)
	return LatencyStats{
		Min: latencies[0],
		Max: latencies[n-1],
		Avg: total / time.Duration(n),
		P50: latencies[int(math.Floor(float64(n)*0.5))],
		P95: latencies[int(math.Floor(float64(n)*0.95))],
		P99: latencies[int(math.Min(float64(n)-1, math.Floor(float64(n)*0.99)))],
	}
}

// FormatResult returns a human-readable benchmark report.
func FormatResult(r *Result) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🐷 Torresmo Benchmark — %s (%s)\n\n", strings.ToUpper(r.Type), r.Target))
	sb.WriteString(fmt.Sprintf("  Duration:    %s\n", r.Duration.Round(time.Millisecond)))
	sb.WriteString(fmt.Sprintf("  Total ops:   %d\n", r.TotalOps))
	sb.WriteString(fmt.Sprintf("  Success:     %d\n", r.SuccessOps))
	sb.WriteString(fmt.Sprintf("  Failed:      %d\n", r.FailedOps))
	sb.WriteString(fmt.Sprintf("  Throughput:  %.0f ops/sec\n", r.OpsPerSec))
	sb.WriteString("\n  Latency:\n")
	sb.WriteString(fmt.Sprintf("    Min:  %s\n", r.Latencies.Min.Round(time.Microsecond)))
	sb.WriteString(fmt.Sprintf("    Avg:  %s\n", r.Latencies.Avg.Round(time.Microsecond)))
	sb.WriteString(fmt.Sprintf("    P50:  %s\n", r.Latencies.P50.Round(time.Microsecond)))
	sb.WriteString(fmt.Sprintf("    P95:  %s\n", r.Latencies.P95.Round(time.Microsecond)))
	sb.WriteString(fmt.Sprintf("    P99:  %s\n", r.Latencies.P99.Round(time.Microsecond)))
	sb.WriteString(fmt.Sprintf("    Max:  %s\n", r.Latencies.Max.Round(time.Microsecond)))

	if len(r.Details) > 0 {
		sb.WriteString("\n  Details:\n")
		for k, v := range r.Details {
			sb.WriteString(fmt.Sprintf("    %s: %s\n", k, v))
		}
	}

	errorRate := float64(0)
	if r.TotalOps > 0 {
		errorRate = float64(r.FailedOps) / float64(r.TotalOps) * 100
	}

	sb.WriteString("\n  Verdict: ")
	if errorRate > 5 {
		sb.WriteString(fmt.Sprintf("⚠ %.1f%% error rate\n", errorRate))
	} else {
		sb.WriteString("✓ Healthy performance\n")
	}

	return sb.String()
}
