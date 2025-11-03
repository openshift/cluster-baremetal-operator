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
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	utilnet "k8s.io/utils/net"
	"k8s.io/utils/ptr"

	configv1 "github.com/openshift/api/config/v1"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
)

const (
	metal3AppName                    = "metal3"
	ironicServiceDeploymentName      = "metal3-ironic-service"
	ironicServiceName                = "metal3-ironic"
	metal3AuthRootDir                = "/auth"
	metal3TlsRootDir                 = "/certs"
	ironicCredentialsVolume          = "metal3-ironic-basic-auth"
	ironicTlsVolume                  = "metal3-ironic-tls"
	ironicInsecureEnvVar             = "IRONIC_INSECURE"
	ironicCertEnvVar                 = "IRONIC_CACERT_FILE"
	sshKeyEnvVar                     = "IRONIC_RAMDISK_SSH_KEY"
	externalIpEnvVar                 = "IRONIC_EXTERNAL_IP"
	externalUrlEnvVar                = "IRONIC_EXTERNAL_URL_V6"
	cboOwnedAnnotation               = "baremetal.openshift.io/owned"
	cboLabelName                     = "baremetal.openshift.io/cluster-baremetal-operator"
	externalTrustBundleConfigMapName = "cbo-trusted-ca"
)

var podTemplateAnnotations = map[string]string{
	"target.workload.openshift.io/management": `{"effect": "PreferredDuringScheduling"}`,
}

var deploymentRolloutStartTime = time.Now()
var deploymentRolloutTimeout = 5 * time.Minute

var ironicCredentialsMount = corev1.VolumeMount{
	Name:      ironicCredentialsVolume,
	MountPath: metal3AuthRootDir + "/ironic",
	ReadOnly:  true,
}

var ironicTlsMount = corev1.VolumeMount{
	Name:      ironicTlsVolume,
	MountPath: metal3TlsRootDir + "/ironic",
	ReadOnly:  true,
}

func trustedCAVolume() corev1.Volume {
	return corev1.Volume{
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
	}
}

func buildEnvVar(name string, baremetalProvisioningConfig *metal3iov1alpha1.ProvisioningSpec) corev1.EnvVar {
	value := getMetal3DeploymentConfig(name, baremetalProvisioningConfig)
	if value != nil {
		return corev1.EnvVar{
			Name:  name,
			Value: *value,
		}
	} else if name == provisioningIP && baremetalProvisioningConfig.ProvisioningNetwork == metal3iov1alpha1.ProvisioningNetworkDisabled &&
		baremetalProvisioningConfig.ProvisioningInterface == "" {
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

func setIronicExternalIp(name string, config *metal3iov1alpha1.ProvisioningSpec) corev1.EnvVar {
	if config.ProvisioningNetwork != metal3iov1alpha1.ProvisioningNetworkDisabled && config.VirtualMediaViaExternalNetwork {
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

func setIronicExternalUrl(info *ProvisioningInfo) (corev1.EnvVar, error) {
	ironicIPs, err := GetRealIronicIPs(info)
	if err != nil {
		return corev1.EnvVar{}, fmt.Errorf("failed to get Ironic IP when setting external url: %w", err)
	}

	var ironicIPv6 string

	for _, ironicIP := range ironicIPs {
		if utilnet.IsIPv6String(ironicIP) {
			ironicIPv6 = ironicIP
			break
		}
	}

	if ironicIPv6 == "" {
		return corev1.EnvVar{
			Name: externalUrlEnvVar,
		}, nil
	}

	// protocol, host, port
	urlTemplate := "%s://[%s]:%s"

	if info.ProvConfig.Spec.DisableVirtualMediaTLS {
		return corev1.EnvVar{
			Name:  externalUrlEnvVar,
			Value: fmt.Sprintf(urlTemplate, "http", ironicIPv6, baremetalHttpPort),
		}, nil
	} else {
		return corev1.EnvVar{
			Name:  externalUrlEnvVar,
			Value: fmt.Sprintf(urlTemplate, "https", ironicIPv6, baremetalVmediaHttpsPort),
		}, nil
	}
}

func createInitContainerMachineOsDownloader(info *ProvisioningInfo, imageURLs string, useLiveImages, setIpOptions bool) corev1.Container {
	var command string
	name := "metal3-machine-os-downloader"
	if useLiveImages {
		command = "/usr/local/bin/get-live-images.sh"
		name = name + "-live-images"
	} else {
		command = "/usr/local/bin/get-resource.sh"
	}

	env := []corev1.EnvVar{
		{
			Name:  machineImageUrl,
			Value: imageURLs,
		},
	}
	if setIpOptions {
		env = append(env,
			corev1.EnvVar{
				Name:  ipOptions,
				Value: info.NetworkStack.IpOption(),
			})
	}
	initContainer := corev1.Container{
		Name:            name,
		Image:           info.Images.MachineOsDownloader,
		Command:         []string{command},
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			// Needed for hostPath image volume mount
			Privileged: ptr.To(true),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		},
		VolumeMounts: []corev1.VolumeMount{imageVolumeMount},
		Env:          env,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("50Mi"),
			},
		},
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
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
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
				Add:  []corev1.Capability{"NET_ADMIN"},
			},
		},
		Env: []corev1.EnvVar{
			buildEnvVar(provisioningIP, config),
			buildEnvVar(provisioningInterface, config),
			buildEnvVar(provisioningMacAddresses, config),
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("50Mi"),
			},
		},
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
	}

	return initContainer
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

