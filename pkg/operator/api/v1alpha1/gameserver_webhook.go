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
		errNoOwner)
}

// validatePortsToExpose validates the portsToExpose slice
func (r *GameServer) validatePortsToExpose() field.ErrorList {
	return validatePortsToExposeInternal(r.Name, &r.Spec.Template.Spec, r.Spec.PortsToExpose, false /* validateHostPort */)
}
