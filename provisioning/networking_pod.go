package provisioning

import (
	"context"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"maps"
	"slices"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
)

const (
	ironicNetworkingDeploymentName = "metal3-ironic-networking"
	ironicNetworkingServiceName    = "metal3-ironic-networking-service"
	ironicNetworkingRPCPort        = 6190
	ironicNetworkingRPCPortName    = "networking-rpc"

	ironicForceDhcpEnvVar            = "IRONIC_FORCE_DHCP"
	ironicNetworkingEnabledEnvVar    = "IRONIC_NETWORKING_ENABLED"
	switchConfigsSecretEnvVar        = "IRONIC_SWITCH_CONFIGS_SECRET" // nolint: gosec
	switchConfigsVolume              = "metal3-switch-configs"
	switchConfigsSecretName          = "metal3-switch-configs" // nolint: gosec
	switchConfigsFileName            = "switch-configs.conf"
	switchConfigsFileNameEnvVar      = "IRONIC_NETWORKING_SWITCH_CONFIGS"
	switchConfigsMountPath           = "/etc/ironic/networking/configs"
	switchCredentialsSecretEnvVar    = "IRONIC_SWITCH_CREDENTIALS_SECRET"   // nolint: gosec
	switchCredentialsVolume          = "metal3-switch-credentials"          // nolint: gosec
	switchCredentialsSecretName      = "metal3-switch-credentials"          // nolint: gosec
	switchCredentialsMountPath       = "/etc/ironic/networking/credentials" // nolint: gosec
	switchCredentialsMountPathEnvVar = "IRONIC_SWITCH_CREDENTIALS_PATH"     // nolint: gosec
)

var ironicRPCCredentialsMount = corev1.VolumeMount{
	Name:      ironicCredentialsVolume,
	MountPath: metal3AuthRootDir + "/ironic-rpc",
	ReadOnly:  true,
}

var switchConfigsMount = corev1.VolumeMount{
	Name:      switchConfigsVolume,
	MountPath: switchConfigsMountPath,
	ReadOnly:  true,
}

var switchCredentialsMount = corev1.VolumeMount{
	Name:      switchCredentialsVolume,
	MountPath: switchCredentialsMountPath,
	ReadOnly:  true,
}

// secretVersionAnnotation computes an FNV-64a hash of the secret's data and
// returns it as a pod template annotation. This causes a deployment rollout
// when the secret content changes, since Kubernetes does not automatically
// restart pods when mounted secrets are updated.
func secretVersionAnnotation(ctx context.Context, kubeClient kubernetes.Interface, namespace, secretType, secretName string) (map[string]string, error) {
	secret, err := kubeClient.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to get secret %s/%s: %w", namespace, secretName, err)
	}

	h := fnv.New64a()
	for _, key := range slices.Sorted(maps.Keys(secret.Data)) {
		h.Write([]byte(key))
		h.Write([]byte{0})
		h.Write(secret.Data[key])
		h.Write([]byte{0})
	}
	hash := hex.EncodeToString(h.Sum(nil))
	return map[string]string{
		fmt.Sprintf("baremetal.openshift.io/%s-version", secretType): hash,
	}, nil
}

var ironicNetworkingRolloutStartTime = time.Now()

var ironicNetworkingVolumes = []corev1.Volume{
	{
		Name: ironicConfigVolume,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	},
	{
		Name: ironicDataVolume,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	},
	{
		Name: ironicTmpVolume,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	},
	{
		Name: ironicCredentialsVolume,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: ironicSecretName,
				Items: []corev1.KeyToPath{
					{Key: ironicUsernameKey, Path: ironicUsernameKey},
					{Key: ironicPasswordKey, Path: ironicPasswordKey},
					{Key: ironicHtpasswdKey, Path: ironicHtpasswdKey},
				},
			},
		},
	},
	{
		Name: ironicTlsVolume,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: tlsSecretName,
			},
		},
	},
	{
		Name: switchConfigsVolume,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: switchConfigsSecretName,
			},
		},
	},
	{
		Name: switchCredentialsVolume,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: switchCredentialsSecretName,
			},
		},
	},
	trustedCAVolume(),
}

