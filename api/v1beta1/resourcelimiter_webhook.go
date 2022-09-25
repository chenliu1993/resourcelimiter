/*
Copyright 2022.

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

package v1beta1

import (
	"fmt"

	"github.com/go-logr/logr"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var resourcelimiterlog = logf.Log.WithName("resourcelimiter-resource")

func (r *ResourceLimiter) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-resources-resourcelimiter-io-v1beta1-resourcelimiter,mutating=true,failurePolicy=fail,sideEffects=None,groups=resources.resourcelimiter.io,resources=resourcelimiters,verbs=create;update,versions=v1beta1,name=mresourcelimiter.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &ResourceLimiter{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *ResourceLimiter) Default() {
	resourcelimiterlog.Info("default", "name", r.Name)

	if len(r.Spec.Targets) == 0 {
		// Empty lists means all namespaces should be applied
		r.Spec.Targets = []ResourceLimiterNamespace{}
	}

	if len(r.Spec.Types) == 0 {
		r.Spec.Types = map[ResourceLimiterType]string{"limits.cpu": "2", "limits.memory": "200Mi",
			"requests.cpu": "1", "requests.memory": "150Mi"}
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-resources-resourcelimiter-io-v1beta1-resourcelimiter,mutating=false,failurePolicy=fail,sideEffects=None,groups=resources.resourcelimiter.io,resources=resourcelimiters,verbs=create;update,versions=v1beta1,name=vresourcelimiter.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &ResourceLimiter{}

func recordR(log *logr.Logger, er error) {
	if err := recover(); err != nil {
		log.Info(fmt.Sprintf("MustParse failed due to %v", err))
		er = fmt.Errorf("MustParse failed due to %v", err)
	}
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ResourceLimiter) ValidateCreate() error {
	resourcelimiterlog.Info("validate create", "name", r.Name)
	var err error
	defer recordR(&resourcelimiterlog, err)
	for t, value := range r.Spec.Types {
		resourcelimiterlog.Info(fmt.Sprintf("validating type field %s for %s", t, r.Name))
		k8sresource.MustParse(value)
	}
	// TODO(user): fill in your validation logic upon object creation.
	if err != nil {
		resourcelimiterlog.Error(err, fmt.Sprintf("validating failed for %s", r.Name))
		return err
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ResourceLimiter) ValidateUpdate(old runtime.Object) error {
	resourcelimiterlog.Info("validate create", "name", r.Name)
	var err error
	defer recordR(&resourcelimiterlog, err)
	for t, value := range r.Spec.Types {
		resourcelimiterlog.Info(fmt.Sprintf("validating type field %s for %s", t, r.Name))
		k8sresource.MustParse(value)
	}
	// TODO(user): fill in your validation logic upon object creation.
	if err != nil {
		resourcelimiterlog.Error(err, fmt.Sprintf("validating failed for %s", r.Name))
		return err
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ResourceLimiter) ValidateDelete() error {
	resourcelimiterlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
