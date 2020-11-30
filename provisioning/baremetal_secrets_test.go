package provisioning

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
)

const testNamespace = "test-namespce"

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
			name:          "new-tls-secret",
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

			switch tc.name {
			case "new-mariadb-secret":
				err := createMariadbPasswordSecret(kubeClient.CoreV1(), testNamespace, baremetalCR, scheme)
				assert.Equal(t, tc.expectedError, err)

				if tc.expectedError == nil {
					secret, _ := kubeClient.Tracker().Get(secretsResource, testNamespace, "metal3-mariadb-password")
					assert.NotEmpty(t, secret.(*v1.Secret).StringData[baremetalSecretKey])
					// Test for making sure that when a secret already exists, a new one is not
					// created and the old one returned.
					if tc.testRecreate {
						original := secret.(*v1.Secret).StringData[baremetalSecretKey]
						err := createMariadbPasswordSecret(kubeClient.CoreV1(), testNamespace, baremetalCR, scheme)
						assert.Equal(t, tc.expectedError, err)
						newSecret, _ := kubeClient.Tracker().Get(secretsResource, testNamespace, "metal3-mariadb-password")
						recreated := newSecret.(*v1.Secret).StringData[baremetalSecretKey]
						assert.True(t, strings.Compare(original, recreated) == 0, "re-created mariadb password is invalid")
					}
				}
			case "new-ironic-secret":
				err := createIronicSecret(kubeClient.CoreV1(), testNamespace, ironicSecretName, ironicUsername, "ironic", baremetalCR, scheme)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				// Check if Ironic secret exits
				secret, err := kubeClient.Tracker().Get(secretsResource, testNamespace, ironicSecretName)
				if apierrors.IsNotFound(err) {
					t.Errorf("Error creating Ironic secret.")
				}
				assert.True(t, strings.Compare(secret.(*v1.Secret).StringData[ironicUsernameKey], ironicUsername) == 0, "ironic password created incorrectly")
			case "new-inspector-secret":
				err := createIronicSecret(kubeClient.CoreV1(), testNamespace, inspectorSecretName, inspectorUsername, "inspector", baremetalCR, scheme)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				// Check if Ironic secret exits
				secret, err := kubeClient.Tracker().Get(secretsResource, testNamespace, inspectorSecretName)
				if apierrors.IsNotFound(err) {
					t.Errorf("Error creating Ironic secret.")
				}
				assert.True(t, strings.Compare(secret.(*v1.Secret).StringData[ironicUsernameKey], inspectorUsername) == 0, "inspector password created incorrectly")
			case "new-ironic-rpc-secret":
				err := createIronicSecret(kubeClient.CoreV1(), testNamespace, ironicrpcSecretName, ironicrpcUsername, "json_rpc", baremetalCR, scheme)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				// Check if Ironic secret exits
				secret, err := kubeClient.Tracker().Get(secretsResource, testNamespace, ironicrpcSecretName)
				if apierrors.IsNotFound(err) {
					t.Errorf("Error creating Ironic secret.")
				}
				assert.True(t, strings.Compare(secret.(*v1.Secret).StringData[ironicUsernameKey], ironicrpcUsername) == 0, "rpc password created incorrectly")
			case "new-tls-secret":
				err := CreateTlsSecret(kubeClient.CoreV1(), testNamespace, baremetalCR, scheme)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				// Check if TLS secret exits
				_, err = kubeClient.Tracker().Get(secretsResource, testNamespace, tlsSecretName)
				if apierrors.IsNotFound(err) {
					t.Errorf("Error creating TLS secret.")
				}
			}
		})
	}
}
