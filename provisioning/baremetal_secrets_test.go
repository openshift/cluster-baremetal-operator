package provisioning

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekube "k8s.io/client-go/kubernetes/fake"
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

// Testing the case where the Mariadb password already exists
// First we create a Mariadb Password using the method being tested.
// Then we attenpt to create it again by calling the method again.
// Instead of creating a new one, it should return the pre-existing secret
func TestCreateMariadbPasswordSecret(t *testing.T) {
	kubeClient := fakekube.NewSimpleClientset(nil...)
	client := kubeClient.CoreV1()

	// First create a mariadb password secret
	if err := CreateMariadbPasswordSecret(kubeClient.CoreV1(), testNamespace); err != nil {
		t.Fatalf("Failed to create first Mariadb password. %s ", err)
	}
	// Read and get Mariadb password from Secret just created.
	oldMariadbPassword, err := client.Secrets(testNamespace).Get(context.Background(), baremetalSecretName, metav1.GetOptions{})
	if err != nil {
		t.Fatal("Failure getting the first Mariadb password that just got created.")
	}
	oldPassword, ok := oldMariadbPassword.StringData[baremetalSecretKey]
	if !ok || oldPassword == "" {
		t.Fatal("Failure reading first Mariadb password from Secret.")
	}

	// The pasword definitely exists. Try creating again.
	if err := CreateMariadbPasswordSecret(kubeClient.CoreV1(), testNamespace); err != nil {
		t.Fatal("Failure creating second Mariadb password.")
	}
	newMariadbPassword, err := client.Secrets(testNamespace).Get(context.Background(), baremetalSecretName, metav1.GetOptions{})
	if err != nil {
		t.Fatal("Failure getting the second Mariadb password.")
	}
	newPassword, ok := newMariadbPassword.StringData[baremetalSecretKey]
	if !ok || newPassword == "" {
		t.Fatal("Failure reading second Mariadb password from Secret.")
	}
	if oldPassword != newPassword {
		t.Fatalf("Both passwords do not match.")
	} else {
		t.Logf("First Mariadb password is being preserved over re-creation as expected.")
	}
}

func TestCreateIronicPasswordSecret(t *testing.T) {
	kubeClient := fakekube.NewSimpleClientset(nil...)
	client := kubeClient.CoreV1()

	err := CreateIronicPasswordSecret(client, testNamespace)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	// Check if Ironic secret exits
	secret, err := client.Secrets(testNamespace).Get(context.Background(), ironicSecretName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		t.Errorf("Error creating Ironic secret.")
	}
	assert.True(t, strings.Compare(secret.StringData[ironicUsernameKey], ironicUsername) == 0, "ironic password created incorrectly")
	return
}

func TestCreateInspectorPasswordSecret(t *testing.T) {
	kubeClient := fakekube.NewSimpleClientset(nil...)
	client := kubeClient.CoreV1()

	err := CreateInspectorPasswordSecret(client, testNamespace)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	// Check if Ironic Inspector secret exits
	secret, err := client.Secrets(testNamespace).Get(context.Background(), inspectorSecretName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		t.Errorf("Error creating Ironic Inspector secret.")
	}
	assert.True(t, strings.Compare(secret.StringData[ironicUsernameKey], inspectorUsername) == 0, "inspector password created incorrectly")
	return
}
