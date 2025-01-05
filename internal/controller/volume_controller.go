package controller

import (
	"bytes"
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
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

func printWarn(format string, args ...interface{}) {
	fmt.Printf("\r⚠️  %s\n", fmt.Sprintf(format, args...))
}

func printError(format string, args ...interface{}) {
	fmt.Printf("\r❌ %s\n", fmt.Sprintf(format, args...))
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

// Add this function to find pods using a PVC
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

// Add this cleanup helper function
func (vc *VolumeController) cleanupResources(namespace string, resources ...string) {
	for _, name := range resources {
		// Try to delete PVC
		if err := vc.clientset.CoreV1().PersistentVolumeClaims(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil {
			fmt.Printf("Warning: failed to cleanup PVC %s: %v\n", name, err)
		}
		// Try to delete Pod
		if err := vc.clientset.CoreV1().Pods(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil {
			fmt.Printf("Warning: failed to cleanup Pod %s: %v\n", name, err)
		}
	}
}

// Add this before ResizeVolume
func (vc *VolumeController) checkDataSize(namespace, name string) (int64, error) {
	// Create a pod to check the data size
	checkPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("size-check-%s", name),
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "size-check",
					Image:   "busybox",
					Command: []string{"sh", "-c", "du -sb /data | cut -f1"},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "data-vol",
							MountPath: "/data",
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
			Volumes: []corev1.Volume{
				{
					Name: "data-vol",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: name,
							ReadOnly:  true,
						},
					},
				},
			},
		},
	}

	// Create and wait for the pod
	pod, err := vc.clientset.CoreV1().Pods(namespace).Create(context.TODO(), checkPod, metav1.CreateOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to create size check pod: %v", err)
	}
	defer vc.clientset.CoreV1().Pods(namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})

	// Wait for pod completion
	for i := 0; i < 30; i++ {
		pod, err = vc.clientset.CoreV1().Pods(namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
		if err != nil {
			return 0, fmt.Errorf("failed to get size check pod: %v", err)
		}
		if pod.Status.Phase == corev1.PodSucceeded {
			break
		} else if pod.Status.Phase == corev1.PodFailed {
			return 0, fmt.Errorf("size check pod failed")
		}
		time.Sleep(time.Second)
	}

	// Get the size from pod logs
	logs, err := vc.clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{}).Do(context.TODO()).Raw()
	if err != nil {
		return 0, fmt.Errorf("failed to get size check logs: %v", err)
	}

	size := int64(0)
	_, err = fmt.Sscanf(string(logs), "%d", &size)
	if err != nil {
		return 0, fmt.Errorf("failed to parse size: %v", err)
	}

	return size, nil
}

