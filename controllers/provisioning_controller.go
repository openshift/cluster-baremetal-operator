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

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
	osconfigv1 "github.com/openshift/api/config/v1"
)

var componentNamespace = "openshift-machine-api"
var componentName = "cluster-baremetal-operator"

// ProvisioningReconciler reconciles a Provisioning object
type ProvisioningReconciler struct {
        // This client, initialized using mgr.Client() above, is a split client
        // that reads objects from the cache and writes to the apiserver
        client client.Client

	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=metal3.io,resources=provisionings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io,resources=provisionings/status,verbs=get;update;patch

// Reconcile updates the cluster settings when the Provisioning
// resource changes
func (r *ProvisioningReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("provisioning", req.NamespacedName)

	log.Info("Reconciling Provisioning")

	infra := &osconfigv1.Infrastructure{}
	err := r.client.Get(ctx, client.ObjectKey{
		Name: "cluster",
		}, infra)
        if err != nil {
                log.Info("Unable to determine Platform that the Operator is running on.")
                return ctrl.Result{}, err
        }

        // Disable ourselves on platforms other than bare metal
        if infra.Status.Platform != osconfigv1.BareMetalPlatformType {
		// set ClusterOperator status to Disabled.
                if err != nil {
                        return ctrl.Result{}, err
                }
                // We're disabled; don't requeue
                return ctrl.Result{}, nil
        }

	return ctrl.Result{}, nil
}

// SetupWithManager configures the manager to run the controller
func (r *ProvisioningReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&metal3iov1alpha1.Provisioning{}).
		Complete(r)
}
