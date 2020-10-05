package provisioning

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakekube "k8s.io/client-go/kubernetes/fake"
	faketesting "k8s.io/client-go/testing"
)

const testNamespace = "test-namespce"

func TestGenerateRandomPassword(t *testing.T) {
	pwd1, err := generateRandomPassword()
	if err != nil {
		t.Errorf("Unexpected error while generating random password: %s", err)
	}
	if pwd1 == "" {
		t.Errorf("Expected a valid string but got null")
	}
	pwd2, err := generateRandomPassword()
	if err != nil {
		t.Errorf("Unexpected error while re-generating random password: %s", err)
	} else {
		assert.False(t, pwd1 == pwd2, "regenerated random password should not match pervious one")
	}
}

func TestCreateMariadbPasswordSecret(t *testing.T) {

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
				err := CreateMariadbPasswordSecret(kubeClient.CoreV1(), testNamespace)
				assert.Equal(t, tc.expectedError, err)

				if tc.expectedError == nil {
					secret, _ := kubeClient.Tracker().Get(secretsResource, testNamespace, "metal3-mariadb-password")
					assert.NotEmpty(t, secret.(*v1.Secret).StringData[baremetalSecretKey])
					// Test for making sure that when a secret already exists, a new one is not
					// created and the old one returned.
					if tc.testRecreate {
						original := secret.(*v1.Secret).StringData[baremetalSecretKey]
						err := CreateMariadbPasswordSecret(kubeClient.CoreV1(), testNamespace)
						assert.Equal(t, tc.expectedError, err)
						newSecret, _ := kubeClient.Tracker().Get(secretsResource, testNamespace, "metal3-mariadb-password")
						recreated := newSecret.(*v1.Secret).StringData[baremetalSecretKey]
						assert.True(t, strings.Compare(original, recreated) == 0, "re-created mariadb password is invalid")
					}
				}
			case "new-ironic-secret":
				err := CreateIronicPasswordSecret(kubeClient.CoreV1(), testNamespace)
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
				err := CreateInspectorPasswordSecret(kubeClient.CoreV1(), testNamespace)
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

			}
		})
	}
}
