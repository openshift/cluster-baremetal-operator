package provisioning

import (
	"fmt"
	"testing"

	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/assert"
)

func TestValidatingWebhookService(t *testing.T) {
	tCases := []struct {
		name     string
		ns       string
		expected []byte
	}{
		{
			name: "valid webhook service",
			ns:   "test-namespace",
			expected: []byte(
				`metadata:
  annotations:
    include.release.openshift.io/self-managed-high-availability: "true"
    include.release.openshift.io/single-node-developer: "true"
    service.beta.openshift.io/serving-cert-secret-name: baremetal-operator-webhook-server-cert
  creationTimestamp: null
  labels:
    baremetal.openshift.io/metal3-validating-webhook: metal3-validating-webhook
    k8s-app: metal3
  name: baremetal-operator-webhook-service
  namespace: test-namespace
spec:
  ports:
  - name: http
    port: 443
    targetPort: 9447
  selector:
    baremetal.openshift.io/metal3-validating-webhook: metal3-validating-webhook
    k8s-app: metal3
  type: ClusterIP
status:
  loadBalancer: {}
`),
		},
	}

	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			actual := newBaremetalOperatorWebhookService(tc.ns)
			a, _ := yaml.Marshal(actual)
			assert.Equal(t, string(tc.expected), string(a), "ValidatingWebhook service is invalid")
		})
	}
}

func TestValidatingWebhookConfiguration(t *testing.T) {
	tCases := []struct {
		name     string
		ns       string
		expected []byte
	}{
		{
			name: "valid webhook configuration",
			ns:   "test-namespace",
			expected: []byte(
				`metadata:
  annotations:
    include.release.openshift.io/self-managed-high-availability: "true"
    include.release.openshift.io/single-node-developer: "true"
    service.beta.openshift.io/inject-cabundle: "true"
  creationTimestamp: null
  name: baremetal-operator-validating-webhook-configuration
webhooks:
- admissionReviewVersions:
  - v1
  - v1beta1
  clientConfig:
    caBundle: Q2c9PQ==
    service:
      name: baremetal-operator-webhook-service
      namespace: test-namespace
      path: /validate-metal3-io-v1alpha1-baremetalhost
  failurePolicy: Fail
  name: baremetalhost.metal3.io
  rules:
  - apiGroups:
    - metal3.io
    apiVersions:
    - v1alpha1
    operations:
    - CREATE
    - UPDATE
    resources:
    - baremetalhosts
  sideEffects: None
- admissionReviewVersions:
  - v1
  - v1beta1
  clientConfig:
    caBundle: Q2c9PQ==
    service:
      name: baremetal-operator-webhook-service
      namespace: test-namespace
      path: /validate-metal3-io-v1alpha1-bmceventsubscription
  failurePolicy: Fail
  name: bmceventsubscription.metal3.io
  rules:
  - apiGroups:
    - metal3.io
    apiVersions:
    - v1alpha1
    operations:
    - CREATE
    - UPDATE
    resources:
    - bmceventsubscriptions
  sideEffects: None
`),
		},
	}

	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			actual := newBaremetalOperatorWebhook(tc.ns)
			a, _ := yaml.Marshal(actual)
			fmt.Printf("Arda %s\n", string(a))
			assert.Equal(t, string(tc.expected), string(a), "ValidatingWebhook configuration is invalid")
		})
	}
}
