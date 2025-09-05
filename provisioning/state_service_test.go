package provisioning

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNewIronicPrometheusRule(t *testing.T) {
	testNamespace := "openshift-machine-api"

	rule, err := NewIronicPrometheusRule(testNamespace)
	require.NoError(t, err)

	// Basic metadata validation
	assert.Equal(t, "monitoring.coreos.com/v1", rule.GetAPIVersion())
	assert.Equal(t, "PrometheusRule", rule.GetKind())
	assert.Equal(t, "metal3-ironic-prometheus-exporter-defaults", rule.GetName())
	assert.Equal(t, testNamespace, rule.GetNamespace())

	// Structure validation
	spec, found, err := unstructured.NestedSlice(rule.Object, "spec", "groups")
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, spec, 2)

	// Rule count validation - health group
	healthGroup := spec[0].(map[string]interface{})
	healthRules, found, err := unstructured.NestedSlice(healthGroup, "rules")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 8, len(healthRules))

	// Rule count validation - monitoring group
	monitoringGroup := spec[1].(map[string]interface{})
	monitoringRules, found, err := unstructured.NestedSlice(monitoringGroup, "rules")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 1, len(monitoringRules))
}
