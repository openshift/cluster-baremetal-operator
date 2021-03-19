/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package provisioning

import (
	"context"
	"fmt"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	configv1 "github.com/openshift/api/config/v1"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
)

const (
	metal3AppName                    = "metal3"
	baremetalDeploymentName          = "metal3"
	baremetalSharedVolume            = "metal3-shared"
	metal3AuthRootDir                = "/auth"
	ironicCredentialsVolume          = "metal3-ironic-basic-auth"
	ironicrpcCredentialsVolume       = "metal3-ironic-rpc-basic-auth"
	inspectorCredentialsVolume       = "metal3-inspector-basic-auth"
	htpasswdEnvVar                   = "HTTP_BASIC_HTPASSWD" // #nosec
	mariadbPwdEnvVar                 = "MARIADB_PASSWORD"    // #nosec
	cboOwnedAnnotation               = "baremetal.openshift.io/owned"
	cboLabelName                     = "baremetal.openshift.io/cluster-baremetal-operator"
	externalTrustBundleConfigMapName = "cbo-trusted-ca"
)

var deploymentRolloutStartTime = time.Now()
var deploymentRolloutTimeout = 5 * time.Minute

var sharedVolumeMount = corev1.VolumeMount{
	Name:      baremetalSharedVolume,
	MountPath: "/shared",
}

var ironicCredentialsMount = corev1.VolumeMount{
	Name:      ironicCredentialsVolume,
	MountPath: metal3AuthRootDir + "/ironic",
	ReadOnly:  true,
}

var rpcCredentialsMount = corev1.VolumeMount{
	Name:      ironicrpcCredentialsVolume,
	MountPath: metal3AuthRootDir + "/ironic-rpc",
	ReadOnly:  true,
}

var inspectorCredentialsMount = corev1.VolumeMount{
	Name:      inspectorCredentialsVolume,
	MountPath: metal3AuthRootDir + "/ironic-inspector",
	ReadOnly:  true,
}

var mariadbPassword = corev1.EnvVar{
	Name: mariadbPwdEnvVar,
	ValueFrom: &corev1.EnvVarSource{
		SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: baremetalSecretName,
			},
			Key: baremetalSecretKey,
		},
	},
}

var metal3Volumes = []corev1.Volume{
	{
		Name: baremetalSharedVolume,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	},
	imageVolume(),
	{
		Name: ironicCredentialsVolume,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: ironicSecretName,
				Items: []corev1.KeyToPath{
					{Key: ironicUsernameKey, Path: ironicUsernameKey},
					{Key: ironicPasswordKey, Path: ironicPasswordKey},
					{Key: ironicConfigKey, Path: ironicConfigKey},
				},
			},
		},
	},
	{
		Name: ironicrpcCredentialsVolume,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: ironicrpcSecretName,
				Items: []corev1.KeyToPath{
					{Key: ironicUsernameKey, Path: ironicUsernameKey},
					{Key: ironicPasswordKey, Path: ironicPasswordKey},
					{Key: ironicConfigKey, Path: ironicConfigKey},
				},
			},
		},
	},
	{
		Name: inspectorCredentialsVolume,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: inspectorSecretName,
				Items: []corev1.KeyToPath{
					{Key: ironicUsernameKey, Path: ironicUsernameKey},
					{Key: ironicPasswordKey, Path: ironicPasswordKey},
					{Key: ironicConfigKey, Path: ironicConfigKey},
				},
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
				Optional: pointer.BoolPtr(true),
			},
		},
	},
}

func buildEnvVar(name string, baremetalProvisioningConfig *metal3iov1alpha1.ProvisioningSpec) corev1.EnvVar {
	value := getMetal3DeploymentConfig(name, baremetalProvisioningConfig)
	if value != nil {
		return corev1.EnvVar{
			Name:  name,
			Value: *value,
		}
	} else if name == provisioningIP && baremetalProvisioningConfig.ProvisioningNetwork == metal3iov1alpha1.ProvisioningNetworkDisabled {
		return corev1.EnvVar{
			Name: name,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "status.hostIP",
				},
			},
		}
	}

	return corev1.EnvVar{
		Name: name,
	}
}

func setIronicHtpasswdHash(name string, secretName string) corev1.EnvVar {
	return corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secretName,
				},
				Key: ironicHtpasswdKey,
			},
		},
	}
}

