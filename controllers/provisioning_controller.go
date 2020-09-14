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
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	osconfigv1 "github.com/openshift/api/config/v1"
	osclientset "github.com/openshift/client-go/config/clientset/versioned"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
)

const (
	// Remove from here when this is brought in as part of baremetal_config.go
	baremetalProvisioningCR = "provisioning-configuration"
	// ComponentNamespace is namespace where CBO resides
	ComponentNamespace = "openshift-machine-api"
	// ComponentName is the full name of CBO
	ComponentName = "cluster-baremetal-operator"
)

// ProvisioningReconciler reconciles a Provisioning object
type ProvisioningReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	Client        client.Client
	Scheme        *runtime.Scheme
	Log           logr.Logger
	OSClient      osclientset.Interface
	EventRecorder record.EventRecorder
}

// +kubebuilder:rbac:groups=metal3.io,resources=provisionings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io,resources=provisionings/status,verbs=get;update;patch

func (r *ProvisioningReconciler) isEnabled() (bool, error) {
	ctx := context.Background()

	infra := &osconfigv1.Infrastructure{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Name: "cluster",
	}, infra)
	if err != nil {
		r.Log.Error(err, "unable to determine Platform")
		return false, err
	}

	r.Log.V(1).Info("reconciling", "platform", infra.Status.Platform)

	// Disable ourselves on platforms other than bare metal
	if infra.Status.Platform != osconfigv1.BareMetalPlatformType {
		r.Log.V(1).Info("disabled", "platform", infra.Status.Platform)
		return false, nil
	}

	return true, nil
}

func (r *ProvisioningReconciler) readProvisioningCR(req ctrl.Request) (*metal3iov1alpha1.Provisioning, error) {
	ctx := context.Background()

	// provisioning.metal3.io is a singleton
	if req.Name != baremetalProvisioningCR {
		r.Log.V(1).Info("ignoring invalid CR", "name", req.Name)
		return nil, nil
	}
	// Fetch the Provisioning instance
	instance := &metal3iov1alpha1.Provisioning{}
	if err := r.Client.Get(ctx, req.NamespacedName, instance); err != nil {
		if apierrors.IsNotFound(err) {
			r.Log.V(1).Info("Provisioning CR not found")
			return nil, nil
		}
		r.Log.Error(err, "unable to read Provisioning CR")
		return nil, err
	}
	return instance, nil
}

// Reconcile updates the cluster settings when the Provisioning
// resource changes
func (r *ProvisioningReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	//log := r.Log.WithValues("provisioning", req.NamespacedName)

	enabled, err := r.isEnabled()
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "could not determine whether to run")
	}
	if !enabled {
		// set ClusterOperator status to disabled=true, available=true
		err = r.updateCOStatusDisabled()
		if err != nil {
			return ctrl.Result{}, err
		}

		// We're disabled; don't requeue
		return ctrl.Result{}, nil
	}

	baremetalConfig, err := r.readProvisioningCR(req)
	if err != nil {
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}
	if baremetalConfig == nil {
		// Provisioning configuration not available at this time.
		// Cannot proceed wtih metal3 deployment.
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
