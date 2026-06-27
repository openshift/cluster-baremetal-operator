package baremetal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[OTP][sig-baremetal] INSTALLER IPI for INSTALLER_DEDICATED job on BareMetal", func() {
	defer g.GinkgoRecover()
	var (
		oc           = compat_otp.NewCLI("host-firmware-components", compat_otp.KubeConfigPath())
		iaasPlatform string
		dirname      string
	)
	g.BeforeEach(func() {
		compat_otp.SkipForSNOCluster(oc)
		iaasPlatform = compat_otp.CheckPlatform(oc)
		if !(iaasPlatform == "baremetal") {
			e2e.Logf("Cluster is: %s", iaasPlatform)
			g.Skip("For Non-baremetal cluster, this is not supported!")
		}
	})
	// author: jhajyahy@redhat.com
	// port=unknown - no data in BigQuery last 60 days
	g.It("Author:jhajyahy-Longduration-NonPreRelease-Medium-75430-Update host FW via HostFirmwareComponents CRD [Disruptive]", func() {
		dirname = "OCP-75430.log"
		host, getBmhErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, "-o=jsonpath={.items[4].metadata.name}").Output()
		if getBmhErr != nil || host == "" {
			g.Skip("Not enough BareMetalHosts (need at least 5) to run this test")
		}
		vendor, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, host, "-o=jsonpath={.status.hardware.firmware.bios.vendor}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		initialVersion, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "-o=jsonpath={.status.components[1].currentVersion}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		oc.SetupProject()
		testNamespace := oc.Namespace()

		downloadUrl, fileName := buildFirmwareURL(vendor, initialVersion)

		// Label worker node 1 to run the web-server hosting the iso
		compat_otp.By("Add a label to first worker node ")
		workerNode, err := compat_otp.GetClusterNodesBy(oc, "worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		nginxNode := workerNode[0]
		compat_otp.AddLabelToNode(oc, nginxNode, "nginx-node", "true")

		compat_otp.By("Create web-server to host the fw file")
		BaseDir := compat_otp.FixturePath("testdata", "installer")
		fwConfigmap := filepath.Join(BaseDir, "baremetal", "firmware-cm.yaml")
		nginxFW := filepath.Join(BaseDir, "baremetal", "nginx-firmware.yaml")
		compat_otp.ModifyYamlFileContent(fwConfigmap, []compat_otp.YamlReplace{
			{
				Path:  "data.firmware_url",
				Value: downloadUrl,
			},
			{
				Path:  "data.component",
				Value: "bmc",
			},
		})

		dcErr := oc.Run("create").Args("-f", fwConfigmap, "-n", testNamespace).Execute()
		o.Expect(dcErr).NotTo(o.HaveOccurred())

		dcErr = oc.Run("create").Args("-f", nginxFW, "-n", testNamespace).Execute()
		o.Expect(dcErr).NotTo(o.HaveOccurred())
		compat_otp.AssertPodToBeReady(oc, "nginx-pod", testNamespace)

		compat_otp.By("Create ingress to access the iso file")
		fileIngress := filepath.Join(BaseDir, "baremetal", "nginx-ingress.yaml")
		nginxIngress := CopyToFile(fileIngress, "nginx-ingress.yaml")
		clusterDomain, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ingress.config/cluster", "-o=jsonpath={.spec.domain}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		fwUrl := "fw." + clusterDomain
		defer func() {
		if err := os.Remove(nginxIngress); err != nil && !os.IsNotExist(err) {
			e2e.Logf("Warning: Failed to cleanup temporary file %s: %v", nginxIngress, err)
		}
	}()
		compat_otp.ModifyYamlFileContent(nginxIngress, []compat_otp.YamlReplace{
			{
				Path:  "spec.rules.0.host",
				Value: fwUrl,
			},
		})

		IngErr := oc.Run("create").Args("-f", nginxIngress, "-n", testNamespace).Execute()
		o.Expect(IngErr).NotTo(o.HaveOccurred())

		compat_otp.By("Update HFC CRD")
		component := "bmc"
		hfcUrl := "http://" + fwUrl + "/" + fileName
		patchConfig := fmt.Sprintf(`[{"op": "replace", "path": "/spec/updates", "value": [{"component":"%s","url":"%s"}]}]`, component, hfcUrl)
		patchErr := oc.AsAdmin().WithoutNamespace().Run("patch").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "--type=json", "-p", patchConfig).Execute()
		o.Expect(patchErr).NotTo(o.HaveOccurred())
		bmcUrl, err := oc.AsAdmin().Run("get").Args("-n", machineAPINamespace, "hostfirmwarecomponents", host, "-o=jsonpath={.spec.updates[0].url}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(bmcUrl).Should(o.Equal(hfcUrl))

		defer func() {
			patchConfig := `[{"op": "replace", "path": "/spec/updates", "value": []}]`
			patchErr := oc.AsAdmin().WithoutNamespace().Run("patch").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "--type=json", "-p", patchConfig).Execute()
			o.Expect(patchErr).NotTo(o.HaveOccurred())
			compat_otp.DeleteLabelFromNode(oc, nginxNode, "nginx-node")
			nodeHealthErr := clusterNodesHealthcheck(oc, 1500)
			compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Cluster did not recover in time!")
			clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1500, dirname)
			compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators did not recover in time!")
		}()

		compat_otp.By("Get machine name of host")
		machine, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, host, "-o=jsonpath={.spec.consumerRef.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Get the origin number of replicas
		machineSet, cmdErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", "-n", machineAPINamespace, "-o=jsonpath={.items[0].metadata.name}").Output()
		o.Expect(cmdErr).NotTo(o.HaveOccurred())
		originReplicasStr, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", machineSet, "-n", machineAPINamespace, "-o=jsonpath={.spec.replicas}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("Annotate worker-01 machine for deletion")
		_, err = oc.AsAdmin().WithoutNamespace().Run("annotate").Args("machine", machine, "machine.openshift.io/cluster-api-delete-machine=yes", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("Scale down machineset")
		originReplicas, err := strconv.Atoi(originReplicasStr)
		o.Expect(err).NotTo(o.HaveOccurred())
		newReplicas := originReplicas - 1
		_, err = oc.AsAdmin().WithoutNamespace().Run("scale").Args("machineset", machineSet, "-n", machineAPINamespace, fmt.Sprintf("--replicas=%d", newReplicas)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForBMHState(oc, host, "available")

		defer func() {
			currentReplicasStr, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", machineSet, "-n", machineAPINamespace, "-o=jsonpath={.spec.replicas}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			if currentReplicasStr != originReplicasStr {
				_, err := oc.AsAdmin().WithoutNamespace().Run("scale").Args("machineset", machineSet, "-n", machineAPINamespace, fmt.Sprintf("--replicas=%s", originReplicasStr)).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				nodeHealthErr := clusterNodesHealthcheck(oc, 1500)
				compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Cluster did not recover in time!")
				clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1500, dirname)
				compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators did not recover in time!")
			}
		}()

		compat_otp.By("Scale up machineset")
		_, err = oc.AsAdmin().WithoutNamespace().Run("scale").Args("machineset", machineSet, "-n", machineAPINamespace, fmt.Sprintf("--replicas=%s", originReplicasStr)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForBMHState(oc, host, "provisioned")
		nodeHealthErr := clusterNodesHealthcheck(oc, 1500)
		compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Cluster did not recover in time!")
		clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1500, dirname)
		compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators did not recover in time!")

		currentVersion, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "-o=jsonpath={.status.components[1].currentVersion}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(currentVersion).ShouldNot(o.Equal(initialVersion))

	})

	// author: jhajyahy@redhat.com
	// port=unknown - no data in BigQuery last 60 days
	g.It("Author:jhajyahy-Longduration-NonPreRelease-Medium-77676-DAY2 Update HFS via HostUpdatePolicy CRD [Disruptive]", func() {
		dirname = "OCP-77676.log"
		host, getBmhErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, "-o=jsonpath={.items[4].metadata.name}").Output()
		if getBmhErr != nil || host == "" {
			g.Skip("Not enough BareMetalHosts (need at least 5) to run this test")
		}

		g.By("Get node name from BMH mapping")
		machine, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, host, "-o=jsonpath={.spec.consumerRef.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machines.machine.openshift.io", "-n", machineAPINamespace, machine, "-o=jsonpath={.status.nodeRef.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("Create host update policy")
		BaseDir := compat_otp.FixturePath("testdata", "installer")
		hostUpdatePolicy := filepath.Join(BaseDir, "baremetal", "host-update-policy.yaml")
		compat_otp.ModifyYamlFileContent(hostUpdatePolicy, []compat_otp.YamlReplace{
			{
				Path:  "metadata.name",
				Value: host,
			},
		})

		dcErr := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", hostUpdatePolicy, "-n", machineAPINamespace).Execute()
		o.Expect(dcErr).NotTo(o.HaveOccurred())
		defer func() {
			err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("HostUpdatePolicy", "-n", machineAPINamespace, host).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			nodeHealthErr := clusterNodesHealthcheck(oc, 3000)
			compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Cluster did not recover in time!")
			clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1500, dirname)
			compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators did not recover in time!")
		}()

		compat_otp.By("Update HFS setting based on vendor")
		vendor, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, host, "-o=jsonpath={.status.hardware.firmware.bios.vendor}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		hfs, value, err := getHfsByVendor(oc, vendor, machineAPINamespace, host)
		o.Expect(err).NotTo(o.HaveOccurred())
		patchConfig := fmt.Sprintf(`[{"op": "replace", "path": "/spec/settings/%s", "value": "%s"}]`, hfs, value)
		patchErr := oc.AsAdmin().WithoutNamespace().Run("patch").Args("hfs", "-n", machineAPINamespace, host, "--type=json", "-p", patchConfig).Execute()
		o.Expect(patchErr).NotTo(o.HaveOccurred())
		defer func() {
			patchConfig := `[{"op": "replace", "path": "/spec/settings", "value": {}}]`
			patchErr := oc.AsAdmin().WithoutNamespace().Run("patch").Args("hfs", "-n", machineAPINamespace, host, "--type=json", "-p", patchConfig).Execute()
			o.Expect(patchErr).NotTo(o.HaveOccurred())
		}()

		specModified, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("hfs", "-n", machineAPINamespace, host, fmt.Sprintf("-o=jsonpath={.spec.settings.%s}", hfs)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(specModified).Should(o.Equal(value))

		compat_otp.By("Reboot baremtalhost worker-01")
		out, err := oc.AsAdmin().WithoutNamespace().Run("annotate").Args("baremetalhosts", host, "reboot.metal3.io=", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("annotated"))

		compat_otp.By("Waiting for the node to return to 'Ready' state")
		// poll for node status to change to NotReady
		err = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", nodeName, "-o=jsonpath={.status.conditions[?(@.type==\"Ready\")].status}").Output()
			if err != nil || string(output) == "True" {
				e2e.Logf("Node is available, status: %s. Trying again", output)
				return false, nil
			}
			if string(output) == "Unknown" {
				e2e.Logf("Node is Ready, status: %s", output)
				return true, nil
			}
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(err, "Node did not change state as expected")

		nodeHealthErr := clusterNodesHealthcheck(oc, 3000)
		compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Cluster did not recover in time!")
		clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1500, dirname)
		compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators did not recover in time!")

		compat_otp.By("Verify hfs setting was actually changed")
		statusModified, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("hfs", "-n", machineAPINamespace, host, fmt.Sprintf("-o=jsonpath={.status.settings.%s}", hfs)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(statusModified).Should(o.Equal(specModified))

	})

	// author: jhajyahy@redhat.com
	// port=unknown - no data in BigQuery last 60 days
	g.It("Author:jhajyahy-Longduration-NonPreRelease-Medium-78361-DAY2 Update host FW via HostFirmwareComponents CRD [Disruptive]", func() {
		dirname = "OCP-78361.log"

		// Dynamically find a provisioned worker BMH instead of hardcoding items[4]
		compat_otp.By("Find a provisioned worker BareMetalHost for testing")
		bmhList, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, "-o=json").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		var bmhJSON struct {
			Items []struct {
				Metadata struct {
					Name string `json:"name"`
				} `json:"metadata"`
				Spec struct {
					ConsumerRef *struct {
						Name string `json:"name"`
					} `json:"consumerRef"`
				} `json:"spec"`
				Status struct {
					Hardware struct {
						Firmware struct {
							BIOS struct {
								Vendor string `json:"vendor"`
							} `json:"bios"`
						} `json:"firmware"`
					} `json:"hardware"`
				} `json:"status"`
			} `json:"items"`
		}
		err = json.Unmarshal([]byte(bmhList), &bmhJSON)
		o.Expect(err).NotTo(o.HaveOccurred())

		var host, vendor, machine, nodeName string
		for _, bmh := range bmhJSON.Items {
			if bmh.Spec.ConsumerRef != nil && bmh.Spec.ConsumerRef.Name != "" {
				// Get machine to check if it's a worker
				machineRole, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machines.machine.openshift.io", "-n", machineAPINamespace, bmh.Spec.ConsumerRef.Name, "-o=jsonpath={.metadata.labels.machine\\.openshift\\.io/cluster-api-machine-role}").Output()
				if err == nil && machineRole == "worker" {
					host = bmh.Metadata.Name
					vendor = bmh.Status.Hardware.Firmware.BIOS.Vendor
					machine = bmh.Spec.ConsumerRef.Name
					// Get the node name for this BMH
					nodeName, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("machines.machine.openshift.io", "-n", machineAPINamespace, machine, "-o=jsonpath={.status.nodeRef.name}").Output()
					if err == nil && nodeName != "" {
						break
					}
				}
			}
		}

		if host == "" || nodeName == "" {
			g.Skip("No suitable provisioned worker BareMetalHost found for firmware update test")
		}

		e2e.Logf("Selected BMH: %s, Node: %s, Vendor: %s", host, nodeName, vendor)

		initialVersion, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "-o=jsonpath={.status.components[1].currentVersion}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("Create host update policy")
		BaseDir := compat_otp.FixturePath("testdata", "installer")
		hostUpdatePolicy := filepath.Join(BaseDir, "baremetal", "host-update-policy.yaml")
		compat_otp.ModifyYamlFileContent(hostUpdatePolicy, []compat_otp.YamlReplace{
			{
				Path:  "metadata.name",
				Value: host,
			},
		})

		dcErr := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", hostUpdatePolicy, "-n", machineAPINamespace).Execute()
		o.Expect(dcErr).NotTo(o.HaveOccurred())
		defer func() {
			err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("HostUpdatePolicy", "-n", machineAPINamespace, host).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			nodeHealthErr := clusterNodesHealthcheck(oc, 3000)
			compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Cluster did not recover in time!")
			clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1500, dirname)
			compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators did not recover in time!")
		}()

		oc.SetupProject()
		testNamespace := oc.Namespace()

		downloadUrl, fileName := buildFirmwareURL(vendor, initialVersion)

		// Select a worker node for nginx that's different from the node being rebooted
		compat_otp.By("Select a worker node for nginx pod (different from node being rebooted)")
		workerNodes, err := compat_otp.GetClusterNodesBy(oc, "worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(workerNodes)).Should(o.BeNumerically(">=", 2), "Need at least 2 worker nodes for this test")

		var nginxNode string
		for _, node := range workerNodes {
			if node != nodeName {
				nginxNode = node
				break
			}
		}
		o.Expect(nginxNode).ShouldNot(o.BeEmpty(), "Failed to find a worker node different from the one being rebooted")
		e2e.Logf("Using worker node %s for nginx pod (rebooting node %s)", nginxNode, nodeName)

		compat_otp.AddLabelToNode(oc, nginxNode, "nginx-node", "true")

		compat_otp.By("Create web-server to host the fw file")
		BaseDir = compat_otp.FixturePath("testdata", "installer")
		fwConfigmap := filepath.Join(BaseDir, "baremetal", "firmware-cm.yaml")
		nginxFW := filepath.Join(BaseDir, "baremetal", "nginx-firmware.yaml")
		compat_otp.ModifyYamlFileContent(fwConfigmap, []compat_otp.YamlReplace{
			{
				Path:  "data.firmware_url",
				Value: downloadUrl,
			},
			{
				Path:  "data.component",
				Value: "bmc",
			},
		})

		dcErr = oc.Run("create").Args("-f", fwConfigmap, "-n", testNamespace).Execute()
		o.Expect(dcErr).NotTo(o.HaveOccurred())

		dcErr = oc.Run("create").Args("-f", nginxFW, "-n", testNamespace).Execute()
		o.Expect(dcErr).NotTo(o.HaveOccurred())
		compat_otp.AssertPodToBeReady(oc, "nginx-pod", testNamespace)

		compat_otp.By("Create ingress to access the iso file")
		fileIngress := filepath.Join(BaseDir, "baremetal", "nginx-ingress.yaml")
		nginxIngress := CopyToFile(fileIngress, "nginx-ingress.yaml")
		clusterDomain, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ingress.config/cluster", "-o=jsonpath={.spec.domain}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		fwUrl := "fw." + clusterDomain
		defer func() {
		if err := os.Remove(nginxIngress); err != nil && !os.IsNotExist(err) {
			e2e.Logf("Warning: Failed to cleanup temporary file %s: %v", nginxIngress, err)
		}
	}()
		compat_otp.ModifyYamlFileContent(nginxIngress, []compat_otp.YamlReplace{
			{
				Path:  "spec.rules.0.host",
				Value: fwUrl,
			},
		})

		IngErr := oc.Run("create").Args("-f", nginxIngress, "-n", testNamespace).Execute()
		o.Expect(IngErr).NotTo(o.HaveOccurred())

		compat_otp.By("Update HFC CRD")
		component := "bmc"
		hfcUrl := "http://" + fwUrl + "/" + fileName
		patchConfig := fmt.Sprintf(`[{"op": "replace", "path": "/spec/updates", "value": [{"component":"%s","url":"%s"}]}]`, component, hfcUrl)
		patchErr := oc.AsAdmin().WithoutNamespace().Run("patch").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "--type=json", "-p", patchConfig).Execute()
		o.Expect(patchErr).NotTo(o.HaveOccurred())
		bmcUrl, err := oc.AsAdmin().Run("get").Args("-n", machineAPINamespace, "hostfirmwarecomponents", host, "-o=jsonpath={.spec.updates[0].url}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(bmcUrl).Should(o.Equal(hfcUrl))

		defer func() {
			patchConfig := `[{"op": "replace", "path": "/spec/updates", "value": []}]`
			patchErr := oc.AsAdmin().WithoutNamespace().Run("patch").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "--type=json", "-p", patchConfig).Execute()
			o.Expect(patchErr).NotTo(o.HaveOccurred())
			compat_otp.DeleteLabelFromNode(oc, nginxNode, "nginx-node")
			nodeHealthErr := clusterNodesHealthcheck(oc, 3000)
			compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Cluster did not recover in time!")
			clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1500, dirname)
			compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators did not recover in time!")
		}()

		g.By("Reboot baremtalhost 'worker-01'")
		out, err := oc.AsAdmin().WithoutNamespace().Run("annotate").Args("baremetalhosts", host, "reboot.metal3.io=", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("annotated"))

		g.By("Waiting for the node to transition to 'NotReady' state")
		// poll for node status to change to NotReady

		err = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", nodeName, "-o=jsonpath={.status.conditions[?(@.type==\"Ready\")].status}").Output()
			if err != nil || string(output) == "True" {
				e2e.Logf("Node is available, status: %s. Trying again", output)
				return false, nil
			}
			if string(output) == "Unknown" {
				e2e.Logf("Node is Ready, status: %s", output)
				return true, nil
			}
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(err, "Node did not change state as expected")

		nodeHealthErr := clusterNodesHealthcheck(oc, 3000)
		compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Cluster did not recover in time!")
		clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1500, dirname)
		compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators did not recover in time!")

		currentVersion, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "-o=jsonpath={.status.components[1].currentVersion}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(currentVersion).ShouldNot(o.Equal(initialVersion))

	})

	// author: jhajyahy@redhat.com
	// port=unknown - no data in BigQuery last 60 days
	g.It("Author:jhajyahy-Longduration-NonPreRelease-Medium-84342-Update NIC FW using HostFirmwareComponents CRD [Disruptive]", func() {
		dirname = "OCP-78361.log"
		host, getBmhErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, "-o=jsonpath={.items[4].metadata.name}").Output()
		if getBmhErr != nil || host == "" {
			g.Skip("Not enough BareMetalHosts (need at least 5) to run this test")
		}
		vendor, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, host, "-o=jsonpath={.status.hardware.firmware.bios.vendor}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		if vendor == "HPE" {
			e2e.Logf("vendor is: %s", vendor)
			g.Skip("Test not supported on vendor HPE")
		}

		nicName := getNicNameByVendor(vendor)
		nicComponent := "nic:" + nicName
		jsonPath := fmt.Sprintf("{.status.components[?(@.component==\"%s\")].currentVersion}", nicComponent)
		initialVersion, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "-o=jsonpath="+jsonPath).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("Create host update policy")
		BaseDir := compat_otp.FixturePath("testdata", "installer")
		hostUpdatePolicy := filepath.Join(BaseDir, "baremetal", "host-update-policy.yaml")
		compat_otp.ModifyYamlFileContent(hostUpdatePolicy, []compat_otp.YamlReplace{
			{
				Path:  "metadata.name",
				Value: host,
			},
		})

		dcErr := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", hostUpdatePolicy, "-n", machineAPINamespace).Execute()
		o.Expect(dcErr).NotTo(o.HaveOccurred())
		defer func() {
			err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("HostUpdatePolicy", "-n", machineAPINamespace, host).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			nodeHealthErr := clusterNodesHealthcheck(oc, 3000)
			compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Cluster did not recover in time!")
			clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1500, dirname)
			compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators did not recover in time!")
		}()

		oc.SetupProject()
		testNamespace := oc.Namespace()

		downloadUrl, fileName := getNicFwDetails(vendor, initialVersion)

		// Get node name from BMH mapping for reboot status checking
		compat_otp.By("Get node name from BMH mapping")
		machine, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, host, "-o=jsonpath={.spec.consumerRef.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machines.machine.openshift.io", "-n", machineAPINamespace, machine, "-o=jsonpath={.status.nodeRef.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Label worker node 1 to run the web-server hosting the iso
		compat_otp.By("Add a label to first worker node ")
		workerNode, err := compat_otp.GetClusterNodesBy(oc, "worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		nginxNode := workerNode[0]
		compat_otp.AddLabelToNode(oc, nginxNode, "nginx-node", "true")

		compat_otp.By("Create web-server to host the fw file")
		BaseDir = compat_otp.FixturePath("testdata", "installer")
		fwConfigmap := filepath.Join(BaseDir, "baremetal", "firmware-cm.yaml")
		nginxFW := filepath.Join(BaseDir, "baremetal", "nginx-firmware.yaml")
		compat_otp.ModifyYamlFileContent(fwConfigmap, []compat_otp.YamlReplace{
			{
				Path:  "data.firmware_url",
				Value: downloadUrl,
			},
			{
				Path:  "data.component",
				Value: "nic",
			},
		})

		dcErr = oc.Run("create").Args("-f", fwConfigmap, "-n", testNamespace).Execute()
		o.Expect(dcErr).NotTo(o.HaveOccurred())

		dcErr = oc.Run("create").Args("-f", nginxFW, "-n", testNamespace).Execute()
		o.Expect(dcErr).NotTo(o.HaveOccurred())
		compat_otp.AssertPodToBeReady(oc, "nginx-pod", testNamespace)

		compat_otp.By("Create ingress to access the iso file")
		fileIngress := filepath.Join(BaseDir, "baremetal", "nginx-ingress.yaml")
		nginxIngress := CopyToFile(fileIngress, "nginx-ingress.yaml")
		clusterDomain, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ingress.config/cluster", "-o=jsonpath={.spec.domain}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		fwUrl := "fw." + clusterDomain
		defer func() {
			if err := os.Remove(nginxIngress); err != nil && !os.IsNotExist(err) {
				e2e.Logf("Warning: Failed to cleanup temporary file %s: %v", nginxIngress, err)
			}
		}()
		compat_otp.ModifyYamlFileContent(nginxIngress, []compat_otp.YamlReplace{
			{
				Path:  "spec.rules.0.host",
				Value: fwUrl,
			},
		})

		IngErr := oc.Run("create").Args("-f", nginxIngress, "-n", testNamespace).Execute()
		o.Expect(IngErr).NotTo(o.HaveOccurred())

		compat_otp.By("Update HFC CRD")
		nicUrl := "http://" + fwUrl + "/" + fileName
		patchConfig := fmt.Sprintf(`[{"op": "replace", "path": "/spec/updates", "value": [{"component":"%s","url":"%s"}]}]`, nicComponent, nicUrl)
		patchErr := oc.AsAdmin().WithoutNamespace().Run("patch").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "--type=json", "-p", patchConfig).Execute()
		o.Expect(patchErr).NotTo(o.HaveOccurred())
		bmcUrl, err := oc.AsAdmin().Run("get").Args("-n", machineAPINamespace, "hostfirmwarecomponents", host, "-o=jsonpath={.spec.updates[0].url}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(bmcUrl).Should(o.Equal(nicUrl))

		defer func() {
			patchConfig := `[{"op": "replace", "path": "/spec/updates", "value": []}]`
			patchErr := oc.AsAdmin().WithoutNamespace().Run("patch").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "--type=json", "-p", patchConfig).Execute()
			o.Expect(patchErr).NotTo(o.HaveOccurred())
			compat_otp.DeleteLabelFromNode(oc, nginxNode, "nginx-node")
			nodeHealthErr := clusterNodesHealthcheck(oc, 3000)
			compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Cluster did not recover in time!")
			clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1500, dirname)
			compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators did not recover in time!")
		}()

		g.By("Reboot baremtalhost 'worker-01'")
		out, err := oc.AsAdmin().WithoutNamespace().Run("annotate").Args("baremetalhosts", host, "reboot.metal3.io=", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("annotated"))

		g.By("Waiting for the node to transition to 'NotReady' state")
		// poll for node status to change to NotReady
		checkNodeStatus(oc, 5*time.Second, 2*time.Minute, nodeName, "Unknown")

		nodeHealthErr := clusterNodesHealthcheck(oc, 3000)
		compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Cluster did not recover in time!")
		clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1500, dirname)
		compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators did not recover in time!")

		// Poll for firmware version update before final assertion
		var currentVersion string
		pollErr := wait.Poll(10*time.Second, 5*time.Minute, func() (bool, error) {
			ver, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "-o=jsonpath="+jsonPath).Output()
			if err != nil {
				return false, nil // keep polling on error
			}
			if ver != initialVersion {
				currentVersion = ver
				return true, nil
			}
			return false, nil
		})
		o.Expect(pollErr).NotTo(o.HaveOccurred(), "Polling for firmware version update failed")
		o.Expect(currentVersion).ShouldNot(o.Equal(initialVersion))

	})

	// author: jhajyahy@redhat.com
	// port=unknown - no data in BigQuery last 60 days
	g.It("Author:jhajyahy-Longduration-NonPreRelease-Medium-84372-Day1-Update NIC FW using HostFirmwareComponents CRD [Disruptive]", func() {
		dirname = "OCP-84372.log"
		host, getBmhErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, "-o=jsonpath={.items[4].metadata.name}").Output()
		if getBmhErr != nil || host == "" {
			g.Skip("Not enough BareMetalHosts (need at least 5) to run this test")
		}
		vendor, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, host, "-o=jsonpath={.status.hardware.firmware.bios.vendor}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		if vendor == "HPE" {
			e2e.Logf("vendor is: %s", vendor)
			g.Skip("Test not supported on vendor HPE")
		}

		nicName := getNicNameByVendor(vendor)
		nicComponent := "nic:" + nicName
		jsonPath := fmt.Sprintf("{.status.components[?(@.component==\"%s\")].currentVersion}", nicComponent)
		initialVersion, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "-o=jsonpath="+jsonPath).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		oc.SetupProject()
		testNamespace := oc.Namespace()

		downloadUrl, fileName := getNicFwDetails(vendor, initialVersion)

		// Label worker node 1 to run the web-server hosting the iso
		compat_otp.By("Add a label to first worker node ")
		workerNode, err := compat_otp.GetClusterNodesBy(oc, "worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		nginxNode := workerNode[0]
		compat_otp.AddLabelToNode(oc, nginxNode, "nginx-node", "true")

		compat_otp.By("Create web-server to host the fw file")
		BaseDir := compat_otp.FixturePath("testdata", "installer")
		fwConfigmap := filepath.Join(BaseDir, "baremetal", "firmware-cm.yaml")
		nginxFW := filepath.Join(BaseDir, "baremetal", "nginx-firmware.yaml")
		compat_otp.ModifyYamlFileContent(fwConfigmap, []compat_otp.YamlReplace{
			{
				Path:  "data.firmware_url",
				Value: downloadUrl,
			},
			{
				Path:  "data.component",
				Value: "nic",
			},
		})

		dcErr := oc.Run("create").Args("-f", fwConfigmap, "-n", testNamespace).Execute()
		o.Expect(dcErr).NotTo(o.HaveOccurred())

		dcErr = oc.Run("create").Args("-f", nginxFW, "-n", testNamespace).Execute()
		o.Expect(dcErr).NotTo(o.HaveOccurred())
		compat_otp.AssertPodToBeReady(oc, "nginx-pod", testNamespace)

		compat_otp.By("Create ingress to access the iso file")
		fileIngress := filepath.Join(BaseDir, "baremetal", "nginx-ingress.yaml")
		nginxIngress := CopyToFile(fileIngress, "nginx-ingress.yaml")
		clusterDomain, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ingress.config/cluster", "-o=jsonpath={.spec.domain}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		fwUrl := "fw." + clusterDomain
		defer func() {
			if err := os.Remove(nginxIngress); err != nil && !os.IsNotExist(err) {
				e2e.Logf("Warning: Failed to cleanup temporary file %s: %v", nginxIngress, err)
			}
		}()
		compat_otp.ModifyYamlFileContent(nginxIngress, []compat_otp.YamlReplace{
			{
				Path:  "spec.rules.0.host",
				Value: fwUrl,
			},
		})

		IngErr := oc.Run("create").Args("-f", nginxIngress, "-n", testNamespace).Execute()
		o.Expect(IngErr).NotTo(o.HaveOccurred())

		compat_otp.By("Update HFC CRD")
		nicUrl := "http://" + fwUrl + "/" + fileName
		patchConfig := fmt.Sprintf(`[{"op": "replace", "path": "/spec/updates", "value": [{"component":"%s","url":"%s"}]}]`, nicComponent, nicUrl)
		patchErr := oc.AsAdmin().WithoutNamespace().Run("patch").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "--type=json", "-p", patchConfig).Execute()
		o.Expect(patchErr).NotTo(o.HaveOccurred())
		bmcUrl, err := oc.AsAdmin().Run("get").Args("-n", machineAPINamespace, "hostfirmwarecomponents", host, "-o=jsonpath={.spec.updates[0].url}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(bmcUrl).Should(o.Equal(nicUrl))

		defer func() {
			patchConfig := `[{"op": "replace", "path": "/spec/updates", "value": []}]`
			patchErr := oc.AsAdmin().WithoutNamespace().Run("patch").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "--type=json", "-p", patchConfig).Execute()
			o.Expect(patchErr).NotTo(o.HaveOccurred())
			compat_otp.DeleteLabelFromNode(oc, nginxNode, "nginx-node")
			nodeHealthErr := clusterNodesHealthcheck(oc, 3000)
			compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Cluster did not recover in time!")
			clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1500, dirname)
			compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators did not recover in time!")
		}()

		compat_otp.By("Get machine name of host")
		machine, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, host, "-o=jsonpath={.spec.consumerRef.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Get the origin number of replicas
		machineSet, cmdErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", "-n", machineAPINamespace, "-o=jsonpath={.items[0].metadata.name}").Output()
		o.Expect(cmdErr).NotTo(o.HaveOccurred())
		originReplicasStr, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", machineSet, "-n", machineAPINamespace, "-o=jsonpath={.spec.replicas}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("Annotate worker-01 machine for deletion")
		_, err = oc.AsAdmin().WithoutNamespace().Run("annotate").Args("machine", machine, "machine.openshift.io/cluster-api-delete-machine=yes", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("Scale down machineset")
		originReplicas, err := strconv.Atoi(originReplicasStr)
		o.Expect(err).NotTo(o.HaveOccurred())
		newReplicas := originReplicas - 1
		_, err = oc.AsAdmin().WithoutNamespace().Run("scale").Args("machineset", machineSet, "-n", machineAPINamespace, fmt.Sprintf("--replicas=%d", newReplicas)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForBMHState(oc, host, "available")

		defer func() {
			currentReplicasStr, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", machineSet, "-n", machineAPINamespace, "-o=jsonpath={.spec.replicas}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			if currentReplicasStr != originReplicasStr {
				_, err := oc.AsAdmin().WithoutNamespace().Run("scale").Args("machineset", machineSet, "-n", machineAPINamespace, fmt.Sprintf("--replicas=%s", originReplicasStr)).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				nodeHealthErr := clusterNodesHealthcheck(oc, 1500)
				compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Cluster did not recover in time!")
				clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1500, dirname)
				compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators did not recover in time!")
			}
		}()

		compat_otp.By("Scale up machineset")
		_, err = oc.AsAdmin().WithoutNamespace().Run("scale").Args("machineset", machineSet, "-n", machineAPINamespace, fmt.Sprintf("--replicas=%s", originReplicasStr)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForBMHState(oc, host, "provisioned")

		nodeHealthErr := clusterNodesHealthcheck(oc, 1500)
		compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Cluster did not recover in time!")
		clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1500, dirname)
		compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators did not recover in time!")

		// Poll for firmware version update before final assertion
		var currentVersion string
		pollErr := wait.Poll(10*time.Second, 5*time.Minute, func() (bool, error) {
			ver, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "-o=jsonpath="+jsonPath).Output()
			if err != nil {
				return false, nil // keep polling on error
			}
			if ver != initialVersion {
				currentVersion = ver
				return true, nil
			}
			return false, nil
		})
		o.Expect(pollErr).NotTo(o.HaveOccurred(), "Polling for firmware version update failed")
		o.Expect(currentVersion).ShouldNot(o.Equal(initialVersion))

	})
})
