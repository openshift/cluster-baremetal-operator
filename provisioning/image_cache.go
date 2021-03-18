package provisioning

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	osconfigv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"

	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
)

const (
	imageCacheSharedVolume                                = "metal3-shared-image-cache"
	imageCacheService                                     = "metal3-image-cache"
	imageCachePort                                        = 6181
	imageCachePortName                                    = "http"
	DaemonSetProgressing    appsv1.DaemonSetConditionType = "Progressing"
	DaemonSetReplicaFailure appsv1.DaemonSetConditionType = "ReplicaFailure"
	DaemonSetAvailable      appsv1.DaemonSetConditionType = "Available"
)

var (
	daemonSetRolloutStartTime = time.Now()
	daemonSetRolloutTimeout   = 5 * time.Minute
	fileCompressionSuffix     = regexp.MustCompile(`\.[gx]z$`)
	imageVolumeMount          = corev1.VolumeMount{
		Name:      imageCacheSharedVolume,
		MountPath: "/shared/html/images",
	}
)

func imageVolume() corev1.Volume {
	volType := corev1.HostPathDirectoryOrCreate
	return corev1.Volume{
		Name: imageCacheSharedVolume,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/var/lib/metal3/images",
				Type: &volType,
			},
		},
	}
}

func imageCacheConfig(targetNamespace string, config metal3iov1alpha1.ProvisioningSpec) (*metal3iov1alpha1.ProvisioningSpec, error) {
	// The download URL looks something like:
	// https://releases-art-rhcos.svc.ci.openshift.org/art/storage/releases/rhcos-4.2/42.80.20190725.1/rhcos-42.80.20190725.1-openstack.qcow2?sha256sum=123
	downloadURL, err := url.Parse(config.ProvisioningOSDownloadURL)
	if err != nil {
		return nil, err
	}
	imageName := path.Base(fileCompressionSuffix.ReplaceAllString(downloadURL.Path, ""))

	// The first level of cache downloads the file and makes an uncompressed
	// version of it available to this second-level cache somewhere like:
	// http://metal3-state.openshift-machine-api:6180/images/rhcos-42.80.20190725.1-openstack.qcow2/rhcos-42.80.20190725.1-openstack.qcow2
	// See https://github.com/openshift/ironic-rhcos-downloader for more details
	cacheURL := url.URL{
		Scheme: "http",
		Host: net.JoinHostPort(fmt.Sprintf("%s.%s", stateService, targetNamespace),
			baremetalHttpPort),
		Path: fmt.Sprintf("/images/%s/%s", imageName, imageName),
	}
	config.ProvisioningOSDownloadURL = cacheURL.String()
	return &config, nil
}

func createContainerImageCache(images *Images) corev1.Container {
	container := corev1.Container{
		Name:            "metal3-httpd",
		Image:           images.Ironic,
		ImagePullPolicy: corev1.PullIfNotPresent,
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
		Command:      []string{"/bin/runhttpd"},
		VolumeMounts: []corev1.VolumeMount{imageVolumeMount},
		Ports: []corev1.ContainerPort{
			{
				Name:          imageCachePortName,
				ContainerPort: imageCachePort,
				HostPort:      imageCachePort,
			},
		},
		Env: []corev1.EnvVar{
			{
				Name:  httpPort,
				Value: strconv.Itoa(imageCachePort),
			},
			// The provisioning IP is not used except:
			// - httpd cannot start until the IP is available on some interface
			// - in the inspector.ipxe file for pointing to the IPA kernel and
			//   initramfs images served from this container. However, in
			//   practice we use the inspector.ipxe from the metal3 Pod anyway.
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
	}
	return container
}

func newImageCacheContainers(images *Images, proxy *osconfigv1.Proxy) []corev1.Container {
	containers := []corev1.Container{
		createContainerImageCache(images),
	}

	return injectProxyAndCA(containers, proxy)
}

func newImageCachePodTemplateSpec(targetNamespace string, images *Images, provisioningConfig *metal3iov1alpha1.ProvisioningSpec, proxy *osconfigv1.Proxy) (*corev1.PodTemplateSpec, error) {
	cacheConfig, err := imageCacheConfig(targetNamespace, *provisioningConfig)
	if err != nil {
		return nil, err
	}

	containers := newImageCacheContainers(images, proxy)

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
			Annotations: podTemplateAnnotations,
			Labels: map[string]string{
				"k8s-app":    metal3AppName,
				cboLabelName: imageCacheService,
			},
		},
		Spec: corev1.PodSpec{
			NodeSelector: map[string]string{
				"node-role.kubernetes.io/master": "",
			},
			Volumes: []corev1.Volume{
				imageVolume(),
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
			},
			InitContainers: injectProxyAndCA([]corev1.Container{
				createInitContainerMachineOsDownloader(images, cacheConfig),
				createInitContainerIpaDownloader(images),
			}, proxy),
			Containers:        containers,
			HostNetwork:       true,
			DNSPolicy:         corev1.DNSClusterFirstWithHostNet,
			PriorityClassName: "system-node-critical",
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: pointer.BoolPtr(false),
			},
			ServiceAccountName: "cluster-baremetal-operator",
			Tolerations:        tolerations,
		},
	}, nil
}

func newImageCacheDaemonSet(targetNamespace string, images *Images, config *metal3iov1alpha1.ProvisioningSpec, proxy *osconfigv1.Proxy) (*appsv1.DaemonSet, error) {
	template, err := newImageCachePodTemplateSpec(targetNamespace, images, config, proxy)
	if err != nil {
		return nil, err
	}

	maxUnavail := intstr.FromString("100%")
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      imageCacheService,
			Namespace: targetNamespace,
			Labels: map[string]string{
				"k8s-app": metal3AppName,
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Template: *template,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"k8s-app":    metal3AppName,
					cboLabelName: imageCacheService,
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &maxUnavail,
				},
			},
		},
	}, nil
}

func EnsureImageCache(info *ProvisioningInfo) (updated bool, err error) {
	imageCacheDaemonSet, err := newImageCacheDaemonSet(info.Namespace, info.Images, &info.ProvConfig.Spec, info.Proxy)
	if err != nil {
		return
	}
	expectedGeneration := resourcemerge.ExpectedDaemonSetGeneration(imageCacheDaemonSet, info.ProvConfig.Status.Generations)

	err = controllerutil.SetControllerReference(info.ProvConfig, imageCacheDaemonSet, info.Scheme)
	if err != nil {
		err = fmt.Errorf("unable to set controllerReference on daemonset: %w", err)
		return
	}
	daemonSetRolloutStartTime = time.Now()
	daemonSet, updated, err := resourceapply.ApplyDaemonSet(
		info.Client.AppsV1(),
		info.EventRecorder,
		imageCacheDaemonSet, expectedGeneration)
	if err != nil {
		err = fmt.Errorf("unable to apply image cache daemonset: %w", err)
		return
	}

	resourcemerge.SetDaemonSetGeneration(&info.ProvConfig.Status.Generations, daemonSet)
	return
}

// Provide the current state of metal3 image-cache daemonset
func GetDaemonSetState(client appsclientv1.DaemonSetsGetter, targetNamespace string, config *metal3iov1alpha1.Provisioning) (appsv1.DaemonSetConditionType, error) {
	existing, err := client.DaemonSets(targetNamespace).Get(context.Background(), imageCacheService, metav1.GetOptions{})
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

func DeleteImageCache(info *ProvisioningInfo) error {
	return client.IgnoreNotFound(info.Client.AppsV1().DaemonSets(info.Namespace).Delete(context.Background(), imageCacheService, metav1.DeleteOptions{}))
}
