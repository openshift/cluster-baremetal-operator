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
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	osconfigv1 "github.com/openshift/api/config/v1"
	osoperatorv1 "github.com/openshift/api/operator/v1"
	osclientset "github.com/openshift/client-go/config/clientset/versioned"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
	provisioning "github.com/openshift/cluster-baremetal-operator/provisioning"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
)

const (
	// ComponentNamespace is namespace where CBO resides
	ComponentNamespace = "openshift-machine-api"
	// ComponentName is the full name of CBO
	ComponentName = "cluster-baremetal-operator"
	// BaremetalProvisioningCR is the name of the provisioning resource
	BaremetalProvisioningCR = "provisioning-configuration"
)

// ProvisioningReconciler reconciles a Provisioning object
type ProvisioningReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	Client         client.Client
	Scheme         *runtime.Scheme
	Log            logr.Logger
	OSClient       osclientset.Interface
	EventRecorder  record.EventRecorder
	KubeClient     kubernetes.Interface
	ReleaseVersion string
	ImagesFilename string

	Generations []osoperatorv1.GenerationStatus
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
		return false, errors.Wrap(err, "unable to determine Platform")
	}

	// Disable ourselves on platforms other than bare metal
	if infra.Status.Platform != osconfigv1.BareMetalPlatformType {
		r.Log.V(1).Info("disabled", "platform", infra.Status.Platform)
		return false, nil
	}

	r.Log.V(1).Info("enabled", "platform", infra.Status.Platform)
	return true, nil
}

func (r *ProvisioningReconciler) readProvisioningCR(req ctrl.Request) (*metal3iov1alpha1.Provisioning, error) {
	ctx := context.Background()

	// provisioning.metal3.io is a singleton
	if req.Name != BaremetalProvisioningCR {
		r.Log.V(1).Info("ignoring invalid CR", "name", req.Name)
		return nil, nil
	}
	// Fetch the Provisioning instance
	instance := &metal3iov1alpha1.Provisioning{}
	if err := r.Client.Get(ctx, req.NamespacedName, instance); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, errors.Wrap(err, "unable to read Provisioning CR")
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
		err = r.updateCOStatus(ReasonUnsupported, "Operator is non-functional", "")
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
		r.Log.V(1).Info("Provisioning CR not found")
		return ctrl.Result{}, nil
	}

	err = r.ensureClusterOperator(baremetalConfig)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := provisioning.ValidateBaremetalProvisioningConfig(baremetalConfig); err != nil {
		// Provisioning configuration is not valid.
		// Requeue request.
		r.Log.Error(err, "invalid config in Provisioning CR")
		err = r.updateCOStatus(ReasonInvalidConfiguration, err.Error(), "Unable to apply Provisioning CR: invalid configuration")
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to put %q ClusterOperator in Degraded state: %v", clusterOperatorName, err)
		}
		// Temporarily not requeuing request
		return ctrl.Result{}, nil
	}

	// Read container images from Config Map
	var containerImages provisioning.Images
	if err := provisioning.GetContainerImages(&containerImages, r.ImagesFilename); err != nil {
		// Images config map is not valid
		// Provisioning configuration is not valid.
		// Requeue request.
		r.Log.Error(err, "invalid contents in images Config Map")
		co_err := r.updateCOStatus(ReasonInvalidConfiguration, err.Error(), "invalid contents in images Config Map")
		if co_err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to put %q ClusterOperator in Degraded state: %v", clusterOperatorName, co_err)
		}
		return ctrl.Result{}, err
	}

	//Create Secrets needed for Metal3 deployment
	if err := provisioning.CreateMariadbPasswordSecret(r.KubeClient.CoreV1(), ComponentNamespace, baremetalConfig, r.Scheme); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to create Mariadb password")
	}
	if err := provisioning.CreateIronicPasswordSecret(r.KubeClient.CoreV1(), ComponentNamespace, baremetalConfig, r.Scheme); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to create Ironic password")
	}
	if err := provisioning.CreateInspectorPasswordSecret(r.KubeClient.CoreV1(), ComponentNamespace, baremetalConfig, r.Scheme); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to create Inspector password")
	}

	// If Metal3 Deployment already exists, do nothing.
	exists, err := provisioning.GetExistingMetal3Deployment(r.KubeClient.AppsV1(), ComponentNamespace)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, errors.Wrap(err, "failed to to find existing Metal3 Deployment")
	}

	if exists {
		r.Log.V(1).Info("metal3 deployment already exists")
		err = r.updateCOStatus(ReasonComplete, "found existing Metal3 deployment", "")
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to put %q ClusterOperator in Available state: %v", clusterOperatorName, err)
		}
		return ctrl.Result{}, nil
	}

	err = r.updateCOStatus(ReasonSyncing, "", "Applying the Metal3 deployment")
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to put %q ClusterOperator in Syncing state: %v", clusterOperatorName, err)
	}

	// Proceed with creating a new Metal3 deployment
	metal3Deployment := provisioning.NewMetal3Deployment(ComponentNamespace, &containerImages, &baremetalConfig.Spec)
	expectedGeneration := resourcemerge.ExpectedDeploymentGeneration(metal3Deployment, r.Generations)

	err = controllerutil.SetControllerReference(baremetalConfig, metal3Deployment, r.Scheme)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to set controllerReference on deployment: %v", err)
	}

	deployment, updated, err := resourceapply.ApplyDeployment(r.KubeClient.AppsV1(),
		events.NewLoggingEventRecorder(ComponentName), metal3Deployment, expectedGeneration)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to apply Metal3 deployment: %v", err)
	}

	if updated {
		resourcemerge.SetDeploymentGeneration(&r.Generations, deployment)
	}
	err = r.updateCOStatus(ReasonComplete, "new Metal3 deployment completed", "")
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to put %q ClusterOperator in Available state: %v", clusterOperatorName, err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager configures the manager to run the controller
func (r *ProvisioningReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&metal3iov1alpha1.Provisioning{}).
		Owns(&corev1.Secret{}).
		Owns(&appsv1.Deployment{}).
		Owns(&osconfigv1.ClusterOperator{}).
		Complete(r)
}
