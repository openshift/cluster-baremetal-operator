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

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	// +kubebuilder:scaffold:imports

	baremetalv1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	osconfigv1 "github.com/openshift/api/config/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	osclientset "github.com/openshift/client-go/config/clientset/versioned"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
	"github.com/openshift/cluster-baremetal-operator/controllers"
	"github.com/openshift/cluster-baremetal-operator/provisioning"
	utiltls "github.com/openshift/controller-runtime-common/pkg/tls"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(metal3iov1alpha1.AddToScheme(scheme))
	utilruntime.Must(osconfigv1.AddToScheme(scheme))
	utilruntime.Must(machinev1beta1.AddToScheme(scheme))
	utilruntime.Must(baremetalv1alpha1.AddToScheme(scheme))

	// +kubebuilder:scaffold:scheme
	// The following is needed to read the Infrastructure CR
	utilruntime.Must(osconfigv1.Install(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var imagesJSONFilename string
	var enforceTLSProfile bool

	klog.InitFlags(nil)
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&imagesJSONFilename, "images-json", "/etc/cluster-baremetal-operator/images/images.json",
		"The location of the file containing the images to use for our operands.")
	flag.BoolVar(&enforceTLSProfile, "enforce-tls-profile", false,
		"Read the TLS security profile from the APIServer CR and enforce it on managed components (Ironic, BMO) and the CBO webhook.")
	flag.Parse()

	ctrl.SetLogger(klogr.New())

	releaseVersion := os.Getenv("RELEASE_VERSION")
	if releaseVersion == "" {
		klog.Info("Environment variable RELEASE_VERSION not provided")
	}

	config := ctrl.GetConfigOrDie()

	// Determine TLS configuration for the webhook server.
	// When --enforce-tls-profile is enabled, read the TLS profile from the
	// APIServer CR and apply it. Otherwise, use the hardcoded TLS 1.2 default.
	var tlsProfileSpec osconfigv1.TLSProfileSpec
	webhookTLSOpts := []func(*tls.Config){
		func(t *tls.Config) { t.MinVersion = tls.VersionTLS12 },
	}
	if enforceTLSProfile {
		k8sClient, err := client.New(config, client.Options{Scheme: scheme})
		if err != nil {
			klog.ErrorS(err, "unable to create client for TLS profile fetch")
			os.Exit(1)
		}

		tlsProfileSpec, err = utiltls.FetchAPIServerTLSProfile(context.Background(), k8sClient)
		if err != nil {
			klog.ErrorS(err, "unable to get TLS profile from APIServer")
			os.Exit(1)
		}

		tlsConfig, unsupportedCiphers := utiltls.NewTLSConfigFromProfile(tlsProfileSpec)
		if len(unsupportedCiphers) > 0 {
			klog.Infof("TLS configuration contains unsupported ciphers that will be ignored: %v", unsupportedCiphers)
		}
		webhookTLSOpts = []func(*tls.Config){tlsConfig}
	}

	controllerOptions := ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress: metricsAddr,
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    9443,
			TLSOpts: webhookTLSOpts,
			CertDir: "/etc/cluster-baremetal-operator/tls",
		}),
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				controllers.ComponentNamespace:        {},
				provisioning.OpenshiftConfigNamespace: {},
			},
			ByObject: map[client.Object]cache.ByObject{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "monitoring.coreos.com/v1",
						"kind":       "ServiceMonitor",
					},
				}: {
					Namespaces: map[string]cache.Config{
						controllers.ComponentNamespace: {},
					},
				},
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "monitoring.coreos.com/v1",
						"kind":       "PrometheusRule",
					},
				}: {
					Namespaces: map[string]cache.Config{
						controllers.ComponentNamespace: {},
					},
				},
			},
		},
	}

	if enableLeaderElection {
		controllerOptions.LeaderElection = true
		controllerOptions.LeaderElectionReleaseOnCancel = true
		controllerOptions.LeaderElectionID = "cluster-baremetal-operator"
		controllerOptions.LeaderElectionNamespace = controllers.ComponentNamespace

		// these values match library-go LeaderElectionDefaulting, to produce this outcome
		// see https://github.com/openshift/library-go/blob/release-4.15/pkg/config/leaderelection/leaderelection.go#L97-L105
		// 1. clock skew tolerance is leaseDuration-renewDeadline == 30s
		// 2. kube-apiserver downtime tolerance is == 78s
		//      lastRetry=floor(renewDeadline/retryPeriod)*retryPeriod == 104
		//      downtimeTolerance = lastRetry-retryPeriod == 78s
		// 3. worst non-graceful lease acquisition is leaseDuration+retryPeriod == 163s
		// 4. worst graceful lease acquisition is retryPeriod == 26s
		leaseDuration := 137 * time.Second
		renewDeadline := 107 * time.Second
		retryPeriod := 26 * time.Second
		controllerOptions.LeaseDuration = &leaseDuration
		controllerOptions.RenewDeadline = &renewDeadline
		controllerOptions.RetryPeriod = &retryPeriod
	}

	mgr, err := ctrl.NewManager(config, controllerOptions)
	if err != nil {
		klog.ErrorS(err, "unable to start manager")
		os.Exit(1)
	}

	osClient := osclientset.NewForConfigOrDie(rest.AddUserAgent(config, controllers.ComponentName))
	kubeClient := kubernetes.NewForConfigOrDie(rest.AddUserAgent(config, controllers.ComponentName))
	dynamicClient := dynamic.NewForConfigOrDie(rest.AddUserAgent(config, controllers.ComponentName))

	enabledFeatures, err := controllers.EnabledFeatures(context.Background(), osClient)
	if err != nil {
		klog.ErrorS(err, "unable to get enabled features")
		os.Exit(1)
	}

	enableWebhook := provisioning.WebhookDependenciesReady(osClient)
	resourceCache := resourceapply.NewResourceCache()

	if err = (&controllers.ProvisioningReconciler{
		Client:            mgr.GetClient(),
		DynamicClient:     dynamicClient,
		Scheme:            mgr.GetScheme(),
		OSClient:          osClient,
		KubeClient:        kubeClient,
		ReleaseVersion:    releaseVersion,
		ImagesFilename:    imagesJSONFilename,
		WebHookEnabled:    enableWebhook,
		EnabledFeatures:   enabledFeatures,
		ResourceCache:     resourceCache,
		EnforceTLSProfile: enforceTLSProfile,
	}).SetupWithManager(mgr); err != nil {
		klog.ErrorS(err, "unable to create controller", "controller", "Provisioning")
		os.Exit(1)
	}
	if controllers.IsEnabled(enabledFeatures) && enableWebhook {
		info := &provisioning.ProvisioningInfo{
			Client:        kubeClient,
			EventRecorder: events.NewLoggingEventRecorder(controllers.ComponentName, clock.RealClock{}),
			Namespace:     controllers.ComponentNamespace,
			OSClient:      osClient,
			ResourceCache: resourceCache,
		}
		if err = provisioning.EnableValidatingWebhook(info, mgr, enabledFeatures); err != nil {
			klog.ErrorS(err, "problem enabling validating webhook")
			os.Exit(1)
		}
	}
	ctx, cancel := context.WithCancel(ctrl.SetupSignalHandler())
	defer cancel()

	if enforceTLSProfile {
		// Set up the TLS security profile watcher controller.
		// This triggers a graceful shutdown when the TLS profile changes,
		// so that CBO restarts with the new TLS configuration.
		if err := (&utiltls.SecurityProfileWatcher{
			Client:                mgr.GetClient(),
			InitialTLSProfileSpec: tlsProfileSpec,
			OnProfileChange: func(ctx context.Context, oldSpec, newSpec osconfigv1.TLSProfileSpec) {
				klog.Infof("TLS profile has changed, initiating a shutdown to reload it. %q: %+v, %q: %+v",
					"old profile", oldSpec,
					"new profile", newSpec,
				)
				cancel()
			},
		}).SetupWithManager(mgr); err != nil {
			klog.ErrorS(err, "unable to create TLS security profile watcher controller")
			os.Exit(1)
		}
	}
	// +kubebuilder:scaffold:builder

	klog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		klog.ErrorS(err, "problem running manager")
		os.Exit(1)
	}
}
