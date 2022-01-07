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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//+kubebuilder:validation:Enum=Healthy;Unhealthy
// GameServerHealth describes the health of the game server
type GameServerHealth string

//+kubebuilder:validation:Enum=Initializing;Active;StandingBy;Crashed;GameCompleted
// GameServerState describes the state of the game server
type GameServerState string

const (
	GameServerStateInitializing  GameServerState = "Initializing"
	GameServerStateStandingBy    GameServerState = "StandingBy"
	GameServerStateActive        GameServerState = "Active"
	GameServerStateCrashed       GameServerState = "Crashed"
	GameServerStateGameCompleted GameServerState = "GameCompleted"
)

const (
	Healthy   GameServerHealth = "Healthy"
	Unhealthy GameServerHealth = "Unhealthy"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// GameServerSpec defines the desired state of GameServer
type GameServerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Template describes the pod template specification of the game server
	Template corev1.PodTemplateSpec `json:"template,omitempty"`
	//+kubebuilder:validation:Required
	// TitleID is the TitleID this GameServer belongs to
	TitleID string `json:"titleID,omitempty"`
	//+kubebuilder:validation:Required
	// Build is the BuildID for this GameServer
	BuildID string `json:"buildID,omitempty"`
	//+kubebuilder:validation:Required
	// PortsToExpose is an array of tuples of container/port names that correspond to the ports that will be exposed on the VM
	PortsToExpose []PortToExpose `json:"portsToExpose,omitempty"`
	// BuildMetadata is the metadata for the GameServerBuild this GameServer belongs to
	BuildMetadata []BuildMetadataItem `json:"buildMetadata,omitempty"`
}

// GameServerStatus defines the observed state of GameServer
type GameServerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Health        GameServerHealth `json:"health,omitempty"`
	State         GameServerState  `json:"state,omitempty"`
	PublicIP      string           `json:"publicIP,omitempty"`
	Ports         string           `json:"ports,omitempty"`
	SessionID     string           `json:"sessionID,omitempty"`
	SessionCookie string           `json:"sessionCookie,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:singular=gameserver,path=gameservers,scope=Namespaced,shortName=gs
//+kubebuilder:printcolumn:name="Health",type=string,JSONPath=`.status.health`
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
//+kubebuilder:printcolumn:name="PublicIP",type=string,JSONPath=`.status.publicIP`
//+kubebuilder:printcolumn:name="Ports",type=string,JSONPath=`.status.ports`
//+kubebuilder:printcolumn:name="SessionID",type=string,JSONPath=`.status.sessionID`

// GameServer is the Schema for the gameservers API
type GameServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GameServerSpec   `json:"spec,omitempty"`
	Status GameServerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GameServerList contains a list of GameServer
type GameServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GameServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GameServer{}, &GameServerList{})
}
