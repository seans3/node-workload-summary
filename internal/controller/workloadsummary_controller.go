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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	uxv1alpha1 "github.com/seans3/node-workload-summary/api/v1alpha1"
)

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

func (r *WorkloadSummaryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var workloadSummary uxv1alpha1.WorkloadSummary
	if err := r.Get(ctx, req.NamespacedName, &workloadSummary); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(req.Namespace)); err != nil {
		return ctrl.Result{}, err
	}

	var ownedPods []corev1.Pod
	for _, pod := range pods.Items {
		for _, ownerRef := range pod.OwnerReferences {
			if ownerRef.Name == workloadSummary.Name {
				ownedPods = append(ownedPods, pod)
			}
		}
	}

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

	workloadSummary.Status.NodeSummaryRefs = make([]uxv1alpha1.NodeSummaryRef, 0, len(nodeSummaryRefs))
	for name, count := range nodeSummaryRefs {
		workloadSummary.Status.NodeSummaryRefs = append(workloadSummary.Status.NodeSummaryRefs, uxv1alpha1.NodeSummaryRef{
			Name:      name,
			NodeCount: count,
		})
	}

	if err := r.Status().Update(ctx, &workloadSummary); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Reconciled WorkloadSummary", "name", workloadSummary.Name)
	return ctrl.Result{}, nil
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
	var summaries uxv1alpha1.WorkloadSummaryList
	if err := r.List(ctx, &summaries, client.InNamespace(pod.GetNamespace())); err != nil {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, 0)
	for _, summary := range summaries.Items {
		for _, ownerRef := range pod.GetOwnerReferences() {
			if ownerRef.Name == summary.Name {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      summary.Name,
						Namespace: summary.Namespace,
					},
				})
			}
		}
	}
	return requests
}