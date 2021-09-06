package provisioning

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakekube "k8s.io/client-go/kubernetes/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	faketesting "k8s.io/client-go/testing"

	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
	"github.com/openshift/library-go/pkg/operator/events"
)

const testNamespace = "test-namespce"

// Manually generated expired certificate
const expiredTlsCertificate = `
-----BEGIN CERTIFICATE-----
MIIC8DCCAdigAwIBAgIUY+w23TNKzzRkN/VJkWzshKCuGSUwDQYJKoZIhvcNAQEL
BQAwFDESMBAGA1UEAwwJbG9jYWxob3N0MB4XDTEwMTIwOTE3MjczN1oXDTExMDEw
ODE3MjczN1owFDESMBAGA1UEAwwJbG9jYWxob3N0MIIBIjANBgkqhkiG9w0BAQEF
AAOCAQ8AMIIBCgKCAQEAzMYkY+GQzEjV+Yrh3/pH50SDOoltMZadnWHRLPD+1tVM
cw+vJTAFzZsxCF6GpwDdgUEZfuc/+ZlhEPfSM7I0zDJmJekM8ipTtA6YO5eh02pC
jc080MIxjgjehuRDZANRTois0MAoscxgI5klETmhuxfAvyCY2TLZoYWlz1YdqznO
i7ezPBhhyKTwfL+4k73ZweQRYfhkLVtUHomHPPO6nqOl7i4VNAk9U5lVKrPE6ZfH
XFcJrlRBqRBVHbJt4JmQYHzdGEzaxtK5RfD1sDHGwxdTVAuWOgpEPmy0K75XlFUS
GL66AbbS/P5pOj4mLQaPEXxqhnM/m6rw6tk5cQYq9QIDAQABozowODAUBgNVHREE
DTALgglsb2NhbGhvc3QwCwYDVR0PBAQDAgeAMBMGA1UdJQQMMAoGCCsGAQUFBwMB
MA0GCSqGSIb3DQEBCwUAA4IBAQBtgZOY4ijwQmGdd1yAjH1CifoBOWasXPY/xQhe
anpbqiUeI9zuNSsYYko+r1hIcX2Pd9XtshfRaB+bsewPbxPs5vCUin7sNNDoKENz
LlqvKczX8Jm18d7GySJzgFZLPQLiGQselVZsqkXVO1ikEW6EXX0JW7o+GUnLhg1+
bfat4QMUFATLxweNkAUqJdp4bQZ+3euCvdl8/gIN9rZ7y/dPkIttZ0PPVb6D/6n+
66eUYqDnLdUxel8eNBgq76tkBJxlpsNNdqR4QZfFXCfZmJ6HRBB4G903762kyCGp
bPnmpVQ+hguo1JNC/lbo5zmZ2mHDA+tUbUhxCYWKq8v/qWLj
-----END CERTIFICATE-----
`

var (
	scheme = runtime.NewScheme()
)

func init() {
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		os.Exit(1)
	}

	if err := metal3iov1alpha1.AddToScheme(scheme); err != nil {
		os.Exit(1)
	}
}

