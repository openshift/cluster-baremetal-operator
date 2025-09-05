package provisioning

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
)

//go:embed prometheusrule.yaml
var prometheusRuleYAML []byte

const (
	stateService             = "metal3-state"
	httpPortName             = "http"
	vmediaHttpsPortName      = "vmedia-https"
	metricsPortName          = "metrics"
	ironicPrometheusRuleName = "metal3-ironic-prometheus-exporter-defaults"
)

func newMetal3StateService(info *ProvisioningInfo) *corev1.Service {
	port, _ := strconv.Atoi(baremetalHttpPort)             // #nosec
	httpsPort, _ := strconv.Atoi(baremetalVmediaHttpsPort) // #nosec
	ironicPort := getControlPlanePort(info)

	ports := []corev1.ServicePort{
		{
			Name: "ironic",
			Port: int32(ironicPort),
		},
		{
			Name: httpPortName,
			Port: int32(port),
		},
	}
	if !info.ProvConfig.Spec.DisableVirtualMediaTLS {
		ports = append(ports, corev1.ServicePort{
			Name: vmediaHttpsPortName,
			Port: int32(httpsPort),
		})
	}
	if info.ProvConfig.Spec.PrometheusExporter != nil && info.ProvConfig.Spec.PrometheusExporter.Enabled {
		ports = append(ports, corev1.ServicePort{
			Name: metricsPortName,
			Port: int32(baremetalMetricsPort),
		})
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      stateService,
			Namespace: info.Namespace,
			Labels: map[string]string{
				cboLabelName: stateService,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				cboLabelName: stateService,
			},
			Ports: ports,
		},
	}
}

func EnsureMetal3StateService(info *ProvisioningInfo) (updated bool, err error) {
	metal3StateService := newMetal3StateService(info)

	err = controllerutil.SetControllerReference(info.ProvConfig, metal3StateService, info.Scheme)
	if err != nil {
		err = fmt.Errorf("unable to set controllerReference on service: %w", err)
		return
	}

	_, updated, err = resourceapply.ApplyService(context.Background(),
		info.Client.CoreV1(), info.EventRecorder, metal3StateService)
	if err != nil {
		err = fmt.Errorf("unable to apply Metal3-state service: %w", err)
	}
	return
}

func DeleteMetal3StateService(info *ProvisioningInfo) error {
	return client.IgnoreNotFound(info.Client.CoreV1().Services(info.Namespace).Delete(context.Background(), stateService, metav1.DeleteOptions{}))
}

// NewIronicServiceMonitor creates a ServiceMonitor for Ironic metrics
func NewIronicServiceMonitor(namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "monitoring.coreos.com/v1",
			"kind":       "ServiceMonitor",
			"metadata": map[string]interface{}{
				"name":      ironicPrometheusExporterName,
				"namespace": namespace,
				"labels": map[string]interface{}{
					cboLabelName: stateService,
				},
			},
			"spec": map[string]interface{}{
				"endpoints": []interface{}{
					map[string]interface{}{
						"port": metricsPortName,
					},
				},
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						cboLabelName: stateService,
					},
				},
			},
		},
	}
}

// EnsureIronicServiceMonitor ensures the ServiceMonitor exists when sensor metrics are enabled
func EnsureIronicServiceMonitor(info *ProvisioningInfo) (bool, error) {
	ctx := context.Background()

	// If metrics are disabled, ensure ServiceMonitor is deleted
	if info.ProvConfig.Spec.PrometheusExporter == nil || !info.ProvConfig.Spec.PrometheusExporter.Enabled {
		return false, DeleteIronicServiceMonitor(info)
	}

	serviceMonitor := NewIronicServiceMonitor(info.Namespace)
	if err := controllerutil.SetControllerReference(info.ProvConfig, serviceMonitor, info.Scheme); err != nil {
		return false, fmt.Errorf("unable to set controllerReference on ServiceMonitor: %w", err)
	}

	_, updated, err := resourceapply.ApplyServiceMonitor(ctx, info.DynamicClient, info.EventRecorder, serviceMonitor)
	if err != nil {
		return false, fmt.Errorf("failed to apply ServiceMonitor: %w", err)
	}

	return updated, nil
}

// DeleteIronicServiceMonitor deletes the ServiceMonitor
func DeleteIronicServiceMonitor(info *ProvisioningInfo) error {
	serviceMonitor := NewIronicServiceMonitor(info.Namespace)
	_, _, err := resourceapply.DeleteServiceMonitor(context.Background(), info.DynamicClient, info.EventRecorder, serviceMonitor)
	return err
}

// NewIronicPrometheusRule creates a PrometheusRule for hardware health alerts
func NewIronicPrometheusRule(namespace string) (*unstructured.Unstructured, error) {
	prometheusRule := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(prometheusRuleYAML, prometheusRule); err != nil {
		return nil, fmt.Errorf("failed to unmarshal PrometheusRule YAML: %w", err)
	}

	// Ensure all required Kubernetes metadata is properly set
	prometheusRule.SetAPIVersion("monitoring.coreos.com/v1")
	prometheusRule.SetKind("PrometheusRule")
	prometheusRule.SetNamespace(namespace)

	// Ensure name is set (should come from YAML but make it explicit)
	if prometheusRule.GetName() == "" {
		prometheusRule.SetName(ironicPrometheusRuleName)
	}

	// Add CBO label for consistent labeling
	labels := prometheusRule.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[cboLabelName] = stateService
	prometheusRule.SetLabels(labels)

	return prometheusRule, nil
}

// EnsureIronicPrometheusRule ensures the PrometheusRule exists when sensor metrics and default rules are enabled
func EnsureIronicPrometheusRule(info *ProvisioningInfo) (bool, error) {
	ctx := context.Background()

	// If metrics are disabled or default rules are disabled, ensure PrometheusRule is deleted
	if info.ProvConfig.Spec.PrometheusExporter == nil ||
		!info.ProvConfig.Spec.PrometheusExporter.Enabled ||
		!info.ProvConfig.Spec.PrometheusExporter.IncludeDefaultPromRules {
		return false, DeleteIronicPrometheusRule(info)
	}

	prometheusRule, err := NewIronicPrometheusRule(info.Namespace)
	if err != nil {
		return false, fmt.Errorf("failed to create PrometheusRule: %w", err)
	}

	if err := controllerutil.SetControllerReference(info.ProvConfig, prometheusRule, info.Scheme); err != nil {
		return false, fmt.Errorf("unable to set controllerReference on PrometheusRule: %w", err)
	}

	// Apply or Update
	_, updated, err := resourceapply.ApplyPrometheusRule(ctx, info.DynamicClient, info.EventRecorder, prometheusRule)
	if err != nil {
		return false, fmt.Errorf("failed to apply PrometheusRule: %w", err)
	}

	return updated, nil
}

// DeleteIronicPrometheusRule deletes the PrometheusRule
func DeleteIronicPrometheusRule(info *ProvisioningInfo) error {
	prometheusRule, err := NewIronicPrometheusRule(info.Namespace)
	if err != nil {
		return fmt.Errorf("failed to create PrometheusRule for deletion: %w", err)
	}
	_, _, err = resourceapply.DeletePrometheusRule(context.Background(), info.DynamicClient, info.EventRecorder, prometheusRule)
	return err
}