// ResizeVolume handles the PVC resize operation
func (vc *VolumeController) ResizeVolume(namespace, name string, newSize string) error {
	ctx := context.TODO()
	newPVCName := fmt.Sprintf("%s-new", name)
	transferPodName := fmt.Sprintf("transfer-pod-%s", name)

	cleanup := func() {
		printStep("Cleaning up resources...")
		vc.cleanupResources(namespace, newPVCName, transferPodName)
	}

	oldPVC, err := vc.clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get PVC: %v", err)
	}

	quantity, err := resource.ParseQuantity(newSize)
	if err != nil {
		return fmt.Errorf("invalid size format: %v", err)
	}

	currentSize := oldPVC.Spec.Resources.Requests.Storage()

	printHeader("Volume Resize Operation")
	printMsg("PVC: %s/%s", namespace, name)
	printMsg("Current size: %s", currentSize.String())
	printMsg("Target size: %s", newSize)
	printFooter()

	if quantity.Cmp(*currentSize) < 0 {
		printStep("Checking current data size")
		dataSize, err := vc.checkDataSize(namespace, name)
		if err != nil {
			return fmt.Errorf("failed to check data size: %v", err)
		}

		newSizeBytes := quantity.Value()
		if dataSize > newSizeBytes {
			return fmt.Errorf("cannot shrink volume: current data size (%d bytes) is larger than requested size (%d bytes)",
				dataSize, newSizeBytes)
		}

		printSuccess("Safe to shrink - Current data size: %d bytes", dataSize)
	}

	// Find and scale down pods using the PVC
	podsUsingPVC, err := vc.findPodsUsingPVC(namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find pods using PVC: %v", err)
	}

	if len(podsUsingPVC) > 0 {
		printStep("Found %d pods using the PVC", len(podsUsingPVC))
		for _, pod := range podsUsingPVC {
			printMsg("    └─ Stopping pod: %s", pod.Name)
			if err := vc.deletePod(ctx, namespace, pod.Name); err != nil {
				return fmt.Errorf("failed to stop pod %s: %v", pod.Name, err)
			}
		}

		printStep("Waiting for pods to terminate...")
		if err := vc.waitForPodTermination(ctx, namespace, name); err != nil {
			return fmt.Errorf("failed waiting for pods to terminate: %v", err)
		}
		printSuccess("All pods terminated")
	}

	// Create new PVC with larger size
	newPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      newPVCName,
			Namespace: namespace,
			Labels:    oldPVC.Labels,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      oldPVC.Spec.AccessModes,
			StorageClassName: oldPVC.Spec.StorageClassName,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: quantity,
				},
			},
		},
	}

	printStep("Creating new PVC: %s", newPVCName)
	newPVC, err = vc.clientset.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, newPVC, metav1.CreateOptions{})
	if err != nil {
		cleanup()
		return fmt.Errorf("failed to create new PVC: %v", err)
	}

	// Wait for the new PVC to be bound
	printStep("Waiting for new PVC to be bound...")
	bound := false
	for i := 0; i < 30; i++ {
		pvc, err := vc.clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, newPVC.Name, metav1.GetOptions{})
		if err != nil {
			cleanup()
			return fmt.Errorf("failed to get PVC status: %v", err)
		}

		if pvc.Status.Phase == corev1.ClaimBound {
			bound = true
			break
		}
		if pvc.Status.Phase != corev1.ClaimBound {
			printStep("    └─ Status: %s", pvc.Status.Phase)
		}
		time.Sleep(time.Second)
	}

	if !bound {
		cleanup()
		return fmt.Errorf("timeout waiting for PVC to be bound")
	}
	printSuccess("New PVC bound successfully")

	// Create data transfer pod
	printStep("Starting data transfer")
	transferPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      transferPodName,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "transfer",
					Image: "busybox",
					Command: []string{"sh", "-c", `
echo "Source contents:" && ls -la /source && \
echo "Starting copy..." && \
cp -av /source/. /target/ && \
echo "Target contents:" && ls -la /target && \
echo "Data transfer complete"
`},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "source-vol",
							MountPath: "/source",
						},
						{
							Name:      "target-vol",
							MountPath: "/target",
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
			Volumes: []corev1.Volume{
				{
					Name: "source-vol",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: name,
							ReadOnly:  true,
						},
					},
				},
				{
					Name: "target-vol",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: newPVC.Name,
						},
					},
				},
			},
		},
	}

	fmt.Printf("\rCreating data transfer pod...\n")
	transferPod, err = vc.clientset.CoreV1().Pods(namespace).Create(ctx, transferPod, metav1.CreateOptions{})
	if err != nil {
		cleanup()
		return fmt.Errorf("failed to create transfer pod: %v", err)
	}

	// Wait for transfer pod to complete
	printStep("Copying data...")
	for {
		pod, err := vc.clientset.CoreV1().Pods(namespace).Get(ctx, transferPod.Name, metav1.GetOptions{})
		if err != nil {
			cleanup()
			return fmt.Errorf("failed to get transfer pod status: %v", err)
		}

		if pod.Status.Phase == corev1.PodSucceeded {
			break
		} else if pod.Status.Phase == corev1.PodFailed {
			logs, _ := vc.clientset.CoreV1().Pods(namespace).GetLogs(transferPod.Name, &corev1.PodLogOptions{}).Do(ctx).Raw()
			cleanup()
			return fmt.Errorf("transfer pod failed: %s", string(logs))
		}
		time.Sleep(time.Second * 2)
	}

	printStep("Data transfer complete")

	// Clean up transfer pod
	printStep("Cleaning up transfer pod...")
	if err := vc.clientset.CoreV1().Pods(namespace).Delete(ctx, transferPod.Name, metav1.DeleteOptions{}); err != nil {
		printStep("Warning: failed to delete transfer pod: %v", err)
	}

	// Delete old PVC
	printStep("Cleaning up old PVC: %s", name)
	if err := vc.clientset.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("failed to delete old PVC: %v", err)
	}

	// Wait for old PVC to be fully deleted
	if err := vc.waitForPVCDeletion(ctx, namespace, name); err != nil {
		return fmt.Errorf("failed while waiting for old PVC deletion: %v", err)
	}

	// Rename new PVC to old name
	printStep("Finalizing volume resize %s/%s to %s", namespace, name, newSize)
	renamePVC := newPVC.DeepCopy()
	renamePVC.ObjectMeta.Name = name
	renamePVC.ObjectMeta.ResourceVersion = ""
	renamePVC.ObjectMeta.UID = ""
	renamePVC.ObjectMeta.CreationTimestamp = metav1.Time{}
	renamePVC.Status = corev1.PersistentVolumeClaimStatus{}

	_, err = vc.clientset.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, renamePVC, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to rename new PVC: %v", err)
	}

	// Clean up the temporary PVC
	printStep("Cleaning up temporary PVC %s...", newPVCName)
	if err := vc.clientset.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, newPVCName, metav1.DeleteOptions{}); err != nil {
		printStep("Warning: failed to cleanup temporary PVC %s: %v", newPVCName, err)
	}

	// Wait for temporary PVC to be deleted
	if err := vc.waitForPVCDeletion(ctx, namespace, newPVCName); err != nil {
		printStep("Warning: timeout waiting for temporary PVC deletion: %v", err)
	}

	printFooter()
	return nil
}

func (vc *VolumeController) execInPod(namespace, podName, containerName string, command []string) (string, error) {
	req := vc.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   command,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(vc.config, "POST", req.URL())
	if err != nil {
		return "", err
	}

	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return "", fmt.Errorf("exec failed: %v, stderr: %v", err, stderr.String())
	}

	return stdout.String(), nil
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

func (vc *VolumeController) waitForPodRunning(ctx context.Context, namespace, podName string) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, podTimeout)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout waiting for pod %s to be ready", podName)
		default:
			pod, err := vc.clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get pod: %v", err)
			}
			if pod.Status.Phase == corev1.PodRunning {
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

// Add this helper function
func (vc *VolumeController) validateStorageClass(pvc *corev1.PersistentVolumeClaim) error {
	if pvc.Spec.StorageClassName == nil {
		return fmt.Errorf("PVC has no storage class specified")
	}

	sc, err := vc.clientset.StorageV1().StorageClasses().Get(context.TODO(), *pvc.Spec.StorageClassName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get storage class: %v", err)
	}

	if sc.Provisioner == "" {
		return fmt.Errorf("storage class %s has no provisioner", *pvc.Spec.StorageClassName)
	}

	return nil
}

// Add this helper function
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
