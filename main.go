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
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	ctrl "sigs.k8s.io/controller-runtime"

	// +kubebuilder:scaffold:imports

	baremetalv1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	osconfigv1 "github.com/openshift/api/config/v1"
	osclientset "github.com/openshift/client-go/config/clientset/versioned"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
	"github.com/openshift/cluster-baremetal-operator/controllers"
	"github.com/openshift/cluster-baremetal-operator/provisioning"
	"github.com/openshift/library-go/pkg/operator/events"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(metal3iov1alpha1.AddToScheme(scheme))
	utilruntime.Must(osconfigv1.AddToScheme(scheme))
	utilruntime.Must(baremetalv1alpha1.AddToScheme(scheme))

	// +kubebuilder:scaffold:scheme
	// The following is needed to read the Infrastructure CR
	utilruntime.Must(osconfigv1.Install(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var imagesJSONFilename string

	klog.InitFlags(nil)
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&imagesJSONFilename, "images-json", "/etc/cluster-baremetal-operator/images/images.json",
		"The location of the file containing the images to use for our operands.")
	flag.Parse()

	ctrl.SetLogger(klogr.New())

	releaseVersion := os.Getenv("RELEASE_VERSION")
	if releaseVersion == "" {
		klog.Info("Environment variable RELEASE_VERSION not provided")
	}

	config := ctrl.GetConfigOrDie()
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Namespace:          controllers.ComponentNamespace,
		LeaderElection:     enableLeaderElection,
		Port:               9443,
		CertDir:            "/etc/cluster-baremetal-operator/tls",
	})
	if err != nil {
		klog.ErrorS(err, "unable to start manager")
		os.Exit(1)
	}

	osClient := osclientset.NewForConfigOrDie(rest.AddUserAgent(config, controllers.ComponentName))
	kubeClient := kubernetes.NewForConfigOrDie(rest.AddUserAgent(config, controllers.ComponentName))

	enabledFeatures, err := controllers.EnabledFeatures(context.Background(), osClient)
	if err != nil {
		klog.ErrorS(err, "unable to get enabled features")
		os.Exit(1)
	}

	enableWebhook := provisioning.WebhookDependenciesReady(osClient)

	if err = (&controllers.ProvisioningReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		OSClient:        osClient,
		KubeClient:      kubeClient,
		ReleaseVersion:  releaseVersion,
		ImagesFilename:  imagesJSONFilename,
		WebHookEnabled:  enableWebhook,
		EnabledFeatures: enabledFeatures,
	}).SetupWithManager(mgr); err != nil {
		klog.ErrorS(err, "unable to create controller", "controller", "Provisioning")
		os.Exit(1)
	}
	if controllers.IsEnabled(enabledFeatures) && enableWebhook {
		info := &provisioning.ProvisioningInfo{
			Client:        kubeClient,
			EventRecorder: events.NewLoggingEventRecorder(controllers.ComponentName),
			Namespace:     controllers.ComponentNamespace,
			OSClient:      osClient,
		}
		if err = provisioning.EnableValidatingWebhook(info, mgr, enabledFeatures); err != nil {
			klog.ErrorS(err, "problem enabling validating webhook")
			os.Exit(1)
		}
	}
	// +kubebuilder:scaffold:builder

	klog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		klog.ErrorS(err, "problem running manager")
		os.Exit(1)
	}
}
