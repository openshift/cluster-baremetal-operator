package baremetal

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/openshift/cluster-baremetal-operator-tests-extension/openshift/test/e2e/testdata"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[OTP][sig-baremetal] IPI BareMetal", func() {
	defer g.GinkgoRecover()
	var (
		oc      = compat_otp.NewCLI("host-firmware-components", compat_otp.KubeConfigPath())
		dirname string
	)
	g.BeforeEach(func() {
		SkipIfNotBaremetalCluster(oc)
	})

	// author: jhajyahy@redhat.com
	g.It("Author:jhajyahy-Longduration-NonPreRelease-Medium-80760-Support for hosts that can be rebooted but not powered off [Disruptive]", func() {
		dirname = "OCP-74940.log"

		compat_otp.By("Find the second worker node")
		workerNodes, err := compat_otp.GetClusterNodesBy(oc, "worker")
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get worker nodes")
		if len(workerNodes) < 2 {
			g.Skip(fmt.Sprintf("Test requires at least 2 worker nodes, found %d", len(workerNodes)))
		}
		workerNode := workerNodes[1]
		e2e.Logf("Using worker node: %s", workerNode)

		compat_otp.By("Map worker node to Machine and BareMetalHost")
		machineAnnotation, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", workerNode, "-o=jsonpath={.metadata.annotations.machine\\.openshift\\.io/machine}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get machine annotation for node %s", workerNode)
		o.Expect(machineAnnotation).NotTo(o.BeEmpty(), "Node %s has no machine annotation", workerNode)
		machineNameParts := strings.Split(machineAnnotation, "/")
		o.Expect(len(machineNameParts)).To(o.Equal(2), "Machine annotation should be in format namespace/name")
		machine := machineNameParts[1]
		e2e.Logf("Node %s is associated with machine: %s", workerNode, machine)

		bmhAnnotation, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machine", machine, "-n", machineAPINamespace, "-o=jsonpath={.metadata.annotations.metal3\\.io/BareMetalHost}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get BMH annotation for machine %s", machine)
		o.Expect(bmhAnnotation).NotTo(o.BeEmpty(), "Machine %s has no BareMetalHost annotation", machine)
		bmhNameParts := strings.Split(bmhAnnotation, "/")
		o.Expect(len(bmhNameParts)).To(o.Equal(2), "BareMetalHost annotation should be in format namespace/name")
		bmhName := bmhNameParts[1]
		e2e.Logf("Machine %s is associated with BMH: %s", machine, bmhName)

		compat_otp.By("Get BMC credentials and BMH properties")
		bmhYaml := testdata.FixturePath("bmh.yaml")
		bmcSecretName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", bmhName, "-n", machineAPINamespace, "-o=jsonpath={.spec.bmc.credentialsName}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get bmh secret name")
		bmcSecretuser, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret", "-n", machineAPINamespace, bmcSecretName, "-o=jsonpath={.data.username}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get bmh secret user")
		bmcSecretPass, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret", "-n", machineAPINamespace, bmcSecretName, "-o=jsonpath={.data.password}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get bmh secret password")
		bmhSecretYaml := testdata.FixturePath("bmh-secret.yaml")

		bmcAddress, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", bmhName, "-n", machineAPINamespace, "-o=jsonpath={.spec.bmc.address}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get BMC address")
		bootMACAddress, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", bmhName, "-n", machineAPINamespace, "-o=jsonpath={.spec.bootMACAddress}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get bootMACAddress")
		rootDeviceHints, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", bmhName, "-n", machineAPINamespace, "-o=jsonpath={.spec.rootDeviceHints.deviceName}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get rootDeviceHints")

		defer os.Remove(bmhSecretYaml)

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
			{
				Path:  "spec.disablePowerOff",
				Value: "false",
			},
			{
				Path:  "spec.online",
				Value: "true",
			},
		})

		bmhYaml1 := CopyToFile(bmhYaml, "bmh.yaml")
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

		compat_otp.ModifyYamlFileContent(bmhYaml1, []compat_otp.YamlReplace{
			{
				Path:  "metadata.name",
				Value: bmhName,
			},
			{
				Path:  "spec.disablePowerOff",
				Value: "true",
			},
			{
				Path:  "spec.online",
				Value: "false",
			},
		})

		compat_otp.By("Find the machineset that owns this machine")
		machineSet, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machine", machine, "-n", machineAPINamespace, "-o=jsonpath={.metadata.labels.machine\\.openshift\\.io/cluster-api-machineset}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get machineset for machine %s", machine)
		o.Expect(machineSet).NotTo(o.BeEmpty(), "Machine %s has no machineset label", machine)
		e2e.Logf("Machine %s belongs to machineset: %s", machine, machineSet)

		originReplicasStr, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", machineSet, "-n", machineAPINamespace, "-o=jsonpath={.spec.replicas}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("Annotate machine for deletion")
		_, err = oc.AsAdmin().WithoutNamespace().Run("annotate").Args("machine", machine, "machine.openshift.io/cluster-api-delete-machine=yes", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("Scale down machineset")
		originReplicas, err := strconv.Atoi(originReplicasStr)
		o.Expect(err).NotTo(o.HaveOccurred())
		newReplicas := originReplicas - 1
		_, err = oc.AsAdmin().WithoutNamespace().Run("scale").Args("machineset", machineSet, "-n", machineAPINamespace, fmt.Sprintf("--replicas=%d", newReplicas)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForBMHState(oc, bmhName, "available")

		compat_otp.By("Delete worker node BMH")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("bmh", "-n", machineAPINamespace, bmhName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			currentReplicasStr, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", machineSet, "-n", machineAPINamespace, "-o=jsonpath={.spec.replicas}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			if currentReplicasStr != originReplicasStr {
				_, err = oc.AsAdmin().WithoutNamespace().Run("scale").Args("machineset", machineSet, "-n", machineAPINamespace, fmt.Sprintf("--replicas=%s", originReplicasStr)).Output()
				o.Expect(err).NotTo(o.HaveOccurred())

				compat_otp.By("Create bmh secret using saved yaml file")
				err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", bmhSecretYaml).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				compat_otp.By("Create bmh using saved yaml file")
				err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", bmhYaml).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				waitForBMHState(oc, bmhName, "provisioned")
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
		out, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", bmhYaml1).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("admission webhook \"baremetalhost.metal3.io\" denied the request: node can't simultaneously have online set to false and have power off disabled"))

		compat_otp.By("Set disablePowerOff and online to true")
		bmhYaml2 := CopyToFile(bmhYaml, "bmh.yaml")
		compat_otp.ModifyYamlFileContent(bmhYaml2, []compat_otp.YamlReplace{
			{
				Path:  "metadata.name",
				Value: bmhName,
			},
			{
				Path:  "spec.disablePowerOff",
				Value: "true",
			},
			{
				Path:  "spec.online",
				Value: "true",
			},
		})

		compat_otp.By("Create bmh using saved yaml file")
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", bmhYaml2).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		errMsg := "Failed to inspect hardware. Reason: unable to start inspection: Failed to set node power state to power off"
		waitForBMHState(oc, bmhName, "inspecting")
		waitForBMHError(oc, bmhName, errMsg)
		out, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, bmhName, "-o=jsonpath={.status.errorMessage}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(errMsg))
	})
})
