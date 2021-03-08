package provisioning

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1 "github.com/openshift/api/config/v1"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
	"github.com/openshift/library-go/pkg/operator/events"
)

type ProvisioningInfo struct {
	Client           kubernetes.Interface
	EventRecorder    events.Recorder
	ProvConfig       *metal3iov1alpha1.Provisioning
	Scheme           *runtime.Scheme
	Namespace        string
	Images           *Images
	PodLabelSelector *metav1.LabelSelector
	Proxy            *configv1.Proxy
}
