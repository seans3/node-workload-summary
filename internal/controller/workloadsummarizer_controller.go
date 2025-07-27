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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	uxv1alpha1 "github.com/seans3/node-workload-summary/api/v1alpha1"
)

// MaxOwnerTraversalDepth is a safeguard against infinite loops from misconfigured owner references.
const MaxOwnerTraversalDepth = 10

// WorkloadSummarizerReconciler reconciles a WorkloadSummarizer object
type WorkloadSummarizerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=ux.sean.example.com,resources=workloadsummarizers,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=ux.sean.example.com,resources=workloadsummaries,verbs=create
//+kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch

func (r *WorkloadSummarizerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling WorkloadSummarizer", "request", req)

	var pod corev1.Pod
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		logger.Error(err, "unable to fetch Pod")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var summarizerList uxv1alpha1.WorkloadSummarizerList
	if err := r.List(ctx, &summarizerList); err != nil {
		logger.Error(err, "unable to list WorkloadSummarizers")
		return ctrl.Result{}, err
	}

	if len(summarizerList.Items) == 0 {
		logger.Info("No WorkloadSummarizer resources found, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	workloadTypes := make(map[schema.GroupKind]bool)
	for _, summarizer := range summarizerList.Items {
		for _, wt := range summarizer.Spec.WorkloadTypes {
			workloadTypes[schema.GroupKind{Group: wt.Group, Kind: wt.Kind}] = true
		}
	}

	rootWorkload, err := FindRootWorkload(ctx, r.Client, &pod, workloadTypes)
	if err != nil {
		logger.Error(err, "unable to find root workload")
		return ctrl.Result{}, err
	}

	if rootWorkload == nil || rootWorkload.GetUID() == pod.GetUID() {
		logger.Info("No root workload found for pod", "pod", pod.Name)
		return ctrl.Result{}, nil
	}

	workloadSummary := &uxv1alpha1.WorkloadSummary{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rootWorkload.GetName(),
			Namespace: rootWorkload.GetNamespace(),
		},
	}

	logger.Info("Creating WorkloadSummary", "name", workloadSummary.Name)
	if err := r.Create(ctx, workloadSummary); err != nil && !apierrors.IsAlreadyExists(err) {
		logger.Error(err, "unable to create WorkloadSummary")
		return ctrl.Result{}, err
	} else if err == nil {
		logger.Info("Created WorkloadSummary", "name", workloadSummary.Name)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkloadSummarizerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Named("workloadsummarizer").
		Complete(r)
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