func newMetal3InitContainers(images *Images, config *metal3iov1alpha1.ProvisioningSpec, proxy *configv1.Proxy) []corev1.Container {
	initContainers := []corev1.Container{
		createInitContainerIpaDownloader(images),
		createInitContainerMachineOsDownloader(images, config),
	}

	// If the provisioning network is disabled, and the user hasn't requested a
	// particular provisioning IP on the machine CIDR, we have nothing for this container
	// to manage.
	if config.ProvisioningIP != "" {
		initContainers = append(initContainers, createInitContainerStaticIpSet(images, config))
	}

	return injectProxyAndCA(initContainers, proxy)
}

func createInitContainerIpaDownloader(images *Images) corev1.Container {
	initContainer := corev1.Container{
		Name:            "metal3-ipa-downloader",
		Image:           images.IpaDownloader,
		Command:         []string{"/usr/local/bin/get-resource.sh"},
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
		VolumeMounts: []corev1.VolumeMount{imageVolumeMount},
		Env:          []corev1.EnvVar{},
	}
	return initContainer
}

func createInitContainerMachineOsDownloader(images *Images, config *metal3iov1alpha1.ProvisioningSpec) corev1.Container {
	initContainer := corev1.Container{
		Name:            "metal3-machine-os-downloader",
		Image:           images.MachineOsDownloader,
		Command:         []string{"/usr/local/bin/get-resource.sh"},
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
		VolumeMounts: []corev1.VolumeMount{imageVolumeMount},
		Env: []corev1.EnvVar{
			buildEnvVar(machineImageUrl, config),
		},
	}
	return initContainer
}

func createInitContainerStaticIpSet(images *Images, config *metal3iov1alpha1.ProvisioningSpec) corev1.Container {
	initContainer := corev1.Container{
		Name:            "metal3-static-ip-set",
		Image:           images.StaticIpManager,
		Command:         []string{"/set-static-ip"},
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
		Env: []corev1.EnvVar{
			buildEnvVar(provisioningIP, config),
			buildEnvVar(provisioningInterface, config),
		},
	}
	return initContainer
}

func newMetal3Containers(images *Images, config *metal3iov1alpha1.ProvisioningSpec, proxy *configv1.Proxy) []corev1.Container {
	containers := []corev1.Container{
		createContainerMetal3BaremetalOperator(images, config),
		createContainerMetal3Mariadb(images),
		createContainerMetal3Httpd(images, config),
		createContainerMetal3IronicConductor(images, config),
		createContainerIronicInspectorRamdiskLogs(images),
		createContainerMetal3IronicApi(images, config),
		createContainerIronicDeployRamdiskLogs(images),
		createContainerMetal3IronicInspector(images, config),
	}

	// If the provisioning network is disabled, and the user hasn't requested a
	// particular provisioning IP on the machine CIDR, we have nothing for this container
	// to manage.
	if config.ProvisioningIP != "" {
		containers = append(containers, createContainerMetal3StaticIpManager(images, config))
	}

	if config.ProvisioningNetwork != metal3iov1alpha1.ProvisioningNetworkDisabled {
		containers = append(containers, createContainerMetal3Dnsmasq(images, config))
	}

	return injectProxyAndCA(containers, proxy)
}