func createContainerIronicNetworking(images *Images) corev1.Container {
	envVars := []corev1.EnvVar{
		{
			Name:  switchConfigsFileNameEnvVar,
			Value: switchConfigsMountPath + "/" + switchConfigsFileName,
		},
		{
			Name:  "IRONIC_NETWORKING_JSON_RPC_HOST",
			Value: "0.0.0.0",
		},
		{
			Name:  "IRONIC_NETWORKING_JSON_RPC_PORT",
			Value: fmt.Sprintf("%d", ironicNetworkingRPCPort),
		},
		{
			Name:  "IRONIC_NETWORKING_ENABLED_SWITCH_DRIVERS",
			Value: "generic-switch",
		},
	}

	return corev1.Container{
		Name:            "metal3-ironic-networking",
		Image:           images.Ironic,
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			ReadOnlyRootFilesystem: ptr.To(true),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		},
		Command: []string{"/bin/runironic-networking"},
		Ports: []corev1.ContainerPort{
			{
				Name:          ironicNetworkingRPCPortName,
				ContainerPort: ironicNetworkingRPCPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromString(ironicNetworkingRPCPortName),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
			TimeoutSeconds:      3,
		},
		Env: envVars,
		VolumeMounts: []corev1.VolumeMount{
			ironicConfigMount,
			ironicDataMount,
			ironicTmpMount,
			ironicRPCCredentialsMount,
			switchConfigsMount,
			switchCredentialsMount,
			ironicTlsMount,
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("50Mi"),
			},
		},
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
	}
}

func newIronicNetworkingPodTemplateSpec(info *ProvisioningInfo, labels *map[string]string) *corev1.PodTemplateSpec {
	containers := injectProxyAndCA([]corev1.Container{createContainerIronicNetworking(info.Images)}, info.Proxy)

	nodeSelector := map[string]string{}
	if !info.IsHyperShift {
		nodeSelector = map[string]string{"node-role.kubernetes.io/master": ""}
	}

	return &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: podTemplateAnnotations,
			Labels:      *labels,
		},
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: ptr.To(false),
			Volumes:                      ironicNetworkingVolumes,
			Containers:                   containers,
			DNSPolicy:                    corev1.DNSClusterFirst,
			PriorityClassName:            "system-node-critical",
			NodeSelector:                 nodeSelector,
			ServiceAccountName:           "cluster-baremetal-operator",
			Tolerations:                  metal3Tolerations(),
		},
	}
}

func newIronicNetworkingDeployment(info *ProvisioningInfo) *appsv1.Deployment {
	selector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"k8s-app":    metal3AppName,
			cboLabelName: ironicNetworkingDeploymentName,
		},
	}
	podSpecLabels := map[string]string{
		"k8s-app":    metal3AppName,
		cboLabelName: ironicNetworkingDeploymentName,
	}
	template := newIronicNetworkingPodTemplateSpec(info, &podSpecLabels)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ironicNetworkingDeploymentName,
			Namespace: info.Namespace,
			Annotations: map[string]string{
				cboOwnedAnnotation: "",
			},
			Labels: map[string]string{
				"k8s-app":    metal3AppName,
				cboLabelName: ironicNetworkingDeploymentName,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Selector: selector,
			Template: *template,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
		},
	}
}

