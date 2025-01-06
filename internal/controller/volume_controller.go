package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	podTimeout    = 5 * time.Minute
	podPollPeriod = 2 * time.Second
)

func printHeader(title string) {
	fmt.Printf("\r\n=== %s ===\n", title)
}

func printFooter() {
	fmt.Printf("\r\n================\n")
}

func printStep(format string, args ...interface{}) {
	fmt.Printf("\r➡️  %s\n", fmt.Sprintf(format, args...))
}

func printSuccess(format string, args ...interface{}) {
	fmt.Printf("\r✅ %s\n", fmt.Sprintf(format, args...))
}

func printMsg(format string, args ...interface{}) {
	fmt.Printf("\r%s\n", fmt.Sprintf(format, args...))
}

type VolumeInfo struct {
	Name         string
	Namespace    string
	Size         string
	Status       string
	StorageClass string
	AccessModes  []corev1.PersistentVolumeAccessMode
	VolumeMode   *corev1.PersistentVolumeMode
	Labels       map[string]string
	Annotations  map[string]string
}

type VolumeController struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
}

func NewVolumeController(clientset *kubernetes.Clientset, config *rest.Config) *VolumeController {
	return &VolumeController{
		clientset: clientset,
		config:    config,
	}
}

// ListVolumes returns a list of all PVCs in the cluster
func (vc *VolumeController) ListVolumes() ([]VolumeInfo, error) {
	pvcs, err := vc.clientset.CoreV1().PersistentVolumeClaims("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list PVCs: %v", err)
	}

	var volumes []VolumeInfo
	for _, pvc := range pvcs.Items {
		volumes = append(volumes, VolumeInfo{
			Name:         pvc.Name,
			Namespace:    pvc.Namespace,
			Size:         pvc.Spec.Resources.Requests.Storage().String(),
			Status:       string(pvc.Status.Phase),
			StorageClass: getStorageClassName(pvc.Spec.StorageClassName),
			AccessModes:  pvc.Spec.AccessModes,
			VolumeMode:   pvc.Spec.VolumeMode,
			Labels:       pvc.Labels,
			Annotations:  pvc.Annotations,
		})
	}
	return volumes, nil
}

func getStorageClassName(sc *string) string {
	if sc == nil {
		return ""
	}
	return *sc
}

func (vc *VolumeController) findPodsUsingPVC(namespace, pvcName string) ([]corev1.Pod, error) {
	pods, err := vc.clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var podsUsingPVC []corev1.Pod
	for _, pod := range pods.Items {
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvcName {
				podsUsingPVC = append(podsUsingPVC, pod)
				break
			}
		}
	}
	return podsUsingPVC, nil
}

func (vc *VolumeController) waitForPodTermination(ctx context.Context, namespace, pvcName string) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, podTimeout)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout waiting for pods to terminate")
		default:
			podsUsingPVC, err := vc.findPodsUsingPVC(namespace, pvcName)
			if err != nil {
				return fmt.Errorf("failed to check if pods are terminated: %v", err)
			}
			if len(podsUsingPVC) == 0 {
				return nil
			}
			time.Sleep(podPollPeriod)
		}
	}
}

func (vc *VolumeController) deletePod(ctx context.Context, namespace, name string) error {
	// Try graceful deletion first
	err := vc.clientset.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	// Wait for pod to be deleted or force delete after timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			// Force delete if graceful deletion takes too long
			gracePeriod := int64(0)
			return vc.clientset.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{
				GracePeriodSeconds: &gracePeriod,
			})
		default:
			_, err := vc.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return nil // Pod is deleted
			}
			time.Sleep(time.Second)
		}
	}
}

func (vc *VolumeController) waitForPVCDeletion(ctx context.Context, namespace, name string) error {
	printStep("Waiting for PVC %s to be deleted...", name)
	for i := 0; i < 30; i++ {
		_, err := vc.clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			// PVC is deleted
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("timeout waiting for PVC %s to be deleted", name)
}

func (vc *VolumeController) DeleteVolume(namespace, name string) error {
	ctx := context.TODO()

	printHeader("Volume Delete Operation")
	printMsg("PVC: %s/%s", namespace, name)
	printFooter()

	// First check if PVC exists
	pvc, err := vc.clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get PVC: %v", err)
	}

	printStep("Found PVC: %s (Status: %s)", name, pvc.Status.Phase)

	// Find deployments using the PVC
	deployments, err := vc.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list deployments: %v", err)
	}

	// Track deployments using this PVC
	var deploymentsToRestore []struct {
		name     string
		replicas int32
	}

	for _, deployment := range deployments.Items {
		usePVC := false
		for _, volume := range deployment.Spec.Template.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == name {
				usePVC = true
				break
			}
		}

		if usePVC {
			printStep("Found deployment %s using the PVC", deployment.Name)
			deploymentsToRestore = append(deploymentsToRestore, struct {
				name     string
				replicas int32
			}{
				name:     deployment.Name,
				replicas: *deployment.Spec.Replicas,
			})

			// Scale down the deployment
			printStep("Scaling down deployment %s", deployment.Name)
			var replicas int32 = 0
			deployment.Spec.Replicas = &replicas
			_, err := vc.clientset.AppsV1().Deployments(namespace).Update(ctx, &deployment, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to scale down deployment %s: %v", deployment.Name, err)
			}
		}
	}

	// Double check for any pods still using the PVC
	podsUsingPVC, err := vc.findPodsUsingPVC(namespace, name)
	if err != nil {
		return fmt.Errorf("failed to check for pods using PVC: %v", err)
	}

	if len(podsUsingPVC) > 0 {
		printStep("Found %d pods still using the PVC", len(podsUsingPVC))
		for _, pod := range podsUsingPVC {
			printMsg("    └─ Pod: %s", pod.Name)
		}

		// Wait for all pods to terminate
		printStep("Waiting for pods to terminate...")
		if err := vc.waitForPodTermination(ctx, namespace, name); err != nil {
			return fmt.Errorf("failed waiting for pods to terminate: %v", err)
		}
		printSuccess("All pods terminated")
	}

	// Delete the PVC
	printStep("Deleting PVC %s...", name)
	err = vc.clientset.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		// Check if it's already gone
		_, getErr := vc.clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
		if getErr != nil {
			printSuccess("PVC already deleted")
			return nil
		}
		return fmt.Errorf("failed to delete PVC: %v", err)
	}

	// Wait for PVC to be deleted
	if err := vc.waitForPVCDeletion(ctx, namespace, name); err != nil {
		return fmt.Errorf("failed while waiting for PVC deletion: %v", err)
	}

	printSuccess("Volume deleted successfully")
	printFooter()
	return nil
}
