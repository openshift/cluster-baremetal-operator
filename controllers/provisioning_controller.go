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
	"net"
	"os"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/stretchr/stew/slice"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	utilnet "k8s.io/utils/net"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	baremetalv1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	osconfigv1 "github.com/openshift/api/config/v1"
	osclientset "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
	"github.com/openshift/cluster-baremetal-operator/provisioning"
	"github.com/openshift/library-go/pkg/operator/events"
)

const (
	// ComponentNamespace is namespace where CBO resides
	ComponentNamespace = "openshift-machine-api"
	// ComponentName is the full name of CBO
	ComponentName = "cluster-baremetal-operator"
	// install-config access details
	clusterConfigName      = "cluster-config-v1"
	clusterConfigKey       = "install-config"
	clusterConfigNamespace = "kube-system"
)

// ProvisioningReconciler reconciles a Provisioning object
type ProvisioningReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	Client          client.Client
	Scheme          *runtime.Scheme
	OSClient        osclientset.Interface
	KubeClient      kubernetes.Interface
	ReleaseVersion  string
	ImagesFilename  string
	WebHookEnabled  bool
	NetworkStack    provisioning.NetworkStackType
	EnabledFeatures v1alpha1.EnabledFeatures
}

type ensureFunc func(*provisioning.ProvisioningInfo) (bool, error)

// +kubebuilder:rbac:namespace=openshift-machine-api,groups="",resources=configmaps;secrets;services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:namespace=openshift-machine-api,groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:namespace=openshift-machine-api,groups=security.openshift.io,resources=securitycontextconstraints,verbs=use
// +kubebuilder:rbac:namespace=openshift-machine-api,groups=apps,resources=deployments;daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:namespace=openshift-machine-api,groups=monitoring.coreos.com,resources=servicemonitors,verbs=create;watch;get;list;patch

// +kubebuilder:rbac:groups=config.openshift.io,resources=proxies,verbs=get;list;watch
// +kubebuilder:rbac:groups=config.openshift.io,resources=infrastructures,verbs=get;list;watch
// +kubebuilder:rbac:groups=config.openshift.io,resources=networks,verbs=get;list;watch
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
// +kubebuilder:rbac:groups=metal3.io,resources=hostfirmwaresettings,verbs=get;create;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io,resources=hostfirmwaresettings/status,verbs=update
// +kubebuilder:rbac:groups=metal3.io,resources=firmwareschemas,verbs=get;create;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io,resources=preprovisioningimages,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io,resources=preprovisioningimages/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=metal3.io,resources=bmceventsubscriptions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io,resources=bmceventsubscriptions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;list;watch;update;patch;create;delete
// +kubebuilder:rbac:groups=metal3.io,resources=hardwaredata,verbs=get;list;watch;create;update;patch;delete

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

type InstallConfigData struct {
	SSHKey string
}

func (r *ProvisioningReconciler) readSSHKey() string {
	installConfigData := InstallConfigData{}
	clusterConfig, err := r.KubeClient.CoreV1().ConfigMaps(clusterConfigNamespace).Get(context.Background(), clusterConfigName, metav1.GetOptions{})
	if err != nil {
		klog.Warningf("Error: %v", err)
		return ""
	}
	err = yaml.Unmarshal([]byte(clusterConfig.Data[clusterConfigKey]), &installConfigData)
	if err != nil {
		klog.Warningf("Error: %v", err)
		return ""
	}
	return strings.TrimSpace(installConfigData.SSHKey)
}

func (r *ProvisioningReconciler) checkDaemonSet(state appsv1.DaemonSetConditionType, stateError error, name string) error {
	if stateError != nil {
		msg := fmt.Sprintf("%s daemonset inaccessible", name)
		err := r.updateCOStatus(ReasonResourceNotFound, msg, "")
		if err != nil {
			return fmt.Errorf("unable to put %q ClusterOperator in Degraded state: %w", clusterOperatorName, err)
		}

		return errors.Wrapf(stateError, "failed to determine state of %s daemonset", name)
	}

	if state == provisioning.DaemonSetReplicaFailure {
		msg := fmt.Sprintf("%s rollout taking too long", name)
		err := r.updateCOStatus(ReasonDeployTimedOut, msg, "")
		if err != nil {
			return fmt.Errorf("unable to put %q ClusterOperator in Degraded state: %w", clusterOperatorName, err)
		}

		return errors.New(msg)
	}

	return nil
}

