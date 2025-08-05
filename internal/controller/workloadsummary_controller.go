/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	uxv1alpha1 "github.com/seans3/node-workload-summary/api/v1alpha1"
)

// MaxOwnerTraversalDepth is a safeguard against infinite loops from misconfigured owner references.
const MaxOwnerTraversalDepth = 10

// WorkloadSummaryReconciler reconciles a WorkloadSummary object
type WorkloadSummaryReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=ux.sean.example.com,resources=workloadsummaries,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ux.sean.example.com,resources=workloadsummaries/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ux.sean.example.com,resources=workloadsummaries/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
//+kubebuilder:rbac:groups=ux.sean.example.com,resources=workloadsummarizers,verbs=get;list;watch

func (r *WorkloadSummaryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling WorkloadSummary", "request", req)

	var workloadSummary uxv1alpha1.WorkloadSummary
	if err := r.Get(ctx, req.NamespacedName, &workloadSummary); err != nil {
		log.Error(err, "unable to fetch WorkloadSummary")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	gvkString, ok := workloadSummary.Annotations["ux.sean.example.com/workload-gvk"]
	if !ok {
		log.Info("WorkloadSummary is missing GVK annotation, requeueing", "name", workloadSummary.Name)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	gvk, err := parseGVK(gvkString)
	if err != nil {
		log.Error(err, "unable to parse GVK from annotation", "gvk", gvkString)
		return ctrl.Result{}, nil
	}

	workload := &unstructured.Unstructured{}
	workload.SetGroupVersionKind(gvk)
	if err := r.Get(ctx, req.NamespacedName, workload); err != nil {
		log.Error(err, "unable to fetch workload object", "gvk", gvk, "name", req.NamespacedName)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	selectorMap, found, err := unstructured.NestedMap(workload.Object, "spec", "selector")
	if err != nil || !found {
		log.Error(err, "unable to get pod selector from workload", "workload", workload.GetName())
		return ctrl.Result{}, nil
	}

	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels:      toStringMap(selectorMap["matchLabels"]),
		MatchExpressions: toLabelSelectorRequirementSlice(selectorMap["matchExpressions"]),
	})
	if err != nil {
		log.Error(err, "unable to create selector from map", "selectorMap", selectorMap)
		return ctrl.Result{}, nil
	}

	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(req.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		log.Error(err, "unable to list pods")
		return ctrl.Result{}, err
	}
	ownedPods := pods.Items
	log.Info("Found owned pods", "count", len(ownedPods))

	workloadSummary.Status.PodCount = len(ownedPods)

	nodeSummaryRefs := make(map[string]int)
	for _, pod := range ownedPods {
		if pod.Spec.NodeName != "" {
			var node corev1.Node
			if err := r.Get(ctx, types.NamespacedName{Name: pod.Spec.NodeName}, &node); err == nil {
				var nodeSummaries uxv1alpha1.NodeSummaryList
				if err := r.List(ctx, &nodeSummaries); err == nil {
					for _, ns := range nodeSummaries.Items {
						selector, _ := metav1.LabelSelectorAsSelector(ns.Spec.Selector)
						if selector.Matches(labels.Set(node.GetLabels())) {
							nodeSummaryRefs[ns.Name]++
						}
					}
				}
			}
		}
	}
	log.Info("Found node summaries", "count", len(nodeSummaryRefs))

	workloadSummary.Status.NodeSummaryRefs = make([]uxv1alpha1.NodeSummaryRef, 0, len(nodeSummaryRefs))
	for name, count := range nodeSummaryRefs {
		workloadSummary.Status.NodeSummaryRefs = append(workloadSummary.Status.NodeSummaryRefs, uxv1alpha1.NodeSummaryRef{
			Name:      name,
			NodeCount: count,
		})
	}

	workloadSummary.Status.LongType = gvk.Group + "." + gvk.Version + "." + gvk.Kind
	switch gvk.Kind {
	case "Deployment":
		workloadSummary.Status.ShortType = "dep"
	case "StatefulSet":
		workloadSummary.Status.ShortType = "sts"
	case "DaemonSet":
		workloadSummary.Status.ShortType = "ds"
	default:
		workloadSummary.Status.ShortType = strings.ToLower(gvk.Kind)
	}

	if err := r.Status().Update(ctx, &workloadSummary); err != nil {
		log.Error(err, "unable to update WorkloadSummary status")
		return ctrl.Result{}, err
	}

	log.Info("Reconciled WorkloadSummary", "name", workloadSummary.Name)
	return ctrl.Result{}, nil
}

func parseGVK(gvkString string) (schema.GroupVersionKind, error) {
	parts := strings.Split(gvkString, ", Kind=")
	if len(parts) != 2 {
		return schema.GroupVersionKind{}, fmt.Errorf("invalid GVK string format: %s", gvkString)
	}
	// now we have to check for extra parts
	if strings.Contains(parts[1], ",") {
		return schema.GroupVersionKind{}, fmt.Errorf("invalid GVK string format: %s", gvkString)
	}
	gv, err := schema.ParseGroupVersion(parts[0])
	if err != nil {
		return schema.GroupVersionKind{}, fmt.Errorf("invalid GroupVersion string: %s", parts[0])
	}
	return gv.WithKind(parts[1]), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkloadSummaryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&uxv1alpha1.WorkloadSummary{}).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(r.findWorkloadSummariesForPod),
		).
		Named("workloadsummary").
		Complete(r)
}

