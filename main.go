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
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	genericapiserver "k8s.io/apiserver/pkg/server"
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
	"github.com/openshift/cluster-baremetal-operator/webhooks"
	"github.com/openshift/generic-admission-server/pkg/apiserver"
	"github.com/openshift/generic-admission-server/pkg/cmd/server"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/operator/genericoperatorclient"
	"github.com/openshift/library-go/pkg/operator/status"
)

var (
	imagesJSONFilename string
	metricsAddr        string
)

func init() {
	utilruntime.Must(metal3iov1alpha1.AddToScheme(clientgoscheme.Scheme))
	utilruntime.Must(osconfigv1.AddToScheme(clientgoscheme.Scheme))
}

func runOperator(ctx context.Context, controllerContext *controllercmd.ControllerContext) error {
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

	operatorClient, dynamicInformers, err := genericoperatorclient.NewClusterScopedOperatorClientWithConfigName(
		controllerContext.KubeConfig,
		metal3iov1alpha1.GroupVersion.WithResource("provisionings"),
		metal3iov1alpha1.ProvisioningSingletonName)
	if err != nil {
		return err
	}

	versionRecorder := status.NewVersionGetter()
	clusterOperator, err := osClient.ConfigV1().ClusterOperators().Get(ctx, "baremetal", metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	for _, version := range clusterOperator.Status.Versions {
		versionRecorder.SetVersion(version.Name, version.Version)
	}

	releaseVersion := os.Getenv("RELEASE_VERSION")
	if releaseVersion == "" {
		klog.Info("Environment variable RELEASE_VERSION not provided")
	} else {
		versionRecorder.SetVersion("operator", releaseVersion)
	}

	provisioningController := controllers.NewProvisioningController(
		operatorClient,
		client, metal3Informers,
		kubeClient, kubeInformersForNamespace,
		osClient, configInformers,
		controllerContext.EventRecorder,
		imagesJSONFilename,
		ns,
	)

	webhookController := controllers.NewWebhookController(
		ns,
		osClient, configInformers,
		kubeClient, kubeInformersForNamespace,
		controllerContext.EventRecorder,
	)

	isBaremetalPlatform, err := controllers.IsBaremetalPlatform(ctx, osClient)
	if err != nil {
		return err
	}

	metal3Informers.Start(ctx.Done())
	kubeInformersForNamespace.Start(ctx.Done())
	configInformers.Start(ctx.Done())
	dynamicInformers.Start(ctx.Done())

	if isBaremetalPlatform {
		clusterOperatorStatus := status.NewClusterOperatorStatusController(
			controllers.ClusterOperatorName,
			controllers.RelatedObjects(ns),
			osClient.ConfigV1(),
			configInformers.Config().V1().ClusterOperators(),
			operatorClient,
			versionRecorder,
			controllerContext.EventRecorder,
		)
		go clusterOperatorStatus.Run(ctx, 1)
	} else {
		versions := []osconfigv1.OperandVersion{}
		for n, v := range versionRecorder.GetVersions() {
			versions = append(versions, osconfigv1.OperandVersion{Name: n, Version: v})
		}
		if err := controllers.SetClusterOperatorDisabled(ctx, osClient, controllers.RelatedObjects(ns), versions); err != nil {
			return err
		}
	}
	go provisioningController.Run(ctx, 1)
	go webhookController.Run(ctx, 1)

	<-ctx.Done()
	return nil
}

func newCommandStartAdmissionServer(out, errOut io.Writer, admissionHooks ...apiserver.AdmissionHook) *cobra.Command {
	o := server.NewAdmissionServerOptions(out, errOut, admissionHooks...)

	cmd := &cobra.Command{
		Use:   "webhook",
		Short: "Start the Provisioning validation webhook server",
		RunE: func(c *cobra.Command, args []string) error {
			stopCh := genericapiserver.SetupSignalHandler()
			if err := o.Complete(); err != nil {
				return err
			}
			if err := o.Validate(args); err != nil {
				return err
			}
			if err := o.RunAdmissionServer(stopCh); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Long = cmd.Short

	flags := cmd.Flags()
	o.RecommendedOptions.AddFlags(flags)

	return cmd
}

func main() {
	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	rootCmd := &cobra.Command{
		Use:   controllers.ComponentName,
		Short: "OpenShift Cluster Baremetal Operator",
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
			os.Exit(1)
		},
	}

	ccc := controllercmd.NewControllerCommandConfig("cluster-baremetal-operator", version.Get(), runOperator)
	ccc.DisableLeaderElection = true

	opCmd := ccc.NewCommand()
	opCmd.Use = "operator"
	opCmd.Short = "Start the Cluster Baremetal Operator"
	opCmd.Flags().StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	opCmd.Flags().StringVar(&imagesJSONFilename, "images-json", "/etc/cluster-baremetal-operator/images/images.json",
		"The location of the file containing the images to use for our operands.")

	rootCmd.AddCommand(opCmd)

	rootCmd.AddCommand(newCommandStartAdmissionServer(os.Stdout, os.Stderr, &webhooks.ProvisioningValidatingWebHook{}))

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