func getSuccessStatus(imageCacheState appsv1.DaemonSetConditionType, ironicProxyState appsv1.DaemonSetConditionType) string {
	parts := []string{"metal3 pod"}
	if imageCacheState == provisioning.DaemonSetAvailable {
		parts = append(parts, "image cache")
	} else if imageCacheState != provisioning.DaemonSetDisabled {
		return ""
	}

	if ironicProxyState == provisioning.DaemonSetAvailable {
		parts = append(parts, "ironic proxy")
	} else if ironicProxyState != provisioning.DaemonSetDisabled {
		return ""
	}

	if len(parts) > 1 {
		return fmt.Sprintf("%s are running", strings.Join(parts, ", "))
	} else {
		return fmt.Sprintf("%s is running", parts[0])
	}
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
	err := r.ensureClusterOperator()
	if err != nil {
		return ctrl.Result{}, err
	}

	if !IsEnabled(r.EnabledFeatures) {
		// set ClusterOperator status to disabled=true, available=true
		// We're disabled; don't requeue
		return ctrl.Result{}, errors.Wrapf(
			r.updateCOStatus(ReasonUnsupported, "Nothing to do on this Platform", ""),
			"unable to put %q ClusterOperator in Disabled state", clusterOperatorName)
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

	baremetalConfig, err := r.readProvisioningCR(ctx)
	if err != nil {
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}
	if baremetalConfig == nil {
		// Provisioning configuration not available at this time.
		// Cannot proceed wtih metal3 deployment.
		klog.Info("Provisioning CR not found")
		return result, errors.Wrapf(
			r.updateCOStatus(ReasonProvisioningCRNotFound, "Waiting for Provisioning CR", ""),
			"unable to put %q ClusterOperator in Available state", clusterOperatorName)
	}

	// Read container images from Config Map
	var containerImages provisioning.Images
	if err := provisioning.GetContainerImages(&containerImages, r.ImagesFilename); err != nil {
		co_err := r.updateCOStatus(ReasonInvalidConfiguration, err.Error(), "invalid contents in images Config Map")
		if co_err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to put %q ClusterOperator in Degraded state: %w", clusterOperatorName, co_err)
		}
		return ctrl.Result{}, err
	}

	info, err := r.provisioningInfo(ctx, baremetalConfig, &containerImages, r.readSSHKey())
	if err != nil {
		return ctrl.Result{}, err
	}

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
			r.updateCOStatus(ReasonResourceNotFound, "all Metal3 resources deleted", ""),
			"unable to put %q ClusterOperator in Available state", clusterOperatorName)
	}

	specChanged := baremetalConfig.Generation != baremetalConfig.Status.ObservedGeneration
	if specChanged {
		err = r.updateCOStatus(ReasonSyncing, "", "Applying metal3 resources")
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to put %q ClusterOperator in Syncing state: %w", clusterOperatorName, err)
		}
	}

	// Always re-validate the provisioning configuration is valid.
	// This can occur if the CR was created prior to cbo getting upgraded.
	if err := baremetalConfig.ValidateBaremetalProvisioningConfig(r.EnabledFeatures); err != nil {
		err = r.updateCOStatus(ReasonInvalidConfiguration, err.Error(), "Unable to apply Provisioning CR: invalid configuration")
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to put %q ClusterOperator in Degraded state: %v", clusterOperatorName, err)
		}
		// Temporarily not requeuing request, as the user will have to fix the CR.
		return ctrl.Result{}, nil
	}

	for _, ensureResource := range []ensureFunc{
		provisioning.EnsureAllSecrets,
		provisioning.EnsureMetal3Deployment,
		provisioning.EnsureMetal3StateService,
		provisioning.EnsureImageCache,
		provisioning.EnsureBaremetalOperatorWebhook,
		provisioning.EnsureImageCustomizationService,
		provisioning.EnsureImageCustomizationDeployment,
		provisioning.EnsureIronicProxy,
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
		err = r.updateCOStatus(ReasonResourceNotFound, "metal3 deployment inaccessible", "")
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

	// Determine the status of the image cache DaemonSet
	imageCacheState, err := provisioning.GetImageCacheState(r.KubeClient.AppsV1(), ComponentNamespace, baremetalConfig)
	err = r.checkDaemonSet(imageCacheState, err, "metal3 image cache")
	if err != nil {
		return ctrl.Result{}, err
	}

	ironicProxyState, err := provisioning.GetIronicProxyState(r.KubeClient.AppsV1(), ComponentNamespace, baremetalConfig)
	err = r.checkDaemonSet(ironicProxyState, err, "ironic proxy")
	if err != nil {
		return ctrl.Result{}, err
	}

	if deploymentState == appsv1.DeploymentAvailable {
		msg := getSuccessStatus(imageCacheState, ironicProxyState)
		if msg != "" {
			err = r.updateCOStatus(ReasonComplete, msg, "")
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("unable to put %q ClusterOperator in Progressing state: %w", clusterOperatorName, err)
			}
		}
	}

	return result, nil
}