func TestCreateMariadbPasswordSecret(t *testing.T) {
	baremetalCR := &metal3iov1alpha1.Provisioning{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Provisioning",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}

	cases := []struct {
		name          string
		secretError   *errors.StatusError
		expectedError error
		testRecreate  bool
	}{
		{
			name:          "new-mariadb-secret",
			expectedError: nil,
			testRecreate:  true,
		},
		{
			name:          "new-ironic-secret",
			expectedError: nil,
		},
		{
			name:          "new-inspector-secret",
			expectedError: nil,
		},
		{
			name:          "new-ironic-rpc-secret",
			expectedError: nil,
		},
		{
			name:          "error-fetching-secret",
			secretError:   errors.NewServiceUnavailable("an error"),
			expectedError: errors.NewServiceUnavailable("an error"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			secretsResource := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}

			kubeClient := fakekube.NewSimpleClientset(nil...)

			if tc.secretError != nil {
				kubeClient.Fake.PrependReactor("get", "secrets", func(action faketesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, &v1.Secret{}, tc.secretError
				})
			}

			info := &ProvisioningInfo{
				Client:        kubeClient,
				Namespace:     testNamespace,
				ProvConfig:    baremetalCR,
				Scheme:        scheme,
				EventRecorder: events.NewLoggingEventRecorder("tests"),
			}
			switch tc.name {
			case "new-mariadb-secret":
				err := createMariadbPasswordSecret(info)
				assert.Equal(t, tc.expectedError, err)

				if tc.expectedError == nil {
					secret, _ := kubeClient.Tracker().Get(secretsResource, testNamespace, "metal3-mariadb-password")
					assert.NotEmpty(t, secret.(*v1.Secret).Data[baremetalSecretKey])
					// Test for making sure that when a secret already exists, a new one is not
					// created and the old one returned.
					if tc.testRecreate {
						original := secret.(*v1.Secret).Data[baremetalSecretKey]
						err := createMariadbPasswordSecret(info)
						assert.Equal(t, tc.expectedError, err)
						newSecret, _ := kubeClient.Tracker().Get(secretsResource, testNamespace, "metal3-mariadb-password")
						recreated := newSecret.(*v1.Secret).Data[baremetalSecretKey]
						assert.True(t, bytes.Compare(original, recreated) == 0, "re-created mariadb password is invalid")
					}
				}
			case "new-ironic-secret":
				err := createIronicSecret(info, ironicSecretName, ironicUsername, "ironic")
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				// Check if Ironic secret exits
				secret, err := kubeClient.Tracker().Get(secretsResource, testNamespace, ironicSecretName)
				if apierrors.IsNotFound(err) {
					t.Errorf("Error creating Ironic secret.")
				}
				assert.True(t, strings.Compare(string(secret.(*v1.Secret).Data[ironicUsernameKey]), ironicUsername) == 0, "ironic password created incorrectly")
			case "new-inspector-secret":
				err := createIronicSecret(info, inspectorSecretName, inspectorUsername, "inspector")
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				// Check if Ironic secret exits
				secret, err := kubeClient.Tracker().Get(secretsResource, testNamespace, inspectorSecretName)
				if apierrors.IsNotFound(err) {
					t.Errorf("Error creating Ironic secret.")
				}
				assert.True(t, strings.Compare(string(secret.(*v1.Secret).Data[ironicUsernameKey]), inspectorUsername) == 0, "inspector password created incorrectly")
			case "new-ironic-rpc-secret":
				err := createIronicSecret(info, ironicrpcSecretName, ironicrpcUsername, "json_rpc")
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				// Check if Ironic secret exits
				secret, err := kubeClient.Tracker().Get(secretsResource, testNamespace, ironicrpcSecretName)
				if apierrors.IsNotFound(err) {
					t.Errorf("Error creating Ironic secret.")
				}
				assert.True(t, strings.Compare(string(secret.(*v1.Secret).Data[ironicUsernameKey]), ironicrpcUsername) == 0, "rpc password created incorrectly")
			}
		})
	}
}

func TestCreateAndUpdateTlsSecret(t *testing.T) {
	baremetalCR := &metal3iov1alpha1.Provisioning{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Provisioning",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}

	cases := []struct {
		name   string
		expire bool
	}{
		{
			name:   "create-and-update",
			expire: false,
		},
		{
			name:   "update-expired",
			expire: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			secretsResource := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
			kubeClient := fakekube.NewSimpleClientset(nil...)
			info := &ProvisioningInfo{
				Client:        kubeClient,
				Namespace:     testNamespace,
				ProvConfig:    baremetalCR,
				Scheme:        scheme,
				EventRecorder: events.NewLoggingEventRecorder("tests"),
			}

			err := createOrUpdateTlsSecret(info)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Check if TLS secret exits
			secret, err := kubeClient.Tracker().Get(secretsResource, testNamespace, tlsSecretName)
			if apierrors.IsNotFound(err) {
				t.Errorf("Error creating TLS secret.")
			}
			original := secret.(*v1.Secret).Data[corev1.TLSCertKey]
			assert.NotEmpty(t, original)

			if tc.expire {
				// Inject an expired certificate
				secret.(*v1.Secret).Data[corev1.TLSCertKey] = []byte(expiredTlsCertificate)
				err = kubeClient.Tracker().Update(secretsResource, secret, testNamespace)
				if err != nil {
					t.Errorf("unexpected error when faking expirted certificate: %v", err)
					return
				}
				original = []byte(expiredTlsCertificate)
			}

			err = createOrUpdateTlsSecret(info)
			if err != nil {
				t.Errorf("unexpected error when re-creating: %v", err)
				return
			}

			newSecret, err := kubeClient.Tracker().Get(secretsResource, testNamespace, tlsSecretName)
			if apierrors.IsNotFound(err) {
				t.Errorf("Error creating TLS secret.")
			}
			recreated := newSecret.(*v1.Secret).Data[corev1.TLSCertKey]

			// In case of expiration, the certificate must be re-created
			assert.Equal(t, tc.expire, bytes.Compare(original, recreated) != 0, "re-created Tls certificate is invalid")
		})
	}
}
