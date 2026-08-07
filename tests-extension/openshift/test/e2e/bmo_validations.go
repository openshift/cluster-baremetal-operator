package baremetal

import (
	"fmt"
	"os"
	"strconv"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/openshift/cluster-baremetal-operator-tests-extension/openshift/test/e2e/testdata"
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

		bmhName := findBMHByName(oc, "worker-00")
		o.Expect(bmhName).NotTo(o.BeEmpty(), "BMH worker-00 not found")
		bmcAddressOrig, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, bmhName, "-o=jsonpath={.spec.bmc.address}").Output()
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

		bmcAddress, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, bmhName, "-o=jsonpath={.spec.bmc.address}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(bmcAddress).To(o.ContainSubstring("redfish-virtualmedia://10.1.234.25/redfish/v1/Systems/System.Embedded.1"))

	})
	// author: jhajyahy@redhat.com
	g.It("Author:jhajyahy-Medium-66491-bootMACAddress can't be changed once set [Disruptive]", func() {
		g.By("Running oc patch bmh -n openshift-machine-api master-00")
		bmhName := findBMHByName(oc, "master-00")
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

		bmhName := findBMHByName(oc, "worker-00")
		e2e.Logf("DEBUG: Found BMH name: %s", bmhName)
		o.Expect(bmhName).NotTo(o.BeEmpty(), "BMH worker-01 not found")

		// Create temporary copies of fixture files to avoid modifying cached originals
		bmhFixtureData := testdata.MustGetFixtureData("bmh.yaml")
		bmhYamlFile, err := os.CreateTemp("", "bmh-*.yaml")
		o.Expect(err).NotTo(o.HaveOccurred())
		bmhYaml := bmhYamlFile.Name()
		_, err = bmhYamlFile.Write(bmhFixtureData)
		o.Expect(err).NotTo(o.HaveOccurred())
		bmhYamlFile.Close()
		defer os.Remove(bmhYaml)

		bmhSecretFixtureData := testdata.MustGetFixtureData("bmh-secret.yaml")
		bmhSecretYamlFile, err := os.CreateTemp("", "bmh-secret-*.yaml")
		o.Expect(err).NotTo(o.HaveOccurred())
		bmhSecretYaml := bmhSecretYamlFile.Name()
		_, err = bmhSecretYamlFile.Write(bmhSecretFixtureData)
		o.Expect(err).NotTo(o.HaveOccurred())
		bmhSecretYamlFile.Close()
		defer os.Remove(bmhSecretYaml)

		// Get current BMH values
		bmcAddress, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, bmhName, "-o=jsonpath={.spec.bmc.address}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		bootMACAddress, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, bmhName, "-o=jsonpath={.spec.bootMACAddress}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get bootMACAddress")
		rootDeviceHints := getFirstDeviceName(oc, bmhName)
		bmcSecretName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, bmhName, "-o=jsonpath={.spec.bmc.credentialsName}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get bmh secret")
		bmcSecretUser, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret", "-n", machineAPINamespace, bmcSecretName, "-o=jsonpath={.data.username}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get bmh secret user")
		bmcSecretPass, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret", "-n", machineAPINamespace, bmcSecretName, "-o=jsonpath={.data.password}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get bmh secret password")

		// Modify temporary BMH secret file
		compat_otp.ModifyYamlFileContent(bmhSecretYaml, []compat_otp.YamlReplace{
			{
				Path:  "data.username",
				Value: bmcSecretUser,
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

		// Modify temporary BMH file with updated rootDeviceHints
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
				nodeHealthErr := clusterNodesHealthcheck(oc, 1800)
				compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Nodes do not recover healthy in time!")
				clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1800, dirname)
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
		nodeHealthErr := clusterNodesHealthcheck(oc, 1800)
		compat_otp.AssertWaitPollNoErr(nodeHealthErr, "Nodes do not recover healthy in time!")
		clusterOperatorHealthcheckErr := clusterOperatorHealthcheck(oc, 1800, dirname)
		compat_otp.AssertWaitPollNoErr(clusterOperatorHealthcheckErr, "Cluster operators do not recover healthy in time!")

		actualRootDeviceHints, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, bmhName, "-o=jsonpath={.spec.rootDeviceHints.deviceName}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(actualRootDeviceHints).Should(o.Equal(rootDeviceHints))

	})
})
