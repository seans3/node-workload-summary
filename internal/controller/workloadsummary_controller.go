/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUTHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
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
//+kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch
//+kubebuilder:rbac:groups=ux.sean.example.com,resources=workloadsummarizers,verbs=get;list;watch

func (r *WorkloadSummaryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling WorkloadSummary", "request", req)

	var workloadSummary uxv1alpha1.WorkloadSummary
	if err := r.Get(ctx, req.NamespacedName, &workloadSummary); err != nil {
		log.Error(err, "unable to fetch WorkloadSummary")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var replicaSets appsv1.ReplicaSetList
	if err := r.List(ctx, &replicaSets, client.InNamespace(req.Namespace)); err != nil {
		log.Error(err, "unable to list replica sets")
		return ctrl.Result{}, err
	}

	var ownedPods []corev1.Pod
	for _, rs := range replicaSets.Items {
		for _, ownerRef := range rs.OwnerReferences {
			if ownerRef.Name == workloadSummary.Name && ownerRef.Kind == "Deployment" {
				var pods corev1.PodList
				podSelector := labels.SelectorFromSet(rs.Spec.Selector.MatchLabels)
				if err := r.List(ctx, &pods, client.InNamespace(req.Namespace), client.MatchingLabelsSelector{Selector: podSelector}); err != nil {
					log.Error(err, "unable to list pods")
					return ctrl.Result{}, err
				}
				ownedPods = append(ownedPods, pods.Items...)
			}
		}
	}
	log.Info("Found owned pods", "count", len(ownedPods))

	workloadSummary.Status.PodCount = len(ownedPods)

	var summarizerList uxv1alpha1.WorkloadSummarizerList
	if err := r.List(ctx, &summarizerList); err != nil {
		log.Error(err, "unable to list WorkloadSummarizers")
		return ctrl.Result{}, err
	}

	if len(summarizerList.Items) > 0 {
		summarizer := summarizerList.Items[0]
		for _, wt := range summarizer.Spec.WorkloadTypes {
			if wt.Kind == "Deployment" {
				workloadSummary.Status.ShortType = "dep"
				workloadSummary.Status.LongType = "apps/v1.Deployment"
			}
		}
	}

	if err := r.Status().Update(ctx, &workloadSummary); err != nil {
		log.Error(err, "unable to update WorkloadSummary status")
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
