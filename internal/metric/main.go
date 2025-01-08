package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
)

type ContainerMetrics struct {
	CPU         int64  `json:"cpu_millicores"`
	Memory      int64  `json:"memory_mb"`
	CPUDelta    string `json:"cpu_delta,omitempty"`
	MemoryDelta string `json:"memory_delta,omitempty"`
}

type PodMetrics struct {
	Namespace  string                      `json:"namespace"`
	Name       string                      `json:"name"`
	Age        string                      `json:"age"`
	Containers map[string]ContainerMetrics `json:"containers"`
}

type SystemMetrics struct {
	TotalCPUCapacity    int64   `json:"total_cpu_millicores"`
	TotalMemoryCapacity int64   `json:"total_memory_mb"`
	UsedCPU             int64   `json:"used_cpu_millicores"`
	UsedMemory          int64   `json:"used_memory_mb"`
	FreeCPU             int64   `json:"free_cpu_millicores"`
	FreeMemory          int64   `json:"free_memory_mb"`
	CPUUsagePercent     float64 `json:"cpu_usage_percent"`
	MemoryUsagePercent  float64 `json:"memory_usage_percent"`
}

type MetricsOutput struct {
	Timestamp    string        `json:"timestamp"`
	System       SystemMetrics `json:"system"`
	PodsUsingRAM int           `json:"pods_using_ram"`
	PodsUsingCPU int           `json:"pods_using_cpu"`
	Pods         []PodMetrics  `json:"pods"`
}

type MetricsCollector struct {
	clientset       *metrics.Clientset
	kubeClient      *kubernetes.Clientset
	previousMetrics map[string]ContainerMetrics
	podAges         map[string]time.Time
}

func NewMetricsCollector(metricsClient *metrics.Clientset, kubeClient *kubernetes.Clientset) *MetricsCollector {
	return &MetricsCollector{
		clientset:       metricsClient,
		kubeClient:      kubeClient,
		previousMetrics: make(map[string]ContainerMetrics),
		podAges:         make(map[string]time.Time),
	}
}

func (mc *MetricsCollector) getClusterCapacity(ctx context.Context) (SystemMetrics, error) {
	var metrics SystemMetrics

	nodes, err := mc.kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return metrics, fmt.Errorf("failed to list nodes: %v", err)
	}

	for _, node := range nodes.Items {
		cpuQuantity := node.Status.Capacity.Cpu().MilliValue()
		memoryBytes := node.Status.Capacity.Memory().Value()
		memoryMB := memoryBytes / (1024 * 1024)

		metrics.TotalCPUCapacity += cpuQuantity
		metrics.TotalMemoryCapacity += memoryMB
	}

	return metrics, nil
}

func (mc *MetricsCollector) updatePodAges(ctx context.Context) error {
	pods, err := mc.kubeClient.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, pod := range pods.Items {
		key := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
		mc.podAges[key] = pod.CreationTimestamp.Time
	}
	return nil
}

func (mc *MetricsCollector) collectMetrics(ctx context.Context) (*metricsv1beta1.PodMetricsList, error) {
	if err := mc.updatePodAges(ctx); err != nil {
		return nil, fmt.Errorf("failed to update pod ages: %v", err)
	}
	return mc.clientset.MetricsV1beta1().PodMetricses("").List(ctx, metav1.ListOptions{})
}

func getMetricKey(namespace, pod, container string) string {
	return fmt.Sprintf("%s/%s/%s", namespace, pod, container)
}

func getPodKey(namespace, pod string) string {
	return fmt.Sprintf("%s/%s", namespace, pod)
}

func formatDelta(current, previous int64) string {
	delta := current - previous
	if delta > 0 {
		return fmt.Sprintf("+%d", delta)
	} else if delta < 0 {
		return fmt.Sprintf("%d", delta)
	}
	return ""
}

