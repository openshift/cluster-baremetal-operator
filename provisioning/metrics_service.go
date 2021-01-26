package provisioning

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
)

const (
	metricsService          = "metal3-metrics"
	metal3ExposeMetricsPort = 8445
)

func newMetal3MetricsService(targetNamespace string, selector *metav1.LabelSelector) *corev1.Service {
	if selector == nil {
		selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"k8s-app": metal3AppName,
			},
		}
	}
	k8sAppLabel := metal3AppName
	for k, v := range selector.MatchLabels {
		if k == "k8s-app" {
			k8sAppLabel = v
			break
		}
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      metricsService,
			Namespace: targetNamespace,
			Labels: map[string]string{
				"k8s-app":    metal3AppName,
				cboLabelName: metricsService,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{
				{
					Name: "metal3-mtrc",
					Port: metal3ExposeMetricsPort,
					TargetPort: intstr.IntOrString{
						Type:   intstr.String,
						StrVal: "metal3-mtrc",
					},
				},
			},
			Selector: map[string]string{
				"k8s-app": k8sAppLabel,
			},
		},
	}
}

func EnsureMetal3MetricsService(info *ProvisioningInfo) (updated bool, err error) {
	metal3MetricsService := newMetal3MetricsService(info.Namespace, info.PodLabelSelector)

	err = controllerutil.SetControllerReference(info.ProvConfig, metal3MetricsService, info.Scheme)
	if err != nil {
		err = fmt.Errorf("unable to set controllerReference on metal3-metrics service: %w", err)
		return
	}

	_, updated, err = resourceapply.ApplyService(info.Client.CoreV1(),
		info.EventRecorder, metal3MetricsService)
	if err != nil {
		err = fmt.Errorf("unable to apply metal3-metrics service: %w", err)
	}
	return
}

func DeleteMetal3MetricsService(info *ProvisioningInfo) error {
	return client.IgnoreNotFound(info.Client.CoreV1().Services(info.Namespace).Delete(context.Background(), metricsService, metav1.DeleteOptions{}))
}
