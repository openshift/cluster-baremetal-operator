package externalclients

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	osconfigv1 "github.com/openshift/api/config/v1"
)

func (e *externalResourceClient) ClusterInfrastructure(ctx context.Context) (*osconfigv1.Infrastructure, error) {
	return e.osClient.ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
}

func (e *externalResourceClient) ClusterProxy(ctx context.Context) (*osconfigv1.Proxy, error) {
	return e.osClient.ConfigV1().Proxies().Get(ctx, "cluster", metav1.GetOptions{})
}

func (e *externalResourceClient) ClusterOperatorCreate(ctx context.Context, co *osconfigv1.ClusterOperator) (*osconfigv1.ClusterOperator, error) {
	return e.osClient.ConfigV1().ClusterOperators().Create(ctx, co, metav1.CreateOptions{})
}

func (e *externalResourceClient) ClusterOperatorGet(ctx context.Context, name string) (*osconfigv1.ClusterOperator, error) {
	return e.osClient.ConfigV1().ClusterOperators().Get(ctx, name, metav1.GetOptions{})
}

func (e *externalResourceClient) ClusterOperatorStatusUpdate(ctx context.Context, co *osconfigv1.ClusterOperator) error {
	_, err := e.osClient.ConfigV1().ClusterOperators().UpdateStatus(ctx, co, metav1.UpdateOptions{})
	return err
}
