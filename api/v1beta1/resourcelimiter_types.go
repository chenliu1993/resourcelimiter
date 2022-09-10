/*
Copyright 2022.

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ResourceLimiterNamespace string
type ResourceLimiterType string

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ResourceLimiterSpec defines the desired state of ResourceLimiter
type ResourceLimiterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Targets []ResourceLimiterNamespace     `json:"targets,omitempty"`
	Types   map[ResourceLimiterType]string `json:"types,omitempty"`
	Applied bool                           `json:"applied,omitempty"`
}

// ResourceLimiterStatus defines the observed state of ResourceLimiter
type ResourceLimiterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	State string `json:"state"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// ResourceLimiter is the Schema for the resourcelimiters API
type ResourceLimiter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceLimiterSpec   `json:"spec,omitempty"`
	Status ResourceLimiterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ResourceLimiterList contains a list of ResourceLimiter
type ResourceLimiterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceLimiter `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ResourceLimiter{}, &ResourceLimiterList{})
}
