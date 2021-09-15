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
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	baremetalv1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	osconfigv1 "github.com/openshift/api/config/v1"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
	"github.com/openshift/cluster-baremetal-operator/pkg/externalclients"
	"github.com/openshift/cluster-baremetal-operator/pkg/network"
	"github.com/openshift/cluster-baremetal-operator/provisioning"
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
	Client          client.Client
	Scheme          *runtime.Scheme
	ReleaseVersion  string
	ImagesFilename  string
	WebHookEnabled  bool
	NetworkStack    network.NetworkStackType
	ExternalClients externalclients.ExternalResourceClient
}

type ensureFunc func(context.Context, *provisioning.ProvisioningInfo) (bool, error)

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
// +kubebuilder:rbac:groups="",resources=configmaps;secrets;services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments;daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io,resources=provisionings;provisionings/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io,resources=provisionings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts/status;baremetalhosts/finalizers,verbs=update
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;list;watch;update;patch;create

func (r *ProvisioningReconciler) isEnabled() (bool, error) {
	infra, err := r.ExternalClients.ClusterInfrastructure(context.Background())
	if err != nil {
		return false, errors.Wrap(err, "unable to determine Platform")
	}

	// Disable ourselves on platforms other than bare metal
	if infra.Status.Platform != osconfigv1.BareMetalPlatformType {
		return false, nil
	}

	return true, nil
}

// Reconcile updates the cluster settings when the Provisioning resource changes
func (r *ProvisioningReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	if r.NetworkStack == 0 {
		infra, err := r.ExternalClients.ClusterInfrastructure(ctx)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "unable to read Infrastructure CR")
		}

		r.NetworkStack, err = network.NetworkStackFromURL(infra.Status.APIServerInternalURL)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "unable to calculate the NetworkStack")
		}
		klog.InfoS("Network stack calculation", "APIServerInternalHost", infra.Status.APIServerInternalURL, "NetworkStack", r.NetworkStack)
	}

	// Make sure ClusterOperator exists
	co, err := r.ExternalClients.ClusterOperatorGet(ctx, clusterOperatorName)
	if apierrors.IsNotFound(err) {
		co, err = r.createClusterOperator()
	}

	defer func() {
		err = r.ExternalClients.ClusterOperatorStatusUpdate(ctx, co)
		if err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, errors.Wrap(err, "failed to update ClusterOperator status")})
		}
	}()

	r.ensureDefaultsClusterOperator(co)

	result := ctrl.Result{}
	if !r.WebHookEnabled {
		if r.ExternalClients.WebhookDependenciesReady(ctx) {
			log.Info("restarting to enable the webhook")
			os.Exit(1)
		}
		// Keep checking for our webhook dependencies to be ready, so we can
		// enable the webhook.
		result.RequeueAfter = 5 * time.Minute
	}

	// provisioning.metal3.io is a singleton
	// Note: this check is here to make sure that the early startup configuration
	// is correct. For day 2 operatations the webhook will validate this.
	if req.Name != metal3iov1alpha1.ProvisioningSingletonName {
		klog.InfoS("ignoring invalid Provisioning CR", "name", req.Name)
		return result, nil
	}

	enabled, err := r.isEnabled()
	if err != nil {
		return result, errors.Wrap(err, "could not determine whether to run")
	}
	if !enabled {
		// set ClusterOperator status to disabled=true, available=true
		// We're disabled; don't requeue
		r.updateCOStatus(co, ReasonUnsupported, "Nothing to do on this Platform", "")
		return result, nil
	}

	b := &metal3iov1alpha1.Provisioning{}
	if err := r.Client.Get(ctx, req.NamespacedName, b); err != nil {
		if apierrors.IsNotFound(err) {
			r.updateCOStatus(co, ReasonProvisioningCRNotFound, "Provisioning CR not found", "")
			return result, nil
		}
		return result, err
	}

	defer func() {
		existingProv := &metal3iov1alpha1.Provisioning{
			ObjectMeta: metav1.ObjectMeta{
				Name:      req.Name,
				Namespace: req.Namespace,
			},
		}
		_, err := controllerutil.CreateOrPatch(ctx, r.Client, existingProv, func() error {
			existingProv = b.DeepCopy()
			return nil
		})

		if err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, errors.Wrap(err, "failed to update Provisioning CR")})
		}
	}()

	// Add finalizer first if not exist to avoid the race condition between init and delete
	if !controllerutil.ContainsFinalizer(b, metal3iov1alpha1.ProvisioningFinalizer) {
		controllerutil.AddFinalizer(b, metal3iov1alpha1.ProvisioningFinalizer)
		return result, nil
	}

	// Handle deletion reconciliation loop.
	if !b.ObjectMeta.DeletionTimestamp.IsZero() {
		result, err = r.reconcileDelete(ctx, b)
		if err != nil {
			r.updateCOStatus(co, ReasonDeployTimedOut, err.Error(), "Unable to delete a metal3 resource on Provisioning CR deletion")
			return result, err
		}
		r.updateCOStatus(co, ReasonComplete, "all Metal3 resources deleted", "")
		return result, nil
	}

	// Handle normal reconciliation loop.
	reconcileRes, err := r.reconcile(ctx, b, co)
	return lowestNonZeroResult(result, reconcileRes), err
}

