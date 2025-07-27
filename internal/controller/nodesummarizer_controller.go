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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	uxv1alpha1 "github.com/seans3/node-workload-summary/api/v1alpha1"
)

// NodeSummarizerReconciler reconciles a NodeSummarizer object
type NodeSummarizerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=ux.sean.example.com,resources=nodesummarizers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ux.sean.example.com,resources=nodesummarizers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ux.sean.example.com,resources=nodesummarizers/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

func (r *NodeSummarizerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling NodeSummarizer", "request", req)

	var nodeSummarizer uxv1alpha1.NodeSummarizer
	if err := r.Get(ctx, req.NamespacedName, &nodeSummarizer); err != nil {
		log.Error(err, "unable to fetch NodeSummarizer")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var nodes corev1.NodeList
	if err := r.List(ctx, &nodes); err != nil {
		log.Error(err, "unable to list nodes")
		return ctrl.Result{}, err
	}

	nodesByLabelValue := make(map[string][]corev1.Node)
	for _, node := range nodes.Items {
		if labelValue, ok := node.Labels[nodeSummarizer.Spec.LabelKey]; ok {
			nodesByLabelValue[labelValue] = append(nodesByLabelValue[labelValue], node)
		}
	}
	log.Info("Grouped nodes by label", "label", nodeSummarizer.Spec.LabelKey, "groups", len(nodesByLabelValue))

	for labelValue, nodesInGroup := range nodesByLabelValue {
		nodeSummaryName := fmt.Sprintf("%s-%s", nodeSummarizer.Name, labelValue)
		nodeSummary := &uxv1alpha1.NodeSummary{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeSummaryName,
			},
		}

		if err := ctrl.SetControllerReference(&nodeSummarizer, nodeSummary, r.Scheme); err != nil {
			log.Error(err, "unable to set controller reference")
			return ctrl.Result{}, err
		}

		op, err := ctrl.CreateOrUpdate(ctx, r.Client, nodeSummary, func() error {
			log.Info("Setting NodeSummary spec and status", "name", nodeSummary.Name)
			nodeSummary.Spec.Selector = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					nodeSummarizer.Spec.LabelKey: labelValue,
				},
			}
			nodeSummary.Status.NodeCount = len(nodesInGroup)
			// TODO: Populate other status fields
			return nil
		})
		if err != nil {
			log.Error(err, "unable to create or update NodeSummary")
			return ctrl.Result{}, err
		}
		log.Info("Reconciled NodeSummary", "operation", op, "name", nodeSummary.Name)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeSummarizerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&uxv1alpha1.NodeSummarizer{}).
		Owns(&uxv1alpha1.NodeSummary{}).
		Named("nodesummarizer").
		Complete(r)
}
