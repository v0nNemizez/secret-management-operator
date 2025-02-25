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

// OpenBaoSpec defines the desired state of OpenBao
type OpenBaoSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of OpenBao. Edit openbao_types.go to remove/update
	Replicas    int32  `json:"replicas"`
	StorageSize string `json:"storageSize"`
	Image       string `json:"image"`
	Config      string `json:"config"`
}

// OpenBaoStatus defines the observed state of OpenBao
type OpenBaoStatus struct {
	ReadyReplicas int32 `json:"readyReplicas"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// OpenBao is the Schema for the openbaoes API
type OpenBao struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenBaoSpec   `json:"spec,omitempty"`
	Status OpenBaoStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpenBaoList contains a list of OpenBao
type OpenBaoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenBao `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenBao{}, &OpenBaoList{})
}
