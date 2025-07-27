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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	uxv1alpha1 "github.com/seans3/node-workload-summary/api/v1alpha1"
)

// WorkloadSummarizerReconciler reconciles a WorkloadSummarizer object
type WorkloadSummarizerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=ux.sean.example.com,resources=workloadsummarizers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ux.sean.example.com,resources=workloadsummarizers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ux.sean.example.com,resources=workloadsummarizers/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

func (r *WorkloadSummarizerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var workloadSummarizer uxv1alpha1.WorkloadSummarizer
	if err := r.Get(ctx, req.NamespacedName, &workloadSummarizer); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var pods corev1.PodList
	if err := r.List(ctx, &pods); err != nil {
		return ctrl.Result{}, err
	}

	workloads := make(map[types.NamespacedName]client.Object)
	for _, pod := range pods.Items {
		for _, ownerRef := range pod.OwnerReferences {
			workload := r.getWorkload(ctx, &pod, ownerRef, &workloadSummarizer)
			if workload != nil {
				workloads[client.ObjectKeyFromObject(workload)] = workload
			}
		}
	}

	for _, workload := range workloads {
		workloadSummaryName := fmt.Sprintf("%s-%s", workload.GetNamespace(), workload.GetName())
		workloadSummary := &uxv1alpha1.WorkloadSummary{
			ObjectMeta: metav1.ObjectMeta{
				Name:      workloadSummaryName,
				Namespace: workload.GetNamespace(),
			},
		}

		op, err := ctrl.CreateOrUpdate(ctx, r.Client, workloadSummary, func() error {
			// TODO: Populate status
			return nil
		})
		if err != nil {
			return ctrl.Result{}, err
		}
		log.Info("Reconciled WorkloadSummary", "operation", op, "name", workloadSummary.Name)
	}

	return ctrl.Result{}, nil
}

func (r *WorkloadSummarizerReconciler) getWorkload(ctx context.Context, pod *corev1.Pod, ownerRef metav1.OwnerReference, summarizer *uxv1alpha1.WorkloadSummarizer) client.Object {
	fmt.Println("Checking owner", ownerRef.Name, "kind", ownerRef.Kind)
	log := log.FromContext(ctx)
	log.Info("Checking owner", "owner", ownerRef.Name, "kind", ownerRef.Kind)
	for _, workloadType := range summarizer.Spec.WorkloadTypes {
		if ownerRef.Kind == workloadType.Kind && ownerRef.APIVersion == workloadType.Group {
			obj := &unstructured.Unstructured{}
			obj.SetAPIVersion(ownerRef.APIVersion)
			obj.SetKind(ownerRef.Kind)
			err := r.Get(ctx, types.NamespacedName{Name: ownerRef.Name, Namespace: pod.Namespace}, obj)
			if err != nil {
				log.Error(err, "unable to get workload", "workload", ownerRef.Name)
				return nil
			}
			log.Info("Found workload for pod", "workload", obj.GetName(), "pod", pod.Name)
			return obj
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkloadSummarizerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&uxv1alpha1.WorkloadSummarizer{}).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(r.findWorkloadSummarizersForPod),
		).
		Named("workloadsummarizer").
		Complete(r)
}

func (r *WorkloadSummarizerReconciler) findWorkloadSummarizersForPod(ctx context.Context, pod client.Object) []reconcile.Request {
	var summarizers uxv1alpha1.WorkloadSummarizerList
	if err := r.List(ctx, &summarizers); err != nil {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, len(summarizers.Items))
	for i, item := range summarizers.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name: item.Name,
			},
		}
	}
	return requests
}