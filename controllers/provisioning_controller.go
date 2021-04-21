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
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/stew/slice"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	osconfigv1 "github.com/openshift/api/config/v1"
	osclientset "github.com/openshift/client-go/config/clientset/versioned"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
	provisioning "github.com/openshift/cluster-baremetal-operator/provisioning"
	"github.com/openshift/library-go/pkg/operator/events"
)

const (
	// ComponentNamespace is namespace where CBO resides
	ComponentNamespace = "openshift-machine-api"
	// ComponentName is the full name of CBO
	ComponentName = "cluster-baremetal-operator"
)

// ProvisioningReconciler reconciles a Provisioning object
type ProvisioningReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	Client         client.Client
	Scheme         *runtime.Scheme
	OSClient       osclientset.Interface
	EventRecorder  record.EventRecorder
	KubeClient     kubernetes.Interface
	ReleaseVersion string
	ImagesFilename string
	WebHookEnabled bool
}

type ensureFunc func(*provisioning.ProvisioningInfo) (bool, error)

// +kubebuilder:rbac:namespace=openshift-machine-api,groups="",resources=configmaps;secrets;services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:namespace=openshift-machine-api,groups=security.openshift.io,resources=securitycontextconstraints,verbs=use
// +kubebuilder:rbac:namespace=openshift-machine-api,groups=apps,resources=deployments;daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:namespace=openshift-machine-api,groups=monitoring.coreos.com,resources=servicemonitors,verbs=create;watch;get;list;patch

// +kubebuilder:rbac:groups=config.openshift.io,resources=proxies,verbs=get;list;watch
// +kubebuilder:rbac:groups=config.openshift.io,resources=infrastructures,verbs=get;list;watch
// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=use
// +kubebuilder:rbac:groups=config.openshift.io,resources=clusteroperators;clusteroperators/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=config.openshift.io,resources=infrastructures;infrastructures/status,verbs=get
// +kubebuilder:rbac:groups="",resources=events,verbs=create;watch;list;patch
// +kubebuilder:rbac:groups="",resources=secrets;services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments;daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io,resources=provisionings;provisionings/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io,resources=provisionings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts/status;baremetalhosts/finalizers,verbs=update
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;list;watch;update;patch;create

func (r *ProvisioningReconciler) isEnabled() (bool, error) {
	ctx := context.Background()

	infra, err := r.OSClient.ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return false, errors.Wrap(err, "unable to determine Platform")
	}

	// Disable ourselves on platforms other than bare metal
	if infra.Status.Platform != osconfigv1.BareMetalPlatformType {
		return false, nil
	}

	return true, nil
}

func (r *ProvisioningReconciler) readProvisioningCR(ctx context.Context) (*metal3iov1alpha1.Provisioning, error) {
	// Fetch the Provisioning instance
	instance := &metal3iov1alpha1.Provisioning{}
	namespacedName := types.NamespacedName{Name: metal3iov1alpha1.ProvisioningSingletonName, Namespace: ""}
	if err := r.Client.Get(ctx, namespacedName, instance); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, errors.Wrap(err, "unable to read Provisioning CR")
	}
	return instance, nil
}

