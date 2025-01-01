package controller

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

type PodInfo struct {
	FormattedString string
	Pod             corev1.Pod
	ReadyCount      string
	Age             string
}

func (c *ContextController) GetPods() ([]PodInfo, error) {
	pods, err := c.clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var podInfos []PodInfo
	for _, pod := range pods.Items {
		ready := getPodReadyCount(pod)
		total := len(pod.Spec.Containers)

		podInfos = append(podInfos, PodInfo{
			FormattedString: fmt.Sprintf("%d/%d", ready, total),
			Pod:             pod,
			ReadyCount:      fmt.Sprintf("%d/%d", ready, total),
			Age:             formatAge(time.Since(pod.CreationTimestamp.Time)),
		})
	}

	return podInfos, nil
}

func getPodReadyCount(pod corev1.Pod) int {
	ready := 0
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.Ready {
			ready++
		}
	}
	return ready
}

func formatAge(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	if h > 48 {
		return fmt.Sprintf("%dd", h/24)
	} else if h > 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dm", m)
}
