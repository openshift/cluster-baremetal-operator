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

	admissionregistration "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	osconfigv1 "github.com/openshift/api/config/v1"
	osclientset "github.com/openshift/client-go/config/clientset/versioned"
	configinformers "github.com/openshift/client-go/config/informers/externalversions"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/apis/metal3.io/v1alpha1"
	"github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
)

const (
	validatingWebhookConfigurationNameOld = "cluster-baremetal-validating-webhook-configuration"
	validatingWebhookConfigurationName    = "provisioning.metal3.io"
	validatingWebhookSecretName           = "cluster-baremetal-webhook-server-cert"
)

type WebhookController struct {
	kubeClient    kubernetes.Interface
	osClient      osclientset.Interface
	eventRecorder events.Recorder

	targetNamespace string
	activated       bool
	deletedOld      bool
}

func (c *WebhookController) filterOutNotUsed(obj interface{}) bool {
	object := obj.(metav1.Object)
	if object.GetNamespace() == c.targetNamespace && object.GetName() == validatingWebhookSecretName {
		// the webhook depends on the server cert
		return false
	}
	if object.GetName() == "service-ca" || object.GetName() == "authentication" {
		// the webhook needs the service-ca operator to be up
		return false
	}
	return true
}

// NewWebhookController watches the dependancies of the webhook and enables
// the webhook when those dependancies are ready.
func NewWebhookController(
	targetNamespace string,
	osClient osclientset.Interface,
	configInformer configinformers.SharedInformerFactory,
	kubeClient kubernetes.Interface,
	kubeInformersForNamespace informers.SharedInformerFactory,
	eventRecorder events.Recorder,
) factory.Controller {
	c := &WebhookController{
		targetNamespace: targetNamespace,
		kubeClient:      kubeClient,
		osClient:        osClient,
		eventRecorder:   eventRecorder,
		activated:       false,
	}

	return factory.New().WithFilteredEventsInformers(c.filterOutNotUsed,
		kubeInformersForNamespace.Core().V1().Secrets().Informer(),
		configInformer.Config().V1().ClusterOperators().Informer(),
	).WithSync(c.sync).ToController("WebhookController", eventRecorder.WithComponentSuffix("WebhookController"))
}

func (c *WebhookController) sync(ctx context.Context, controllerContext factory.SyncContext) error {
	if !c.deletedOld {
		// delete the old webhook
		err := c.kubeClient.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Delete(ctx, validatingWebhookConfigurationNameOld, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		c.deletedOld = true
	}
	if !c.activated && c.dependenciesReady(ctx) {
		// if we are ready, apply the webhook configuration so we start validating.
		err := c.apply()
		if err != nil {
			return fmt.Errorf("unable to enable ValidatingWebhook %w", err)
		}
		klog.Info("enabled validating webhook")
		c.activated = true
	}
	return nil
}

func (c *WebhookController) dependenciesReady(ctx context.Context) bool {
	for _, operator := range []string{"service-ca", "authentication"} {
		co, err := c.osClient.ConfigV1().ClusterOperators().Get(ctx, operator, metav1.GetOptions{})
		if err != nil {
			return false
		}

		for condName, condVal := range map[osconfigv1.ClusterStatusConditionType]osconfigv1.ConditionStatus{
			osconfigv1.OperatorDegraded:    osconfigv1.ConditionFalse,
			osconfigv1.OperatorProgressing: osconfigv1.ConditionFalse,
			osconfigv1.OperatorAvailable:   osconfigv1.ConditionTrue} {
			if !v1helpers.IsStatusConditionPresentAndEqual(co.Status.Conditions, condName, condVal) {
				klog.V(1).InfoS("dependenciesReady", operator, "not ready", condName, condVal)
				return false
			}
		}
	}
	sec, err := c.kubeClient.CoreV1().Secrets(c.targetNamespace).Get(ctx, validatingWebhookSecretName, metav1.GetOptions{})
	if err != nil || len(sec.Data) == 0 {
		klog.V(1).InfoS("dependenciesReady", validatingWebhookSecretName, "not ready", "err", err.Error(), "dataLen", len(sec.Data))
		return false
	}

	klog.Info("dependenciesReady: everything ready for webhooks")
	return true
}

func (c *WebhookController) apply() error {
	fail := admissionregistration.Fail
	noSideEffects := admissionregistration.SideEffectClassNone
	instance := &admissionregistration.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: validatingWebhookConfigurationName,
			Annotations: map[string]string{
				"include.release.openshift.io/self-managed-high-availability": "true",
				"include.release.openshift.io/single-node-developer":          "true",
				"service.beta.openshift.io/inject-cabundle":                   "true",
			},
		},
		Webhooks: []admissionregistration.ValidatingWebhook{
			{
				Name:                    validatingWebhookConfigurationName,
				SideEffects:             &noSideEffects,
				FailurePolicy:           &fail,
				AdmissionReviewVersions: []string{"v1beta1"},
				TimeoutSeconds:          pointer.Int32Ptr(30),
				ClientConfig: admissionregistration.WebhookClientConfig{
					CABundle: []byte("Cg=="),
					Service: &admissionregistration.ServiceReference{
						Name:      "cluster-baremetal-webhook-service",
						Namespace: c.targetNamespace,
						Path:      pointer.StringPtr("/apis/admission.metal3.io/v1alpha1/provisioningvalidators"),
					},
				},
				Rules: []admissionregistration.RuleWithOperations{
					{
						Operations: []admissionregistration.OperationType{
							admissionregistration.Create,
							admissionregistration.Update,
						},
						Rule: admissionregistration.Rule{
							Resources:   []string{metal3iov1alpha1.ProvisioningKindPlural},
							APIGroups:   []string{metal3iov1alpha1.GroupVersion.Group},
							APIVersions: []string{metal3iov1alpha1.GroupVersion.Version},
						},
					},
				},
			},
		},
	}

	// we do not have a baremetalCR (when disabled), so we have no where to store
	// the expectedGeneration, so just fake it.
	expectedGeneration := int64(1)
	_, _, err := resourceapply.ApplyValidatingWebhookConfiguration(c.kubeClient.AdmissionregistrationV1(), c.eventRecorder, instance, expectedGeneration)
	return err
}
