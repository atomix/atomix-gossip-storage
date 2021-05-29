// Copyright 2019-present Open Networking Foundation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v2beta1

import (
	"github.com/atomix/atomix-controller/pkg/apis/core/v2beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GossipProtocolSpec specifies a GossipProtocol configuration
type GossipProtocolSpec struct {
	// Replicas is the number of replicas
	Replicas int32 `json:"replicas,omitempty"`

	// Partitions is the number of partitions
	Partitions int32 `json:"partitions,omitempty"`

	// Image is the image to run
	Image string `json:"image,omitempty"`

	// ImagePullPolicy is the pull policy to apply
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// ImagePullSecrets is a list of secrets for pulling images
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

// GossipProtocolStatus defines the status of a GossipProtocol
type GossipProtocolStatus struct {
	*v2beta1.ProtocolStatus `json:",inline"`
	Pods                    []PodStatus `json:"pods,omitempty"`
}

// PodStatus represents the status of the protocol configuration for a pod
type PodStatus struct {
	corev1.ObjectReference `json:",inline"`
	Revision               int64 `json:"revision,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GossipProtocol is the Schema for the GossipProtocol API
// +k8s:openapi-gen=true
type GossipProtocol struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              GossipProtocolSpec   `json:"spec,omitempty"`
	Status            GossipProtocolStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GossipProtocolList contains a list of GossipProtocol
type GossipProtocolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the GossipProtocol of items in the list
	Items []GossipProtocol `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GossipProtocol{}, &GossipProtocolList{})
}
