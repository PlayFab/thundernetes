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
// Important: Run "make" and "make manifests" to regenerate code after modifying this file

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
	// PortsToExpose is an array of ports that will be exposed on the VM
	PortsToExpose []int32 `json:"portsToExpose,omitempty"`
	// BuildMetadata is the metadata for the GameServerBuild this GameServer belongs to
	BuildMetadata []BuildMetadataItem `json:"buildMetadata,omitempty"`
}

// GameServerStatus defines the observed state of GameServer
type GameServerStatus struct {
	// Health defines the health of the game server
	Health GameServerHealth `json:"health,omitempty"`
	// State defines the state of the game server (Initializing, StandingBy, Active etc.)
	State GameServerState `json:"state,omitempty"`
	// PublicIP is the PublicIP of the game server
	PublicIP string `json:"publicIP,omitempty"`
	// Ports is a concatenated list of the ports this game server listens to
	Ports string `json:"ports,omitempty"`
	// SessionID is used during allocation to uniquely identify a game session
	SessionID string `json:"sessionID,omitempty"`
	// SessionCookie is an optional parameter that can be set during allocation. It is passed to the game server process
	SessionCookie string `json:"sessionCookie,omitempty"`
	// InitialPlayers is an optional list of usernames of the initial players that will enter the server. It is used for validation via the game server process
	InitialPlayers []string `json:"initialPlayers,omitempty"`
	// NodeAge is the age in days of the Node (VM) hosting this game server
	NodeAge int `json:"nodeAge,omitempty"`
	// NodeName is the name of the Node (VM) hosting this game server
	NodeName string `json:"nodeName,omitempty"`
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
