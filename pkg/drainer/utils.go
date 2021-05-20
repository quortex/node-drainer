// Package drainer provides the node drains logic.
package drainer

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ByCreationTimestamp is an implementation of the Sort interface
// to sort Nodes slice by CreationTimestamp.
type ByCreationTimestampDescending []corev1.Node

func (ct ByCreationTimestampDescending) Len() int {
	return len(ct)
}

func (ct ByCreationTimestampDescending) Less(i, j int) bool {
	return ct[i].CreationTimestamp.Before(&ct[j].CreationTimestamp)
}

func (ct ByCreationTimestampDescending) Swap(i, j int) {
	ct[i], ct[j] = ct[j], ct[i]
}

// isNodeReady returns if the node Status is Ready
func isNodeReady(node corev1.Node) bool {
	for _, e := range node.Status.Conditions {
		if e.Type == corev1.NodeReady {
			return e.Status == corev1.ConditionTrue
		}
	}
	return false
}

// isNodeOlderThan returns if the node is older than the provided duration
func isNodeOlderThan(node corev1.Node, d time.Duration) bool {
	return node.CreationTimestamp.Add(d).Before(time.Now())
}

// PodFilter describes functions to filter pods from slice
type PodFilter func([]corev1.Pod) []corev1.Pod

// filterPods filter a pod slice with given filters
func filterPods(pods []corev1.Pod, filters ...PodFilter) []corev1.Pod {
	for _, f := range filters {
		pods = f(pods)
	}
	return pods
}

// daemonSetFilter filters out dameonsets pods
func (d *Drainer) daemonSetFilter(pods []corev1.Pod) []corev1.Pod {
	for i := len(pods) - 1; i >= 0; i-- {
		pod := pods[i]
		controllerRef := metav1.GetControllerOf(&pod)
		if controllerRef == nil || controllerRef.Kind != appsv1.SchemeGroupVersion.WithKind("DaemonSet").Kind {
			continue
		}

		// Any finished pod can be removed.
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}

		if _, err := d.Cli.AppsV1().DaemonSets(pod.Namespace).Get(context.TODO(), controllerRef.Name, metav1.GetOptions{}); err != nil {
			// remove orphaned pods
			if apierrors.IsNotFound(err) {
				pods = append(pods[:i], pods[i+1:]...)
				continue
			}

			continue
		}

		pods = append(pods[:i], pods[i+1:]...)
	}

	return pods
}

// deletedFilter filters out already deleted pods
func deletedFilter(pods []corev1.Pod) []corev1.Pod {
	for i := len(pods) - 1; i >= 0; i-- {
		pod := pods[i]
		if !pod.ObjectMeta.DeletionTimestamp.IsZero() {
			pods = append(pods[:i], pods[i+1:]...)
		}
	}
	return pods
}
