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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	uxv1alpha1 "github.com/seans3/node-workload-summary/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
)

// WorkloadSummarizerReconciler reconciles a WorkloadSummarizer object
type WorkloadSummarizerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=*,resources=*,verbs=get;list;watch
//+kubebuilder:rbac:groups=ux.sean.example.com,resources=workloadsummarizers,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=ux.sean.example.com,resources=workloadsummaries,verbs=create
//+kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch

func (r *WorkloadSummarizerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling WorkloadSummarizer", "request", req)

	var summarizer uxv1alpha1.WorkloadSummarizer
	if err := r.Get(ctx, req.NamespacedName, &summarizer); err != nil {
		logger.Error(err, "unable to fetch WorkloadSummarizer")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	for _, workloadType := range summarizer.Spec.WorkloadTypes {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   workloadType.Group,
			Version: workloadType.Version,
			Kind:    workloadType.Kind,
		})
		if err := r.List(ctx, list); err != nil {
			logger.Error(err, "unable to list objects for workload type", "workloadType", workloadType)
			return ctrl.Result{}, err
		}

		for _, item := range list.Items {
			workloadSummary := &uxv1alpha1.WorkloadSummary{
				ObjectMeta: metav1.ObjectMeta{
					Name:      item.GetName(),
					Namespace: item.GetNamespace(),
				},
			}

			logger.Info("Creating WorkloadSummary if it does not exist", "name", workloadSummary.Name)
			if err := r.Create(ctx, workloadSummary); err != nil && !apierrors.IsAlreadyExists(err) {
				logger.Error(err, "unable to create WorkloadSummary")
				return ctrl.Result{}, err
			} else if err == nil {
				logger.Info("Created WorkloadSummary", "name", workloadSummary.Name)
			}
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkloadSummarizerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&uxv1alpha1.WorkloadSummarizer{}).
		Watches(
			&appsv1.Deployment{},
			handler.EnqueueRequestsFromMapFunc(r.findAllWorkloadSummarizers),
		).
		Named("workloadsummarizer").
		Complete(r)
}

func (r *WorkloadSummarizerReconciler) findAllWorkloadSummarizers(ctx context.Context, obj client.Object) []reconcile.Request {
	log := log.FromContext(ctx)
	var summarizerList uxv1alpha1.WorkloadSummarizerList
	if err := r.List(ctx, &summarizerList); err != nil {
		log.Error(err, "unable to list WorkloadSummarizers")
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, len(summarizerList.Items))
	for i, summarizer := range summarizerList.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      summarizer.Name,
				Namespace: summarizer.Namespace,
			},
		}
	}
	return requests
}