func (r *ProvisioningReconciler) reconcile(ctx context.Context, b *metal3iov1alpha1.Provisioning, co *osconfigv1.ClusterOperator) (ctrl.Result, error) {
	if err := b.ValidateBaremetalProvisioningConfig(); err != nil {
		klog.Error(err, "invalid config in Provisioning CR")
		r.updateCOStatus(co, ReasonInvalidConfiguration, err.Error(), "Unable to apply Provisioning CR: invalid configuration")
		// Temporarily not requeuing request
		return ctrl.Result{}, nil
	}

	info, err := r.provisioningInfo(ctx, b)
	if err != nil {
		r.updateCOStatus(co, ReasonInvalidConfiguration, err.Error(), "invalid contents in images Config Map")
		return ctrl.Result{}, err
	}

	specChanged := b.Generation != b.Status.ObservedGeneration
	if specChanged {
		r.updateCOStatus(co, ReasonSyncing, "", "Applying metal3 resources")
	}

	for _, ensureResource := range []ensureFunc{
		provisioning.EnsureAllSecrets,
		provisioning.EnsureMetal3Deployment,
		provisioning.EnsureMetal3StateService,
		provisioning.EnsureImageCache,
	} {
		updated, err := ensureResource(ctx, info)
		if err != nil || updated {
			return ctrl.Result{}, err
		}
	}
	b.Status.ObservedGeneration = b.Generation

	// Determine the status of the deployment
	deploymentState, err := provisioning.GetDeploymentState(ctx, r.Client, ComponentNamespace, b)
	if err != nil {
		r.updateCOStatus(co, ReasonResourceNotFound, "metal3 deployment inaccessible", "")
		return ctrl.Result{}, errors.Wrap(err, "failed to determine state of metal3 deployment")
	}
	if deploymentState == appsv1.DeploymentReplicaFailure {
		r.updateCOStatus(co, ReasonDeployTimedOut, "metal3 deployment rollout taking too long", "")
	}

	// Determine the status of the DaemonSet
	daemonSetState, err := provisioning.GetDaemonSetState(ctx, r.Client, ComponentNamespace, b)
	if err != nil {
		r.updateCOStatus(co, ReasonResourceNotFound, "metal3 image cache daemonset inaccessible", "")
		return ctrl.Result{}, errors.Wrap(err, "failed to determine state of metal3 image cache daemonset")
	}
	if daemonSetState == provisioning.DaemonSetReplicaFailure {
		r.updateCOStatus(co, ReasonDeployTimedOut, "metal3 image cache rollout taking too long", "")
	}
	if deploymentState == appsv1.DeploymentAvailable && daemonSetState == provisioning.DaemonSetAvailable {
		r.updateCOStatus(co, ReasonComplete, "metal3 pod and image cache are running", "")
	}

	return ctrl.Result{}, nil
}

func (r *ProvisioningReconciler) provisioningInfo(ctx context.Context, provConfig *metal3iov1alpha1.Provisioning) (*provisioning.ProvisioningInfo, error) {
	proxy, err := r.ExternalClients.ClusterProxy(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get cluster proxy")
	}

	openshiftConfigSecret, err := r.ExternalClients.OpenshiftConfigSecret(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get OpenShift config secret")
	}

	sshkey, err := r.ExternalClients.ReadSSHKey(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not read sshkey")
	}

	images := &provisioning.Images{}
	err = provisioning.GetContainerImages(images, r.ImagesFilename)
	if err != nil {
		return nil, errors.Wrap(err, "invalid contents in images Config Map")
	}

	if err := r.updateProvisioningMacAddresses(ctx, provConfig); err != nil {
		return nil, err
	}

	return &provisioning.ProvisioningInfo{
		Client:                r.Client,
		EventRecorder:         events.NewLoggingEventRecorder(ComponentName),
		ProvConfig:            provConfig,
		Scheme:                r.Scheme,
		Namespace:             ComponentNamespace,
		Images:                images,
		Proxy:                 proxy,
		NetworkStack:          r.NetworkStack,
		SSHKey:                sshkey,
		OpenshiftConfigSecret: openshiftConfigSecret,
	}, nil
}

