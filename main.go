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
	goflag "flag"
	"fmt"
	"os"
	"time"

	"github.com/spf13/pflag"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	utilflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/version"
	"k8s.io/klog/v2"

	// +kubebuilder:scaffold:imports

	osconfigv1 "github.com/openshift/api/config/v1"
	osclientset "github.com/openshift/client-go/config/clientset/versioned"
	configv1informers "github.com/openshift/client-go/config/informers/externalversions"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/apis/metal3.io/v1alpha1"
	metal3externalinformers "github.com/openshift/cluster-baremetal-operator/client/informers/externalversions"
	metal3ioClient "github.com/openshift/cluster-baremetal-operator/client/versioned"
	"github.com/openshift/cluster-baremetal-operator/controllers"
	"github.com/openshift/cluster-baremetal-operator/provisioning"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/operator/events"
)

var (
	imagesJSONFilename string
	metricsAddr        string
)

func init() {
	utilruntime.Must(metal3iov1alpha1.AddToScheme(clientgoscheme.Scheme))
	utilruntime.Must(osconfigv1.AddToScheme(clientgoscheme.Scheme))
}

func run(ctx context.Context, controllerContext *controllercmd.ControllerContext) error {
	ns := os.Getenv("COMPONENT_NAMESPACE")
	if ns == "" {
		if controllerContext.ComponentConfig != nil {
			ns = controllerContext.ComponentConfig.GetNamespace()
		}
		if ns == "" {
			ns = controllers.ComponentNamespace
		}
	}

	client, err := metal3ioClient.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}
	metal3Informers := metal3externalinformers.NewSharedInformerFactory(client, 10*time.Minute)

	kubeClient, err := kubernetes.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}
	kubeInformersForNamespace := informers.NewSharedInformerFactoryWithOptions(kubeClient, 10*time.Minute, informers.WithNamespace(ns))

	osClient, err := osclientset.NewForConfig(rest.AddUserAgent(controllerContext.KubeConfig, "config-shared-informer"))
	if err != nil {
		return err
	}
	configInformers := configv1informers.NewSharedInformerFactory(osClient, 10*time.Minute)

	releaseVersion := os.Getenv("RELEASE_VERSION")
	if releaseVersion == "" {
		klog.Info("Environment variable RELEASE_VERSION not provided")
	}

	enableWebhook := false // TODO provisioning.WebhookDependenciesReady(osClient)

	provisioningController := controllers.NewProvisioningController(
		client,
		kubeClient,
		osClient,
		kubeInformersForNamespace,
		metal3Informers,
		configInformers,
		controllerContext.EventRecorder,
		releaseVersion,
		imagesJSONFilename,
		enableWebhook,
	)

	metal3Informers.Start(ctx.Done())
	kubeInformersForNamespace.Start(ctx.Done())
	configInformers.Start(ctx.Done())

	if enableWebhook {
		info := &provisioning.ProvisioningInfo{
			Client:        kubeClient,
			EventRecorder: events.NewLoggingEventRecorder(controllers.ComponentName),
			Namespace:     controllers.ComponentNamespace,
		}
		if err = provisioning.EnableValidatingWebhook(info); err != nil {
			klog.ErrorS(err, "problem enabling validating webhook")
			os.Exit(1)
		}
	}
	go provisioningController.Run(ctx, 1)

	<-ctx.Done()
	return nil
}

func main() {
	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	ccc := controllercmd.NewControllerCommandConfig("cluster-baremetal-operator", version.Get(), run)
	ccc.DisableLeaderElection = true

	cmd := ccc.NewCommand()
	cmd.Use = "cluster-baremetal-operator"
	cmd.Short = "Start the Cluster Baremetal Operator"
	cmd.Flags().StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	cmd.Flags().StringVar(&imagesJSONFilename, "images-json", "/etc/cluster-baremetal-operator/images/images.json",
		"The location of the file containing the images to use for our operands.")
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