func (r *ProvisioningReconciler) provisioningInfo(ctx context.Context, provConfig *metal3iov1alpha1.Provisioning, images *provisioning.Images, sshkey string) (*provisioning.ProvisioningInfo, error) {
	proxy, err := r.OSClient.ConfigV1().Proxies().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if err := r.updateProvisioningMacAddresses(ctx, provConfig); err != nil {
		return nil, err
	}

	enableBaremetalWebhook := provisioning.BaremetalWebhookDependenciesReady(r.OSClient)

	return &provisioning.ProvisioningInfo{
		Client:                  r.KubeClient,
		EventRecorder:           events.NewLoggingEventRecorder(ComponentName),
		ProvConfig:              provConfig,
		Scheme:                  r.Scheme,
		Namespace:               ComponentNamespace,
		Images:                  images,
		Proxy:                   proxy,
		NetworkStack:            r.NetworkStack,
		SSHKey:                  sshkey,
		BaremetalWebhookEnabled: enableBaremetalWebhook,
		OSClient:                r.OSClient,
	}, nil
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
	if err := provisioning.DeleteValidatingWebhook(info); err != nil {
		return errors.Wrap(err, "failed to delete validatingwebhook and service")
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
	if err := provisioning.DeleteImageCustomizationService(info); err != nil {
		return errors.Wrap(err, "failed to delete metal3 image customization service")
	}
	if err := provisioning.DeleteImageCustomizationDeployment(info); err != nil {
		return errors.Wrap(err, "failed to delete metal3 image customization deployment")
	}
	if err := provisioning.DeleteIronicProxy(info); err != nil {
		return errors.Wrap(err, "failed to delete ironic proxy")
	}
	return nil
}

func (r *ProvisioningReconciler) networkStackFromServiceNetwork(ctx context.Context) (provisioning.NetworkStackType, error) {
	ns := provisioning.NetworkStackType(0)
	network, err := r.OSClient.ConfigV1().Networks().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return ns, errors.Wrap(err, "unable to read Network CR")
	}

	serviceNetwork := network.Status.ServiceNetwork
	if len(serviceNetwork) == 0 {
		serviceNetwork = network.Spec.ServiceNetwork
	}
	for _, snet := range serviceNetwork {
		_, cidr, err := net.ParseCIDR(snet)
		if err != nil {
			return ns, errors.Wrapf(err, "could not parse Service Network %s", snet)
		}
		if utilnet.IsIPv6CIDR(cidr) {
			ns |= provisioning.NetworkStackV6
		} else {
			ns |= provisioning.NetworkStackV4
		}
	}
	return ns, nil
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
	return r.Client.Update(ctx, provConfig)
}

// SetupWithManager configures the manager to run the controller
func (r *ProvisioningReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctx := context.Background()
	err := r.ensureClusterOperator()
	if err != nil {
		return errors.Wrap(err, "unable to set get baremetal ClusterOperator")
	}

	// Check the Platform Type to determine the state of the CO
	enabled := IsEnabled(r.EnabledFeatures)
	if !enabled {
		//Set ClusterOperator status to disabled=true, available=true
		err = r.updateCOStatus(ReasonUnsupported, "Nothing to do on this Platform", "")
		if err != nil {
			return fmt.Errorf("unable to put %q ClusterOperator in Disabled state: %w", clusterOperatorName, err)
		}
		return nil
	}

	// If Platform is BareMetal, we could still be missing the Provisioning CR
	if enabled {
		baremetalConfig, err := r.readProvisioningCR(context.Background())
		if err != nil || baremetalConfig == nil {
			err = r.updateCOStatus(ReasonProvisioningCRNotFound, "Waiting for Provisioning CR on BareMetal Platform", "")
			if err != nil {
				return fmt.Errorf("unable to put %q ClusterOperator in Available state: %w", clusterOperatorName, err)
			}
		}
	}

	r.NetworkStack, err = r.networkStackFromServiceNetwork(ctx)
	if err != nil {
		return fmt.Errorf("unable to determine network stack from service network: %w", err)
	}

	klog.InfoS("Network stack calculation", "NetworkStack", r.NetworkStack)

	return ctrl.NewControllerManagedBy(mgr).
		For(&metal3iov1alpha1.Provisioning{}).
		Owns(&corev1.Secret{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&osconfigv1.ClusterOperator{}).
		Owns(&osconfigv1.Proxy{}).
		Complete(r)
}
