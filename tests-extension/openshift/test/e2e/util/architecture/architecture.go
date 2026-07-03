package architecture

import (
	"maps"
	"slices"
	"strings"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type Architecture string

const (
	AMD64   Architecture = "amd64"
	ARM64   Architecture = "arm64"
	MULTI   Architecture = "multi"
	UNKNOWN Architecture = "unknown"
)

const (
	NodeArchitectureLabel = "kubernetes.io/arch"
)

// GetAvailableArchitecturesSet returns multi-arch node cluster's Architectures
func GetAvailableArchitecturesSet(oc *exutil.CLI) []Architecture {
	output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("nodes", "-o=jsonpath={.items[*].status.nodeInfo.architecture}").Output()
	if err != nil {
		e2e.Failf("unable to get the cluster architecture: %v", err)
	}
	if output == "" {
		e2e.Failf("the retrieved architecture is empty")
	}
	// Use strings.Fields to split and ignore whitespace-only entries
	architectureList := strings.Fields(output)
	archMap := make(map[Architecture]bool, len(architectureList))
	for _, nodeArchitecture := range architectureList {
		archMap[Architecture(nodeArchitecture)] = true
	}
	return slices.Collect(maps.Keys(archMap))
}

// IsMultiArchCluster check if the cluster is multi-arch cluster
func IsMultiArchCluster(oc *exutil.CLI) bool {
	architectures := GetAvailableArchitecturesSet(oc)
	return len(architectures) > 1
}

// ClusterArchitecture returns the cluster's Architecture
// If the cluster uses the multi-arch payload, this function returns Architecture.multi
func ClusterArchitecture(oc *exutil.CLI) Architecture {
	architectures := GetAvailableArchitecturesSet(oc)
	if len(architectures) > 1 {
		e2e.Logf("Found multi-arch node cluster")
		return MULTI
	}
	return architectures[0]
}

func (a Architecture) GNUString() string {
	switch a {
	case AMD64:
		return "x86_64"
	case ARM64:
		return "aarch64"
	default:
		e2e.Failf("Unknown architecture %s", a)
	}
	return ""
}

// GetControlPlaneArch get the architecture of the contol plane node
func GetControlPlaneArch(oc *exutil.CLI) Architecture {
	masterNode, err := compat_otp.GetFirstMasterNode(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	architecture, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", masterNode, "-o=jsonpath={.status.nodeInfo.architecture}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return Architecture(architecture)
}
