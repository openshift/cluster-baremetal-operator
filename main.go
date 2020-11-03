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
	"flag"
	"os"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	// +kubebuilder:scaffold:imports

	osconfigv1 "github.com/openshift/api/config/v1"
	osclientset "github.com/openshift/client-go/config/clientset/versioned"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
	"github.com/openshift/cluster-baremetal-operator/controllers"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		setupLog.Error(err, "Error adding k8s client to scheme.")
		os.Exit(1)
	}

	if err := metal3iov1alpha1.AddToScheme(scheme); err != nil {
		setupLog.Error(err, "Error adding k8s client to scheme.")
		os.Exit(1)
	}

	if err := osconfigv1.AddToScheme(scheme); err != nil {
		setupLog.Error(err, "Error adding k8s client to scheme.")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:scheme
	// The following is needed to read the Infrastructure CR
	if err := osconfigv1.Install(scheme); err != nil {
		setupLog.Error(err, "")
		os.Exit(1)
	}
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var imagesJSONFilename string

	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&imagesJSONFilename, "images-json", "/etc/cluster-baremetal-operator/images/images.json",
		"The location of the file containing the images to use for our operands.")
	flag.Parse()

	ctrl.SetLogger(zap.New(func(o *zap.Options) {
		o.Development = true
	}))

	releaseVersion := os.Getenv("RELEASE_VERSION")
	if releaseVersion == "" {
		ctrl.Log.Info("Environment variable RELEASE_VERSION not provided")
	}

	config := ctrl.GetConfigOrDie()
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     enableLeaderElection,
		Port:               9443,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	osClient := osclientset.NewForConfigOrDie(rest.AddUserAgent(config, controllers.ComponentName))
	recorder := record.NewBroadcaster().NewRecorder(clientgoscheme.Scheme, v1.EventSource{Component: controllers.ComponentName})
	kubeClient := kubernetes.NewForConfigOrDie(rest.AddUserAgent(config, controllers.ComponentName))

	// Check the Platform Type to determine the state of the CO
	enabled, err := controllers.IsEnabled(mgr.GetClient())
	if err != nil {
		setupLog.Error(err, "unable to determine Infrastructure Platform type")
		os.Exit(1)
	}
	if !enabled {
		//Set ClusterOperator status to disabled=true, available=true
		err = controllers.SetCOInDisabledState(osClient, releaseVersion)
		if err != nil {
			setupLog.Error(err, "unable to set Baremetal CO to disabled")
			os.Exit(1)
		}
	}

	if err = (&controllers.ProvisioningReconciler{
		Client:         mgr.GetClient(),
		Log:            ctrl.Log.WithName("controllers").WithName("Provisioning"),
		Scheme:         mgr.GetScheme(),
		OSClient:       osClient,
		EventRecorder:  recorder,
		KubeClient:     kubeClient,
		ReleaseVersion: releaseVersion,
		ImagesFilename: imagesJSONFilename,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Provisioning")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
