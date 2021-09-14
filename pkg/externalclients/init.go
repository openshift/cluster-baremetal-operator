package externalclients

import (
	"context"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	osconfigv1 "github.com/openshift/api/config/v1"
	osclientset "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/library-go/pkg/operator/events"
)

type externalResourceClient struct {
	kubeClient kubernetes.Interface
	osClient   osclientset.Interface
}

// ExternalResourceClient provides access to resources outside of the namespace
// watched by controller-runtime client.
type ExternalResourceClient interface {
	ReadSSHKey(ctx context.Context) (string, error)
	OpenshiftConfigSecret(ctx context.Context) ([]byte, error)
	ClusterInfrastructure(ctx context.Context) (*osconfigv1.Infrastructure, error)
	ClusterProxy(ctx context.Context) (*osconfigv1.Proxy, error)
	ClusterOperatorCreate(ctx context.Context, co *osconfigv1.ClusterOperator) (*osconfigv1.ClusterOperator, error)
	ClusterOperatorGet(ctx context.Context, name string) (*osconfigv1.ClusterOperator, error)
	ClusterOperatorStatusUpdate(ctx context.Context, co *osconfigv1.ClusterOperator) error
	WebhookEnable(mgr manager.Manager, namespace string, eventRecorder events.Recorder) error
	WebhookDependenciesReady(ctx context.Context) bool
}

var _ ExternalResourceClient = &externalResourceClient{}

func NewExternalResourceClient(kubeClient kubernetes.Interface, osClient osclientset.Interface) ExternalResourceClient {
	return &externalResourceClient{
		kubeClient: kubeClient,
		osClient:   osClient,
	}
}