// reconcileDelete delete resources and remove Finalizer when it is
func (r *ProvisioningReconciler) reconcileDelete(ctx context.Context, b *metal3iov1alpha1.Provisioning) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(b, metal3iov1alpha1.ProvisioningFinalizer) {
		// The Provisioning object is being deleted
		return ctrl.Result{}, nil
	}

	info, err := r.provisioningInfo(ctx, b)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "unable to create provionsingInfo")
	}

	if err := provisioning.DeleteAllSecrets(ctx, info); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to delete one or more metal3 secrets")
	}
	if err := provisioning.DeleteMetal3Deployment(ctx, info); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to delete metal3 deployment")
	}
	if err := provisioning.DeleteMetal3StateService(ctx, info); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to delete metal3 service")
	}
	if err := provisioning.DeleteImageCache(ctx, info); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to delete metal3 image cache")
	}

	// Remove our finalizer from the list.
	controllerutil.RemoveFinalizer(b, metal3iov1alpha1.ProvisioningFinalizer)

	return ctrl.Result{}, nil
}

// lowestNonZeroResult compares two reconciliation results
// and returns the one with lowest requeue time.
func lowestNonZeroResult(i, j ctrl.Result) ctrl.Result {
	switch {
	case i.IsZero():
		return j
	case j.IsZero():
		return i
	case i.Requeue:
		return i
	case j.Requeue:
		return j
	case i.RequeueAfter < j.RequeueAfter:
		return i
	default:
		return j
	}
}

func (r *ProvisioningReconciler) updateProvisioningMacAddresses(ctx context.Context, provConfig *metal3iov1alpha1.Provisioning) error {
	if len(provConfig.Spec.ProvisioningMacAddresses) != 0 {
		return nil
	}

	macs := []string{}
	bmhl := baremetalv1alpha1.BareMetalHostList{}
	if err := r.Client.List(ctx, &bmhl, &client.ListOptions{Namespace: ComponentNamespace}); err != nil {
		return err
	}
	for _, bmh := range bmhl.Items {
		if strings.Contains(bmh.Name, "master") && len(bmh.Spec.BootMACAddress) > 0 {
			macs = append(macs, bmh.Spec.BootMACAddress)
		}
	}
	provConfig.Spec.ProvisioningMacAddresses = macs
	return nil
}

// SetupWithManager configures the manager to run the controller
func (r *ProvisioningReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// The below code in production should set NetworkStack = 0 and then in the
	// first reconcile it will lookup the apiInternalHost.
	// In dev when we are running "go run main.go" the lookup will not work, so
	// use the Env to figure it out..
	switch os.Getenv("IP_STACK") {
	case "v4":
		// running in IPv4 dev environment
		r.NetworkStack = network.NetworkStackV4
	case "v6":
		// running in IPv6 dev environment
		r.NetworkStack = network.NetworkStackV6
	default:
		r.NetworkStack = 0
	}

	// This is a channel Source so that even if we don't have a Provisioning CR
	// we will at least get one event to:
	// - intialize the ClusterOperator and set it's status.
	// - look up the NetworkStack
	// We need to do this in the Reconcile proper as outside of it, the client
	// is not synced and we can't use it.
	oneInitialEvent := make(chan event.GenericEvent)
	go func() {
		time.Sleep(time.Second)
		oneInitialEvent <- event.GenericEvent{
			Object: &metal3iov1alpha1.Provisioning{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fake-object-for-once-off-event",
				},
			},
		}
	}()
	return ctrl.NewControllerManagedBy(mgr).
		For(&metal3iov1alpha1.Provisioning{}).
		Owns(&corev1.Secret{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&osconfigv1.ClusterOperator{}).
		Owns(&osconfigv1.Proxy{}).
		Watches(&source.Channel{Source: oneInitialEvent}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
