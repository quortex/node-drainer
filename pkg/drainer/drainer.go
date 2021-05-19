// Package drainer provides the node drains logic.
package drainer

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	// evictionKind represents the kind of evictions object
	evictionKind = "Eviction"
	// evictionSubresource represents the kind of evictions object as pod's subresource
	evictionSubresource = "pods/eviction"
	// The delete pod polling interval
	pollInterval = time.Second
)

// Configuration wraps Drainer configuration
type Configuration struct {
	EvictionGlobalTimeout int
	Cli                   *kubernetes.Clientset
	Log                   logr.Logger
}

// Drainer handle nodes cordon / drain
type Drainer struct {
	Configuration
}

// New returns a newly instantiated Drainer
func New(config Configuration) *Drainer {
	return &Drainer{Configuration: config}
}

// Drain perform draining operations
// It will drain a given number of nodes matching selector and of an age greater than the one given in parameter.
// Older nodes will be drained first.
func (d *Drainer) Drain(
	ctx context.Context,
	selector map[string]string,
	olderThan time.Duration,
	nodeCount int,
) error {
	// Compute selector
	s := labels.SelectorFromSet(selector).String()
	d.Log.Info("Starting nodes draining process", "selector", s, "olderThan", olderThan, "nodeCount", nodeCount)

	// If node count is invalid we return immediatly
	if nodeCount <= 0 {
		d.Log.Info("Aborting, no node to drain", "nodeCount", nodeCount)
		return nil
	}

	// Get Nodes matching selctor
	d.Log.V(1).Info("Listing nodes", "selector", s, "oderThan", olderThan)
	nodesList, err := d.Cli.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: s})
	if err != nil {
		d.Log.Error(err, "Failed to list nodes", "selector", s)
		return err
	}
	nodes := nodesList.Items
	if len(nodes) == 0 {
		d.Log.Info("No nodes matching selector", "selector", s)
		return nil
	}

	// Sort nodes by descending creation timestamp
	sort.Sort(ByCreationTimestampDescending(nodes))

	// List pods with status not Running and not Succeeded
	// We do not perform drain if some pods don't match thes status
	d.Log.V(1).Info("Listing non running / succeeded pods")
	podsList, err := d.Cli.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: "status.phase!=" + string(corev1.PodRunning) + ",status.phase!=" + string(corev1.PodSucceeded),
	})
	if err != nil {
		d.Log.Error(err, "Failed to list non running / succeeded pods")
		return err
	}
	if len(podsList.Items) > 0 {
		d.Log.Info("Aborting drain process, pods not ready", "count", len(podsList.Items))
		return nil
	}

	count := 0
	notReady := 0
	notOldEnough := 0
	// Iterate on nodes to drain the older ones matching rules
	for i, n := range nodes {
		if count >= nodeCount {
			// All nodes have been drined, return immediatly
			return nil
		}
		if !isNodeOlderThan(n, olderThan) {
			// Remaining nodes are too young to be drained
			notOldEnough = len(nodes) - i
			break
		}

		// Matching node to drain
		// Drain only nodes that are ready and Schedulable
		if isNodeReady(n) && !n.Spec.Unschedulable {
			d.Log.Info("Draining node", "node", n.Name)
			err := d.drainNode(ctx, &n)
			// Failed to drain node, return immediatly
			if err != nil {
				return err
			}

			count++
			continue
		}
		notReady++
	}

	d.Log.Info("No candidate for drain", "selector", s, "nodeCount", len(nodes), "notOldEnough", notOldEnough, "notReady", notReady)

	return nil
}

