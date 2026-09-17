package baremetal

import (
	"fmt"
	"os"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/openshift/cluster-baremetal-operator-tests-extension/openshift/test/e2e/testdata"
	"github.com/openshift/cluster-baremetal-operator-tests-extension/openshift/test/e2e/util/architecture"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[OTP][sig-baremetal] IPI BareMetal", func() {
	defer g.GinkgoRecover()
	var (
		oc              = compat_otp.NewCLI("cluster-baremetal-operator", compat_otp.KubeConfigPath())
		isoUrl          string
		nbIsoUrl        string
		nginxIngress    string
		labeledNodeName string
	)
	g.BeforeEach(func() {
		SkipIfNotBaremetalCluster(oc)
		SkipIfNotVirtualMediaCluster(oc)

		// Check if we have at least 2 Ready worker nodes
		compat_otp.By("2) Check for at least 2 Ready worker nodes")
		workerNode, err := compat_otp.GetClusterNodesBy(oc, "worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		readyWorkers := getReadyNodes(oc, workerNode)
		if len(readyWorkers) < 2 {
			g.Skip(fmt.Sprintf("These tests require at least 2 Ready worker nodes, found %d Ready out of %d total", len(readyWorkers), len(workerNode)))
		}

		// Check if the second worker node is x86_64 architecture
		compat_otp.By("3) Check that second Ready worker node is x86_64 architecture")
		labeledNodeName = readyWorkers[1]
		nodeArch := architecture.GetNodeArch(oc, labeledNodeName)
		if nodeArch != architecture.AMD64 {
			g.Skip(fmt.Sprintf("These tests require x86_64/amd64 worker node for nginx pod, but node %s is %s architecture", labeledNodeName, nodeArch))
		}

		// Label worker node 2 to run the web-server hosting the iso
		compat_otp.By("4) Add a label to second Ready worker node")
		compat_otp.AddLabelToNode(oc, labeledNodeName, "nginx-node", "true")

		compat_otp.By("5) Create new project for nginx web-server")
		oc.SetupProject()
		testNamespace := oc.Namespace()

		// nginx-iso.yaml contains the base64 content of a gzip iso
		compat_otp.By("6) Set the ISO URL")
		clusterDomain, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ingress.config/cluster", "-o=jsonpath={.spec.domain}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		isoUrl = "nb-iso." + clusterDomain
		nbIsoUrl = "http://" + isoUrl + "/non-bootable.iso"

		compat_otp.By("7) Create web-server to host the iso file")
		nginxIso := testdata.FixturePath("nginx-iso.yaml")
		dcErr := oc.Run("create").Args("-f", nginxIso, "-n", testNamespace).Execute()
		o.Expect(dcErr).NotTo(o.HaveOccurred())

		// Wait for nginx pod to be ready with 3 minute timeout
		err = waitForPodReady(oc, "nginx-pod", testNamespace, 3*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("8) Create ingress to access the iso file")
		fileIngress := testdata.FixturePath("nginx-ingress.yaml")
		e2e.Logf("FixturePath returned: %s", fileIngress)
		nginxIngress = CopyToFile(fileIngress, "nginx-ingress.yaml")
		e2e.Logf("CopyToFile returned: %s", nginxIngress)
		defer os.Remove(nginxIngress)
		compat_otp.ModifyYamlFileContent(nginxIngress, []compat_otp.YamlReplace{
			{
				Path:  "spec.rules.0.host",
				Value: isoUrl,
			},
		})
		e2e.Logf("About to create ingress from file: %s", nginxIngress)

		IngErr := oc.Run("create").Args("-f", nginxIngress, "-n", testNamespace).Execute()
		o.Expect(IngErr).NotTo(o.HaveOccurred())
	})

	g.AfterEach(func() {
		// Remove label from the specific node that was labeled during BeforeEach
		if labeledNodeName != "" {
			e2e.Logf("Removing nginx-node label from node: %s", labeledNodeName)
			compat_otp.DeleteLabelFromNode(oc, labeledNodeName, "nginx-node")
		}
	})

	// Helper function to check if no dataImage exists for the BMH
	checkNoDataImageExists := func(bmhName string) {
		out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("dataImage", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.ContainSubstring(bmhName))
	}

	// Helper function to check if an ISO is attached to the node using DebugNodeWithChroot
	checkISOAttachedOnNode := func(nodeName string) bool {
		// Check if /dev/sr0 (CD-ROM block device) exists on the node
		cmd := "test -b /dev/sr0"
		_, err := compat_otp.DebugNodeWithChroot(oc, nodeName, "bash", "-c", cmd)
		if err != nil {
			e2e.Logf("ISO not attached (test -b /dev/sr0 failed)")
			return false
		}
		return true
	}

	// Helper function to create dataImage YAML file
	createDataImageYAML := func(bmhName, nodeType string) string {
		dataPath := testdata.FixturePath("non-bootable-iso.yaml")
		dataPathCopy := CopyToFile(dataPath, fmt.Sprintf("non-bootable-iso-%s.yaml", nodeType))
		e2e.Logf("Created dataImage YAML file: %s", dataPathCopy)
		compat_otp.ModifyYamlFileContent(dataPathCopy, []compat_otp.YamlReplace{
			{
				Path:  "metadata.name",
				Value: bmhName,
			},
			{
				Path:  "spec.url",
				Value: nbIsoUrl,
			},
		})
		return dataPathCopy
	}

	// Helper function to apply dataImage and verify it's created
	applyDataImage := func(dataPathCopy, bmhName string) {
		_, err := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", dataPathCopy, "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("dataImage", "-n", machineAPINamespace, "-o=jsonpath={.items[0].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(bmhName))
	}

	// Helper function to reboot BMH and wait for node to be ready
	rebootAndWaitForNode := func(bmhName, nodeName string) {
		out, err := oc.AsAdmin().WithoutNamespace().Run("annotate").Args("baremetalhosts", bmhName, "reboot.metal3.io=", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("annotated"))

		// Poll for node status to change to NotReady/Unknown
		checkNodeStatus(oc, 5*time.Second, 80*time.Second, nodeName, "Unknown")

		// Poll for node status to change to Ready
		checkNodeStatus(oc, 15*time.Second, 20*time.Minute, nodeName, "True")
	}

	// Helper function to verify ISO is attached by polling
	verifyISOAttached := func(nodeName string) {
		err := wait.Poll(15*time.Second, 10*time.Minute, func() (bool, error) {
			if checkISOAttachedOnNode(nodeName) {
				e2e.Logf("DataImage was attached")
				return true, nil
			}
			e2e.Logf("DataImage was not attached yet, checking again...")
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(err, "DataImage was not attached to the node as expected")
	}

	// Helper function to mount and verify ISO contents
	verifyISOContents := func(nodeName, mountPath string) {
		cmdReadme := fmt.Sprintf(`mkdir -p %s;
                mount -o loop /dev/sr0 %s;
                cat %s/readme`, mountPath, mountPath, mountPath)
		readMe, err := compat_otp.DebugNodeWithChroot(oc, nodeName, "bash", "-c", cmdReadme)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(readMe).To(o.ContainSubstring("Non bootable ISO"))
	}

	// Helper function to cleanup dataImage
	cleanupDataImage := func(bmhName, nodeName, dataPathCopy, mountPath string) {
		compat_otp.By("Cleanup: Clear DataImage URL to detach ISO")
		// Clear URL first, then wait for Metal3 to detach before rebooting
		compat_otp.ModifyYamlFileContent(dataPathCopy, []compat_otp.YamlReplace{
			{
				Path:  "spec",
				Value: "url: \"\"",
			},
		})
		_, err := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", dataPathCopy, "-n", machineAPINamespace).Output()
		if err != nil {
			e2e.Logf("Warning: Failed to clear DataImage URL: %v", err)
		}

		// Wait for Metal3 to process the URL change and detach virtual media
		compat_otp.By("Waiting for Metal3 to detach virtual media (60s)")
		e2e.Logf("Waiting 60 seconds for Metal3 to detach ISO from BMC virtual media...")
		time.Sleep(60 * time.Second)

		// Now it's safe to reboot - the ISO should be detached
		compat_otp.By("Trigger reboot to recover node")
		_, err = oc.AsAdmin().WithoutNamespace().Run("annotate").Args("baremetalhosts", bmhName, "reboot.metal3.io=", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to annotate BMH for reboot")

		// Poll for node status to change to NotReady
		checkNodeStatus(oc, 5*time.Second, 80*time.Second, nodeName, "Unknown")

		// Poll for node status to change to Ready
		checkNodeStatus(oc, 15*time.Second, 20*time.Minute, nodeName, "True")

		// Clean up DataImage
		compat_otp.By("Delete DataImage")
		_, err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("dataImage/"+bmhName, "-n", machineAPINamespace).Output()
		if err != nil {
			e2e.Logf("Warning: Failed to delete DataImage: %v", err)
		}

		// Clean up temporary files on the node
		cmdRm := fmt.Sprintf("rm -fr %s %s", mountPath, dataPathCopy)
		_, err = compat_otp.DebugNodeWithChroot(oc, nodeName, "bash", "-c", cmdRm)
		if err != nil {
			e2e.Logf("Warning: Failed to clean up files on node: %v", err)
		}
	}

	// author: sgoveas@redhat.com
	g.It("Author:sgoveas-Longduration-NonPreRelease-Medium-74736-Attach non-bootable iso to a worker node [Disruptive]", func() {
		cd := "/tmp/cdrom"

		compat_otp.By("9) Find a BMH that corresponds to a worker node")
		bmhName, nodeName := findBMHByNodeType(oc, "worker")
		if bmhName == "" {
			g.Skip("No BMH found that corresponds to a worker node")
		}

		compat_otp.By("10) Check no dataImage exists")
		checkNoDataImageExists(bmhName)

		compat_otp.By("11) Check if an image is already attached to the node")
		if checkISOAttachedOnNode(nodeName) {
			e2e.Logf("An image is already attached, dataImage should override")
		} else {
			e2e.Logf("No image attached")
		}

		compat_otp.By("12) Create dataImage")
		dataPathCopy := createDataImageYAML(bmhName, "worker")

		defer func() {
			cleanupDataImage(bmhName, nodeName, dataPathCopy, cd)
		}()

		compat_otp.By("13) Apply dataImage")
		applyDataImage(dataPathCopy, bmhName)

		compat_otp.By("14) Reboot baremetalhost and wait for node ready")
		rebootAndWaitForNode(bmhName, nodeName)

		compat_otp.By("15) Verify ISO image is attached to the node")
		verifyISOAttached(nodeName)

		compat_otp.By("16) Mount the iso image on the node to check contents")
		verifyISOContents(nodeName, cd)
	})
})
