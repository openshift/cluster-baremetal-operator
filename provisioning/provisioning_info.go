package provisioning

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1 "github.com/openshift/api/config/v1"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
	"github.com/openshift/cluster-baremetal-operator/pkg/network"
	"github.com/openshift/library-go/pkg/operator/events"
)

type ProvisioningInfo struct {
	Client                client.Client
	EventRecorder         events.Recorder
	ProvConfig            *metal3iov1alpha1.Provisioning
	Scheme                *runtime.Scheme
	Namespace             string
	Images                *Images
	Proxy                 *configv1.Proxy
	NetworkStack          network.NetworkStackType
	MasterMacAddresses    []string
	SSHKey                string
	OpenshiftConfigSecret []byte
}