func getWatchNamespace(config *metal3iov1alpha1.ProvisioningSpec) corev1.EnvVar {
	if config.WatchAllNamespaces {
		return corev1.EnvVar{
			Name:  "WATCH_NAMESPACE",
			Value: "",
		}
	} else {
		return corev1.EnvVar{
			Name: "WATCH_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		}
	}
}

func createContainerMetal3BaremetalOperator(images *Images, config *metal3iov1alpha1.ProvisioningSpec) corev1.Container {
	container := corev1.Container{
		Name:  "metal3-baremetal-operator",
		Image: images.BaremetalOperator,
		Ports: []corev1.ContainerPort{
			{
				Name:          "metrics",
				ContainerPort: 60000,
				HostPort:      60000,
			},
		},
		Command:         []string{"/baremetal-operator"},
		ImagePullPolicy: "IfNotPresent",
		VolumeMounts: []corev1.VolumeMount{
			ironicCredentialsMount,
			inspectorCredentialsMount,
		},
		Env: []corev1.EnvVar{
			getWatchNamespace(config),
			{
				Name: "POD_NAMESPACE",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			},
			{
				Name: "POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
			{
				Name:  "OPERATOR_NAME",
				Value: "baremetal-operator",
			},
			buildEnvVar(deployKernelUrl, config),
			buildEnvVar(deployRamdiskUrl, config),
			buildEnvVar(ironicEndpoint, config),
			buildEnvVar(ironicInspectorEndpoint, config),
			{
				Name:  "METAL3_AUTH_ROOT_DIR",
				Value: metal3AuthRootDir,
			},
		},
	}
	return container
}

func createContainerMetal3Dnsmasq(images *Images, config *metal3iov1alpha1.ProvisioningSpec) corev1.Container {
	container := corev1.Container{
		Name:            "metal3-dnsmasq",
		Image:           images.Ironic,
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
		Command: []string{"/bin/rundnsmasq"},
		VolumeMounts: []corev1.VolumeMount{
			sharedVolumeMount,
			imageVolumeMount,
		},
		Env: []corev1.EnvVar{
			buildEnvVar(httpPort, config),
			buildEnvVar(provisioningInterface, config),
			buildEnvVar(dhcpRange, config),
		},
	}
	return container
}

func createContainerMetal3Mariadb(images *Images) corev1.Container {
	container := corev1.Container{
		Name:            "metal3-mariadb",
		Image:           images.Ironic,
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
		Command:      []string{"/bin/runmariadb"},
		VolumeMounts: []corev1.VolumeMount{sharedVolumeMount},
		Env: []corev1.EnvVar{
			mariadbPassword,
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "mysql",
				ContainerPort: 3306,
				HostPort:      3306,
			},
		},
	}
	return container
}

func createContainerMetal3Httpd(images *Images, config *metal3iov1alpha1.ProvisioningSpec) corev1.Container {
	port, _ := strconv.Atoi(baremetalHttpPort) // #nosec
	container := corev1.Container{
		Name:            "metal3-httpd",
		Image:           images.Ironic,
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
		Command: []string{"/bin/runhttpd"},
		VolumeMounts: []corev1.VolumeMount{
			sharedVolumeMount,
			imageVolumeMount,
		},
		Env: []corev1.EnvVar{
			buildEnvVar(httpPort, config),
			buildEnvVar(provisioningIP, config),
			buildEnvVar(provisioningInterface, config),
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          httpPortName,
				ContainerPort: int32(port),
				HostPort:      int32(port),
			},
		},
	}
	return container
}

func createContainerMetal3IronicConductor(images *Images, config *metal3iov1alpha1.ProvisioningSpec) corev1.Container {
	container := corev1.Container{
		Name:            "metal3-ironic-conductor",
		Image:           images.Ironic,
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
		Command: []string{"/bin/runironic-conductor"},
		VolumeMounts: []corev1.VolumeMount{
			sharedVolumeMount,
			inspectorCredentialsMount,
			rpcCredentialsMount,
		},
		Env: []corev1.EnvVar{
			mariadbPassword,
			buildEnvVar(httpPort, config),
			buildEnvVar(provisioningIP, config),
			buildEnvVar(provisioningInterface, config),
			setIronicHtpasswdHash(htpasswdEnvVar, ironicrpcSecretName),
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "json-rpc",
				ContainerPort: 8089,
				HostPort:      8089,
			},
		},
	}
	return container
}

func createContainerMetal3IronicApi(images *Images, config *metal3iov1alpha1.ProvisioningSpec) corev1.Container {
	container := corev1.Container{
		Name:            "metal3-ironic-api",
		Image:           images.Ironic,
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
		Command: []string{"/bin/runironic-api"},
		VolumeMounts: []corev1.VolumeMount{
			sharedVolumeMount,
			rpcCredentialsMount,
		},
		Env: []corev1.EnvVar{
			mariadbPassword,
			buildEnvVar(httpPort, config),
			buildEnvVar(provisioningIP, config),
			buildEnvVar(provisioningInterface, config),
			setIronicHtpasswdHash(htpasswdEnvVar, ironicSecretName),
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "ironic",
				ContainerPort: 6385,
				HostPort:      6385,
			},
		},
	}
	return container
}

func createContainerIronicDeployRamdiskLogs(images *Images) corev1.Container {
	container := corev1.Container{
		Name:            "ironic-deploy-ramdisk-logs",
		Image:           images.Ironic,
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
		Command:      []string{"/bin/runlogwatch.sh"},
		VolumeMounts: []corev1.VolumeMount{sharedVolumeMount},
	}
	return container
}