// Reconcile updates the cluster settings when the Provisioning
// resource changes
func (r *ProvisioningReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// provisioning.metal3.io is a singleton
	// Note: this check is here to make sure that the early startup configuration
	// is correct. For day 2 operatations the webhook will validate this.
	if req.Name != metal3iov1alpha1.ProvisioningSingletonName {
		klog.Info("ignoring invalid CR", "name", req.Name)
		return ctrl.Result{}, nil
	}

	// Make sure ClusterOperator exists
	err := r.ensureClusterOperator(nil)
	if err != nil {
		return ctrl.Result{}, err
	}

	result := ctrl.Result{}
	if !r.WebHookEnabled {
		if provisioning.WebhookDependenciesReady(r.OSClient) {
			klog.Info("restarting to enable the webhook")
			os.Exit(1)
		}
		// Keep checking for our webhook dependencies to be ready, so we can
		// enable the webhook.
		result.RequeueAfter = 5 * time.Minute
	}

	enabled, err := r.isEnabled()
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "could not determine whether to run")
	}
	if !enabled {
		// set ClusterOperator status to disabled=true, available=true
		// We're disabled; don't requeue
		return ctrl.Result{}, errors.Wrapf(
			r.updateCOStatus(ReasonUnsupported, "Nothing to do on this Platform", ""),
			"unable to put %q ClusterOperator in Disabled state", clusterOperatorName)
	}

	baremetalConfig, err := r.readProvisioningCR(ctx)
	if err != nil {
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}
	if baremetalConfig == nil {
		// Provisioning configuration not available at this time.
		// Cannot proceed wtih metal3 deployment.
		klog.Info("Provisioning CR not found")
		return result, nil
	}

	// Make sure ClusterOperator's ownership is updated
	err = r.ensureClusterOperator(baremetalConfig)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Read container images from Config Map
	var containerImages provisioning.Images
	if err := provisioning.GetContainerImages(&containerImages, r.ImagesFilename); err != nil {
		// Images config map is not valid
		// Provisioning configuration is not valid.
		// Requeue request.
		klog.ErrorS(err, "invalid contents in images Config Map")
		co_err := r.updateCOStatus(ReasonInvalidConfiguration, err.Error(), "invalid contents in images Config Map")
		if co_err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to put %q ClusterOperator in Degraded state: %w", clusterOperatorName, co_err)
		}
		return ctrl.Result{}, err
	}

	// Get cluster-wide proxy information
	clusterWideProxy, err := r.OSClient.ConfigV1().Proxies().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return ctrl.Result{}, err
	}

	info := r.provisioningInfo(baremetalConfig, &containerImages, clusterWideProxy)

	// Check if Provisioning Configuartion is being deleted
	deleted, err := r.checkForCRDeletion(ctx, info)
	if err != nil {
		var coErr error
		if deleted {
			coErr = r.updateCOStatus(ReasonDeployTimedOut, err.Error(), "Unable to delete a metal3 resource on Provisioning CR deletion")
		} else {
			coErr = r.updateCOStatus(ReasonInvalidConfiguration, err.Error(), "Unable to add Finalizer on Provisioning CR")
		}
		if coErr != nil {
			return ctrl.Result{}, fmt.Errorf("unable to put %q ClusterOperator in Degraded state: %w", clusterOperatorName, coErr)
		}
		return ctrl.Result{}, err
	}
	if deleted {
		return result, errors.Wrapf(
			r.updateCOStatus(ReasonComplete, "all Metal3 resources deleted", ""),
			"unable to put %q ClusterOperator in Available state", clusterOperatorName)
	}

	specChanged := baremetalConfig.Generation != baremetalConfig.Status.ObservedGeneration
	if specChanged {
		err = r.updateCOStatus(ReasonSyncing, "", "Applying metal3 resources")
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to put %q ClusterOperator in Syncing state: %w", clusterOperatorName, err)
		}
	}

	if !r.WebHookEnabled {
		// Check if provisioning configuration is valid
		if err := baremetalConfig.ValidateBaremetalProvisioningConfig(); err != nil {
			// Provisioning configuration is not valid.
			// Requeue request.
			klog.Error(err, "invalid config in Provisioning CR")
			err = r.updateCOStatus(ReasonInvalidConfiguration, err.Error(), "Unable to apply Provisioning CR: invalid configuration")
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("unable to put %q ClusterOperator in Degraded state: %v", clusterOperatorName, err)
			}
			// Temporarily not requeuing request
			return ctrl.Result{}, nil
		}
	}

	//Create Secrets needed for Metal3 deployment
	if err := provisioning.CreateAllSecrets(r.KubeClient.CoreV1(), ComponentNamespace, baremetalConfig, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// Check Metal3 Deployment already exists and managed by MAO.
	metal3DeploymentSelector, maoOwned, err := provisioning.CheckExistingMetal3Deployment(r.KubeClient.AppsV1(), ComponentNamespace)
	info.PodLabelSelector = metal3DeploymentSelector
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, errors.Wrap(err, "failed to check for existing Metal3 Deployment")
	}

	if maoOwned {
		klog.Info("Adding annotation for CBO to take ownership of metal3 deployment created by MAO")
	}

	for _, ensureResource := range []ensureFunc{
		provisioning.EnsureMetal3Deployment,
		provisioning.EnsureMetal3StateService,
		provisioning.EnsureImageCache,
	} {
		updated, err := ensureResource(info)
		if err != nil {
			return ctrl.Result{}, err
		}
		if updated {
			return result, r.Client.Status().Update(ctx, baremetalConfig)
		}
	}

	if specChanged {
		baremetalConfig.Status.ObservedGeneration = baremetalConfig.Generation
		err = r.Client.Status().Update(ctx, baremetalConfig)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to update observed generation: %w", err)
		}
	}

	// Determine the status of the deployment
	deploymentState, err := provisioning.GetDeploymentState(r.KubeClient.AppsV1(), ComponentNamespace, baremetalConfig)
	if err != nil {
		err = r.updateCOStatus(ReasonNotFound, "metal3 deployment inaccessible", "")
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to put %q ClusterOperator in Degraded state: %w", clusterOperatorName, err)
		}
		return ctrl.Result{}, errors.Wrap(err, "failed to determine state of metal3 deployment")
	}
	if deploymentState == appsv1.DeploymentReplicaFailure {
		err = r.updateCOStatus(ReasonDeployTimedOut, "metal3 deployment rollout taking too long", "")
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to put %q ClusterOperator in Degraded state: %w", clusterOperatorName, err)
		}
	}

	// Determine the status of the DaemonSet
	daemonSetState, err := provisioning.GetDaemonSetState(r.KubeClient.AppsV1(), ComponentNamespace, baremetalConfig)
	if err != nil {
		err = r.updateCOStatus(ReasonNotFound, "metal3 image cache daemonset inaccessible", "")
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to put %q ClusterOperator in Degraded state: %w", clusterOperatorName, err)
		}
		return ctrl.Result{}, errors.Wrap(err, "failed to determine state of metal3 image cache daemonset")
	}
	if daemonSetState == provisioning.DaemonSetReplicaFailure {
		err = r.updateCOStatus(ReasonDeployTimedOut, "metal3 image cache rollout taking too long", "")
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to put %q ClusterOperator in Degraded state: %w", clusterOperatorName, err)
		}
	}
	if deploymentState == appsv1.DeploymentAvailable && daemonSetState == provisioning.DaemonSetAvailable {
		err = r.updateCOStatus(ReasonComplete, "metal3 pod and image cache are running", "")
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to put %q ClusterOperator in Progressing state: %w", clusterOperatorName, err)
		}
	}

	return result, nil
}

