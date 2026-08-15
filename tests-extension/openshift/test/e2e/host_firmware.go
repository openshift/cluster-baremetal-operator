package baremetal

import (
	"fmt"
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
		oc      = compat_otp.NewCLI("host-firmware-components", compat_otp.KubeConfigPath())
		dirname string
	)
	g.BeforeEach(func() {
		compat_otp.SkipForSNOCluster(oc)
		skipIfNotBaremetal(oc)
	})
	// author: jhajyahy@redhat.com
	// port=unknown - no data in BigQuery last 60 days
	g.It("Author:jhajyahy-Longduration-NonPreRelease-Medium-75430-DAY1 Update host FW of bmc, bios and nic with reboot annotation [Disruptive]", func() {
		dirname = "OCP-75430.log"
		host, machineName := getWorkerBMH(oc)
		vendor, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("hardwaredata", "-n", machineAPINamespace, host, "-o=jsonpath={.spec.hardware.firmware.bios.vendor}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		initialBmcVersion, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "-o=jsonpath={.status.components[?(@.component==\"bmc\")].currentVersion}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(initialBmcVersion).NotTo(o.BeEmpty(), "BMC firmware version must not be empty")

		initialBiosVersion, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "-o=jsonpath={.status.components[?(@.component==\"bios\")].currentVersion}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(initialBiosVersion).NotTo(o.BeEmpty(), "BIOS firmware version must not be empty")

		nicComponent, initialNicVersion := getBastionNicComponent(oc, host)

		e2e.Logf("Selected BMH: %s, Vendor: %s, BMC FW: %s, BIOS FW: %s, NIC: %s FW: %s", host, vendor, initialBmcVersion, initialBiosVersion, nicComponent, initialNicVersion)

		bmcFwUrl := bastionBmcFirmwareURL(vendor, initialBmcVersion)
		biosFwUrl := bastionBiosFirmwareURL(vendor, initialBiosVersion)
		nicFwUrl := bastionNicFirmwareURL(initialNicVersion)

		compat_otp.By("Update HFC CRD with BMC, BIOS and NIC firmware")
		patchConfig := fmt.Sprintf(`[{"op": "replace", "path": "/spec/updates", "value": [{"component":"bmc","url":"%s"},{"component":"bios","url":"%s"},{"component":"%s","url":"%s"}]}]`, bmcFwUrl, biosFwUrl, nicComponent, nicFwUrl)
		patchErr := oc.AsAdmin().WithoutNamespace().Run("patch").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "--type=json", "-p", patchConfig).Execute()
		o.Expect(patchErr).NotTo(o.HaveOccurred())
		specUpdates, err := oc.AsAdmin().Run("get").Args("-n", machineAPINamespace, "hostfirmwarecomponents", host, "-o=jsonpath={.spec.updates}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get HFC spec updates")
		o.Expect(specUpdates).Should(o.ContainSubstring(bmcFwUrl), "HFC spec should contain BMC firmware URL")
		o.Expect(specUpdates).Should(o.ContainSubstring(biosFwUrl), "HFC spec should contain BIOS firmware URL")
		o.Expect(specUpdates).Should(o.ContainSubstring(nicFwUrl), "HFC spec should contain NIC firmware URL")

		defer func() {
			patchConfig := `[{"op": "replace", "path": "/spec/updates", "value": []}]`
			patchErr := oc.AsAdmin().WithoutNamespace().Run("patch").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "--type=json", "-p", patchConfig).Execute()
			o.Expect(patchErr).NotTo(o.HaveOccurred())
			nodeHealthErr := clusterNodesHealthcheck(oc, 1500)
			compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Cluster did not recover in time!")
			clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1500, dirname)
			compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators did not recover in time!")
		}()

		// Get the MachineSet that owns this Machine from owner reference (machineName already fetched above)
		machineSet, cmdErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("machines.machine.openshift.io", machineName, "-n", machineAPINamespace, "-o=jsonpath={.metadata.ownerReferences[?(@.kind==\"MachineSet\")].name}").Output()
		o.Expect(cmdErr).NotTo(o.HaveOccurred())
		o.Expect(machineSet).NotTo(o.BeEmpty(), "Machine should have a MachineSet owner")

		// Get the origin number of replicas from the owning MachineSet
		originReplicasStr, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machinesets.machine.openshift.io", machineSet, "-n", machineAPINamespace, "-o=jsonpath={.spec.replicas}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("Annotate worker machine for deletion")
		_, err = oc.AsAdmin().WithoutNamespace().Run("annotate").Args("machines.machine.openshift.io", machineName, "machine.openshift.io/cluster-api-delete-machine=yes", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("Scale down machineset")
		originReplicas, err := strconv.Atoi(originReplicasStr)
		o.Expect(err).NotTo(o.HaveOccurred())
		newReplicas := originReplicas - 1
		_, err = oc.AsAdmin().WithoutNamespace().Run("scale").Args("machinesets.machine.openshift.io", machineSet, "-n", machineAPINamespace, fmt.Sprintf("--replicas=%d", newReplicas)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForBMHState(oc, host, "available")

		defer func() {
			currentReplicasStr, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machinesets.machine.openshift.io", machineSet, "-n", machineAPINamespace, "-o=jsonpath={.spec.replicas}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			if currentReplicasStr != originReplicasStr {
				_, err := oc.AsAdmin().WithoutNamespace().Run("scale").Args("machinesets.machine.openshift.io", machineSet, "-n", machineAPINamespace, fmt.Sprintf("--replicas=%s", originReplicasStr)).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				nodeHealthErr := clusterNodesHealthcheck(oc, 1500)
				compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Cluster did not recover in time!")
				clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1500, dirname)
				compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators did not recover in time!")
			}
		}()

		compat_otp.By("Scale up machineset")
		_, err = oc.AsAdmin().WithoutNamespace().Run("scale").Args("machinesets.machine.openshift.io", machineSet, "-n", machineAPINamespace, fmt.Sprintf("--replicas=%s", originReplicasStr)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForBMHState(oc, host, "provisioned")
		nodeHealthErr := clusterNodesHealthcheck(oc, 1500)
		compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Cluster did not recover in time!")
		clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1500, dirname)
		compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators did not recover in time!")

		compat_otp.By("Verify BMC firmware version changed")
		currentBmcVersion := pollForFirmwareVersionChange(oc, host, "bmc", initialBmcVersion)
		e2e.Logf("BMC firmware updated: %s -> %s", initialBmcVersion, currentBmcVersion)

		compat_otp.By("Verify BIOS firmware version changed")
		currentBiosVersion := pollForFirmwareVersionChange(oc, host, "bios", initialBiosVersion)
		e2e.Logf("BIOS firmware updated: %s -> %s", initialBiosVersion, currentBiosVersion)

		compat_otp.By("Verify NIC firmware version changed")
		currentNicVersion := pollForFirmwareVersionChange(oc, host, nicComponent, initialNicVersion)
		e2e.Logf("NIC firmware updated: %s -> %s", initialNicVersion, currentNicVersion)

	})

	// author: jhajyahy@redhat.com
	// port=unknown - no data in BigQuery last 60 days
	g.It("Author:jhajyahy-Longduration-NonPreRelease-Medium-77676-DAY2 Update HFS via HostUpdatePolicy CRD [Disruptive]", func() {
		dirname = "OCP-77676.log"
		host, machineName := getWorkerBMH(oc)

		g.By("Get node name from BMH mapping")
		nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machines.machine.openshift.io", "-n", machineAPINamespace, machineName, "-o=jsonpath={.status.nodeRef.name}").Output()
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
		vendor, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("hardwaredata", "-n", machineAPINamespace, host, "-o=jsonpath={.spec.hardware.firmware.bios.vendor}").Output()
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

		compat_otp.By("Wait for HFS ChangeDetected condition")
		hfsCondErr := wait.Poll(5*time.Second, 2*time.Minute, func() (bool, error) {
			cond, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("hfs", "-n", machineAPINamespace, host, `-o=jsonpath={.status.conditions[?(@.type=="ChangeDetected")].status}`).Output()
			if err != nil {
				return false, err
			}
			if cond == "True" {
				e2e.Logf("HFS ChangeDetected condition is True")
				return true, nil
			}
			e2e.Logf("HFS ChangeDetected condition: %s, waiting...", cond)
			return false, nil
		})
		o.Expect(hfsCondErr).NotTo(o.HaveOccurred(), "HFS ChangeDetected condition did not become True")

		compat_otp.By("Reboot BareMetalHost")
		out, err := oc.AsAdmin().WithoutNamespace().Run("annotate").Args("baremetalhosts", host, "reboot.metal3.io=", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("annotated"))

		compat_otp.By("Verify BMH operationalStatus transitions to servicing")
		servicingErr := wait.Poll(5*time.Second, 2*time.Minute, func() (bool, error) {
			opStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, host, "-o=jsonpath={.status.operationalStatus}").Output()
			if err != nil {
				return false, err
			}
			if opStatus == "servicing" {
				e2e.Logf("BMH operationalStatus is now 'servicing'")
				return true, nil
			}
			e2e.Logf("BMH operationalStatus: %s, waiting for 'servicing'...", opStatus)
			return false, nil
		})
		o.Expect(servicingErr).NotTo(o.HaveOccurred(), "BMH did not transition to servicing state")

		compat_otp.By("Waiting for the node to transition to NotReady state after reboot")
		err = wait.Poll(5*time.Second, 180*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", nodeName, "-o=jsonpath={.status.conditions[?(@.type==\"Ready\")].status}").Output()
			if err != nil {
				e2e.Logf("Error getting node status: %v", err)
				return false, err
			}
			if string(output) == "True" {
				e2e.Logf("Node still Ready, status: %s. Waiting for reboot to start...", output)
				return false, nil
			}
			if string(output) == "Unknown" || string(output) == "False" {
				e2e.Logf("Node is rebooting, Ready status changed to: %s", output)
				return true, nil
			}
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(err, "Node did not transition to NotReady state after reboot annotation")

		nodeHealthErr := clusterNodesHealthcheck(oc, 3000)
		compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Cluster did not recover in time!")
		clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1500, dirname)
		compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators did not recover in time!")

		compat_otp.By("Verify hfs setting was actually changed")
		statusModified, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("hfs", "-n", machineAPINamespace, host, fmt.Sprintf("-o=jsonpath={.status.settings.%s}", hfs)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(statusModified).Should(o.Equal(specModified))

		compat_otp.By("Verify HFS ChangeDetected condition is False after update")
		hfsCond, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("hfs", "-n", machineAPINamespace, host, `-o=jsonpath={.status.conditions[?(@.type=="ChangeDetected")].status}`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hfsCond).Should(o.Equal("False"), "HFS ChangeDetected condition should be False after update")

		compat_otp.By("Verify BMH operationalStatus is OK and no error")
		opStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, host, "-o=jsonpath={.status.operationalStatus}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(opStatus).Should(o.Equal("OK"), "BMH operationalStatus should be OK after firmware settings update")
		bmhError, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, host, "-o=jsonpath={.status.errorMessage}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(bmhError).Should(o.BeEmpty(), "BMH should have no error message after firmware settings update")

	})

	// author: jhajyahy@redhat.com
	// port=unknown - no data in BigQuery last 60 days
	g.It("Author:jhajyahy-Longduration-NonPreRelease-Medium-78361-DAY2 Update host FW of bmc, bios and nic with reboot annotation  [Disruptive]", func() {
		dirname = "OCP-78361.log"
		compat_otp.By("Find a provisioned worker BareMetalHost for testing")
		host, machineName := getWorkerBMH(oc)
		nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machines.machine.openshift.io", "-n", machineAPINamespace, machineName, "-o=jsonpath={.status.nodeRef.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(nodeName).NotTo(o.BeEmpty(), "Worker BMH has no associated node")

		vendor, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("hardwaredata", "-n", machineAPINamespace, host, "-o=jsonpath={.spec.hardware.firmware.bios.vendor}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		initialBmcVersion, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "-o=jsonpath={.status.components[?(@.component==\"bmc\")].currentVersion}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(initialBmcVersion).NotTo(o.BeEmpty(), "BMC firmware version must not be empty")

		initialBiosVersion, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "-o=jsonpath={.status.components[?(@.component==\"bios\")].currentVersion}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(initialBiosVersion).NotTo(o.BeEmpty(), "BIOS firmware version must not be empty")

		nicComponent, initialNicVersion := getBastionNicComponent(oc, host)

		e2e.Logf("Selected BMH: %s, Node: %s, Vendor: %s, BMC FW: %s, BIOS FW: %s, NIC: %s FW: %s", host, nodeName, vendor, initialBmcVersion, initialBiosVersion, nicComponent, initialNicVersion)

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

		bmcFwUrl := bastionBmcFirmwareURL(vendor, initialBmcVersion)
		biosFwUrl := bastionBiosFirmwareURL(vendor, initialBiosVersion)
		nicFwUrl := bastionNicFirmwareURL(initialNicVersion)

		compat_otp.By("Update HFC CRD with BMC, BIOS and NIC firmware")
		patchConfig := fmt.Sprintf(`[{"op": "replace", "path": "/spec/updates", "value": [{"component":"bmc","url":"%s"},{"component":"bios","url":"%s"},{"component":"%s","url":"%s"}]}]`, bmcFwUrl, biosFwUrl, nicComponent, nicFwUrl)
		patchErr := oc.AsAdmin().WithoutNamespace().Run("patch").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "--type=json", "-p", patchConfig).Execute()
		o.Expect(patchErr).NotTo(o.HaveOccurred())
		specUpdates, err := oc.AsAdmin().Run("get").Args("-n", machineAPINamespace, "hostfirmwarecomponents", host, "-o=jsonpath={.spec.updates}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get HFC spec updates")
		o.Expect(specUpdates).Should(o.ContainSubstring(bmcFwUrl), "HFC spec should contain BMC firmware URL")
		o.Expect(specUpdates).Should(o.ContainSubstring(biosFwUrl), "HFC spec should contain BIOS firmware URL")
		o.Expect(specUpdates).Should(o.ContainSubstring(nicFwUrl), "HFC spec should contain NIC firmware URL")

		compat_otp.By("Wait for HFC ChangeDetected condition")
		hfcCondErr := wait.Poll(5*time.Second, 2*time.Minute, func() (bool, error) {
			cond, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, `-o=jsonpath={.status.conditions[?(@.type=="ChangeDetected")].status}`).Output()
			if err != nil {
				return false, err
			}
			if cond == "True" {
				e2e.Logf("HFC ChangeDetected condition is True")
				return true, nil
			}
			e2e.Logf("HFC ChangeDetected condition: %s, waiting...", cond)
			return false, nil
		})
		o.Expect(hfcCondErr).NotTo(o.HaveOccurred(), "HFC ChangeDetected condition did not become True")

		defer func() {
			patchConfig := `[{"op": "replace", "path": "/spec/updates", "value": []}]`
			patchErr := oc.AsAdmin().WithoutNamespace().Run("patch").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "--type=json", "-p", patchConfig).Execute()
			o.Expect(patchErr).NotTo(o.HaveOccurred())
			nodeHealthErr := clusterNodesHealthcheck(oc, 3000)
			compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Cluster did not recover in time!")
			clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1500, dirname)
			compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators did not recover in time!")
		}()

		g.By("Reboot BareMetalHost")
		out, err := oc.AsAdmin().WithoutNamespace().Run("annotate").Args("baremetalhosts", host, "reboot.metal3.io=", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("annotated"))

		compat_otp.By("Verify BMH operationalStatus transitions to servicing")
		servicingErr := wait.Poll(5*time.Second, 2*time.Minute, func() (bool, error) {
			opStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, host, "-o=jsonpath={.status.operationalStatus}").Output()
			if err != nil {
				return false, err
			}
			if opStatus == "servicing" {
				e2e.Logf("BMH operationalStatus is now 'servicing'")
				return true, nil
			}
			e2e.Logf("BMH operationalStatus: %s, waiting for 'servicing'...", opStatus)
			return false, nil
		})
		o.Expect(servicingErr).NotTo(o.HaveOccurred(), "BMH did not transition to servicing state")

		g.By("Waiting for the node to transition to NotReady state after reboot")
		err = wait.Poll(5*time.Second, 180*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", nodeName, "-o=jsonpath={.status.conditions[?(@.type==\"Ready\")].status}").Output()
			if err != nil {
				e2e.Logf("Error getting node status: %v", err)
				return false, err
			}
			if string(output) == "True" {
				e2e.Logf("Node still Ready, status: %s. Waiting for reboot to start...", output)
				return false, nil
			}
			if string(output) == "Unknown" || string(output) == "False" {
				e2e.Logf("Node is rebooting, Ready status changed to: %s", output)
				return true, nil
			}
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(err, "Node did not transition to NotReady state after reboot annotation")

		nodeHealthErr := clusterNodesHealthcheck(oc, 3000)
		compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Cluster did not recover in time!")
		clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1500, dirname)
		compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators did not recover in time!")

		compat_otp.By("Verify BMC firmware version changed")
		currentBmcVersion := pollForFirmwareVersionChange(oc, host, "bmc", initialBmcVersion)
		e2e.Logf("BMC firmware updated: %s -> %s", initialBmcVersion, currentBmcVersion)

		compat_otp.By("Verify BIOS firmware version changed")
		currentBiosVersion := pollForFirmwareVersionChange(oc, host, "bios", initialBiosVersion)
		e2e.Logf("BIOS firmware updated: %s -> %s", initialBiosVersion, currentBiosVersion)

		compat_otp.By("Verify NIC firmware version changed")
		currentNicVersion := pollForFirmwareVersionChange(oc, host, nicComponent, initialNicVersion)
		e2e.Logf("NIC firmware updated: %s -> %s", initialNicVersion, currentNicVersion)

		compat_otp.By("Verify HFC ChangeDetected condition is False after update")
		hfcCond, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, `-o=jsonpath={.status.conditions[?(@.type=="ChangeDetected")].status}`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hfcCond).Should(o.Equal("False"), "HFC ChangeDetected condition should be False after update")

		compat_otp.By("Verify BMH operationalStatus is OK and no error")
		opStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, host, "-o=jsonpath={.status.operationalStatus}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(opStatus).Should(o.Equal("OK"), "BMH operationalStatus should be OK after firmware update")
		bmhError, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, host, "-o=jsonpath={.status.errorMessage}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(bmhError).Should(o.BeEmpty(), "BMH should have no error message after firmware update")

	})

	// author: jhajyahy@redhat.com
	g.It("Author:jhajyahy-Longduration-NonPreRelease-Medium-90291-DAY2 Batched firmware Update of bmc, bios and nic using service annotation [Disruptive]", func() {
		dirname = "OCP-90291.log"
		compat_otp.By("Find a provisioned worker BareMetalHost for testing")
		host, machineName := getWorkerBMH(oc)
		nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machines.machine.openshift.io", "-n", machineAPINamespace, machineName, "-o=jsonpath={.status.nodeRef.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(nodeName).NotTo(o.BeEmpty(), "Worker BMH has no associated node")

		vendor, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("hardwaredata", "-n", machineAPINamespace, host, "-o=jsonpath={.spec.hardware.firmware.bios.vendor}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		initialBmcVersion, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "-o=jsonpath={.status.components[?(@.component==\"bmc\")].currentVersion}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(initialBmcVersion).NotTo(o.BeEmpty(), "BMC firmware version must not be empty")

		initialBiosVersion, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "-o=jsonpath={.status.components[?(@.component==\"bios\")].currentVersion}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(initialBiosVersion).NotTo(o.BeEmpty(), "BIOS firmware version must not be empty")

		nicComponent, initialNicVersion := getBastionNicComponent(oc, host)

		e2e.Logf("Selected BMH: %s, Node: %s, Vendor: %s, BMC FW: %s, BIOS FW: %s, NIC: %s FW: %s", host, nodeName, vendor, initialBmcVersion, initialBiosVersion, nicComponent, initialNicVersion)

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

		bmcFwUrl := bastionBmcFirmwareURL(vendor, initialBmcVersion)
		biosFwUrl := bastionBiosFirmwareURL(vendor, initialBiosVersion)
		nicFwUrl := bastionNicFirmwareURL(initialNicVersion)

		compat_otp.By("Update HFC CRD with BMC, BIOS and NIC firmware")
		patchConfig := fmt.Sprintf(`[{"op": "replace", "path": "/spec/updates", "value": [{"component":"bmc","url":"%s"},{"component":"bios","url":"%s"},{"component":"%s","url":"%s"}]}]`, bmcFwUrl, biosFwUrl, nicComponent, nicFwUrl)
		patchErr := oc.AsAdmin().WithoutNamespace().Run("patch").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "--type=json", "-p", patchConfig).Execute()
		o.Expect(patchErr).NotTo(o.HaveOccurred())
		specUpdates, err := oc.AsAdmin().Run("get").Args("-n", machineAPINamespace, "hostfirmwarecomponents", host, "-o=jsonpath={.spec.updates}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get HFC spec updates")
		o.Expect(specUpdates).Should(o.ContainSubstring(bmcFwUrl), "HFC spec should contain BMC firmware URL")
		o.Expect(specUpdates).Should(o.ContainSubstring(biosFwUrl), "HFC spec should contain BIOS firmware URL")
		o.Expect(specUpdates).Should(o.ContainSubstring(nicFwUrl), "HFC spec should contain NIC firmware URL")

		compat_otp.By("Wait for HFC ChangeDetected condition")
		hfcCondErr := wait.Poll(5*time.Second, 2*time.Minute, func() (bool, error) {
			cond, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, `-o=jsonpath={.status.conditions[?(@.type=="ChangeDetected")].status}`).Output()
			if err != nil {
				return false, err
			}
			if cond == "True" {
				e2e.Logf("HFC ChangeDetected condition is True")
				return true, nil
			}
			e2e.Logf("HFC ChangeDetected condition: %s, waiting...", cond)
			return false, nil
		})
		o.Expect(hfcCondErr).NotTo(o.HaveOccurred(), "HFC ChangeDetected condition did not become True")

		defer func() {
			patchConfig := `[{"op": "replace", "path": "/spec/updates", "value": []}]`
			patchErr := oc.AsAdmin().WithoutNamespace().Run("patch").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "--type=json", "-p", patchConfig).Execute()
			o.Expect(patchErr).NotTo(o.HaveOccurred())
			nodeHealthErr := clusterNodesHealthcheck(oc, 3000)
			compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Cluster did not recover in time!")
			clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1500, dirname)
			compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators did not recover in time!")
		}()

		g.By("Annotate BMH with service annotation to trigger cordon, drain, and firmware servicing")
		out, err := oc.AsAdmin().WithoutNamespace().Run("annotate").Args("baremetalhosts", host, "service.baremetal.openshift.io=", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("annotated"))

		compat_otp.By("Verify node is cordoned")
		cordonErr := wait.Poll(5*time.Second, 2*time.Minute, func() (bool, error) {
			unschedulable, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", nodeName, "-o=jsonpath={.spec.unschedulable}").Output()
			if err != nil {
				return false, nil
			}
			if unschedulable == "true" {
				e2e.Logf("Node %s is cordoned (unschedulable=true)", nodeName)
				return true, nil
			}
			e2e.Logf("Node %s not yet cordoned, unschedulable=%s", nodeName, unschedulable)
			return false, nil
		})
		o.Expect(cordonErr).NotTo(o.HaveOccurred(), "Node was not cordoned after service annotation")

		compat_otp.By("Verify BMH operationalStatus transitions to servicing")
		servicingErr := wait.Poll(5*time.Second, 5*time.Minute, func() (bool, error) {
			opStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, host, "-o=jsonpath={.status.operationalStatus}").Output()
			if err != nil {
				return false, err
			}
			if opStatus == "servicing" {
				e2e.Logf("BMH operationalStatus is now 'servicing'")
				return true, nil
			}
			e2e.Logf("BMH operationalStatus: %s, waiting for 'servicing'...", opStatus)
			return false, nil
		})
		o.Expect(servicingErr).NotTo(o.HaveOccurred(), "BMH did not transition to servicing state")

		compat_otp.By("Wait for node to recover and become Ready")
		nodeHealthErr := clusterNodesHealthcheck(oc, 3000)
		compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Cluster did not recover in time!")
		clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1500, dirname)
		compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators did not recover in time!")

		compat_otp.By("Verify node is uncordoned after servicing")
		uncordonErr := wait.Poll(5*time.Second, 5*time.Minute, func() (bool, error) {
			unschedulable, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", nodeName, "-o=jsonpath={.spec.unschedulable}").Output()
			if err != nil {
				return false, nil
			}
			if unschedulable == "" || unschedulable == "false" {
				e2e.Logf("Node %s is uncordoned", nodeName)
				return true, nil
			}
			e2e.Logf("Node %s still cordoned, unschedulable=%s", nodeName, unschedulable)
			return false, nil
		})
		o.Expect(uncordonErr).NotTo(o.HaveOccurred(), "Node was not uncordoned after servicing completed")

		compat_otp.By("Verify BMC firmware version changed")
		currentBmcVersion := pollForFirmwareVersionChange(oc, host, "bmc", initialBmcVersion)
		e2e.Logf("BMC firmware updated: %s -> %s", initialBmcVersion, currentBmcVersion)

		compat_otp.By("Verify BIOS firmware version changed")
		currentBiosVersion := pollForFirmwareVersionChange(oc, host, "bios", initialBiosVersion)
		e2e.Logf("BIOS firmware updated: %s -> %s", initialBiosVersion, currentBiosVersion)

		compat_otp.By("Verify NIC firmware version changed")
		currentNicVersion := pollForFirmwareVersionChange(oc, host, nicComponent, initialNicVersion)
		e2e.Logf("NIC firmware updated: %s -> %s", initialNicVersion, currentNicVersion)

		compat_otp.By("Verify HFC ChangeDetected condition is False after update")
		hfcCond, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, `-o=jsonpath={.status.conditions[?(@.type=="ChangeDetected")].status}`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hfcCond).Should(o.Equal("False"), "HFC ChangeDetected condition should be False after update")

		compat_otp.By("Verify BMH operationalStatus is OK and no error")
		opStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, host, "-o=jsonpath={.status.operationalStatus}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(opStatus).Should(o.Equal("OK"), "BMH operationalStatus should be OK after firmware update")
		bmhError, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, host, "-o=jsonpath={.status.errorMessage}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(bmhError).Should(o.BeEmpty(), "BMH should have no error message after firmware update")

		compat_otp.By("Verify service annotation was removed after servicing")
		svcAnnotation, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, host, `-o=jsonpath={.metadata.annotations.service\.baremetal\.openshift\.io}`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(svcAnnotation).Should(o.BeEmpty(), "Service annotation should be removed after servicing completes")
	})
})