func (r *WorkloadSummaryReconciler) findWorkloadSummariesForPod(ctx context.Context, pod client.Object) []reconcile.Request {
	log := log.FromContext(ctx)
	var summarizerList uxv1alpha1.WorkloadSummarizerList
	if err := r.List(ctx, &summarizerList); err != nil {
		log.Error(err, "unable to list WorkloadSummarizers")
		return []reconcile.Request{}
	}

	if len(summarizerList.Items) == 0 {
		return []reconcile.Request{}
	}

	workloadTypes := make(map[schema.GroupKind]bool)
	for _, summarizer := range summarizerList.Items {
		for _, wt := range summarizer.Spec.WorkloadTypes {
			workloadTypes[schema.GroupKind{Group: wt.Group, Kind: wt.Kind}] = true
		}
	}

	rootWorkload, err := FindRootWorkload(ctx, r.Client, pod, workloadTypes)
	if err != nil {
		log.Error(err, "unable to find root workload for pod", "pod", pod.GetName())
		return []reconcile.Request{}
	}

	if rootWorkload == nil || rootWorkload.GetUID() == pod.GetUID() {
		return []reconcile.Request{}
	}

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      rootWorkload.GetName(),
				Namespace: rootWorkload.GetNamespace(),
			},
		},
	}
}

// FindRootWorkload traverses the ownerReferences of an object to find the top-level
// workload that the controller is configured to summarize.
func FindRootWorkload(
	ctx context.Context,
	k8sClient client.Client,
	obj client.Object,
	workloadTypes map[schema.GroupKind]bool,
) (client.Object, error) {
	currentObj := obj

	for i := 0; i < MaxOwnerTraversalDepth; i++ {
		currentGVK := currentObj.GetObjectKind().GroupVersionKind()
		currentGK := currentGVK.GroupKind()
		if workloadTypes[currentGK] {
			return currentObj, nil
		}

		ownerRef := metav1.GetControllerOf(currentObj)
		if ownerRef == nil {
			return currentObj, nil
		}

		ownerGK := schema.FromAPIVersionAndKind(ownerRef.APIVersion, ownerRef.Kind).GroupKind()
		if workloadTypes[ownerGK] {
			ownerObj, err := getObjectFromRef(ctx, k8sClient, ownerRef, currentObj.GetNamespace())
			if err != nil {
				return nil, fmt.Errorf("failed to get final workload object %s: %w", ownerRef.Name, err)
			}
			return ownerObj, nil
		}

		nextObj, err := getObjectFromRef(ctx, k8sClient, ownerRef, currentObj.GetNamespace())
		if err != nil {
			if client.IgnoreNotFound(err) == nil {
				return currentObj, nil
			}
			return nil, fmt.Errorf("failed to get owner object %s: %w", ownerRef.Name, err)
		}
		currentObj = nextObj
	}

	return nil, fmt.Errorf("ownership chain is too deep for object %s/%s", obj.GetNamespace(), obj.GetName())
}

// getObjectFromRef fetches a full object from the API server based on an OwnerReference.
func getObjectFromRef(ctx context.Context, k8sClient client.Client, ref *metav1.OwnerReference, namespace string) (client.Object, error) {
	owner := &metav1.PartialObjectMetadata{}
	owner.SetGroupVersionKind(schema.FromAPIVersionAndKind(ref.APIVersion, ref.Kind))

	err := k8sClient.Get(ctx, client.ObjectKey{
		Name:      ref.Name,
		Namespace: namespace,
	}, owner)

	return owner, err
}

func toStringMap(in interface{}) map[string]string {
	if in == nil {
		return nil
	}
	m, ok := in.(map[string]interface{})
	if !ok {
		return nil
	}
	res := make(map[string]string)
	for k, v := range m {
		s, ok := v.(string)
		if ok {
			res[k] = s
		}
	}
	return res
}

func toLabelSelectorRequirementSlice(in interface{}) []metav1.LabelSelectorRequirement {
	if in == nil {
		return nil
	}
	s, ok := in.([]interface{})
	if !ok {
		return nil
	}
	res := make([]metav1.LabelSelectorRequirement, len(s))
	for i, v := range s {
		m, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		req := metav1.LabelSelectorRequirement{}
		if key, ok := m["key"].(string); ok {
			req.Key = key
		}
		if op, ok := m["operator"].(string); ok {
			req.Operator = metav1.LabelSelectorOperator(op)
		}
		if values, ok := m["values"].([]interface{}); ok {
			req.Values = make([]string, len(values))
			for j, val := range values {
				if sval, ok := val.(string); ok {
					req.Values[j] = sval
				}
			}
		}
		res[i] = req
	}
	return res
}
