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
var gameserverbuildlog = logf.Log.WithName("gameserverbuild-resource")

func (r *GameServerBuild) SetupWebhookWithManager(mgr ctrl.Manager) error {
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
	return r.ValidateGameServerBuild()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *GameServerBuild) ValidateUpdate(old runtime.Object) error {
	gameserverbuildlog.Info("validate update", "name", r.Name)
	return r.ValidateGameServerBuild()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *GameServerBuild) ValidateDelete() error {
	gameserverbuildlog.Info("validate delete", "name", r.Name)
	return nil
}

func (r *GameServerBuild) ValidateGameServerBuild() error {
	var allErrs field.ErrorList
	if err := r.ValidateStandingBy(); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(
		schema.GroupKind{Group: "mps.playfab.com", Kind: "GameServerBuild"},
		r.Name, allErrs)
}

func (r *GameServerBuild) ValidateStandingBy() *field.Error {
	if r.Spec.StandingBy > r.Spec.Max {
		return field.Invalid(field.NewPath("spec").Child("standingby"),
							 r.Name, "standingby must be less or equal than max")
	}
	return nil
}
