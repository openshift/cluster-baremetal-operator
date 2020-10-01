package provisioning

import (
	"testing"

	"golang.org/x/net/context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekube "k8s.io/client-go/kubernetes/fake"
)

const testNamespace = "test-namespce"

func TestGenerateRandomPassword(t *testing.T) {
	pwd, err := generateRandomPassword()
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if pwd == "" {
		t.Errorf("Expected a valid string but got null")
	}
}

//Testing the case where the password already exists
func TestCreateMariadbPasswordSecret(t *testing.T) {
	kubeClient := fakekube.NewSimpleClientset(nil...)
	client := kubeClient.CoreV1()

	// First create a mariadb password secret
	if err := createMariadbPasswordSecret(kubeClient.CoreV1(), testNamespace); err != nil {
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
	if err := createMariadbPasswordSecret(kubeClient.CoreV1(), testNamespace); err != nil {
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

func TestCreateMetal3PasswordSecrets(t *testing.T) {
	kubeClient := fakekube.NewSimpleClientset(nil...)
	client := kubeClient.CoreV1()

	err := CreateMetal3PasswordSecrets(client, testNamespace)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	// Check if Mariadb password exists
	_, err = client.Secrets(testNamespace).Get(context.Background(), baremetalSecretName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		t.Errorf("Error creating Mariadb password.")
	}
	// Check if Ironic secret exits
	_, err = client.Secrets(testNamespace).Get(context.Background(), ironicSecretName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		t.Errorf("Error creating Ironic secret.")
	}
	// Check if Ironic Inspector secret exits
	_, err = client.Secrets(testNamespace).Get(context.Background(), inspectorSecretName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		t.Errorf("Error creating Ironic Inspector secret.")
	}
	return
}