func formatAge(duration time.Duration) string {
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func formatMetrics(podMetricsList *metricsv1beta1.PodMetricsList, collector *MetricsCollector) {
	ctx := context.Background()
	sysMetrics, err := collector.getClusterCapacity(ctx)
	if err != nil {
		fmt.Printf("Error getting cluster capacity: %v\n", err)
		return
	}

	output := MetricsOutput{
		Timestamp: time.Now().Format(time.RFC3339),
		System:    sysMetrics,
		Pods:      make([]PodMetrics, 0),
	}

	podMap := make(map[string]*PodMetrics)
	podsWithCPU := make(map[string]bool)
	podsWithRAM := make(map[string]bool)

	// Calculate total resource usage from pods
	for _, pod := range podMetricsList.Items {
		podKey := getPodKey(pod.Namespace, pod.Name)
		age := time.Since(collector.podAges[podKey])

		podMetrics := &PodMetrics{
			Namespace:  pod.Namespace,
			Name:       pod.Name,
			Age:        formatAge(age),
			Containers: make(map[string]ContainerMetrics),
		}

		for _, container := range pod.Containers {
			cpuQuantity := container.Usage.Cpu().MilliValue()
			memoryBytes := container.Usage.Memory().Value()
			memoryMB := memoryBytes / (1024 * 1024)

			key := getMetricKey(pod.Namespace, pod.Name, container.Name)

			metrics := ContainerMetrics{
				CPU:    cpuQuantity,
				Memory: memoryMB,
			}

			if prev, exists := collector.previousMetrics[key]; exists {
				if cpuDelta := formatDelta(cpuQuantity, prev.CPU); cpuDelta != "" {
					metrics.CPUDelta = cpuDelta
				}
				if memDelta := formatDelta(memoryMB, prev.Memory); memDelta != "" {
					metrics.MemoryDelta = memDelta
				}
			}

			collector.previousMetrics[key] = ContainerMetrics{
				CPU:    cpuQuantity,
				Memory: memoryMB,
			}

			podMetrics.Containers[container.Name] = metrics

			// Update system metrics
			output.System.UsedCPU += cpuQuantity
			output.System.UsedMemory += memoryMB

			// Track pods using resources
			if cpuQuantity > 0 {
				podsWithCPU[podKey] = true
			}
			if memoryMB > 0 {
				podsWithRAM[podKey] = true
			}
		}

		podMap[podKey] = podMetrics
	}

	// Calculate free resources and percentages
	output.System.FreeCPU = output.System.TotalCPUCapacity - output.System.UsedCPU
	output.System.FreeMemory = output.System.TotalMemoryCapacity - output.System.UsedMemory

	if output.System.TotalCPUCapacity > 0 {
		output.System.CPUUsagePercent = float64(output.System.UsedCPU) / float64(output.System.TotalCPUCapacity) * 100
	}
	if output.System.TotalMemoryCapacity > 0 {
		output.System.MemoryUsagePercent = float64(output.System.UsedMemory) / float64(output.System.TotalMemoryCapacity) * 100
	}

	// Update pod counts
	output.PodsUsingCPU = len(podsWithCPU)
	output.PodsUsingRAM = len(podsWithRAM)

	// Convert map to slice for consistent ordering
	for _, podMetrics := range podMap {
		output.Pods = append(output.Pods, *podMetrics)
	}

	// Output JSON
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		return
	}

	clearScreen()
	fmt.Println(string(jsonData))
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
	os.Stdout.Sync()
}

func main() {
	home := os.Getenv("HOME")
	kubeconfig := filepath.Join(home, ".kube", "config")

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	metricsClient, err := metrics.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	collector := NewMetricsCollector(metricsClient, kubeClient)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	fmt.Println("Starting metrics collection (Press Ctrl+C to stop)...")
	fmt.Println("Collecting metrics every 2 seconds...")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Collect initial metrics
	if metrics, err := collector.collectMetrics(ctx); err == nil {
		formatMetrics(metrics, collector)
	}

	for {
		select {
		case <-sigChan:
			fmt.Println("\nShutting down...")
			return
		case <-ticker.C:
			metrics, err := collector.collectMetrics(ctx)
			if err != nil {
				fmt.Printf("Error collecting metrics: %v\n", err)
				continue
			}
			formatMetrics(metrics, collector)
		}
	}
}
