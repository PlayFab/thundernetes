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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var gameserverlog = logf.Log.WithName("gameserver-resource")

func (r *GameServer) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-mps-playfab-com-v1alpha1-gameserver,mutating=false,failurePolicy=fail,sideEffects=None,groups=mps.playfab.com,resources=gameservers,verbs=create;update,versions=v1alpha1,name=vgameserver.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &GameServer{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *GameServer) ValidateCreate() error {
	gameserverlog.Info("validate create", "name", r.Name)
	var allErrs field.ErrorList
	if err := r.validateOwnerReferences(); err != nil {
		allErrs = append(allErrs, err)
	}
	if errs := r.validatePortsToExpose(); errs != nil {
		allErrs = append(allErrs, errs...)
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(
		schema.GroupKind{Group: "mps.playfab.com", Kind: "GameServer"},
		r.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *GameServer) ValidateUpdate(old runtime.Object) error {
	gameserverlog.Info("validate update", "name", r.Name)
	var allErrs field.ErrorList
	if err := r.validateOwnerReferences(); err != nil {
		allErrs = append(allErrs, err)
	}
	if errs := r.validatePortsToExpose(); errs != nil {
		allErrs = append(allErrs, errs...)
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(
		schema.GroupKind{Group: "mps.playfab.com", Kind: "GameServer"},
		r.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *GameServer) ValidateDelete() error {
	gameserverlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}

// validateOwnerReference checks that OwnerReferences points to a GameServerBuild
func (r *GameServer) validateOwnerReferences() *field.Error {
	for i := 0; i < len(r.OwnerReferences); i++ {
		if r.OwnerReferences[i].Kind == "GameServerBuild" {
			return nil
		}
	}
	return field.Invalid(field.NewPath("OwnerReferences"), r.Name,
		"a GameServer must have a GameServerBuild as an owner")
}

// validatePortsToExpose makes the following validations for ports in portsToExpose:
// 1. if a port number is in portsToExpose, there must be at least one
//    matching port in the pod containers spec, this validation is skipped
//    if the GameServer has HostNetwork enabled
// 2. if a port number is in portsToExpose, the matching ports in the
//    pod containers spec must have a name
func (r *GameServer) validatePortsToExpose() field.ErrorList {
	var portsGroupedByNumber = make(map[int32][]corev1.ContainerPort)
	for i := 0; i < len(r.Spec.Template.Spec.Containers); i++ {
		container := r.Spec.Template.Spec.Containers[i]
		for j := 0; j < len(container.Ports); j++ {
			port := container.Ports[j]
			if port.ContainerPort != 0 {
				portsGroupedByNumber[port.ContainerPort] = append(portsGroupedByNumber[port.ContainerPort], port)
			}
		}
	}
	var errs field.ErrorList
	for i := 0; i < len(r.Spec.PortsToExpose); i++ {
		ports := portsGroupedByNumber[r.Spec.PortsToExpose[i]]
		if len(ports) < 1 && !r.Spec.Template.Spec.HostNetwork {
			errs = append(errs, field.Invalid(field.NewPath("spec").Child("portsToExpose"), r.Name,
				fmt.Sprintf("there must be at least one port that matches each value in portsToExpose, error in port %d", r.Spec.PortsToExpose[i])))
		}
		for j := 0; j < len(ports); j++ {
			port := ports[j]
			if port.Name == "" {
				errs = append(errs, field.Invalid(field.NewPath("spec").Child("portsToExpose"), r.Name,
					fmt.Sprintf("ports to expose must have a name, error in port %d", port.ContainerPort)))
			}
		}
	}
	return errs
}
