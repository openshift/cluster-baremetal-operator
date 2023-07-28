package provisioning

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
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

const oldPullSecret = `{"auths":{"registry.example.com:5000":{"auth":"dXNlcm5hbWU6cGFzc3dvcmQK"}}}`      //nolint:gosec
const newPullSecret = `{"auths":{"registry2.example.com:5000":{"auth":"dXNlcm5hbWUyOnBhc3N3b3JkMgo="}}}` //nolint:gosec

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

func TestCreatePasswordSecret(t *testing.T) {
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
			name:          "new-ironic-secret",
			expectedError: nil,
		},
		{
			name:          "new-inspector-secret",
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

func TestRegistryPullSecret(t *testing.T) {
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
		name               string
		secrets            []*corev1.Secret
		expectedPullSecret string
		expectUpdate       bool
	}{
		{
			name: "Create new machine API secret",
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      PullSecretName,
						Namespace: OpenshiftConfigNamespace,
					},
					StringData: map[string]string{
						openshiftConfigSecretKey: oldPullSecret,
					},
				},
			},
			expectedPullSecret: oldPullSecret,
			expectUpdate:       false,
		},
		{
			name: "Update machine API secret if the contents are different",
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      PullSecretName,
						Namespace: OpenshiftConfigNamespace,
					},
					StringData: map[string]string{
						openshiftConfigSecretKey: newPullSecret,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      PullSecretName,
						Namespace: testNamespace,
					},
					StringData: map[string]string{
						openshiftConfigSecretKey: base64.StdEncoding.EncodeToString([]byte(oldPullSecret)),
					},
				},
			},
			expectedPullSecret: newPullSecret,
			expectUpdate:       true,
		},
		{
			name: "Do not update machine API secret if the contents are the same",
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      PullSecretName,
						Namespace: OpenshiftConfigNamespace,
					},
					StringData: map[string]string{
						openshiftConfigSecretKey: newPullSecret,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      PullSecretName,
						Namespace: testNamespace,
					},
					StringData: map[string]string{
						openshiftConfigSecretKey: base64.StdEncoding.EncodeToString([]byte(newPullSecret)),
					},
				},
			},
			expectedPullSecret: newPullSecret,
			expectUpdate:       false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Create the client set. Create a PrependReactor which copies content from StringData to Data, upon create
			// and update. Then, create all secrets.
			kubeClient := fakekube.NewSimpleClientset()
			kubeClient.PrependReactor("create", "secrets", secretDataReactor)
			kubeClient.PrependReactor("update", "secrets", secretDataReactor)
			for _, secret := range tc.secrets {
				_, err := kubeClient.CoreV1().Secrets(secret.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("could not populate clientset with secret %s/%s, err: %q", secret.Namespace, secret.Name, err)
				}
			}

			info := &ProvisioningInfo{
				Client:        kubeClient,
				Namespace:     testNamespace,
				ProvConfig:    baremetalCR,
				Scheme:        scheme,
				EventRecorder: events.NewLoggingEventRecorder("tests"),
			}

			// Overwrite the reportRegistryPullSecretReconcile callback. This allows us to track if applySecret deems
			// that an update to the secret is necessary.
			reconcilerTriggered := false
			reportRegistryPullSecretReconcile = func() {
				reconcilerTriggered = true
			}

			// Run the method under test.
			if err := createRegistryPullSecret(info); err != nil {
				t.Fatalf("createRegistryPullSecret returned an error, err: %q", err)
			}

			// This should be true if the secret in testNamespace exists but is different. It should be false
			// if the secret content (double encoded) is the same, or if the secret must be created.
			if tc.expectUpdate != reconcilerTriggered {
				t.Fatalf("expected an update: %t, but reconcilerTriggered returns: %t",
					tc.expectUpdate, reconcilerTriggered)
			}

			// Get the generated / update pull secret from testNamespace.
			s, err := kubeClient.CoreV1().Secrets(testNamespace).Get(context.TODO(), PullSecretName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("could not get secret %s/%s, err: %q", testNamespace, PullSecretName, err)
			}

			// Compare the generated to the expected secret.
			// Remember that the expect secret is double encoded due to PR
			// https://github.com/openshift/cluster-baremetal-operator/pull/184
			generatedPullSecret := string(s.Data[openshiftConfigSecretKey])
			doubleEncodedExpectedPullSecret := base64.StdEncoding.EncodeToString(
				[]byte(base64.StdEncoding.EncodeToString([]byte(tc.expectedPullSecret))))
			if generatedPullSecret != doubleEncodedExpectedPullSecret {
				t.Fatalf("expected generated pull-secret %q to match %q", generatedPullSecret, doubleEncodedExpectedPullSecret)
			}
		})
	}
}

// secretDataReactor copies the base64 encoded contents of a secret's StringData to the Data field, upon create and
// update actions.
func secretDataReactor(action faketesting.Action) (bool, runtime.Object, error) {
	switch a := action.(type) {
	case faketesting.CreateAction:
		secret, ok := a.GetObject().(*corev1.Secret)
		if !ok {
			return false, nil, fmt.Errorf("unsupported object type %T", a.GetObject())
		}
		secretStringDataToData(secret)
	case faketesting.UpdateAction: //nolint: staticcheck
		secret, ok := a.GetObject().(*corev1.Secret)
		if !ok {
			return false, nil, fmt.Errorf("unsupported object type %T", a.GetObject())
		}
		secretStringDataToData(secret)
	default:
		return false, nil, fmt.Errorf("unsupported action %T", a)
	}
	return false, nil, nil
}

// secretStringDataToData takes a secret and copies the base64 encoded contents of StringData to Data. After encoding,
// it erases the content from StringData, as StringData is a write only field.
func secretStringDataToData(secret *corev1.Secret) {
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	for k, v := range secret.StringData {
		secret.Data[k] = []byte(base64.StdEncoding.EncodeToString([]byte(v)))
		delete(secret.StringData, k)
	}
}
