/*
Copyright 2021.

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

// GameServerDetailSpec defines the desired state of GameServerDetail
type GameServerDetailSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	InitialPlayers        []string `json:"initialPlayers,omitempty"`
	ConnectedPlayersCount int      `json:"connectedPlayersCount,omitempty"`
	ConnectedPlayers      []string `json:"connectedPlayers,omitempty"`
}

// GameServerDetailStatus defines the observed state of GameServerDetail
type GameServerDetailStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:singular=gameserverdetail,path=gameserverdetails,scope=Namespaced,shortName=gsd
//+kubebuilder:printcolumn:name="ConnectedPlayersCount",type=string,JSONPath=`.spec.connectedPlayersCount`

// GameServerDetail is the Schema for the gameserverdetails API
type GameServerDetail struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GameServerDetailSpec   `json:"spec,omitempty"`
	Status GameServerDetailStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GameServerDetailList contains a list of GameServerDetail
type GameServerDetailList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GameServerDetail `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GameServerDetail{}, &GameServerDetailList{})
}
