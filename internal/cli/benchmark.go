package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/torresmo-dev/torresmo/internal/benchmark"
)

var benchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Run performance benchmarks on the cluster",
}

var benchmarkRedisCmd = &cobra.Command{
	Use:   "redis",
	Short: "Benchmark a Redis instance in the cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		clusterFlag, _ := cmd.Flags().GetString("cluster")
		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		namespace, _ := cmd.Flags().GetString("namespace")
		duration, _ := cmd.Flags().GetDuration("duration")
		concurrency, _ := cmd.Flags().GetInt("concurrency")

		o, err := getOrchestrator()
		if err != nil {
			return err
		}

		pod, err := o.GetPod(ctx, clusterName, namespace, "redis-0")
		if err != nil {
			return fmt.Errorf("failed to find redis pod: %w", err)
		}
		if pod.PodIP == "" {
			return fmt.Errorf("redis pod has no IP address (phase: %s)", pod.Phase)
		}

		target := fmt.Sprintf("%s:6379", pod.PodIP)
		fmt.Printf("Running benchmark against redis at %s...\n", target)

		result, err := benchmark.BenchmarkRedis(ctx, benchmark.Options{
			Target:      target,
			Duration:    duration,
			Concurrency: concurrency,
		})
		if err != nil {
			return fmt.Errorf("benchmark failed: %w", err)
		}

		fmt.Println()
		fmt.Print(benchmark.FormatResult(result))
		return nil
	},
}

var benchmarkPostgresCmd = &cobra.Command{
	Use:   "postgres",
	Short: "Benchmark a Postgres instance in the cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		clusterFlag, _ := cmd.Flags().GetString("cluster")
		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		namespace, _ := cmd.Flags().GetString("namespace")
		duration, _ := cmd.Flags().GetDuration("duration")
		concurrency, _ := cmd.Flags().GetInt("concurrency")

		o, err := getOrchestrator()
		if err != nil {
			return err
		}

		pod, err := o.GetPod(ctx, clusterName, namespace, "postgres-0")
		if err != nil {
			return fmt.Errorf("failed to find postgres pod: %w", err)
		}
		if pod.PodIP == "" {
			return fmt.Errorf("postgres pod has no IP address (phase: %s)", pod.Phase)
		}

		target := fmt.Sprintf("%s:5432", pod.PodIP)
		fmt.Printf("Running benchmark against postgres at %s...\n", target)

		result, err := benchmark.BenchmarkPostgres(ctx, benchmark.Options{
			Target:      target,
			Duration:    duration,
			Concurrency: concurrency,
		})
		if err != nil {
			return fmt.Errorf("benchmark failed: %w", err)
		}

		fmt.Println()
		fmt.Print(benchmark.FormatResult(result))
		return nil
	},
}

var benchmarkHTTPCmd = &cobra.Command{
	Use:   "http <pod-name>",
	Short: "Benchmark an HTTP endpoint in the cluster",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		podName := args[0]

		clusterFlag, _ := cmd.Flags().GetString("cluster")
		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		namespace, _ := cmd.Flags().GetString("namespace")
		duration, _ := cmd.Flags().GetDuration("duration")
		concurrency, _ := cmd.Flags().GetInt("concurrency")
		port, _ := cmd.Flags().GetInt("port")
		path, _ := cmd.Flags().GetString("path")
		method, _ := cmd.Flags().GetString("method")

		o, err := getOrchestrator()
		if err != nil {
			return err
		}

		pod, err := o.GetPod(ctx, clusterName, namespace, podName)
		if err != nil {
			return fmt.Errorf("failed to find pod %q: %w", podName, err)
		}
		if pod.PodIP == "" {
			return fmt.Errorf("pod %q has no IP address (phase: %s)", podName, pod.Phase)
		}

		target := fmt.Sprintf("%s:%d", pod.PodIP, port)
		fmt.Printf("Running benchmark against http at %s...\n", target)

		result, err := benchmark.BenchmarkHTTP(ctx, benchmark.Options{
			Target:      target,
			Duration:    duration,
			Concurrency: concurrency,
			Path:        path,
			Method:      method,
		})
		if err != nil {
			return fmt.Errorf("benchmark failed: %w", err)
		}

		fmt.Println()
		fmt.Print(benchmark.FormatResult(result))
		return nil
	},
}

func init() {
	benchmarkRedisCmd.Flags().StringP("namespace", "n", "default", "Namespace")
	benchmarkRedisCmd.Flags().String("cluster", "", "Cluster name")
	benchmarkRedisCmd.Flags().Duration("duration", 10*time.Second, "Benchmark duration")
	benchmarkRedisCmd.Flags().Int("concurrency", 10, "Number of concurrent workers")

	benchmarkPostgresCmd.Flags().StringP("namespace", "n", "default", "Namespace")
	benchmarkPostgresCmd.Flags().String("cluster", "", "Cluster name")
	benchmarkPostgresCmd.Flags().Duration("duration", 10*time.Second, "Benchmark duration")
	benchmarkPostgresCmd.Flags().Int("concurrency", 5, "Number of concurrent workers")

	benchmarkHTTPCmd.Flags().StringP("namespace", "n", "default", "Namespace")
	benchmarkHTTPCmd.Flags().String("cluster", "", "Cluster name")
	benchmarkHTTPCmd.Flags().Duration("duration", 10*time.Second, "Benchmark duration")
	benchmarkHTTPCmd.Flags().Int("concurrency", 10, "Number of concurrent workers")
	benchmarkHTTPCmd.Flags().Int("port", 80, "Target port")
	benchmarkHTTPCmd.Flags().String("path", "/", "HTTP path")
	benchmarkHTTPCmd.Flags().String("method", "GET", "HTTP method")

	benchmarkCmd.AddCommand(benchmarkRedisCmd)
	benchmarkCmd.AddCommand(benchmarkPostgresCmd)
	benchmarkCmd.AddCommand(benchmarkHTTPCmd)
	addCommand(benchmarkCmd)
}
