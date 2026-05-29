package baremetal

import (
	"fmt"
	"os"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	"github.com/openshift/cluster-baremetal-operator-tests-extension/openshift/test/e2e/util/architecture"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	bootMethodVMedia = "vmedia"
	bootMethodPXE    = "pxe"
)

// getBootMethod determines if a BareMetalHost was booted via vmedia or PXE
// vmedia uses protocols like redfish-virtualmedia://, idrac-virtualmedia://, ilo5-virtualmedia://
// PXE uses protocols like ipmi://, redfish://, idrac://, redfish+https://
func getBootMethod(oc *exutil.CLI, bmhName string) (string, error) {
	bmcAddress, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("baremetalhost", "-n", "openshift-machine-api", bmhName, "-o=jsonpath={.spec.bmc.address}").Output()
	if err != nil {
		return "", err
	}
	if bmcAddress == "" {
		return "", fmt.Errorf("BMC address is empty for host %s", bmhName)
	}
	// Check if BMC address contains virtualmedia in the protocol
	if strings.Contains(bmcAddress, "virtualmedia") {
		return bootMethodVMedia, nil
	}
	// Explicitly check for known PXE-style protocols
	pxeProtocols := []string{"ipmi://", "redfish://", "idrac://", "redfish+https://", "ilo5://"}
	for _, protocol := range pxeProtocols {
		if strings.HasPrefix(bmcAddress, protocol) {
			return bootMethodPXE, nil
		}
	}
	return "", fmt.Errorf("unknown boot protocol in BMC address: %s", bmcAddress)
}