func createContainerMetal3IronicInspector(images *Images, config *metal3iov1alpha1.ProvisioningSpec) corev1.Container {
	container := corev1.Container{
		Name:            "metal3-ironic-inspector",
		Image:           images.IronicInspector,
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
		VolumeMounts: []corev1.VolumeMount{
			sharedVolumeMount,
			ironicCredentialsMount,
		},
		Env: []corev1.EnvVar{
			buildEnvVar(provisioningIP, config),
			buildEnvVar(provisioningInterface, config),
			setIronicHtpasswdHash(htpasswdEnvVar, inspectorSecretName),
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "inspector",
				ContainerPort: 5050,
				HostPort:      5050,
			},
		},
	}
	return container
}

func createContainerIronicInspectorRamdiskLogs(images *Images) corev1.Container {
	container := corev1.Container{
		Name:            "ironic-inspector-ramdisk-logs",
		Image:           images.IronicInspector,
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
		Command:      []string{"/bin/runlogwatch.sh"},
		VolumeMounts: []corev1.VolumeMount{sharedVolumeMount},
	}
	return container
}

func createContainerMetal3StaticIpManager(images *Images, config *metal3iov1alpha1.ProvisioningSpec) corev1.Container {
	container := corev1.Container{
		Name:            "metal3-static-ip-manager",
		Image:           images.StaticIpManager,
		Command:         []string{"/refresh-static-ip"},
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
		Env: []corev1.EnvVar{
			buildEnvVar(provisioningIP, config),
			buildEnvVar(provisioningInterface, config),
		},
	}
	return container
}

func newMetal3PodTemplateSpec(images *Images, config *metal3iov1alpha1.ProvisioningSpec, labels *map[string]string, proxy *configv1.Proxy) *corev1.PodTemplateSpec {
	initContainers := newMetal3InitContainers(images, config, proxy)
	containers := newMetal3Containers(images, config, proxy)

	tolerations := []corev1.Toleration{
		{
			Key:      "node-role.kubernetes.io/master",
			Effect:   corev1.TaintEffectNoSchedule,
			Operator: corev1.TolerationOpExists,
		},
		{
			Key:      "CriticalAddonsOnly",
			Operator: corev1.TolerationOpExists,
		},
		{
			Key:               "node.kubernetes.io/not-ready",
			Effect:            corev1.TaintEffectNoExecute,
			Operator:          corev1.TolerationOpExists,
			TolerationSeconds: pointer.Int64Ptr(120),
		},
		{
			Key:               "node.kubernetes.io/unreachable",
			Effect:            corev1.TaintEffectNoExecute,
			Operator:          corev1.TolerationOpExists,
			TolerationSeconds: pointer.Int64Ptr(120),
		},
	}

	return &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: *labels,
		},
		Spec: corev1.PodSpec{
			Volumes:           metal3Volumes,
			InitContainers:    initContainers,
			Containers:        containers,
			HostNetwork:       true,
			DNSPolicy:         corev1.DNSClusterFirstWithHostNet,
			PriorityClassName: "system-node-critical",
			NodeSelector:      map[string]string{"node-role.kubernetes.io/master": ""},
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: pointer.BoolPtr(false),
			},
			ServiceAccountName: "cluster-baremetal-operator",
			Tolerations:        tolerations,
		},
	}
}

func mountsWithTrustedCA(mounts []corev1.VolumeMount) []corev1.VolumeMount {
	mounts = append(mounts, corev1.VolumeMount{
		MountPath: "/etc/pki/ca-trust/extracted/pem",
		Name:      "trusted-ca",
		ReadOnly:  true,
	})

	return mounts
}

func injectProxyAndCA(containers []corev1.Container, proxy *configv1.Proxy) []corev1.Container {
	var injectedContainers []corev1.Container

	for _, container := range containers {
		container.Env = envWithProxy(proxy, container.Env)
		container.VolumeMounts = mountsWithTrustedCA(container.VolumeMounts)
		injectedContainers = append(injectedContainers, container)
	}

	return injectedContainers
}

func envWithProxy(proxy *configv1.Proxy, envVars []corev1.EnvVar) []corev1.EnvVar {
	if proxy == nil {
		return envVars
	}

	if proxy.Status.HTTPProxy != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "HTTP_PROXY",
			Value: proxy.Status.HTTPProxy,
		})
	}
	if proxy.Status.HTTPSProxy != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "HTTPS_PROXY",
			Value: proxy.Status.HTTPSProxy,
		})
	}
	if proxy.Status.NoProxy != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "NO_PROXY",
			Value: proxy.Status.NoProxy,
		})
	}

	return envVars
}

