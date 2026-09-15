package provisioning

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekube "k8s.io/client-go/kubernetes/fake"

	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
)

func TestOperandServiceAccounts(t *testing.T) {
	info := &ProvisioningInfo{
		Namespace:  "openshift-machine-api",
		Images:     &Images{},
		ProvConfig: &metal3iov1alpha1.Provisioning{Spec: *managedProvisioning().build()},
		Client: fakekube.NewSimpleClientset(&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "openshift-machine-api",
				Labels: map[string]string{
					"k8s-app":    metal3AppName,
					cboLabelName: stateService,
				},
			},
			Status: corev1.PodStatus{
				PodIPs: []corev1.PodIP{{IP: "192.0.2.1"}},
			},
		}),
	}

	t.Run("metal3", func(t *testing.T) {
		template := newMetal3PodTemplateSpec(info, &map[string]string{})
		assertTokenlessOperand(t, template, metal3ServiceAccountName, privilegedSCC)
	})

	t.Run("image cache", func(t *testing.T) {
		template, err := newImageCachePodTemplateSpec(info)
		require.NoError(t, err)
		assertTokenlessOperand(t, template, metal3ServiceAccountName, privilegedSCC)
	})

	t.Run("image customization", func(t *testing.T) {
		template := newImageCustomizationPodTemplateSpec(info, &map[string]string{}, []string{"192.0.2.1"})
		assert.Equal(t, imageCustomizationServiceAccountName, template.Spec.ServiceAccountName)
		require.NotNil(t, template.Spec.AutomountServiceAccountToken)
		assert.True(t, *template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, privilegedSCC, template.Annotations[requiredSCCAnnotation])
	})

	t.Run("ironic proxy", func(t *testing.T) {
		template, err := newIronicProxyPodTemplateSpec(info)
		require.NoError(t, err)
		assertTokenlessOperand(t, template, metal3ServiceAccountName, hostNetworkV2SCC)
	})
}

func assertTokenlessOperand(t *testing.T, template *corev1.PodTemplateSpec, serviceAccount, scc string) {
	t.Helper()
	assert.Equal(t, serviceAccount, template.Spec.ServiceAccountName)
	require.NotNil(t, template.Spec.AutomountServiceAccountToken)
	assert.False(t, *template.Spec.AutomountServiceAccountToken)
	assert.Equal(t, scc, template.Annotations[requiredSCCAnnotation])
}
