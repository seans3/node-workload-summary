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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// WorkloadSummarySpec defines the desired state of WorkloadSummary
type WorkloadSummarySpec struct {
}

// WorkloadSummaryStatus defines the observed state of WorkloadSummary.
type WorkloadSummaryStatus struct {
	PodCount        int              `json:"podCount"`
	ShortType       string           `json:"shortType"`
	LongType        string           `json:"longType"`
	NodeSummaryRefs []NodeSummaryRef `json:"nodeSummaryRefs,omitempty"`
}

// NodeSummaryRef is a reference to a NodeSummary.
type NodeSummaryRef struct {
	// Name of the NodeSummary.
	Name string `json:"name"`
	// Number of nodes in that group used by this workload.
	NodeCount int `json:"nodeCount"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// WorkloadSummary is the Schema for the workloadsummaries API
type WorkloadSummary struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkloadSummarySpec   `json:"spec,omitempty"`
	Status WorkloadSummaryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WorkloadSummaryList contains a list of WorkloadSummary
type WorkloadSummaryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkloadSummary `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WorkloadSummary{}, &WorkloadSummaryList{})
}