func (r *ProvisioningReconciler) provisioningInfo(provConfig *metal3iov1alpha1.Provisioning, images *provisioning.Images, proxy *osconfigv1.Proxy) *provisioning.ProvisioningInfo {
	return &provisioning.ProvisioningInfo{
		Client:        r.KubeClient,
		EventRecorder: events.NewLoggingEventRecorder(ComponentName),
		ProvConfig:    provConfig,
		Scheme:        r.Scheme,
		Namespace:     ComponentNamespace,
		Images:        images,
		Proxy:         proxy,
	}
}

//Ensure Finalizer is present on the Provisioning CR when not deleted and
//delete resources and remove Finalizer when it is
func (r *ProvisioningReconciler) checkForCRDeletion(ctx context.Context, info *provisioning.ProvisioningInfo) (bool, error) {
	// examine DeletionTimestamp to determine if object is under deletion
	if info.ProvConfig.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// to registering our finalizer.
		if slice.Contains(info.ProvConfig.ObjectMeta.Finalizers,
			metal3iov1alpha1.ProvisioningFinalizer) {
			return false, nil
		}

		// Add finalizer becasue it doesn't already exist
		controllerutil.AddFinalizer(info.ProvConfig, metal3iov1alpha1.ProvisioningFinalizer)

		return false, errors.Wrap(
			r.Client.Update(ctx, info.ProvConfig),
			"failed to update Provisioning CR with its finalizer")
	} else {
		// The Provisioning object is being deleted
		if !slice.Contains(info.ProvConfig.ObjectMeta.Finalizers, metal3iov1alpha1.ProvisioningFinalizer) {
			return false, nil
		}

		if err := r.deleteMetal3Resources(info); err != nil {
			return false, errors.Wrap(err, "failed to delete metal3 resource")
		}
		// Remove our finalizer from the list and update it.
		controllerutil.RemoveFinalizer(info.ProvConfig, metal3iov1alpha1.ProvisioningFinalizer)

		return true, errors.Wrap(
			r.Client.Update(ctx, info.ProvConfig),
			"failed to remove finalizer from Provisioning CR")
	}
}

