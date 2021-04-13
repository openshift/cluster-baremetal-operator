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

	"github.com/pkg/errors"
	"github.com/stretchr/stew/slice"
	appsv1 "k8s.io/api/apps/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	osconfigv1 "github.com/openshift/api/config/v1"
	osclientset "github.com/openshift/client-go/config/clientset/versioned"
	configinformers "github.com/openshift/client-go/config/informers/externalversions"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/apis/metal3.io/v1alpha1"
	metal3externalinformers "github.com/openshift/cluster-baremetal-operator/client/informers/externalversions"
	metal3ioClient "github.com/openshift/cluster-baremetal-operator/client/versioned"
	provisioning "github.com/openshift/cluster-baremetal-operator/provisioning"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
)

const (
	// ComponentNamespace is namespace where CBO resides
	ComponentNamespace = "openshift-machine-api"
	// ComponentName is the full name of CBO
	ComponentName = "cluster-baremetal-operator"
)

// ProvisioningController watches the metal3 deployment, create if not exists
type ProvisioningController struct {
	client     metal3ioClient.Interface
	kubeClient kubernetes.Interface
	osClient   osclientset.Interface

	releaseVersion string
	imagesFilename string
	webHookEnabled bool
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

func (r *ProvisioningController) isEnabled() (bool, error) {
	ctx := context.Background()

	infra, err := r.osClient.ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return false, errors.Wrap(err, "unable to determine Platform")
	}

	// Disable ourselves on platforms other than bare metal
	if infra.Status.Platform != osconfigv1.BareMetalPlatformType {
		return false, nil
	}

	return true, nil
}

func (r *ProvisioningController) readProvisioningCR(ctx context.Context) (*metal3iov1alpha1.Provisioning, error) {
	instance, err := r.client.Metal3V1alpha1().Provisionings().Get(ctx, metal3iov1alpha1.ProvisioningSingletonName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, errors.Wrap(err, "unable to read Provisioning CR")
	}
	return instance, nil
}