func (d *Drainer) drainNode(
	ctx context.Context,
	node *corev1.Node,
) error {
	nodeName := node.Name
	// First, we cordon the Node (set it as unschedulable)
	d.Log.Info("Cordon node", "node", nodeName)
	err := d.cordonNode(ctx, node)
	if err != nil {
		d.Log.Error(err, "Unable to cordon Node", "node", nodeName)
		return err
	}

	// Get pods scheduled on that Node
	podsList, err := d.Cli.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		d.Log.Error(err, "Failed to list node's pods", "node", nodeName)
		return err
	}

	// Evict pods
	// We don't care about errors here.
	// Either we can't process them or the eviction has timeout.
	d.Log.Info("Evicting pods", "node", nodeName)
	if err := d.evictPods(ctx, nodeName, filterPods(podsList.Items, deletedFilter, d.daemonSetFilter)); err != nil {
		d.Log.Error(err, "Failed to evict pods", "node", nodeName)
		return err
	}

	return nil
}

// cordonNode cordon the given Node (mark it as unschedulable).
func (d *Drainer) cordonNode(
	ctx context.Context,
	node *corev1.Node,
) error {
	// To cordon a Node, patch it to set it Unschedulable.
	old, err := json.Marshal(node)
	if err != nil {
		return err
	}
	node.Spec.Unschedulable = true
	new, err := json.Marshal(node)
	if err != nil {
		return err
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(old, new, node)
	if err != nil {
		return err
	}
	if _, err := d.Cli.CoreV1().Nodes().Patch(ctx, node.Name, types.StrategicMergePatchType, patch, metav1.PatchOptions{}); err != nil {
		return err
	}

	return nil
}

// evictPods evict given pods and returns when all pods have been
// successfully deleted, error occurred or evictionGlobalTimeout expired.
// This code is largely inspired by kubectl cli source code.
func (d *Drainer) evictPods(
	ctx context.Context,
	nodeName string,
	pods []corev1.Pod,
) error {
	returnCh := make(chan error, 1)
	policyGroupVersion, err := d.checkEvictionSupport()
	if err != nil {
		d.Log.Error(err, "Failed to check eviction support")
		return err
	}

	evictionGlobalTimeout := time.Duration(d.EvictionGlobalTimeout) * time.Second
	ctx, cancel := context.WithTimeout(ctx, evictionGlobalTimeout)
	defer cancel()

	for _, pod := range pods {
		go func(pod corev1.Pod, returnCh chan error) {
			for {
				d.Log.Info("Evicting pod", "name", pod.Name, "namespace", pod.Namespace)
				select {
				case <-ctx.Done():
					// return here or we'll leak a goroutine.
					returnCh <- fmt.Errorf("error when evicting pods/%q -n %q: global timeout reached: %v", pod.Name, pod.Namespace, evictionGlobalTimeout)
					return
				default:
				}

				// Create a temporary pod so we don't mutate the pod in the loop.
				activePod := pod
				err := d.evictPod(ctx, activePod, policyGroupVersion)
				if err == nil {
					break
				} else if apierrors.IsNotFound(err) {
					returnCh <- nil
					return
				} else if apierrors.IsTooManyRequests(err) {
					d.Log.Error(err, "Failed to evict pod (will retry after 5s)", "name", pod.Name, "namespace", pod.Namespace)
					time.Sleep(5 * time.Second)
				} else if !activePod.ObjectMeta.DeletionTimestamp.IsZero() && apierrors.IsForbidden(err) && apierrors.HasStatusCause(err, corev1.NamespaceTerminatingCause) {
					// an eviction request in a deleting namespace will throw a forbidden error,
					// if the pod is already marked deleted, we can ignore this error, an eviction
					// request will never succeed, but we will waitForDelete for this pod.
					break
				} else if apierrors.IsForbidden(err) && apierrors.HasStatusCause(err, corev1.NamespaceTerminatingCause) {
					// an eviction request in a deleting namespace will throw a forbidden error,
					// if the pod is not marked deleted, we retry until it is.
					d.Log.Error(err, "Failed to evict pod (will retry after 5s)", "name", pod.Name, "namespace", pod.Namespace)
					time.Sleep(5 * time.Second)
				} else {
					returnCh <- fmt.Errorf("error when evicting pods/%q -n %q: %v", activePod.Name, activePod.Namespace, err)
					return
				}
			}
			_, err := d.waitForDelete(ctx, []corev1.Pod{pod})
			if err == nil {
				returnCh <- nil
			} else {
				returnCh <- fmt.Errorf("error when waiting for pod %q terminating: %v", pod.Name, err)
			}
		}(pod, returnCh)
	}

	doneCount := 0
	var errors []error

	numPods := len(pods)
	for doneCount < numPods {
		//nolint:gosimple
		select {
		case err := <-returnCh:
			doneCount++
			if err != nil {
				errors = append(errors, err)
			}
		}
	}

	return utilerrors.NewAggregate(errors)
}

// checkEvictionSupport uses Discovery API to find out if the server support
// eviction subresource If support, it will return its groupVersion; Otherwise,
// it will return an empty string.
// This code is largely inspired by kubectl cli source code.
func (d *Drainer) checkEvictionSupport() (string, error) {
	discoveryClient := d.Cli.Discovery()
	groupList, err := discoveryClient.ServerGroups()
	if err != nil {
		return "", err
	}
	foundPolicyGroup := false
	var policyGroupVersion string
	for _, group := range groupList.Groups {
		if group.Name == "policy" {
			foundPolicyGroup = true
			policyGroupVersion = group.PreferredVersion.GroupVersion
			break
		}
	}
	if !foundPolicyGroup {
		return "", nil
	}
	resourceList, err := discoveryClient.ServerResourcesForGroupVersion("v1")
	if err != nil {
		return "", err
	}
	for _, resource := range resourceList.APIResources {
		if resource.Name == evictionSubresource && resource.Kind == evictionKind {
			return policyGroupVersion, nil
		}
	}
	return "", nil
}

// evictPod will evict the given pod, or return an error if it couldn't
// This code is largely inspired by kubectl cli source code.
func (d *Drainer) evictPod(ctx context.Context, pod corev1.Pod, policyGroupVersion string) error {

	gracePeriod := int64(time.Second * 30)
	if pod.Spec.TerminationGracePeriodSeconds != nil && *pod.Spec.TerminationGracePeriodSeconds < gracePeriod {
		gracePeriod = *pod.Spec.TerminationGracePeriodSeconds
	}

	eviction := &policyv1beta1.Eviction{
		TypeMeta: metav1.TypeMeta{
			APIVersion: policyGroupVersion,
			Kind:       evictionKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
		DeleteOptions: &metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod},
	}

	// Remember to change change the URL manipulation func when Eviction's version change
	return d.Cli.PolicyV1beta1().Evictions(eviction.Namespace).Evict(ctx, eviction)
}

// deleteTimeout compute the delete timeout from given pods.
func deleteTimeout(pods []corev1.Pod) time.Duration {
	// We return the max DeletionGracePeriodSeconds from pods with
	// a 30sec overhead.
	maxGrace := int64(30)
	for _, e := range pods {
		if grace := e.DeletionGracePeriodSeconds; grace != nil {
			if *grace > maxGrace {
				maxGrace = *grace
			}
		}
	}

	return time.Duration(maxGrace+30) * time.Second
}

// waitForDelete poll pods to check their deletion.
// This code is largely inspired by kubectl cli source code.
func (d *Drainer) waitForDelete(ctx context.Context, pods []corev1.Pod) ([]corev1.Pod, error) {
	err := wait.PollImmediate(pollInterval, deleteTimeout(pods), func() (bool, error) {
		pendingPods := []corev1.Pod{}
		for i, pod := range pods {
			p, err := d.Cli.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) || (p != nil && p.ObjectMeta.UID != pod.ObjectMeta.UID) {
				continue
			} else if err != nil {
				return false, err
			} else {
				pendingPods = append(pendingPods, pods[i])
			}
		}
		pods = pendingPods
		if len(pendingPods) > 0 {
			select {
			case <-ctx.Done():
				return false, fmt.Errorf("Eviction global timeout reached")
			default:
				return false, nil
			}
		}
		return true, nil
	})
	return pods, err
}
