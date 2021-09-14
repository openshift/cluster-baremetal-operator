package externalclients

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	openshiftConfigNamespace = "openshift-config"
	pullSecretName           = "pull-secret"
)

func (e *externalResourceClient) OpenshiftConfigSecret(ctx context.Context) ([]byte, error) {
	openshiftConfigSecret, err := e.kubeClient.CoreV1().Secrets(openshiftConfigNamespace).Get(ctx, pullSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return openshiftConfigSecret.Data[corev1.DockerConfigJsonKey], nil
}