func newMetal3Deployment(targetNamespace string, images *Images, config *metal3iov1alpha1.ProvisioningSpec, selector *metav1.LabelSelector, proxy *configv1.Proxy) *appsv1.Deployment {
	if selector == nil {
		selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"k8s-app":    metal3AppName,
				cboLabelName: stateService,
			},
		}
	}
	k8sAppLabel := metal3AppName
	apiLabelValue := ""
	for k, v := range selector.MatchLabels {
		if k == "k8s-app" {
			k8sAppLabel = v
		}
		if k == "api" {
			apiLabelValue = v
		}
	}
	podSpecLabels := map[string]string{
		"k8s-app":    k8sAppLabel,
		cboLabelName: stateService,
	}
	if apiLabelValue != "" {
		podSpecLabels["api"] = apiLabelValue
	}
	template := newMetal3PodTemplateSpec(images, config, &podSpecLabels, proxy)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      baremetalDeploymentName,
			Namespace: targetNamespace,
			Annotations: map[string]string{
				cboOwnedAnnotation: "",
			},
			Labels: map[string]string{
				"k8s-app":    k8sAppLabel,
				cboLabelName: stateService,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(1),
			Selector: selector,
			Template: *template,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
		},
	}
}

func CheckExistingMetal3Deployment(client appsclientv1.DeploymentsGetter, targetNamespace string) (*metav1.LabelSelector, bool, error) {
	existing, err := client.Deployments(targetNamespace).Get(context.Background(), baremetalDeploymentName, metav1.GetOptions{})
	if existing != nil && err == nil {
		_, maoOwned := existing.Annotations["machine.openshift.io/owned"]
		return existing.Spec.Selector, maoOwned, nil
	}
	return nil, false, err
}

func EnsureMetal3Deployment(info *ProvisioningInfo) (updated bool, err error) {
	// Create metal3 deployment object based on current baremetal configuration
	// It will be created with the cboOwnedAnnotation
	metal3Deployment := newMetal3Deployment(info.Namespace, info.Images, &info.ProvConfig.Spec, info.PodLabelSelector, info.Proxy)

	expectedGeneration := resourcemerge.ExpectedDeploymentGeneration(metal3Deployment, info.ProvConfig.Status.Generations)

	err = controllerutil.SetControllerReference(info.ProvConfig, metal3Deployment, info.Scheme)
	if err != nil {
		err = fmt.Errorf("unable to set controllerReference on deployment: %w", err)
		return
	}

	deploymentRolloutStartTime = time.Now()
	deployment, updated, err := resourceapply.ApplyDeployment(info.Client.AppsV1(),
		info.EventRecorder, metal3Deployment, expectedGeneration)
	if err != nil {
		err = fmt.Errorf("unable to apply Metal3 deployment: %w", err)
		return
	}

	if updated {
		resourcemerge.SetDeploymentGeneration(&info.ProvConfig.Status.Generations, deployment)
	}
	return
}

func getDeploymentCondition(deployment *appsv1.Deployment) appsv1.DeploymentConditionType {
	for _, cond := range deployment.Status.Conditions {
		if cond.Status == corev1.ConditionTrue {
			return cond.Type
		}
	}
	return appsv1.DeploymentProgressing
}

// Provide the current state of metal3 deployment
func GetDeploymentState(client appsclientv1.DeploymentsGetter, targetNamespace string, config *metal3iov1alpha1.Provisioning) (appsv1.DeploymentConditionType, error) {
	existing, err := client.Deployments(targetNamespace).Get(context.Background(), baremetalDeploymentName, metav1.GetOptions{})
	if err != nil || existing == nil {
		// There were errors accessing the deployment.
		return appsv1.DeploymentReplicaFailure, err
	}
	deploymentState := getDeploymentCondition(existing)
	if deploymentState == appsv1.DeploymentProgressing && deploymentRolloutTimeout <= time.Since(deploymentRolloutStartTime) {
		return appsv1.DeploymentReplicaFailure, nil
	}
	return deploymentState, nil
}

func DeleteMetal3Deployment(info *ProvisioningInfo) error {
	return client.IgnoreNotFound(info.Client.AppsV1().Deployments(info.Namespace).Delete(context.Background(), baremetalDeploymentName, metav1.DeleteOptions{}))
}
