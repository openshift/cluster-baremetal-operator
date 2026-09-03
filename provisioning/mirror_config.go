package provisioning

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"

	osconfigv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
)

const mirrorConfigHashAnnotation = "machine-os-images-mirror-config/hash"

const mirrorConfigName = "machine-os-images-mirror-config"

var mirrorConfigVolumeMount = corev1.VolumeMount{
	Name:      mirrorConfigName,
	MountPath: "/etc/image-mirrors",
	ReadOnly:  true,
}

func mirrorConfigVolume() corev1.Volume {
	return corev1.Volume{
		Name: mirrorConfigName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: mirrorConfigName},
				Items:                []corev1.KeyToPath{{Key: "idms.yaml", Path: "idms.yaml"}},
				Optional:             ptr.To(true),
			},
		},
	}
}

// EnsureMirrorConfig lists all cluster ImageDigestMirrorSet resources,
// serializes them into a ConfigMap, and applies it so that the
// machine-os-images init container can pass --idms-file to oc image extract
// in disconnected environments.
func EnsureMirrorConfig(info *ProvisioningInfo) (bool, error) {
	ctx := context.Background()

	idmsList, err := info.OSClient.ConfigV1().ImageDigestMirrorSets().List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to list ImageDigestMirrorSets: %w", err)
	}

	if len(idmsList.Items) == 0 {
		info.MirrorConfigHash = ""
		err := client.IgnoreNotFound(
			info.Client.CoreV1().ConfigMaps(info.Namespace).Delete(ctx, mirrorConfigName, metav1.DeleteOptions{}),
		)
		if err != nil {
			return false, fmt.Errorf("failed to delete mirror config ConfigMap: %w", err)
		}
		return false, nil
	}

	stripVolatileMetadata(idmsList)

	idmsYAML, err := marshalIDMSMultiDoc(idmsList.Items)
	if err != nil {
		return false, fmt.Errorf("failed to serialize ImageDigestMirrorSets: %w", err)
	}

	info.MirrorConfigHash = fmt.Sprintf("%x", sha256.Sum256(idmsYAML))

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mirrorConfigName,
			Namespace: info.Namespace,
			Labels: map[string]string{
				"k8s-app":    metal3AppName,
				cboLabelName: imageCustomizationService,
			},
		},
		Data: map[string]string{
			"idms.yaml": string(idmsYAML),
		},
	}

	if err := controllerutil.SetControllerReference(info.ProvConfig, configMap, info.Scheme); err != nil {
		return false, fmt.Errorf("unable to set controllerReference on mirror config ConfigMap: %w", err)
	}

	_, updated, err := resourceapply.ApplyConfigMap(ctx, info.Client.CoreV1(), info.EventRecorder, configMap)
	if err != nil {
		return false, fmt.Errorf("failed to apply mirror config ConfigMap: %w", err)
	}
	return updated, nil
}

// marshalIDMSMultiDoc serializes each ImageDigestMirrorSet as an individual
// YAML document separated by "---". oc image extract --idms-file expects
// top-level ImageDigestMirrorSet objects, not a Kubernetes List wrapper.
func marshalIDMSMultiDoc(items []osconfigv1.ImageDigestMirrorSet) ([]byte, error) {
	var buf bytes.Buffer
	for i := range items {
		if i > 0 {
			buf.WriteString("---\n")
		}
		items[i].APIVersion = "config.openshift.io/v1"
		items[i].Kind = "ImageDigestMirrorSet"
		doc, err := yaml.Marshal(&items[i])
		if err != nil {
			return nil, fmt.Errorf("failed to marshal ImageDigestMirrorSet %q: %w", items[i].Name, err)
		}
		buf.Write(doc)
	}
	return buf.Bytes(), nil
}

// stripVolatileMetadata removes server-set metadata fields (resourceVersion,
// uid, generation, creationTimestamp, managedFields) from every item in the
// list so that the serialized YAML is stable across API reads. Without this,
// resourceVersion changes on every ConfigMap write and causes an infinite
// reconciliation loop.
func stripVolatileMetadata(list *osconfigv1.ImageDigestMirrorSetList) {
	list.ResourceVersion = ""
	for i := range list.Items {
		list.Items[i].ResourceVersion = ""
		list.Items[i].UID = ""
		list.Items[i].Generation = 0
		list.Items[i].CreationTimestamp = metav1.Time{}
		list.Items[i].ManagedFields = nil
	}
}
