package provisioning

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	fakekube "k8s.io/client-go/kubernetes/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/clock"

	osconfigv1 "github.com/openshift/api/config/v1"
	fakeconfigclientset "github.com/openshift/client-go/config/clientset/versioned/fake"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
	"github.com/openshift/library-go/pkg/operator/events"
)

var mirrorTestScheme = func() *runtime.Scheme {
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		os.Exit(1)
	}
	if err := metal3iov1alpha1.AddToScheme(s); err != nil {
		os.Exit(1)
	}
	return s
}()

func newTestProvisioningInfo(idmsSets ...osconfigv1.ImageDigestMirrorSet) *ProvisioningInfo {
	objects := make([]runtime.Object, 0, len(idmsSets))
	for i := range idmsSets {
		objects = append(objects, &idmsSets[i])
	}
	return &ProvisioningInfo{
		Client:        fakekube.NewSimpleClientset(),
		EventRecorder: events.NewLoggingEventRecorder("test", clock.RealClock{}),
		OSClient:      fakeconfigclientset.NewSimpleClientset(objects...),
		Namespace:     "openshift-machine-api",
		Scheme:        mirrorTestScheme,
		ProvConfig: &metal3iov1alpha1.Provisioning{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Provisioning",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		},
	}
}

func TestEnsureMirrorConfigNoIDMS(t *testing.T) {
	info := newTestProvisioningInfo()

	updated, err := EnsureMirrorConfig(info)
	require.NoError(t, err)
	assert.False(t, updated)
	assert.Empty(t, info.MirrorConfigHash, "hash should be empty when no IDMS resources exist")

	cms, err := info.Client.CoreV1().ConfigMaps(info.Namespace).List(
		t.Context(), metav1.ListOptions{},
	)
	require.NoError(t, err)
	assert.Empty(t, cms.Items, "no ConfigMap should exist when there are no IDMS resources")
}

func TestEnsureMirrorConfigWithIDMS(t *testing.T) {
	idms := osconfigv1.ImageDigestMirrorSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mirror",
		},
		Spec: osconfigv1.ImageDigestMirrorSetSpec{
			ImageDigestMirrors: []osconfigv1.ImageDigestMirrors{
				{
					Source:  "quay.io/openshift-release-dev/ocp-v4.0-art-dev",
					Mirrors: []osconfigv1.ImageMirror{"mirror.local/openshift/release"},
				},
			},
		},
	}
	info := newTestProvisioningInfo(idms)

	updated, err := EnsureMirrorConfig(info)
	require.NoError(t, err)
	assert.True(t, updated)

	cm, err := info.Client.CoreV1().ConfigMaps(info.Namespace).Get(
		t.Context(), mirrorConfigName, metav1.GetOptions{},
	)
	require.NoError(t, err)

	yamlData, ok := cm.Data["idms.yaml"]
	assert.True(t, ok, "ConfigMap should contain idms.yaml key")
	assert.Contains(t, yamlData, "quay.io/openshift-release-dev/ocp-v4.0-art-dev")
	assert.Contains(t, yamlData, "mirror.local/openshift/release")
	assert.NotEmpty(t, info.MirrorConfigHash, "hash should be set when IDMS resources exist")
	assert.Len(t, info.MirrorConfigHash, 64, "hash should be a SHA-256 hex string")
}

func TestEnsureMirrorConfigIdempotent(t *testing.T) {
	idms := osconfigv1.ImageDigestMirrorSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mirror",
		},
		Spec: osconfigv1.ImageDigestMirrorSetSpec{
			ImageDigestMirrors: []osconfigv1.ImageDigestMirrors{
				{
					Source:  "quay.io/openshift-release-dev/ocp-v4.0-art-dev",
					Mirrors: []osconfigv1.ImageMirror{"mirror.local/openshift/release"},
				},
			},
		},
	}
	info := newTestProvisioningInfo(idms)

	_, err := EnsureMirrorConfig(info)
	require.NoError(t, err)

	updated, err := EnsureMirrorConfig(info)
	require.NoError(t, err)
	assert.False(t, updated, "second call should be a no-op")
}

func TestStripVolatileMetadata(t *testing.T) {
	ts := metav1.Now()
	list := &osconfigv1.ImageDigestMirrorSetList{
		ListMeta: metav1.ListMeta{
			ResourceVersion: "99999",
		},
		Items: []osconfigv1.ImageDigestMirrorSet{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "mirror-a",
					ResourceVersion:   "12345",
					UID:               types.UID("abc-def"),
					Generation:        3,
					CreationTimestamp: ts,
					ManagedFields:     []metav1.ManagedFieldsEntry{{Manager: "test"}},
				},
				Spec: osconfigv1.ImageDigestMirrorSetSpec{
					ImageDigestMirrors: []osconfigv1.ImageDigestMirrors{
						{Source: "quay.io/example"},
					},
				},
			},
		},
	}

	stripVolatileMetadata(list)

	assert.Empty(t, list.ResourceVersion)
	item := list.Items[0]
	assert.Equal(t, "mirror-a", item.Name, "name must be preserved")
	assert.Empty(t, item.ResourceVersion)
	assert.Empty(t, string(item.UID))
	assert.Zero(t, item.Generation)
	assert.True(t, item.CreationTimestamp.IsZero())
	assert.Nil(t, item.ManagedFields)
	assert.Len(t, item.Spec.ImageDigestMirrors, 1, "spec must be preserved")
}

func TestEnsureMirrorConfigStableWithVolatileMetadata(t *testing.T) {
	idms := osconfigv1.ImageDigestMirrorSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-mirror",
			ResourceVersion: "100",
			UID:             types.UID("uid-1"),
			Generation:      1,
		},
		Spec: osconfigv1.ImageDigestMirrorSetSpec{
			ImageDigestMirrors: []osconfigv1.ImageDigestMirrors{
				{
					Source:  "quay.io/openshift-release-dev/ocp-v4.0-art-dev",
					Mirrors: []osconfigv1.ImageMirror{"mirror.local/openshift/release"},
				},
			},
		},
	}
	info := newTestProvisioningInfo(idms)

	updated, err := EnsureMirrorConfig(info)
	require.NoError(t, err)
	assert.True(t, updated, "first call should create the ConfigMap")
	firstHash := info.MirrorConfigHash

	updated, err = EnsureMirrorConfig(info)
	require.NoError(t, err)
	assert.False(t, updated, "second call must be a no-op despite server-set metadata")
	assert.Equal(t, firstHash, info.MirrorConfigHash, "hash must be stable across calls")
}
