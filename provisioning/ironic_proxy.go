package provisioning

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"

	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
)

const (
	ironicProxyService       = "ironic-proxy"
	ironicPrivatePort        = 6388
	ironicUpstreamIPEnvVar   = "IRONIC_UPSTREAM_IP"
	ironicUpstreamPortEnvVar = "IRONIC_UPSTREAM_PORT"
	ironicProxyPortEnvVar    = "IRONIC_PROXY_PORT"
)

func createContainerIronicProxy(ironicHost string, ironicPort int, images *Images) corev1.Container {
	container := corev1.Container{
		Name:            "ironic-proxy",
		Image:           images.Ironic,
		ImagePullPolicy: corev1.PullIfNotPresent,
		SecurityContext: &corev1.SecurityContext{
			// Set this for ironic-proxy with custom dirs
			Privileged: ptr.To(true),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		},
		Command: []string{"/bin/runironic-proxy"},
		VolumeMounts: []corev1.VolumeMount{
			ironicTlsMount,
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "ironic-proxy",
				ContainerPort: int32(baremetalIronicPort),
				HostPort:      int32(baremetalIronicPort),
			},
		},
		Env: []corev1.EnvVar{
			{
				Name:  ironicProxyPortEnvVar,
				Value: fmt.Sprint(baremetalIronicPort),
			},
			{
				Name:  ironicUpstreamIPEnvVar,
				Value: ironicHost,
			},
			{
				Name:  ironicUpstreamPortEnvVar,
				Value: fmt.Sprint(ironicPort),
			},
			// The provisioning IP is not used except that
			// httpd cannot start until the IP is available on some interface
			{
				Name: provisioningIP,
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "status.hostIP",
					},
				},
			},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("5m"),
				corev1.ResourceMemory: resource.MustParse("50Mi"),
			},
		},
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
	}
	return container
}

func newIronicProxyPodTemplateSpec(info *ProvisioningInfo) *corev1.PodTemplateSpec {
	// Connect to the Ironic service created by IrSO (service name is hardcoded to "*-ironic")
	ironicServiceHost := fmt.Sprintf("%s.%s.svc.cluster.local", ironicServiceName, info.Namespace)

	containers := []corev1.Container{
		createContainerIronicProxy(ironicServiceHost, 443, info.Images),
	}

	tolerations := []corev1.Toleration{
		{
			Key:    "node-role.kubernetes.io/master",
			Effect: corev1.TaintEffectNoSchedule,
		},
		{
			Key:      "CriticalAddonsOnly",
			Operator: corev1.TolerationOpExists,
		},
		{
			Key:               "node.kubernetes.io/not-ready",
			Effect:            corev1.TaintEffectNoExecute,
			Operator:          corev1.TolerationOpExists,
			TolerationSeconds: ptr.To[int64](120),
		},
		{
			Key:               "node.kubernetes.io/unreachable",
			Effect:            corev1.TaintEffectNoExecute,
			Operator:          corev1.TolerationOpExists,
			TolerationSeconds: ptr.To[int64](120),
		},
	}

	return &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: podTemplateAnnotations,
			Labels: map[string]string{
				"k8s-app":    metal3AppName,
				cboLabelName: ironicProxyService,
			},
		},
		Spec: corev1.PodSpec{
			NodeSelector: map[string]string{
				"node-role.kubernetes.io/master": "",
			},
			Volumes: []corev1.Volume{
				{
					Name: ironicTlsVolume,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: tlsSecretName,
						},
					},
				},
				{
					Name: "trusted-ca",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							Items: []corev1.KeyToPath{{Key: "ca-bundle.crt", Path: "tls-ca-bundle.pem"}},
							LocalObjectReference: corev1.LocalObjectReference{
								Name: externalTrustBundleConfigMapName,
							},
							Optional: ptr.To(true),
						},
					},
				},
			},
			Containers:        injectProxyAndCA(containers, info.Proxy),
			HostNetwork:       true,
			DNSPolicy:         corev1.DNSClusterFirstWithHostNet,
			PriorityClassName: "system-node-critical",
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: ptr.To(false),
			},
			ServiceAccountName: "cluster-baremetal-operator",
			Tolerations:        tolerations,
		},
	}
}

func newIronicProxyDaemonSet(info *ProvisioningInfo) *appsv1.DaemonSet {
	template := newIronicProxyPodTemplateSpec(info)
	maxUnavail := intstr.FromString("100%")
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ironicProxyService,
			Namespace: info.Namespace,
			Labels: map[string]string{
				"k8s-app": metal3AppName,
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Template: *template,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"k8s-app":    metal3AppName,
					cboLabelName: ironicProxyService,
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &maxUnavail,
				},
			},
		},
	}
}

func UseIronicProxy(info *ProvisioningInfo) bool {
	// TODO(dtantsur): is it safe to use VirtualMediaViaExternalNetwork here?
	if info.IsHyperShift {
		return false
	} else {
		return info.ProvConfig.Spec.ProvisioningNetwork == metal3iov1alpha1.ProvisioningNetworkDisabled || info.ProvConfig.Spec.VirtualMediaViaExternalNetwork
	}
}

func EnsureIronicProxy(info *ProvisioningInfo) (updated bool, err error) {
	if !UseIronicProxy(info) {
		return
	}

	ironicProxyDaemonSet := newIronicProxyDaemonSet(info)
	expectedGeneration := resourcemerge.ExpectedDaemonSetGeneration(ironicProxyDaemonSet, info.ProvConfig.Status.Generations)

	err = controllerutil.SetControllerReference(info.ProvConfig, ironicProxyDaemonSet, info.Scheme)
	if err != nil {
		err = fmt.Errorf("unable to set controllerReference on daemonset: %w", err)
		return
	}
	daemonSetRolloutStartTime = time.Now()
	daemonSet, updated, err := resourceapply.ApplyDaemonSet(
		context.Background(),
		info.Client.AppsV1(),
		info.EventRecorder,
		ironicProxyDaemonSet, expectedGeneration)
	if err != nil {
		err = fmt.Errorf("unable to apply ironic-proxy daemonset: %w", err)
		return
	}

	resourcemerge.SetDaemonSetGeneration(&info.ProvConfig.Status.Generations, daemonSet)
	return
}

// Provide the current state of ironic-proxy daemonset
func GetIronicProxyState(client appsclientv1.DaemonSetsGetter, targetNamespace string, info *ProvisioningInfo) (appsv1.DaemonSetConditionType, error) {
	if !UseIronicProxy(info) {
		return DaemonSetDisabled, nil
	}

	existing, err := client.DaemonSets(targetNamespace).Get(context.Background(), ironicProxyService, metav1.GetOptions{})
	if err != nil || existing == nil {
		// There were errors accessing the deployment.
		return DaemonSetReplicaFailure, err
	}
	if existing.Status.NumberReady == existing.Status.DesiredNumberScheduled {
		return DaemonSetAvailable, nil
	}
	if daemonSetRolloutTimeout <= time.Since(daemonSetRolloutStartTime) {
		return DaemonSetReplicaFailure, nil
	}
	return DaemonSetProgressing, nil
}

func DeleteIronicProxy(info *ProvisioningInfo) error {
	return client.IgnoreNotFound(info.Client.AppsV1().DaemonSets(info.Namespace).Delete(context.Background(), ironicProxyService, metav1.DeleteOptions{}))
}
