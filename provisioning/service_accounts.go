package provisioning

const (
	bmoServiceAccountName                = "baremetal-operator"
	metal3ServiceAccountName             = "metal3"
	imageCustomizationServiceAccountName = "metal3-image-customization"

	requiredSCCAnnotation = "openshift.io/required-scc"
	privilegedSCC         = "privileged"
	hostNetworkV2SCC      = "hostnetwork-v2"
	restrictedV2SCC       = "restricted-v2"
)

func podAnnotationsWithRequiredSCC(scc string) map[string]string {
	annotations := make(map[string]string, len(podTemplateAnnotations)+1)
	for key, value := range podTemplateAnnotations {
		annotations[key] = value
	}
	annotations[requiredSCCAnnotation] = scc
	return annotations
}
