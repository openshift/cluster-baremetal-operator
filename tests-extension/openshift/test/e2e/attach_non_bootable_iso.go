package baremetal

import (
	"fmt"
	"net/url"
	"os"
	"strings"
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
		redfishUrl      string
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

		// nginx-iso.yaml contains the base64 content of a gzip iso
		compat_otp.By("5) Create new project for nginx web-server")
		clusterDomain, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ingress.config/cluster", "-o=jsonpath={.spec.domain}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		isoUrl = "nb-iso." + clusterDomain
		nbIsoUrl = "http://" + isoUrl + "/non-bootable.iso"

		oc.SetupProject()
		testNamespace := oc.Namespace()

		compat_otp.By("6) Create web-server to host the iso file")
		nginxIso := testdata.FixturePath("nginx-iso.yaml")
		dcErr := oc.Run("create").Args("-f", nginxIso, "-n", testNamespace).Execute()
		o.Expect(dcErr).NotTo(o.HaveOccurred())

		// Wait for nginx pod to be ready with 3 minute timeout
		err = waitForPodReady(oc, "nginx-pod", testNamespace, 3*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("7) Create ingress to access the iso file")
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

	// author: sgoveas@redhat.com
	// port=unknown - no data in BigQuery last 60 days
	g.It("Author:sgoveas-Longduration-NonPreRelease-Medium-74737-Attach non-bootable iso to a master node [Disruptive]", func() {

		compat_otp.By("8) Find a BMH that corresponds to a master node and get BMC credentials")
		bmhName, nodeName := findBMHByNodeType(oc, "master")
		if bmhName == "" {
			g.Skip("No BMH found that corresponds to a master node")
		}

		bmcAddressUrl, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("baremetalhosts", bmhName, "-n", machineAPINamespace, "-o=jsonpath={.spec.bmc.address}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		bmcCredFile, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("baremetalhosts", bmhName, "-n", machineAPINamespace, "-o=jsonpath={.spec.bmc.credentialsName}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		bmcUser := getUserFromSecret(oc, machineAPINamespace, bmcCredFile)
		bmcPass := getPassFromSecret(oc, machineAPINamespace, bmcCredFile)

		compat_otp.By("9) Get redfish URL")
		bmcURL, err := url.Parse(bmcAddressUrl)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Failed to parse BMC address URL: %s", bmcAddressUrl))
		bmcAddress := bmcURL.Host
		bmcVendor, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("baremetalhosts", bmhName, "-n", machineAPINamespace, "-o=jsonpath={.status.hardware.systemVendor.manufacturer}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(bmcVendor, "Dell") {
			if bmcURL.Path != "" && bmcURL.Path != "/" {
				redfishUrl = fmt.Sprintf("https://%s%s/VirtualMedia/1", bmcAddress, strings.TrimRight(bmcURL.Path, "/"))
			} else {
				redfishUrl = fmt.Sprintf("https://%s/redfish/v1/Systems/System.Embedded.1/VirtualMedia/1", bmcAddress)
			}
		} else if strings.Contains(bmcVendor, "HPE") {
			redfishUrl = fmt.Sprintf("https://%s/redfish/v1/Managers/1/VirtualMedia/2", bmcAddress)
		} else {
			g.Skip(fmt.Sprintf("Unknown vendor %s, skipping test", bmcVendor))
		}

		compat_otp.By("10) Check no dataImage exists")
		out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("dataImage", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.ContainSubstring(bmhName))

		compat_otp.By("11) Check if an image is already attached to the node")
		setProxyEnv()
		img, err := redfishGet(redfishUrl, bmcUser, bmcPass)
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to query Redfish virtual media endpoint")
		if img != "" && img != "null" {
			e2e.Logf("An image is already attached (%s), dataImage should override", img)
		} else {
			e2e.Logf("No image attached")
		}
		unsetProxyEnv()

		compat_otp.By("11) Create dataImage")
		cd := "/tmp/cdrom"
		dataPath := testdata.FixturePath("non-bootable-iso.yaml")
		dataPathCopy := CopyToFile(dataPath, "non-bootable-iso-master.yaml")
		// Log only the filename to avoid exposing internal cluster hostname
		e2e.Logf("ISO filename: non-bootable.iso")
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

		defer func() {
			compat_otp.By("16) Cleanup: Clear DataImage URL to detach ISO")
			// CRITICAL FIX: Clear URL first, then wait for Metal3 to detach before rebooting
			compat_otp.ModifyYamlFileContent(dataPathCopy, []compat_otp.YamlReplace{
				{
					Path:  "spec",
					Value: "url: \"\"",
				},
			})
			_, err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", dataPathCopy, "-n", machineAPINamespace).Output()
			if err != nil {
				e2e.Logf("Warning: Failed to clear DataImage URL: %v", err)
			}

			// Wait for Metal3 to process the URL change and detach virtual media
			compat_otp.By("17) Waiting for Metal3 to detach virtual media (60s)")
			e2e.Logf("Waiting 60 seconds for Metal3 to detach ISO from BMC virtual media...")
			time.Sleep(60 * time.Second)

			// Now it's safe to reboot - the ISO should be detached
			compat_otp.By("18) Trigger reboot to recover node")
			_, err = oc.AsAdmin().WithoutNamespace().Run("annotate").Args("baremetalhosts", bmhName, "reboot.metal3.io=", "-n", machineAPINamespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to annotate BMH for reboot")

			// poll for node status to change to NotReady
			checkNodeStatus(oc, 5*time.Second, 80*time.Second, nodeName, "Unknown")

			// poll for node status to change to Ready
			checkNodeStatus(oc, 15*time.Second, 20*time.Minute, nodeName, "True")

			// Clean up DataImage
			compat_otp.By("19) Delete DataImage")
			_, err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("dataImage/"+bmhName, "-n", machineAPINamespace).Output()
			if err != nil {
				e2e.Logf("Warning: Failed to delete DataImage: %v", err)
			}

			// Clean up temporary files on the node
			cmdRm := `rm -fr %s %s`
			cmdRm = fmt.Sprintf(cmdRm, cd, dataPathCopy)
			_, err = compat_otp.DebugNodeWithChroot(oc, nodeName, "bash", "-c", cmdRm)
			if err != nil {
				e2e.Logf("Warning: Failed to clean up files on node: %v", err)
			}
		}()

		_, err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", dataPathCopy, "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("dataImage", "-n", machineAPINamespace, "-o=jsonpath={.items[0].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(bmhName))

		compat_otp.By("12) Reboot baremtalhost 'master-02'")
		out, err = oc.AsAdmin().WithoutNamespace().Run("annotate").Args("baremetalhosts", bmhName, "reboot.metal3.io=", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("annotated"))

		compat_otp.By("13) Waiting for the node to return to 'Ready' state")
		// poll for node status to change to NotReady
		checkNodeStatus(oc, 5*time.Second, 80*time.Second, nodeName, "Unknown")

		// poll for node status to change to Ready
		checkNodeStatus(oc, 15*time.Second, 20*time.Minute, nodeName, "True")

		compat_otp.By("14) Check ISO image is attached to the node")
		setProxyEnv()
		defer unsetProxyEnv()
		err = wait.Poll(15*time.Second, 60*time.Minute, func() (bool, error) {
			img, err := redfishGet(redfishUrl, bmcUser, bmcPass)
			if err != nil || !strings.Contains(img, ".iso") {
				e2e.Logf("dataImage was not attached, Checking again: %v", err)
				return false, nil
			}
			e2e.Logf("DataImage was attached")
			return true, nil
		})
		compat_otp.AssertWaitPollNoErr(err, "DataImage was not attached to the node as expected")
		unsetProxyEnv()

		compat_otp.By("15) Mount the iso image on the node to check contents")
		cmdReadme := fmt.Sprintf(`mkdir %s;
                mount -o loop /dev/sr0 %s;
                cat %s/readme`, cd, cd, cd)
		readMe, err := compat_otp.DebugNodeWithChroot(oc, nodeName, "bash", "-c", cmdReadme)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(readMe).To(o.ContainSubstring("Non bootable ISO"))

	})

	// author: sgoveas@redhat.com
	// port=unknown - no data in BigQuery last 60 days
	g.It("Author:sgoveas-Longduration-NonPreRelease-Medium-74736-Attach non-bootable iso to a worker node [Disruptive]", func() {

		compat_otp.By("8) Find a BMH that corresponds to a worker node and get BMC credentials")
		bmhName, nodeName := findBMHByNodeType(oc, "worker")
		if bmhName == "" {
			g.Skip("No BMH found that corresponds to a worker node")
		}

		bmcAddressUrl, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("baremetalhosts", bmhName, "-n", machineAPINamespace, "-o=jsonpath={.spec.bmc.address}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		bmcCredFile, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("baremetalhosts", bmhName, "-n", machineAPINamespace, "-o=jsonpath={.spec.bmc.credentialsName}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		bmcUser := getUserFromSecret(oc, machineAPINamespace, bmcCredFile)
		bmcPass := getPassFromSecret(oc, machineAPINamespace, bmcCredFile)

		compat_otp.By("9) Get redfish URL")
		bmcURL, err := url.Parse(bmcAddressUrl)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Failed to parse BMC address URL: %s", bmcAddressUrl))
		bmcAddress := bmcURL.Host
		bmcVendor, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("baremetalhosts", bmhName, "-n", machineAPINamespace, "-o=jsonpath={.status.hardware.systemVendor.manufacturer}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(bmcVendor, "Dell") {
			if bmcURL.Path != "" && bmcURL.Path != "/" {
				redfishUrl = fmt.Sprintf("https://%s%s/VirtualMedia/1", bmcAddress, strings.TrimRight(bmcURL.Path, "/"))
			} else {
				redfishUrl = fmt.Sprintf("https://%s/redfish/v1/Systems/System.Embedded.1/VirtualMedia/1", bmcAddress)
			}
		} else if strings.Contains(bmcVendor, "HPE") {
			redfishUrl = fmt.Sprintf("https://%s/redfish/v1/Managers/1/VirtualMedia/2", bmcAddress)
		} else {
			g.Skip(fmt.Sprintf("Unknown vendor %s, skipping test", bmcVendor))
		}

		compat_otp.By("10) Check no dataImage exists")
		out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("dataImage", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.ContainSubstring(bmhName))

		compat_otp.By("11) Check if an image is already attached to the node")
		setProxyEnv()
		img, err := redfishGet(redfishUrl, bmcUser, bmcPass)
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to query Redfish virtual media endpoint")
		if img != "" && img != "null" {
			e2e.Logf("An image is already attached (%s), dataImage should override", img)
		} else {
			e2e.Logf("No image attached")
		}
		unsetProxyEnv()

		compat_otp.By("11) Create dataImage")
		cd := "/tmp/cdrom"
		dataPath := testdata.FixturePath("non-bootable-iso.yaml")
		dataPathCopy := CopyToFile(dataPath, "non-bootable-iso-worker.yaml")
		// Log only the filename to avoid exposing internal cluster hostname
		e2e.Logf("ISO filename: non-bootable.iso")
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

		defer func() {
			compat_otp.By("16) Cleanup: Clear DataImage URL to detach ISO")
			// CRITICAL FIX: Clear URL first, then wait for Metal3 to detach before rebooting
			compat_otp.ModifyYamlFileContent(dataPathCopy, []compat_otp.YamlReplace{
				{
					Path:  "spec",
					Value: "url: \"\"",
				},
			})
			_, err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", dataPathCopy, "-n", machineAPINamespace).Output()
			if err != nil {
				e2e.Logf("Warning: Failed to clear DataImage URL: %v", err)
			}

			// Wait for Metal3 to process the URL change and detach virtual media
			compat_otp.By("17) Waiting for Metal3 to detach virtual media (60s)")
			e2e.Logf("Waiting 60 seconds for Metal3 to detach ISO from BMC virtual media...")
			time.Sleep(60 * time.Second)

			// Now it's safe to reboot - the ISO should be detached
			compat_otp.By("18) Trigger reboot to recover node")
			_, err = oc.AsAdmin().WithoutNamespace().Run("annotate").Args("baremetalhosts", bmhName, "reboot.metal3.io=", "-n", machineAPINamespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to annotate BMH for reboot")

			// poll for node status to change to NotReady
			checkNodeStatus(oc, 5*time.Second, 80*time.Second, nodeName, "Unknown")

			// poll for node status to change to Ready
			checkNodeStatus(oc, 15*time.Second, 20*time.Minute, nodeName, "True")

			// Clean up DataImage
			compat_otp.By("19) Delete DataImage")
			_, err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("dataImage/"+bmhName, "-n", machineAPINamespace).Output()
			if err != nil {
				e2e.Logf("Warning: Failed to delete DataImage: %v", err)
			}

			// Clean up temporary files on the node
			cmdRm := `rm -fr %s %s`
			cmdRm = fmt.Sprintf(cmdRm, cd, dataPathCopy)
			_, err = compat_otp.DebugNodeWithChroot(oc, nodeName, "bash", "-c", cmdRm)
			if err != nil {
				e2e.Logf("Warning: Failed to clean up files on node: %v", err)
			}
		}()

		_, err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", dataPathCopy, "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("dataImage", "-n", machineAPINamespace, "-o=jsonpath={.items[0].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(bmhName))

		compat_otp.By("12) Reboot baremtalhost 'worker-00'")
		out, err = oc.AsAdmin().WithoutNamespace().Run("annotate").Args("baremetalhosts", bmhName, "reboot.metal3.io=", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("annotated"))

		compat_otp.By("13) Waiting for the node to return to 'Ready' state")
		// poll for node status to change to NotReady
		checkNodeStatus(oc, 5*time.Second, 80*time.Second, nodeName, "Unknown")

		// poll for node status to change to Ready
		checkNodeStatus(oc, 15*time.Second, 20*time.Minute, nodeName, "True")

		compat_otp.By("14) Check ISO image is attached to the node")
		setProxyEnv()
		defer unsetProxyEnv()
		err = wait.Poll(5*time.Second, 60*time.Minute, func() (bool, error) {
			img, err = redfishGet(redfishUrl, bmcUser, bmcPass)
			if err != nil || !strings.Contains(img, ".iso") {
				e2e.Logf("dataImage was not attached, Checking again: %v", err)
				return false, nil
			}
			e2e.Logf("DataImage was attached")
			return true, nil
		})
		compat_otp.AssertWaitPollNoErr(err, "DataImage was not attached to the node as expected")
		unsetProxyEnv()

		compat_otp.By("15) Mount the iso image on the node to check contents")
		cmdReadme := fmt.Sprintf(`mkdir %s;
                mount -o loop /dev/sr0 %s;
                cat %s/readme`, cd, cd, cd)
		readMe, err := compat_otp.DebugNodeWithChroot(oc, nodeName, "bash", "-c", cmdReadme)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(readMe).To(o.ContainSubstring("Non bootable ISO"))
	})
})
