package provisioning

import (
	"context"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"

	"github.com/pkg/errors"
	admissionregistration "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	osclientset "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
)

const (
	validatingWebhookService           = "baremetal-operator-webhook-service"
	validatingWebhookConfigurationName = "baremetal-operator-validating-webhook-configuration"
	validatingWebhookServiceHttpsPort  = 443
	validatingWebhookHttpPortName      = "http"
)

// EnsureBaremetalOperatorWebhook ensures ValidatingWebhook resources are ready to serve.
func EnsureBaremetalOperatorWebhook(info *ProvisioningInfo) (bool, error) {
	webhookService := newBaremetalOperatorWebhookService(info.Namespace)
	_, _, err := resourceapply.ApplyService(info.Client.CoreV1(), info.EventRecorder, webhookService)
	if err != nil {
		err = errors.Wrap(err, "unable to create validatingwebhook service")
		return false, err
	}

	if !info.BaremetalWebhookEnabled {
		return false, nil
	}

	ds, _ := GetDeploymentState(info.Client.AppsV1(), info.Namespace)

	// If Metal3 deployment is not available, we should not
	// create validatingwebhook resources.
	if ds != appsv1.DeploymentAvailable {
		return false, nil
	}

	vw := newBaremetalOperatorWebhook(info.Namespace)
	expectedGeneration := resourcemerge.ExpectedValidatingWebhooksConfiguration(validatingWebhookConfigurationName, info.ProvConfig.Status.Generations)
	validatingWebhook, updated, err := resourceapply.ApplyValidatingWebhookConfiguration(info.Client.AdmissionregistrationV1(), info.EventRecorder, vw, expectedGeneration)
	if err != nil {
		err = errors.Wrap(err, "unable to create validatingwebhook configuration")
		return false, err
	}

	if updated {
		resourcemerge.SetValidatingWebhooksConfigurationGeneration(&info.ProvConfig.Status.Generations, validatingWebhook)
	}

	return updated, nil
}

// BaremetalWebhookDependenciesReady checks dependencies to enable Baremetal
// Operator ValidatingWebhook.
func BaremetalWebhookDependenciesReady(client osclientset.Interface) bool {
	// Service CA operator will inject required certificates.
	// If Service CA is not ready, ValidatingWebhook should not be enabled.
	return serviceCAOperatorReady(client)
}

// DeleteValidatingWebhook deletes ValidatingWebhookConfiguration and
// service resources.
func DeleteValidatingWebhook(c kubernetes.Interface, namespace string) error {
	err := client.IgnoreNotFound(c.CoreV1().Services(namespace).Delete(context.Background(), validatingWebhookService, metav1.DeleteOptions{}))
	if err != nil {
		return err
	}

	return client.IgnoreNotFound(c.AdmissionregistrationV1().
		ValidatingWebhookConfigurations().
		Delete(context.Background(), validatingWebhookConfigurationName, metav1.DeleteOptions{}))
}

func newBaremetalOperatorWebhookService(targetNamespace string) *corev1.Service {
	webhookPort, _ := strconv.ParseInt(baremetalWebhookPort, 10, 32) // #nosec
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      validatingWebhookService,
			Namespace: targetNamespace,
			Annotations: map[string]string{
				"include.release.openshift.io/self-managed-high-availability": "true",
				"include.release.openshift.io/single-node-developer":          "true",
				"service.beta.openshift.io/serving-cert-secret-name":          baremetalWebhookSecretName,
			},
			Labels: map[string]string{
				"k8s-app":                 metal3AppName,
				baremetalWebhookLabelName: baremetalWebhookServiceLabel,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"k8s-app":                 metal3AppName,
				baremetalWebhookLabelName: baremetalWebhookServiceLabel,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       validatingWebhookHttpPortName,
					Port:       validatingWebhookServiceHttpsPort,
					TargetPort: intstr.FromInt(int(webhookPort)),
				},
			},
		},
	}
}

func newBaremetalOperatorWebhook(targetNamespace string) *admissionregistration.ValidatingWebhookConfiguration {
	failurePolicy := admissionregistration.Fail
	sideEffect := admissionregistration.SideEffectClassNone
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
				ClientConfig: admissionregistration.WebhookClientConfig{
					CABundle: []byte("Cg=="),
					Service: &admissionregistration.ServiceReference{
						Name:      validatingWebhookService,
						Namespace: targetNamespace,
						Path:      pointer.StringPtr("/validate-metal3-io-v1alpha1-baremetalhost"),
					},
				},
				SideEffects:             &sideEffect,
				FailurePolicy:           &failurePolicy,
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				Name:                    "baremetalhost.metal3.io",
				Rules: []admissionregistration.RuleWithOperations{
					{
						Operations: []admissionregistration.OperationType{
							admissionregistration.Create,
							admissionregistration.Update,
						},
						Rule: admissionregistration.Rule{
							Resources:   []string{"baremetalhosts"},
							APIGroups:   []string{"metal3.io"},
							APIVersions: []string{"v1alpha1"},
						},
					},
				},
			},
		},
	}

	return instance
}
