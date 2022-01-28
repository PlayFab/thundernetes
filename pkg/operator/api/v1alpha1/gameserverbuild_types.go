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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

//+kubebuilder:validation:Enum=Healthy;Unhealthy
// GameServerBuildHealth describes the health of the game server build
type GameServerBuildHealth string

const (
	BuildHealthy   GameServerBuildHealth = "Healthy"
	BuildUnhealthy GameServerBuildHealth = "Unhealthy"
)

// GameServerBuildSpec defines the desired state of GameServerBuild
type GameServerBuildSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	//+kubebuilder:validation:Minimum=0
	// StandingBy is the requested number of standingBy servers
	StandingBy int `json:"standingBy,omitempty"`
	//+kubebuilder:validation:Minimum=0
	// Max is the maximum number of servers in any state
	Max int `json:"max,omitempty"`

	// Template describes the pod template specification of the game server
	Template corev1.PodTemplateSpec `json:"template,omitempty"`

	//+kubebuilder:validation:Required
	// TitleID is the TitleID this Build belongs to
	TitleID string `json:"titleID,omitempty"`

	//+kubebuilder:validation:Required
	// Build is is the BuildID for this Build
	BuildID string `json:"buildID,omitempty"`

	//+kubebuilder:validation:Required
	// PortsToExpose is an array of tuples of container/port names that correspond to the ports that will be exposed on the VM
	PortsToExpose []PortToExpose `json:"portsToExpose,omitempty"`

	//+kubebuilder:default=5
	//+kubebuilder:validation:Minimum=0
	// CrashesToMarkUnhealthy is the number of crashes needed to mark the build unhealthy
	CrashesToMarkUnhealthy int `json:"crashesToMarkUnhealthy,omitempty"`

	// BuildMetadata is the metadata for this GameServerBuild
	BuildMetadata []BuildMetadataItem `json:"buildMetadata,omitempty"`
}

// GameServerBuildStatus defines the observed state of GameServerBuild
type GameServerBuildStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	CurrentPending                int                   `json:"currentPending,omitempty"`
	CurrentInitializing           int                   `json:"currentInitializing,omitempty"`
	CurrentStandingBy             int                   `json:"currentStandingBy,omitempty"`
	CurrentStandingByReadyDesired string                `json:"currentStandingByReadyDesired,omitempty"`
	CurrentActive                 int                   `json:"currentActive,omitempty"`
	CrashesCount                  int                   `json:"crashesCount,omitempty"`
	Health                        GameServerBuildHealth `json:"health,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:singular=gameserverbuild,path=gameserverbuilds,scope=Namespaced,shortName=gsb
//+kubebuilder:printcolumn:name="StandBy",type=string,JSONPath=`.status.currentStandingByReadyDesired`
//+kubebuilder:printcolumn:name="Active",type=string,JSONPath=`.status.currentActive`
//+kubebuilder:printcolumn:name="Crashes",type=string,JSONPath=`.status.crashesCount`
//+kubebuilder:printcolumn:name="Health",type=string,JSONPath=`.status.health`

// GameServerBuild is the Schema for the gameserverbuilds API
type GameServerBuild struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GameServerBuildSpec   `json:"spec,omitempty"`
	Status GameServerBuildStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GameServerBuildList contains a list of GameServerBuild
type GameServerBuildList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GameServerBuild `json:"items"`
}

// PortToExpose is a tuple of container/port names that correspond to the ports that will be exposed on the VM
type PortToExpose struct {
	ContainerName string `json:"containerName"`
	PortName      string `json:"portName"`
}

func init() {
	SchemeBuilder.Register(&GameServerBuild{}, &GameServerBuildList{})
}

// BuildMetadataItem is a metadata item for a GameServerBuild
type BuildMetadataItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
