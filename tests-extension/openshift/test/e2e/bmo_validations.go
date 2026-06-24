package baremetal

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-baremetal] INSTALLER IPI for INSTALLER_DEDICATED job on BareMetal", func() {
	defer g.GinkgoRecover()
	var (
		oc           = compat_otp.NewCLI("cluster-baremetal-operator", compat_otp.KubeConfigPath())
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
	g.It("Author:jhajyahy-Medium-66490-Allow modification of BMC address after installation [Disruptive]", func() {
		g.By("Get the count of BareMetalHosts")
		bmhCount, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("baremetalhosts", "-n", machineAPINamespace, "-o=jsonpath={.items}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		// Check if we have at least 5 BMHs (index 4 exists)
		if len(bmhCount) == 0 || bmhCount == "[]" {
			g.Skip("No BareMetalHosts found in the cluster")
		}

		g.By("Running oc patch bmh -n openshift-machine-api")
		bmhName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("baremetalhosts", "-n", machineAPINamespace, "-o=jsonpath={.items[4].metadata.name}").Output()
		// If index 4 doesn't exist, skip the test
		if err != nil || bmhName == "" {
			g.Skip("Not enough BareMetalHosts (need at least 5) to run this test")
		}

		bmcAddressOrig, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("baremetalhosts", "-n", machineAPINamespace, bmhName, "-o=jsonpath={.spec.bmc.address}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
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

		defer func() {
			g.By("Revert changes")
			patchConfig = fmt.Sprintf(`[{"op": "replace", "path": "/spec/bmc/address", "value": "%s"}]`, bmcAddressOrig)
			_, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("bmh", "-n", machineAPINamespace, bmhName, "--type=json", "-p", patchConfig).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			patchConfig = `[{"op": "remove", "path": "/metadata/annotations/baremetalhost.metal3.io~1detached"}]`
			_, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("bmh", "-n", machineAPINamespace, bmhName, "--type=json", "-p", patchConfig).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		bmcAddress, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("baremetalhosts", "-n", machineAPINamespace, bmhName, "-o=jsonpath={.spec.bmc.address}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(bmcAddress).To(o.ContainSubstring("redfish-virtualmedia://10.1.234.25/redfish/v1/Systems/System.Embedded.1"))

	})
	// author: jhajyahy@redhat.com
	// port=unknown - no data in BigQuery last 60 days
	g.It("Author:jhajyahy-Medium-66491-bootMACAddress can't be changed once set [Disruptive]", func() {
		g.By("Get the first BareMetalHost")
		bmhName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("baremetalhosts", "-n", machineAPINamespace, "-o=jsonpath={.items[0].metadata.name}").Output()
		if err != nil || bmhName == "" {
			g.Skip("No BareMetalHosts found in the cluster")
		}

		g.By("Running oc patch bmh -n openshift-machine-api")
		patchConfig := `[{"op": "replace", "path": "/spec/bootMACAddress", "value":"f4:02:70:b8:d8:ff"}]`
		out, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("bmh", "-n", machineAPINamespace, bmhName, "--type=json", "-p", patchConfig).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("bootMACAddress can not be changed once it is set"))

	})

	// author: jhajyahy@redhat.com
	// port=unknown - no data in BigQuery last 60 days
	g.It("Author:jhajyahy-Longduration-NonPreRelease-Medium-74940-Root device hints should accept by-path device alias [Disruptive]", func() {
		dirname = "OCP-74940.log"

		g.By("Get the 5th BareMetalHost (index 4)")
		bmhName, getBmhErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, "-o=jsonpath={.items[4].metadata.name}").Output()
		if getBmhErr != nil || bmhName == "" {
			g.Skip("Not enough BareMetalHosts (need at least 5) to run this test")
		}

		baseDir := compat_otp.FixturePath("testdata", "installer")
		bmhYaml := filepath.Join(baseDir, "baremetal", "bmh.yaml")
		bmcAddress, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, bmhName, "-o=jsonpath={.spec.bmc.address}").Output()
		bootMACAddress, getBbootMACAddressErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, bmhName, "-o=jsonpath={.spec.bootMACAddress}").Output()
		o.Expect(getBbootMACAddressErr).NotTo(o.HaveOccurred(), "Failed to get bootMACAddress")
		rootDeviceHints := getBypathDeviceName(oc, bmhName)
		bmcSecretName, getBMHSecretErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, bmhName, "-o=jsonpath={.spec.bmc.credentialsName}").Output()
		o.Expect(getBMHSecretErr).NotTo(o.HaveOccurred(), "Failed to get bmh secret")
		bmcSecretuser, getBmcUserErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret", "-n", machineAPINamespace, bmcSecretName, "-o=jsonpath={.data.username}").Output()
		o.Expect(getBmcUserErr).NotTo(o.HaveOccurred(), "Failed to get bmh secret user")
		bmcSecretPass, getBmcPassErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret", "-n", machineAPINamespace, bmcSecretName, "-o=jsonpath={.data.password}").Output()
		o.Expect(getBmcPassErr).NotTo(o.HaveOccurred(), "Failed to get bmh secret password")
		bmhSecretYaml := filepath.Join(baseDir, "baremetal", "bmh-secret.yaml")
		defer func() {
			if err := os.Remove(bmhSecretYaml); err != nil && !os.IsNotExist(err) {
				e2e.Logf("Warning: Failed to cleanup temporary file %s: %v", bmhSecretYaml, err)
			}
		}()

		compat_otp.ModifyYamlFileContent(bmhSecretYaml, []compat_otp.YamlReplace{
			{
				Path:  "data.username",
				Value: bmcSecretuser,
			},
			{
				Path:  "data.password",
				Value: bmcSecretPass,
			},
			{
				Path:  "metadata.name",
				Value: bmcSecretName,
			},
		})

		compat_otp.ModifyYamlFileContent(bmhYaml, []compat_otp.YamlReplace{
			{
				Path:  "metadata.name",
				Value: bmhName,
			},
			{
				Path:  "spec.bmc.address",
				Value: bmcAddress,
			},
			{
				Path:  "spec.bootMACAddress",
				Value: bootMACAddress,
			},
			{
				Path:  "spec.rootDeviceHints.deviceName",
				Value: rootDeviceHints,
			},
			{
				Path:  "spec.bmc.credentialsName",
				Value: bmcSecretName,
			},
		})

		compat_otp.By("Get machine name of host")
		machine, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, bmhName, "-o=jsonpath={.spec.consumerRef.name}").Output()
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
		waitForBMHState(oc, bmhName, "available")

		compat_otp.By("Delete worker node")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("bmh", "-n", machineAPINamespace, bmhName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {

			currentReplicasStr, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", machineSet, "-n", machineAPINamespace, "-o=jsonpath={.spec.replicas}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			// Only scale back if the new number of replicas is different from the original
			if currentReplicasStr != originReplicasStr {
				compat_otp.By("Create bmh secret using saved yaml file")
				err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", bmhSecretYaml).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				compat_otp.By("Create bmh using saved yaml file")
				err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", bmhYaml).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				_, err := oc.AsAdmin().WithoutNamespace().Run("scale").Args("machineset", machineSet, "-n", machineAPINamespace, fmt.Sprintf("--replicas=%s", originReplicasStr)).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				nodeHealthErr := clusterNodesHealthcheck(oc, 1500)
				compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Nodes do not recover healthy in time!")
				clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1500, dirname)
				compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators do not recover healthy in time!")
			}
		}()

		waitForBMHDeletion(oc, bmhName)

		compat_otp.By("Create bmh secret using saved yaml file")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", bmhSecretYaml).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("Create bmh using saved yaml file")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", bmhYaml).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = oc.AsAdmin().WithoutNamespace().Run("scale").Args("machineset", machineSet, "-n", machineAPINamespace, fmt.Sprintf("--replicas=%s", originReplicasStr)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForBMHState(oc, bmhName, "provisioned")
		nodeHealthErr := clusterNodesHealthcheck(oc, 1500)
		compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Nodes do not recover healthy in time!")
		clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1500, dirname)
		compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators do not recover healthy in time!")

		actualRootDeviceHints, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, bmhName, "-o=jsonpath={.spec.rootDeviceHints.deviceName}").Output()
		o.Expect(actualRootDeviceHints).Should(o.Equal(rootDeviceHints))

	})
})
