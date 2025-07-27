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

// NodeSummaryReconciler reconciles a NodeSummary object
type NodeSummaryReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=ux.sean.example.com,resources=nodesummaries,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ux.sean.example.com,resources=nodesummaries/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ux.sean.example.com,resources=nodesummaries/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

func (r *NodeSummaryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var nodeSummary uxv1alpha1.NodeSummary
	if err := r.Get(ctx, req.NamespacedName, &nodeSummary); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var nodes corev1.NodeList
	selector, err := metav1.LabelSelectorAsSelector(nodeSummary.Spec.Selector)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.List(ctx, &nodes, client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return ctrl.Result{}, err
	}

	nodeSummary.Status.NodeCount = len(nodes.Items)
	// TODO: aggregate other status fields

	if err := r.Status().Update(ctx, &nodeSummary); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Reconciled NodeSummary", "name", nodeSummary.Name)
	return ctrl.Result{}, nil
}

func (r *NodeSummaryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&uxv1alpha1.NodeSummary{}).
		Watches(
			&corev1.Node{},
			handler.EnqueueRequestsFromMapFunc(r.findNodeSummariesForNode),
		).
		Named("nodesummary").
		Complete(r)
}

func (r *NodeSummaryReconciler) findNodeSummariesForNode(ctx context.Context, node client.Object) []reconcile.Request {
	var nodeSummaries uxv1alpha1.NodeSummaryList
	if err := r.List(ctx, &nodeSummaries); err != nil {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, 0)
	for _, item := range nodeSummaries.Items {
		selector, _ := metav1.LabelSelectorAsSelector(item.Spec.Selector)
		if selector.Matches(labels.Set(node.GetLabels())) {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: item.Name,
				},
			})
		}
	}
	return requests
}