func EnsureIronicNetworkingDeployment(info *ProvisioningInfo) (updated bool, err error) {
	if !info.ProvConfig.Spec.IsSwitchManagementEnabled() {
		return false, DeleteIronicNetworkingDeployment(info)
	}

	networkingDeployment := newIronicNetworkingDeployment(info)

	// Build a new annotations map to avoid mutating the shared
	// podTemplateAnnotations variable. Add secret version annotations to
	// trigger a rollout when secret content changes, since the
	// ironic-networking process does not watch for file changes on its
	// mounted secrets.
	podAnnotations := make(map[string]string)
	maps.Copy(podAnnotations, podTemplateAnnotations)

	configAnn, err := secretVersionAnnotation(info.Context, info.Client, info.Namespace, "switch-configs", switchConfigsSecretName)
	if err != nil {
		return false, fmt.Errorf("unable to compute switch-configs secret version: %w", err)
	}
	maps.Copy(podAnnotations, configAnn)

	credsAnn, err := secretVersionAnnotation(info.Context, info.Client, info.Namespace, "switch-credentials", switchCredentialsSecretName)
	if err != nil {
		return false, fmt.Errorf("unable to compute switch-credentials secret version: %w", err)
	}
	maps.Copy(podAnnotations, credsAnn)

	networkingDeployment.Spec.Template.Annotations = podAnnotations

	expectedGeneration := resourcemerge.ExpectedDeploymentGeneration(networkingDeployment, info.ProvConfig.Status.Generations)

	err = controllerutil.SetControllerReference(info.ProvConfig, networkingDeployment, info.Scheme)
	if err != nil {
		err = fmt.Errorf("unable to set controllerReference on ironic-networking deployment: %w", err)
		return
	}

	deployment, updated, err := resourceapply.ApplyDeployment(info.Context,
		info.Client.AppsV1(), info.EventRecorder, networkingDeployment, expectedGeneration)
	if err != nil {
		err = fmt.Errorf("unable to apply ironic-networking deployment: %w", err)
		return
	}
	if updated {
		ironicNetworkingRolloutStartTime = time.Now()
		resourcemerge.SetDeploymentGeneration(&info.ProvConfig.Status.Generations, deployment)
	}
	return updated, nil
}

func GetIronicNetworkingDeploymentState(client appsclientv1.DeploymentsGetter, targetNamespace string, config *metal3iov1alpha1.Provisioning) (appsv1.DeploymentConditionType, error) {
	if !config.Spec.IsSwitchManagementEnabled() {
		return appsv1.DeploymentAvailable, nil
	}

	existing, err := client.Deployments(targetNamespace).Get(context.Background(), ironicNetworkingDeploymentName, metav1.GetOptions{})
	if err != nil || existing == nil {
		return appsv1.DeploymentReplicaFailure, err
	}
	deploymentState := getDeploymentCondition(existing)
	if deploymentState == appsv1.DeploymentProgressing && deploymentRolloutTimeout <= time.Since(ironicNetworkingRolloutStartTime) {
		return appsv1.DeploymentReplicaFailure, nil
	}
	return deploymentState, nil
}

func DeleteIronicNetworkingDeployment(info *ProvisioningInfo) error {
	return client.IgnoreNotFound(info.Client.AppsV1().Deployments(info.Namespace).Delete(info.Context, ironicNetworkingDeploymentName, metav1.DeleteOptions{}))
}

// newIronicNetworkingService creates a ClusterIP service for the ironic-networking
// deployment that exposes the JSON-RPC port. This allows the ironic container
// (running in a separate pod) to reach the networking service via DNS.
func newIronicNetworkingService(info *ProvisioningInfo) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ironicNetworkingServiceName,
			Namespace: info.Namespace,
			Labels: map[string]string{
				cboLabelName: ironicNetworkingDeploymentName,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				cboLabelName: ironicNetworkingDeploymentName,
			},
			Ports: []corev1.ServicePort{
				{
					Name: ironicNetworkingRPCPortName,
					Port: ironicNetworkingRPCPort,
				},
			},
		},
	}
}

// EnsureIronicNetworkingService ensures the service for ironic-networking exists
// when switch management is enabled, and deletes it when disabled.
func EnsureIronicNetworkingService(info *ProvisioningInfo) (updated bool, err error) {
	if !info.ProvConfig.Spec.IsSwitchManagementEnabled() {
		return false, DeleteIronicNetworkingService(info)
	}

	networkingSvc := newIronicNetworkingService(info)

	err = controllerutil.SetControllerReference(info.ProvConfig, networkingSvc, info.Scheme)
	if err != nil {
		err = fmt.Errorf("unable to set controllerReference on ironic-networking service: %w", err)
		return
	}

	_, updated, err = resourceapply.ApplyService(info.Context,
		info.Client.CoreV1(), info.EventRecorder, networkingSvc)
	if err != nil {
		err = fmt.Errorf("unable to apply ironic-networking service: %w", err)
	}
	return
}

// DeleteIronicNetworkingService deletes the ironic-networking service
func DeleteIronicNetworkingService(info *ProvisioningInfo) error {
	return client.IgnoreNotFound(info.Client.CoreV1().Services(info.Namespace).Delete(info.Context, ironicNetworkingServiceName, metav1.DeleteOptions{}))
}
