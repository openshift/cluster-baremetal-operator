package provisioning

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"

	"github.com/openshift/library-go/pkg/operator/events"

	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
)

type ProvisioningInfo struct {
	Client        kubernetes.Interface
	EventRecorder events.Recorder
	ProvConfig    *metal3iov1alpha1.Provisioning
	Scheme        *runtime.Scheme
	Namespace     string
	Images        *Images
}