// verifyMultiArchWorker verifies arm64 worker nodes with the specified boot method
func verifyMultiArchWorker(oc *exutil.CLI, expectedBootMethod string) {
	compat_otp.By("1) Verify control plane is x86_64 (amd64)")
	controlPlaneArch := architecture.GetControlPlaneArch(oc)
	o.Expect(controlPlaneArch).To(o.Equal(architecture.AMD64), "Control plane should be amd64/x86_64")
	e2e.Logf("Control plane architecture is: %s", controlPlaneArch)

	compat_otp.By("2) Verify cluster is multi-arch")
	isMultiArch := architecture.IsMultiArchCluster(oc)
	o.Expect(isMultiArch).To(o.BeTrue(), "Cluster should be multi-arch")
	e2e.Logf("Cluster is multi-arch")

	compat_otp.By("3) Get all worker nodes and their architectures")
	workerNodes, err := compat_otp.GetClusterNodesBy(oc, "worker")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(len(workerNodes)).To(o.BeNumerically(">", 0), "Cluster should have at least one worker node")
	e2e.Logf("Found %d worker nodes", len(workerNodes))

	compat_otp.By("4) Find arm64 worker nodes")
	arm64WorkerNodes := []string{}
	for _, workerNode := range workerNodes {
		nodeArch, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", workerNode, "-o=jsonpath={.status.nodeInfo.architecture}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Worker node %s has architecture: %s", workerNode, nodeArch)

		if nodeArch == "arm64" {
			arm64WorkerNodes = append(arm64WorkerNodes, workerNode)
		}
	}

	o.Expect(len(arm64WorkerNodes)).To(o.BeNumerically(">", 0), "At least one arm64 worker node should be present")
	e2e.Logf("Found %d arm64 worker node(s)", len(arm64WorkerNodes))

	compat_otp.By("5) Verify arm64 worker nodes are in Ready state")
	for _, arm64Node := range arm64WorkerNodes {
		nodeStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", arm64Node, "-o=jsonpath={.status.conditions[?(@.type=='Ready')].status}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(nodeStatus).To(o.Equal("True"), "arm64 worker node %s should be Ready", arm64Node)
		e2e.Logf("arm64 worker node %s is Ready", arm64Node)
	}

	compat_otp.By("6) Verify arm64 worker nodes have correct role label")
	for _, arm64Node := range arm64WorkerNodes {
		// Check if the worker role label key exists in node's metadata.labels
		nodeLabels, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", arm64Node, "-o=jsonpath={.metadata.labels}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(nodeLabels).To(o.ContainSubstring("node-role.kubernetes.io/worker"), "arm64 worker node %s should have worker role label", arm64Node)
		e2e.Logf("arm64 worker node %s has worker role label", arm64Node)
	}

	compat_otp.By("7) Verify arm64 worker nodes are schedulable")
	for _, arm64Node := range arm64WorkerNodes {
		nodeSchedulable, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", arm64Node, "-o=jsonpath={.spec.unschedulable}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		// unschedulable field is empty or false when node is schedulable
		o.Expect(nodeSchedulable).NotTo(o.Equal("true"), "arm64 worker node %s should be schedulable", arm64Node)
		e2e.Logf("arm64 worker node %s is schedulable", arm64Node)
	}

	compat_otp.By("8) Verify arm64 worker nodes boot method and skip if not matching expected method")
	matchingWorkers := []string{}
	for _, arm64Node := range arm64WorkerNodes {
		// Get the Machine resource for this node
		machineName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", arm64Node, "-o=jsonpath={.metadata.annotations.machine\\.openshift\\.io/machine}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(machineName).NotTo(o.BeEmpty(), "Node %s should have a machine annotation", arm64Node)
		e2e.Logf("Node %s is associated with machine: %s", arm64Node, machineName)

		// Extract machine name from the full path (format: openshift-machine-api/machine-name)
		machineNameParts := strings.Split(machineName, "/")
		o.Expect(len(machineNameParts)).To(o.Equal(2), "Machine annotation should be in format namespace/name")
		actualMachineName := machineNameParts[1]

		// Get the BareMetalHost from the Machine
		bmhName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machine", "-n", "openshift-machine-api", actualMachineName, "-o=jsonpath={.metadata.annotations.metal3\\.io/BareMetalHost}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(bmhName).NotTo(o.BeEmpty(), "Machine %s should have a BareMetalHost annotation", actualMachineName)
		e2e.Logf("Machine %s is associated with BareMetalHost: %s", actualMachineName, bmhName)

		// Extract BareMetalHost name from the full path (format: namespace/bmh-name)
		bmhNameParts := strings.Split(bmhName, "/")
		o.Expect(len(bmhNameParts)).To(o.Equal(2), "BareMetalHost annotation should be in format namespace/name")
		actualBMHName := bmhNameParts[1]

		// Determine boot method
		bootMethod, err := getBootMethod(oc, actualBMHName)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Get BMC address protocol scheme for logging (avoid exposing full address)
		bmcAddress, bmcErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("baremetalhost", "-n", "openshift-machine-api", actualBMHName, "-o=jsonpath={.spec.bmc.address}").Output()
		bmcProtocol := "unknown"
		if bmcErr != nil {
			e2e.Logf("Warning: Failed to get BMC address for %s: %v", actualBMHName, bmcErr)
		} else if bmcAddress != "" {
			// Extract protocol scheme only (e.g., "redfish-virtualmedia", "ipmi")
			if idx := strings.Index(bmcAddress, "://"); idx > 0 {
				bmcProtocol = bmcAddress[:idx]
			}
		}

		if bootMethod == expectedBootMethod {
			matchingWorkers = append(matchingWorkers, arm64Node)
			if bootMethod == bootMethodVMedia {
				e2e.Logf("*** Worker node %s was added with VMEDIA boot (protocol: %s) ***", arm64Node, bmcProtocol)
			} else {
				e2e.Logf("*** Worker node %s was added with PXE boot (protocol: %s) ***", arm64Node, bmcProtocol)
			}
		} else {
			e2e.Logf("Skipping worker node %s - boot method is %s (protocol: %s), expected %s", arm64Node, bootMethod, bmcProtocol, expectedBootMethod)
		}
	}

	if len(matchingWorkers) == 0 {
		g.Skip("No arm64 worker nodes found with boot method: " + expectedBootMethod)
	}
	e2e.Logf("Found %d arm64 worker node(s) with %s boot method", len(matchingWorkers), expectedBootMethod)

	compat_otp.By("9) Verify available architectures in cluster")
	availableArchs := architecture.GetAvailableArchitecturesSet(oc)
	e2e.Logf("Available architectures in cluster: %v", availableArchs)

	hasAMD64 := false
	hasARM64 := false
	for _, arch := range availableArchs {
		if arch == architecture.AMD64 {
			hasAMD64 = true
		}
		if arch == architecture.ARM64 {
			hasARM64 = true
		}
	}

	o.Expect(hasAMD64).To(o.BeTrue(), "Cluster should have amd64 nodes")
	o.Expect(hasARM64).To(o.BeTrue(), "Cluster should have arm64 nodes")
	e2e.Logf("Successfully verified arm64 worker node(s) added to x86_64 cluster using %s", expectedBootMethod)
}

var _ = g.Describe("[OTP][sig-baremetal] IPI BareMetal", func() {
	defer g.GinkgoRecover()
	var (
		oc = compat_otp.NewCLI("multi-arch-worker", compat_otp.KubeConfigPath())
	)
	g.BeforeEach(func() {
		// Skip if not running in a multi-arch job
		jobName := os.Getenv("JOB_NAME")
		if !strings.Contains(jobName, "multi-arch") {
			g.Skip("Test only runs when JOB_NAME contains 'multi-arch'")
		}

		SkipIfNotBaremetalCluster(oc)
	})

	// author: sgoveas@redhat.com
	// port=unknown - no data in BigQuery last 60 days
	g.It("Author:sgoveas-Medium-87524-Verify arm64 worker node added successfully to x86_64 cluster with vmedia", func() {
		verifyMultiArchWorker(oc, bootMethodVMedia)
	})

	// author: sgoveas@redhat.com
	g.It("Author:sgoveas-Medium-89002-Verify arm64 worker node added successfully to x86_64 cluster with PXE", func() {
		verifyMultiArchWorker(oc, bootMethodPXE)
	})
})
