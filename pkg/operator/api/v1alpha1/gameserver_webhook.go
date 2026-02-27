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
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var gameserverlog = logf.Log.WithName("gameserver-resource")

func (r *GameServer) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-mps-playfab-com-v1alpha1-gameserver,mutating=false,failurePolicy=fail,sideEffects=None,groups=mps.playfab.com,resources=gameservers,verbs=create;update,versions=v1alpha1,name=vgameserver.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &GameServer{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *GameServer) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	gs := obj.(*GameServer)
	gameserverlog.Info("validate create", "name", gs.Name)
	var allErrs field.ErrorList
	if err := gs.validateOwnerReferences(); err != nil {
		allErrs = append(allErrs, err)
	}
	if errs := gs.validatePortsToExpose(); errs != nil {
		allErrs = append(allErrs, errs...)
	}
	if len(allErrs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(
		schema.GroupKind{Group: "mps.playfab.com", Kind: "GameServer"},
		gs.Name, allErrs)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *GameServer) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	gs := newObj.(*GameServer)
	gameserverlog.Info("validate update", "name", gs.Name)
	var allErrs field.ErrorList
	if err := gs.validateOwnerReferences(); err != nil {
		allErrs = append(allErrs, err)
	}
	if errs := gs.validatePortsToExpose(); errs != nil {
		allErrs = append(allErrs, errs...)
	}
	if len(allErrs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(
		schema.GroupKind{Group: "mps.playfab.com", Kind: "GameServer"},
		gs.Name, allErrs)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (r *GameServer) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	gs := obj.(*GameServer)
	gameserverlog.Info("validate delete", "name", gs.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
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