func buildSSHKeyEnvVar(sshKey string) corev1.EnvVar {
	return corev1.EnvVar{Name: sshKeyEnvVar, Value: sshKey}
}

func createContainerMetal3StaticIpManager(images *Images, config *metal3iov1alpha1.ProvisioningSpec) corev1.Container {
	container := corev1.Container{
		Name:            "metal3-static-ip-manager",
		Image:           images.StaticIpManager,
		Command:         []string{"/refresh-static-ip"},
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			// Needed for mounting /proc to set the addr_gen_mode
			Privileged: ptr.To(true),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
				Add: []corev1.Capability{
					"NET_ADMIN",
					"FOWNER", // Needed for setting the addr_gen_mode
				},
			},
		},
		Env: []corev1.EnvVar{
			buildEnvVar(provisioningIP, config),
			buildEnvVar(provisioningInterface, config),
			buildEnvVar(provisioningMacAddresses, config),
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
		container.Env = envWithProxy(proxy, container.Env, nil)
		container.VolumeMounts = mountsWithTrustedCA(container.VolumeMounts)
		injectedContainers = append(injectedContainers, container)
	}

	return injectedContainers
}

func envWithProxy(proxy *configv1.Proxy, envVars []corev1.EnvVar, noproxy []string) []corev1.EnvVar {
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
	if proxy.Status.NoProxy != "" || noproxy != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "NO_PROXY",
			Value: proxy.Status.NoProxy + "," + strings.Join(noproxy, ","),
		})
	}

	return envVars
}

func getDeploymentCondition(deployment *appsv1.Deployment) appsv1.DeploymentConditionType {
	var progressing, available, replicaFailure bool
	for _, cond := range deployment.Status.Conditions {
		if cond.Type == appsv1.DeploymentProgressing && cond.Status == corev1.ConditionTrue {
			progressing = true
		}
		if cond.Type == appsv1.DeploymentAvailable && cond.Status == corev1.ConditionTrue {
			available = true
		}
		if cond.Type == appsv1.DeploymentReplicaFailure && cond.Status == corev1.ConditionTrue {
			replicaFailure = true
		}
	}
	switch {
	case replicaFailure && !progressing:
		return appsv1.DeploymentReplicaFailure
	case available && !replicaFailure:
		return appsv1.DeploymentAvailable
	default:
		return appsv1.DeploymentProgressing
	}
}