// sync updates the cluster settings when the Provisioning
// resource changes
func (r *ProvisioningController) sync(ctx context.Context, controllerContext factory.SyncContext) error {
	// Make sure ClusterOperator exists
	err := r.ensureClusterOperator(nil)
	if err != nil {
		return err
	}

	enabled, err := r.isEnabled()
	if err != nil {
		return errors.Wrap(err, "could not determine whether to run")
	}
	if !enabled {
		// set ClusterOperator status to disabled=true, available=true
		// We're disabled; don't requeue
		return errors.Wrapf(
			r.updateCOStatus(ReasonUnsupported, "Nothing to do on this Platform", ""),
			"unable to put %q ClusterOperator in Disabled state", clusterOperatorName)
	}

	baremetalConfig, err := r.readProvisioningCR(ctx)
	if err != nil {
		// Error reading the object - requeue the request.
		return err
	}
	if baremetalConfig == nil {
		// Provisioning configuration not available at this time.
		// Cannot proceed wtih metal3 deployment.
		klog.Info("Provisioning CR not found")
		return nil
	}

	if !r.webHookEnabled {
		if provisioning.WebhookDependenciesReady(r.osClient) &&
			slice.Contains(baremetalConfig.ObjectMeta.Finalizers, metal3iov1alpha1.ProvisioningFinalizer) {
			klog.Info("restarting to enable the webhook")
			os.Exit(1)
		}
		klog.Info("Webhook Dependencies not Ready checking later")
	}

	// Make sure ClusterOperator's ownership is updated
	err = r.ensureClusterOperator(baremetalConfig)
	if err != nil {
		return err
	}

	// Read container images from Config Map
	var containerImages provisioning.Images
	if err := provisioning.GetContainerImages(&containerImages, r.imagesFilename); err != nil {
		// Images config map is not valid
		// Provisioning configuration is not valid.
		// Requeue request.
		klog.ErrorS(err, "invalid contents in images Config Map")
		co_err := r.updateCOStatus(ReasonInvalidConfiguration, err.Error(), "invalid contents in images Config Map")
		if co_err != nil {
			return fmt.Errorf("unable to put %q ClusterOperator in Degraded state: %w", clusterOperatorName, co_err)
		}
		return err
	}

	// Get cluster-wide proxy information
	clusterWideProxy, err := r.osClient.ConfigV1().Proxies().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return err
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
			return fmt.Errorf("unable to put %q ClusterOperator in Degraded state: %w", clusterOperatorName, coErr)
		}
		return err
	}
	if deleted {
		return errors.Wrapf(
			r.updateCOStatus(ReasonComplete, "all Metal3 resources deleted", ""),
			"unable to put %q ClusterOperator in Available state", clusterOperatorName)
	}

	specChanged := baremetalConfig.Generation != baremetalConfig.Status.ObservedGeneration
	if specChanged {
		err = r.updateCOStatus(ReasonSyncing, "", "Applying metal3 resources")
		if err != nil {
			return fmt.Errorf("unable to put %q ClusterOperator in Syncing state: %w", clusterOperatorName, err)
		}
	}

	if !r.webHookEnabled {
		// Check if provisioning configuration is valid
		if err := baremetalConfig.ValidateBaremetalProvisioningConfig(); err != nil {
			// Provisioning configuration is not valid.
			// Requeue request.
			klog.Error(err, "invalid config in Provisioning CR")
			err = r.updateCOStatus(ReasonInvalidConfiguration, err.Error(), "Unable to apply Provisioning CR: invalid configuration")
			if err != nil {
				return fmt.Errorf("unable to put %q ClusterOperator in Degraded state: %v", clusterOperatorName, err)
			}
			// Temporarily not requeuing request
			return nil
		}
	}

	for _, ensureResource := range []ensureFunc{
		provisioning.EnsureAllSecrets,
		provisioning.EnsureMetal3Deployment,
		provisioning.EnsureMetal3StateService,
		provisioning.EnsureImageCache,
	} {
		updated, err := ensureResource(info)
		if err != nil {
			return err
		}
		if updated {
			_, err = r.client.Metal3V1alpha1().Provisionings().UpdateStatus(ctx, baremetalConfig, metav1.UpdateOptions{})
			return err
		}
	}

	if specChanged {
		baremetalConfig.Status.ObservedGeneration = baremetalConfig.Generation
		_, err = r.client.Metal3V1alpha1().Provisionings().UpdateStatus(ctx, baremetalConfig, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("unable to update observed generation: %w", err)
		}
	}

	// Determine the status of the deployment
	deploymentState, err := provisioning.GetDeploymentState(r.kubeClient.AppsV1(), ComponentNamespace, baremetalConfig)
	if err != nil {
		err = r.updateCOStatus(ReasonNotFound, "metal3 deployment inaccessible", "")
		if err != nil {
			return fmt.Errorf("unable to put %q ClusterOperator in Degraded state: %w", clusterOperatorName, err)
		}
		return errors.Wrap(err, "failed to determine state of metal3 deployment")
	}
	if deploymentState == appsv1.DeploymentReplicaFailure {
		err = r.updateCOStatus(ReasonDeployTimedOut, "metal3 deployment rollout taking too long", "")
		if err != nil {
			return fmt.Errorf("unable to put %q ClusterOperator in Degraded state: %w", clusterOperatorName, err)
		}
	}

	// Determine the status of the DaemonSet
	daemonSetState, err := provisioning.GetDaemonSetState(r.kubeClient.AppsV1(), ComponentNamespace, baremetalConfig)
	if err != nil {
		err = r.updateCOStatus(ReasonNotFound, "metal3 image cache daemonset inaccessible", "")
		if err != nil {
			return fmt.Errorf("unable to put %q ClusterOperator in Degraded state: %w", clusterOperatorName, err)
		}
		return errors.Wrap(err, "failed to determine state of metal3 image cache daemonset")
	}
	if daemonSetState == provisioning.DaemonSetReplicaFailure {
		err = r.updateCOStatus(ReasonDeployTimedOut, "metal3 image cache rollout taking too long", "")
		if err != nil {
			return fmt.Errorf("unable to put %q ClusterOperator in Degraded state: %w", clusterOperatorName, err)
		}
	}
	if deploymentState == appsv1.DeploymentAvailable && daemonSetState == provisioning.DaemonSetAvailable {
		err = r.updateCOStatus(ReasonComplete, "metal3 pod and image cache are running", "")
		if err != nil {
			return fmt.Errorf("unable to put %q ClusterOperator in Progressing state: %w", clusterOperatorName, err)
		}
	}

	return nil
}

