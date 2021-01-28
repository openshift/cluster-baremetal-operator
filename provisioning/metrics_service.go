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
	// Port where BMO metrics can be found
	defaultMetal3MetricsAddress = ":60000"
	certStoreName               = "cluster-baremetal-operator-tls"
)

// This Service Monitor should be able to discover the metal3-metrics
// NodePort Service. To do that it looks for Services that satisfy
// the matchLabels and the named port specified in the endpoints below.

const metal3ServiceMonitorDefinition = `
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  namespace: openshift-machine-api
  name: metal3-metrics-servicemonitor
  labels:
    cboLabelName: metricsService
  annotations:
    exclude.release.openshift.io/internal-openshift-hosted: "true"
spec:
  namespaceSelector:
    matchNames:
      - openshift-machine-api
  selector:
    matchLabels:
      cboLabelName: metricsService
 endpoints:
  - port: metal3-mtrc
    bearerTokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
    interval: 30s
    scheme: https
    tlsConfig:
      caFile: /etc/prometheus/configmaps/serving-certs-ca-bundle/service-ca.crt
      serverName: cluster-baremetal-operator.openshift-machine-api.svc
`

// This NodePort Service would is created to expose metrics from pods in the
// metal3 deployment. The Selector is used to match Pods that this Service
// represents. Since the metal3 deployment pods could contain the 4.6 or 4.7
// based labels, the Selector should also be constructed that way.
// The Named port in the ServiceMonitor is defined as a ServicePort here.
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
			Name:      "metal3-metrics-service",
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
	return client.IgnoreNotFound(info.Client.CoreV1().Services(info.Namespace).Delete(context.Background(), "metal3-metrics-service", metav1.DeleteOptions{}))
}

func EnsureMetal3ServiceMonitor(info *ProvisioningInfo) (updated bool, err error) {
	metal3ServiceMonitor := []byte(metal3ServiceMonitorDefinition)
	// Do we need to set the controller refernce for the service monitor?
	updated, err = resourceapply.ApplyServiceMonitor(info.DynamicClient,
		info.EventRecorder, metal3ServiceMonitor)
	return
}

func DeleteMetal3ServiceMonitor(info *ProvisioningInfo) error {
	// TODO: Figure how how to delete the ServiceMonitor
	return nil
}
