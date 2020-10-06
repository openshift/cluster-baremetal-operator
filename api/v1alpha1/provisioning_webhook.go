/*

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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var provisioninglog = logf.Log.WithName("provisioning-resource")

func (r *Provisioning) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// see https://book.kubebuilder.io/cronjob-tutorial/webhook-implementation.html

// +kubebuilder:webhook:verbs=create;update,path=/validate-metal-io-v1alpha1-provisioning,mutating=false,failurePolicy=fail,groups=metal.io,resources=provisionings,versions=v1alpha1,name=vprovisioning.kb.io

var _ webhook.Validator = &Provisioning{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Provisioning) ValidateCreate() error {
	provisioninglog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Provisioning) ValidateUpdate(old runtime.Object) error {
	provisioninglog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Provisioning) ValidateDelete() error {
	provisioninglog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