//Delete Secrets and the Metal3 Deployment objects
func (r *ProvisioningReconciler) deleteMetal3Resources(info *provisioning.ProvisioningInfo) error {
	if err := provisioning.DeleteAllSecrets(info); err != nil {
		return errors.Wrap(err, "failed to delete one or more metal3 secrets")
	}
	if err := provisioning.DeleteMetal3Deployment(info); err != nil {
		return errors.Wrap(err, "failed to delete metal3 deployment")
	}
	if err := provisioning.DeleteMetal3StateService(info); err != nil {
		return errors.Wrap(err, "failed to delete metal3 service")
	}
	if err := provisioning.DeleteImageCache(info); err != nil {
		return errors.Wrap(err, "failed to delete metal3 image cache")
	}
	return nil
}

func mapObjectToCR(o client.Object) []reconcile.Request {
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{Name: metal3iov1alpha1.ProvisioningSingletonName, Namespace: ""},
		},
	}
}

// SetupWithManager configures the manager to run the controller
func (r *ProvisioningReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := r.ensureClusterOperator(nil)
	if err != nil {
		return errors.Wrap(err, "unable to set get baremetal ClusterOperator")
	}

	// Check the Platform Type to determine the state of the CO
	enabled, err := r.isEnabled()
	if err != nil {
		return errors.Wrap(err, "could not determine whether to run")
	}
	if !enabled {
		//Set ClusterOperator status to disabled=true, available=true
		err = r.updateCOStatus(ReasonUnsupported, "Nothing to do on this Platform", "")
		if err != nil {
			return fmt.Errorf("unable to put %q ClusterOperator in Disabled state: %w", clusterOperatorName, err)
		}
	}

	// If Platform is BareMetal, we could still be missing the Provisioning CR
	if enabled {
		baremetalConfig, err := r.readProvisioningCR(context.Background())
		if err != nil || baremetalConfig == nil {
			err = r.updateCOStatus(ReasonComplete, "Provisioning CR not found on BareMetal Platform; marking operator as available", "")
			if err != nil {
				return fmt.Errorf("unable to put %q ClusterOperator in Available state: %w", clusterOperatorName, err)
			}
		}
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&metal3iov1alpha1.Provisioning{}).
		Owns(&corev1.Secret{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&osconfigv1.ClusterOperator{}).
		Watches(&source.Kind{Type: &osconfigv1.Proxy{}}, handler.EnqueueRequestsFromMapFunc(mapObjectToCR)).
		Complete(r)
}
