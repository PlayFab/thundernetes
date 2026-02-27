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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var (
	gameserverbuildlog = logf.Log.WithName("gameserverbuild-resource")
	c                  client.Client
)

const (
	errNoHostPort                 = "ports to expose must not have a hostPort value"
	errNoPortName                 = "ports to expose must have a name"
	errBuildIdUnique              = "cannot have more than one GameServerBuild with the same BuildID"
	errBuildIdImmutable           = "changing buildID on an existing GameServerBuild is not allowed"
	errPortsMatchingPortsToExpose = "there must be at least one port that matches each value in portsToExpose"
	errNoOwner                    = "a GameServer must have a GameServerBuild as an owner"
	errStandingByLessThanMax      = "standingby must be less or equal than max"
)

func (r *GameServerBuild) SetupWebhookWithManager(mgr ctrl.Manager) error {
	// this should be a live API reader but this won't in this case since we're querying the GameServerBuild via spec.buildID
	// and arbitrary field CRD selectors are not working at this time
	// https://github.com/kubernetes/kubernetes/issues/53459
	c = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-mps-playfab-com-v1alpha1-gameserverbuild,mutating=false,failurePolicy=fail,sideEffects=None,groups=mps.playfab.com,resources=gameserverbuilds,verbs=create;update,versions=v1alpha1,name=vgameserverbuild.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &GameServerBuild{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *GameServerBuild) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	gsb := obj.(*GameServerBuild)
	gameserverbuildlog.Info("validate create", "name", gsb.Name)
	var allErrs field.ErrorList
	if err := gsb.validateCreateBuildID(); err != nil {
		allErrs = append(allErrs, err)
	}
	if errs := gsb.validatePortsToExpose(); errs != nil {
		allErrs = append(allErrs, errs...)
	}
	if err := gsb.validateStandingBy(); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(
		schema.GroupKind{Group: "mps.playfab.com", Kind: "GameServerBuild"},
		gsb.Name, allErrs)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *GameServerBuild) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	gsb := newObj.(*GameServerBuild)
	gameserverbuildlog.Info("validate update", "name", gsb.Name)
	var allErrs field.ErrorList
	if err := gsb.validateUpdateBuildID(oldObj); err != nil {
		allErrs = append(allErrs, err)
	}
	if errs := gsb.validatePortsToExpose(); errs != nil {
		allErrs = append(allErrs, errs...)
	}
	if err := gsb.validateStandingBy(); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(
		schema.GroupKind{Group: "mps.playfab.com", Kind: "GameServerBuild"},
		gsb.Name, allErrs)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (r *GameServerBuild) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	gsb := obj.(*GameServerBuild)
	gameserverbuildlog.V(1).Info("validate delete", "name", gsb.Name)
	return nil, nil
}

// validateCreateBuildID checks that there is not another GameServerBuild with different name
// but with the same buildID
func (r *GameServerBuild) validateCreateBuildID() *field.Error {
	var gsbList GameServerBuildList
	if err := c.List(context.Background(), &gsbList, client.InNamespace(r.Namespace), client.MatchingFields{"spec.buildID": r.Spec.BuildID}); err != nil {
		return field.Invalid(field.NewPath("spec").Child("buildID"),
			r.Name, err.Error())
	}
	for i := 0; i < len(gsbList.Items); i++ {
		gsb := gsbList.Items[i]
		if r.Name != gsb.Name {
			return field.Invalid(field.NewPath("spec").Child("buildID"),
				r.Name, errBuildIdUnique)
		}
	}
	return nil
}

// validateUpdateBuildID validates that the buildID is not changed
func (r *GameServerBuild) validateUpdateBuildID(old runtime.Object) *field.Error {
	if r.Spec.BuildID != old.(*GameServerBuild).Spec.BuildID {
		return field.Invalid(field.NewPath("spec").Child("buildID"),
			r.Name, errBuildIdImmutable)
	}
	return nil
}

// validatePortsToExpose validates the portsToExpose slice
func (r *GameServerBuild) validatePortsToExpose() field.ErrorList {
	return validatePortsToExposeInternal(r.Name, &r.Spec.Template.Spec, r.Spec.PortsToExpose, true /* validateHostPort */)
}

// validateStandingBy checks that the standingBy value is less or equal than max
func (r *GameServerBuild) validateStandingBy() *field.Error {
	if r.Spec.StandingBy > r.Spec.Max {
		return field.Invalid(field.NewPath("spec").Child("standingby"),
			r.Name, errStandingByLessThanMax)
	}
	return nil
}

// validatePortsToExposeInternal validates portsToExpose slice
// it performs the following validations
//   - if a port number is in portsToExpose, there must be at least one
//     matching port in the pod containers spec
//     This part of validation is skipped if the GameServer has HostNetwork enabled
//     This can happen when the user creates a multi-container GameServer with hostNetwork enabled
//     and has selected a hostPort for an existing container
//   - if a port number is in portsToExpose, the matching ports in the
//     pod containers spec must have a name. This is because the name will be used by the GSDK to reference the port
//   - if a port number is in portsToExpose, the matching ports in the
//     pod containers spec must not have a hostPort
//     We set validateHostPort to true only for GameServerBuild validation. When the GameServer is created, we assign a HostPort so no need for validation
func validatePortsToExposeInternal(name string, spec *corev1.PodSpec, portsToExpose []int32, validateHostPort bool) field.ErrorList {
	var portsGroupedByNumber = make(map[int32][]corev1.ContainerPort)
	for i := 0; i < len(spec.Containers); i++ {
		container := spec.Containers[i]
		for j := 0; j < len(container.Ports); j++ {
			port := container.Ports[j]
			if port.ContainerPort != 0 {
				portsGroupedByNumber[port.ContainerPort] = append(portsGroupedByNumber[port.ContainerPort], port)
			}
		}
	}
	var errs field.ErrorList
	for i := 0; i < len(portsToExpose); i++ {
		ports := portsGroupedByNumber[portsToExpose[i]]
		if !spec.HostNetwork && len(ports) < 1 {
			errs = append(errs, field.Invalid(field.NewPath("spec").Child("portsToExpose"), name,
				fmt.Sprintf("%s: error in port %d", errPortsMatchingPortsToExpose, portsToExpose[i])))
		}
		for j := 0; j < len(ports); j++ {
			port := ports[j]
			if port.Name == "" {
				errs = append(errs, field.Invalid(field.NewPath("spec").Child("portsToExpose"), name,
					fmt.Sprintf("%s: error in port %d", errNoPortName, port.ContainerPort)))
			}
			if validateHostPort && port.HostPort != 0 {
				errs = append(errs, field.Invalid(field.NewPath("spec").Child("portsToExpose"), name,
					fmt.Sprintf("%s: error in port %d", errNoHostPort, port.ContainerPort)))
			}
		}
	}
	return errs
}
