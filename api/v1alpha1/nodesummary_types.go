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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeSummarySpec defines the desired state of NodeSummary
type NodeSummarySpec struct {
	// The selector is set by the controller and is immutable.
	Selector *metav1.LabelSelector `json:"selector"`
}

// NodeSummaryStatus defines the observed state of NodeSummary.
type NodeSummaryStatus struct {
	PodCount        int                 `json:"podCount"`
	NodeCount       int                 `json:"nodeCount"`
	NodeNamesPrefix string              `json:"nodeNamesPrefix"`
	Conditions      []metav1.Condition  `json:"conditions,omitempty"`
	Allocatable     corev1.ResourceList `json:"allocatable,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// NodeSummary is the Schema for the nodesummaries API
type NodeSummary struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeSummarySpec   `json:"spec,omitempty"`
	Status NodeSummaryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NodeSummaryList contains a list of NodeSummary
type NodeSummaryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeSummary `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeSummary{}, &NodeSummaryList{})
}
