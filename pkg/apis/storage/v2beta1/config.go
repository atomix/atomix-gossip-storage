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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GossipClock struct {
	Logical  *LogicalClock  `json:"logical,omitempty"`
	Physical *PhysicalClock `json:"physical,omitempty"`
	Epoch    *EpochClock    `json:"epoch,omitempty"`
}

type LogicalClock struct{}

type PhysicalClock struct{}

type EpochClock struct {
	Election corev1.LocalObjectReference `json:"election,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GossipConfig is the Schema for the GossipConfig API
// +k8s:openapi-gen=true
type GossipConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Clock             GossipClock `json:"clock,omitempty"`
	ReplicationFactor int32       `json:"replicationFactor,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GossipConfigList contains a list of GossipConfig
type GossipConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the GossipConfig of items in the list
	Items []GossipConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GossipConfig{}, &GossipConfigList{})
}
