package baremetal

import (
	"fmt"
	"strconv"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[OTP][sig-baremetal] IPI BareMetal", func() {
	defer g.GinkgoRecover()
	var (
		oc      = compat_otp.NewCLI("cluster-baremetal-operator", compat_otp.KubeConfigPath())
		dirname string
	)
	g.BeforeEach(func() {
		SkipIfNotBaremetalCluster(oc)
	})
	// author: jhajyahy@redhat.com
	g.It("Author:jhajyahy-Medium-66490-Allow modification of BMC address after installation [Disruptive]", func() {
		g.By("Running oc patch bmh -n openshift-machine-api worker-00")

		// Check for at least 2 ready worker nodes before running test
		readyWorkers := getReadyWorkerCount(oc)
		e2e.Logf("Found %d ready worker nodes", readyWorkers)
		if readyWorkers < 2 {
			g.Skip(fmt.Sprintf("Test requires at least 2 ready worker nodes, found %d", readyWorkers))
		}

		bmhName, _ := getWorkerBMH(oc)
		bmcAddressOrig, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, bmhName, "-o=jsonpath={.spec.bmc.address}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		defer func() {
			g.By("Revert changes")
			revertPatch := fmt.Sprintf(`[{"op": "replace", "path": "/spec/bmc/address", "value": "%s"}]`, bmcAddressOrig)
			_, revertErr := oc.AsAdmin().WithoutNamespace().Run("patch").Args("bmh", "-n", machineAPINamespace, bmhName, "--type=json", "-p", revertPatch).Output()
			if revertErr != nil {
				e2e.Logf("Warning: failed to revert BMC address: %v", revertErr)
			}

			removePatch := `[{"op": "remove", "path": "/metadata/annotations/baremetalhost.metal3.io~1detached"}]`
			_, removeErr := oc.AsAdmin().WithoutNamespace().Run("patch").Args("bmh", "-n", machineAPINamespace, bmhName, "--type=json", "-p", removePatch).Output()
			if removeErr != nil {
				e2e.Logf("Warning: failed to remove detached annotation (may not have been set): %v", removeErr)
			}
		}()

		patchConfig := `[{"op": "replace", "path": "/spec/bmc/address", "value":"redfish-virtualmedia://10.1.234.25/redfish/v1/Systems/System.Embedded.1"}]`
		out, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("bmh", "-n", machineAPINamespace, bmhName, "--type=json", "-p", patchConfig).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("denied the request: BMC address can not be changed if the BMH is not in the Registering state, or if the BMH is not detached"))

		g.By("Detach the BareMetal host")
		patch := `{"metadata":{"annotations":{"baremetalhost.metal3.io/detached": ""}}}`
		_, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("bmh", "-n", machineAPINamespace, bmhName, "--type=merge", "-p", patch).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Modify BMC address of BareMetal host")
		_, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("bmh", "-n", machineAPINamespace, bmhName, "--type=json", "-p", patchConfig).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		bmcAddress, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, bmhName, "-o=jsonpath={.spec.bmc.address}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(bmcAddress).To(o.ContainSubstring("redfish-virtualmedia://10.1.234.25/redfish/v1/Systems/System.Embedded.1"))

	})
	// author: jhajyahy@redhat.com
	g.It("Author:jhajyahy-Medium-66491-bootMACAddress can't be changed once set [Disruptive]", func() {
		g.By("Running oc patch bmh -n openshift-machine-api master-00")
		bmhName := findBMHBySuffix(oc, "master-00")
		o.Expect(bmhName).NotTo(o.BeEmpty(), "BMH master-00 not found")
		patchConfig := `[{"op": "replace", "path": "/spec/bootMACAddress", "value":"f4:02:70:b8:d8:ff"}]`
		out, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("bmh", "-n", machineAPINamespace, bmhName, "--type=json", "-p", patchConfig).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("bootMACAddress can not be changed once it is set"))

	})

	// author: jhajyahy@redhat.com
	g.It("Author:jhajyahy-Longduration-NonPreRelease-Medium-74940-Root device hints should accept by-path device alias [Disruptive]", func() {
		dirname = "OCP-74940.log"

		// Check for at least 2 ready worker nodes before running test
		readyWorkers := getReadyWorkerCount(oc)
		e2e.Logf("Found %d ready worker nodes", readyWorkers)
		if readyWorkers < 2 {
			g.Skip(fmt.Sprintf("Test requires at least 2 ready worker nodes, found %d", readyWorkers))
		}

		bmhName, _ := getWorkerBMH(oc)
		e2e.Logf("Found BMH name: %s", bmhName)

		// Get the by-path device name to use for rootDeviceHints
		rootDeviceHints := getFirstDeviceName(oc, bmhName)

		compat_otp.By("Get machine name of host")
		machine, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, bmhName, "-o=jsonpath={.spec.consumerRef.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Get the origin number of replicas
		machineSet, cmdErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", "-n", machineAPINamespace, "-o=jsonpath={.items[0].metadata.name}").Output()
		o.Expect(cmdErr).NotTo(o.HaveOccurred())
		originReplicasStr, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", machineSet, "-n", machineAPINamespace, "-o=jsonpath={.spec.replicas}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		compat_otp.By("Annotate worker machine for deletion")
		_, err = oc.AsAdmin().WithoutNamespace().Run("annotate").Args("machine", machine, "machine.openshift.io/cluster-api-delete-machine=yes", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		compat_otp.By("Scale down machineset")
		originReplicas, err := strconv.Atoi(originReplicasStr)
		o.Expect(err).NotTo(o.HaveOccurred())
		newReplicas := originReplicas - 1
		_, err = oc.AsAdmin().WithoutNamespace().Run("scale").Args("machineset", machineSet, "-n", machineAPINamespace, fmt.Sprintf("--replicas=%d", newReplicas)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForBMHState(oc, bmhName, "available")

		defer func() {
			// Cleanup: ensure machineset is scaled back to original replicas
			currentReplicasStr, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", machineSet, "-n", machineAPINamespace, "-o=jsonpath={.spec.replicas}").Output()
			if err == nil && currentReplicasStr != originReplicasStr {
				_, _ = oc.AsAdmin().WithoutNamespace().Run("scale").Args("machineset", machineSet, "-n", machineAPINamespace, fmt.Sprintf("--replicas=%s", originReplicasStr)).Output()
			}
		}()

		compat_otp.By("Patch BMH with rootDeviceHints")
		patchConfig := fmt.Sprintf(`{"spec":{"rootDeviceHints":{"deviceName":"%s"}}}`, rootDeviceHints)
		_, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("bmh", "-n", machineAPINamespace, bmhName, "--type=merge", "-p", patchConfig).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("Scale up machineset to trigger reprovisioning")
		_, err = oc.AsAdmin().WithoutNamespace().Run("scale").Args("machineset", machineSet, "-n", machineAPINamespace, fmt.Sprintf("--replicas=%s", originReplicasStr)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForBMHState(oc, bmhName, "provisioned")
		nodeHealthErr := clusterNodesHealthcheck(oc, 1800)
		compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Nodes do not recover healthy in time!")
		clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1800, dirname)
		compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators do not recover healthy in time!")

		actualRootDeviceHints, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, bmhName, "-o=jsonpath={.spec.rootDeviceHints.deviceName}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(actualRootDeviceHints).Should(o.Equal(rootDeviceHints))

	})
})
