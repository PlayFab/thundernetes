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
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var (
	gameserverbuildlog = logf.Log.WithName("gameserverbuild-resource")
	// c is a live API client so we can bypass the cache when validating
	c client.Client
)

func (r *GameServerBuild) SetupWebhookWithManager(mgr ctrl.Manager) error {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(AddToScheme(scheme))

	var err error
	c, err = client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: runtime.NewScheme()})
	if err != nil {
		return err
	}
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-mps-playfab-com-v1alpha1-gameserverbuild,mutating=false,failurePolicy=fail,sideEffects=None,groups=mps.playfab.com,resources=gameserverbuilds,verbs=create;update,versions=v1alpha1,name=vgameserverbuild.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &GameServerBuild{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *GameServerBuild) ValidateCreate() error {
	gameserverbuildlog.Info("validate create", "name", r.Name)
	var allErrs field.ErrorList
	if err := r.validateCreateBuildID(); err != nil {
		allErrs = append(allErrs, err)
	}
	if errs := r.validatePortsToExpose(); errs != nil {
		allErrs = append(allErrs, errs...)
	}
	if err := r.validateStandingBy(); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(
		schema.GroupKind{Group: "mps.playfab.com", Kind: "GameServerBuild"},
		r.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *GameServerBuild) ValidateUpdate(old runtime.Object) error {
	gameserverbuildlog.Info("validate update", "name", r.Name)
	var allErrs field.ErrorList
	if err := r.validateUpdateBuildID(old); err != nil {
		allErrs = append(allErrs, err)
	}
	if errs := r.validatePortsToExpose(); errs != nil {
		allErrs = append(allErrs, errs...)
	}
	if err := r.validateStandingBy(); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(
		schema.GroupKind{Group: "mps.playfab.com", Kind: "GameServerBuild"},
		r.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *GameServerBuild) ValidateDelete() error {
	gameserverbuildlog.V(1).Info("validate delete", "name", r.Name)
	return nil
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
				r.Name, "cannot have more than one GameServerBuild with the same BuildID")
		}
	}
	return nil
}

// validateUpdateBuildID validates that the buildID is not changed
func (r *GameServerBuild) validateUpdateBuildID(old runtime.Object) *field.Error {
	if r.Spec.BuildID != old.(*GameServerBuild).Spec.BuildID {
		return field.Invalid(field.NewPath("spec").Child("buildID"),
			r.Name, "changing buildID on an existing GameServerBuild is not allowed")
	}
	return nil
}

// validatePortsToExpose makes the following validations for ports in portsToExpose:
// 1. if a port number is in portsToExpose, there must be at least one
//    matching port in the pod containers spec
// 2. if a port number is in portsToExpose, the matching ports in the
//    pod containers spec must have a name
// 3. if a port number is in portsToExpose, the matching ports in the
//    pod containers spec must not have a hostPort
func (r *GameServerBuild) validatePortsToExpose() field.ErrorList {
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
		if len(ports) < 1 {
			errs = append(errs, field.Invalid(field.NewPath("spec").Child("portsToExpose"), r.Name,
				fmt.Sprintf("there must be at least one port that matches each value in portsToExpose, error in port %d", r.Spec.PortsToExpose[i])))
		}
		for j := 0; j < len(ports); j++ {
			port := ports[j]
			if port.Name == "" {
				errs = append(errs, field.Invalid(field.NewPath("spec").Child("portsToExpose"), r.Name,
					fmt.Sprintf("ports to expose must have a name, error in port %d", port.ContainerPort)))
			}
			if port.HostPort != 0 {
				errs = append(errs, field.Invalid(field.NewPath("spec").Child("portsToExpose"), r.Name,
					fmt.Sprintf("ports to expose must not have a hostPort value, error in port %d", port.ContainerPort)))
			}
		}
	}
	return errs
}

// validateStandingBy checks that the standingBy value is less or equal than max
func (r *GameServerBuild) validateStandingBy() *field.Error {
	if r.Spec.StandingBy > r.Spec.Max {
		return field.Invalid(field.NewPath("spec").Child("standingby"),
			r.Name, "standingby must be less or equal than max")
	}
	return nil
}