func (r *ProvisioningController) provisioningInfo(provConfig *metal3iov1alpha1.Provisioning, images *provisioning.Images, proxy *osconfigv1.Proxy) *provisioning.ProvisioningInfo {
	return &provisioning.ProvisioningInfo{
		Client:        r.kubeClient,
		EventRecorder: events.NewLoggingEventRecorder(ComponentName),
		ProvConfig:    provConfig,
		Scheme:        clientgoscheme.Scheme,
		Namespace:     ComponentNamespace,
		Images:        images,
		Proxy:         proxy,
	}
}

//Ensure Finalizer is present on the Provisioning CR when not deleted and
//delete resources and remove Finalizer when it is
func (r *ProvisioningController) checkForCRDeletion(ctx context.Context, info *provisioning.ProvisioningInfo) (bool, error) {
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

		_, err := r.client.Metal3V1alpha1().Provisionings().Update(ctx, info.ProvConfig, metav1.UpdateOptions{})
		return false, errors.Wrap(err, "failed to update Provisioning CR with its finalizer")
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

		_, err := r.client.Metal3V1alpha1().Provisionings().Update(ctx, info.ProvConfig, metav1.UpdateOptions{})
		return true, errors.Wrap(err, "failed to remove finalizer from Provisioning CR")
	}
}

//Delete Secrets and the Metal3 Deployment objects
func (r *ProvisioningController) deleteMetal3Resources(info *provisioning.ProvisioningInfo) error {
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

func (r *ProvisioningController) preStart(ctx context.Context, syncContext factory.SyncContext) error {
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
		return nil
	}

	// If Platform is BareMetal, we could still be missing the Provisioning CR
	baremetalConfig, err := r.readProvisioningCR(ctx)
	if err != nil || baremetalConfig == nil {
		err = r.updateCOStatus(ReasonComplete, "Provisioning CR not found on BareMetal Platform; marking operator as available", "")
		if err != nil {
			return fmt.Errorf("unable to put %q ClusterOperator in Available state: %w", clusterOperatorName, err)
		}
	}

	return nil
}

// this fakes out a function called HasSyned, so return true when we don't
// want to visit an object.
func filterOutNotOwned(obj interface{}) bool {
	object := obj.(metav1.Object)
	for _, owner := range object.GetOwnerReferences() {
		if owner.APIVersion == metal3iov1alpha1.GroupVersion.Version &&
			owner.Kind == "Provisioning" &&
			owner.Name == metal3iov1alpha1.ProvisioningSingletonName {
			return false
		}
	}
	return true
}

func filterOutClusterOperators(obj interface{}) bool {
	object := obj.(metav1.Object)
	if object.GetName() == "baremetal" || object.GetName() == "service-ca" || object.GetName() == "authentication" {
		return false
	}
	return true
}

func NewProvisioningController(
	client metal3ioClient.Interface,
	kubeClient kubernetes.Interface,
	osClient osclientset.Interface,
	kubeInformersForNamespace informers.SharedInformerFactory,
	metal3Informers metal3externalinformers.SharedInformerFactory,
	configInformer configinformers.SharedInformerFactory,
	eventRecorder events.Recorder,
	releaseVersion string,
	imagesFilename string,
	webHookEnabled bool,
) factory.Controller {
	c := &ProvisioningController{
		client:         client,
		kubeClient:     kubeClient,
		osClient:       osClient,
		releaseVersion: releaseVersion,
		imagesFilename: imagesFilename,
		webHookEnabled: webHookEnabled,
	}

	return factory.New().WithPostStartHooks(c.preStart).WithInformers(
		metal3Informers.Metal3().V1alpha1().Provisionings().Informer(),
		configInformer.Config().V1().Proxies().Informer(),
	).WithFilteredEventsInformers(filterOutNotOwned,
		kubeInformersForNamespace.Core().V1().Secrets().Informer(),
		kubeInformersForNamespace.Core().V1().Services().Informer(),
		kubeInformersForNamespace.Apps().V1().Deployments().Informer(),
		kubeInformersForNamespace.Apps().V1().DaemonSets().Informer(),
	).WithFilteredEventsInformers(filterOutClusterOperators,
		configInformer.Config().V1().ClusterOperators().Informer(),
	).WithSync(c.sync).ToController("ProvisioningController", eventRecorder.WithComponentSuffix(ComponentName))
}
