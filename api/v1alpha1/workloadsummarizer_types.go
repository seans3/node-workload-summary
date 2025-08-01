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

// WorkloadSummarizerSpec defines the desired state of WorkloadSummarizer
type WorkloadSummarizerSpec struct {
	// Defines which root-level objects are considered workloads.
	WorkloadTypes []WorkloadType `json:"workloadTypes"`
}

// WorkloadType defines a type of workload to summarize.
type WorkloadType struct {
	// Group of the workload type.
	Group string `json:"group"`
	// Kind of the workload type.
	Kind string `json:"kind"`
}

// WorkloadSummarizerStatus defines the observed state of WorkloadSummarizer.
type WorkloadSummarizerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// WorkloadSummarizer is the Schema for the workloadsummarizers API
type WorkloadSummarizer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkloadSummarizerSpec   `json:"spec,omitempty"`
	Status WorkloadSummarizerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WorkloadSummarizerList contains a list of WorkloadSummarizer
type WorkloadSummarizerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkloadSummarizer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WorkloadSummarizer{}, &WorkloadSummarizerList{})
}
