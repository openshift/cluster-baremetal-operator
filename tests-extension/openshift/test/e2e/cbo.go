package baremetal

import (
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
)

var _ = g.Describe("[OTP][sig-baremetal] IPI BareMetal", func() {
	defer g.GinkgoRecover()
	var (
		oc = compat_otp.NewCLI("cluster-baremetal-operator", compat_otp.KubeConfigPath())
	)
	g.BeforeEach(func() {
		SkipIfNotBaremetalCluster(oc)
	})
	// author: jhajyahy@redhat.com
	g.It("Author:jhajyahy-Medium-33516-Verify that cluster baremetal operator is active", func() {
		g.By("Running oc get clusteroperators baremetal")
		status, err := checkOperator(oc, "baremetal")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(status).To(o.BeTrue())

		g.By("Run oc describe clusteroperators baremetal")
		output, err := oc.AsAdmin().Run("get").Args("clusteroperator", "baremetal").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).ShouldNot(o.BeEmpty())
	})

	// author: jhajyahy@redhat.com
	g.It("Author:jhajyahy-Medium-36446-Verify openshift-machine-api namespace is still there and Ready", func() {
		g.By("Running oc get project openshift-machine-api")
		nsStatus, err := oc.AsAdmin().Run("get").Args("project", machineAPINamespace, "-o=jsonpath={.status.phase}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(nsStatus).Should(o.Equal("Active"))

	})

	// author: jhajyahy@redhat.com
	g.It("Author:jhajyahy-Medium-36909-Verify metal3 pod is controlled by cluster baremetal operator", func() {
		g.By("Running oc get deployment -n openshift-machine-api")
		annotations, err := oc.AsAdmin().Run("get").Args("deployment", "-n", machineAPINamespace, "metal3", "-o=jsonpath={.metadata.annotations}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(annotations).Should(o.ContainSubstring("baremetal.openshift.io/owned"))

	})

	// author: jhajyahy@redhat.com
	g.It("Author:jhajyahy-Medium-36445-Verify new additions to openshift-machine-api project", func() {
		g.By("Running oc get serviceaccount -n openshift-machine-api cluster-baremetal-operator")
		serviceAccount, err := oc.AsAdmin().Run("get").Args("serviceaccount", "-n", machineAPINamespace, "cluster-baremetal-operator", "-o=jsonpath={.metadata.name}:{.kind}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(serviceAccount).Should(o.Equal("cluster-baremetal-operator:ServiceAccount"))

		g.By("Running oc get provisioning provisioning-configuration")
		prov, err := oc.AsAdmin().Run("get").Args("provisioning", "provisioning-configuration", "-o=jsonpath={.metadata.name}:{.kind}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(prov).Should(o.Equal("provisioning-configuration:Provisioning"))

		g.By("Running oc get deploy -n openshift-machine-api metal3")
		priority, err := oc.AsAdmin().Run("get").Args("deployment", "-n", machineAPINamespace, "metal3", "-o=jsonpath={.spec.template.spec.priorityClassName}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(priority).Should(o.Equal("system-node-critical"))

	})

	// author: jhajyahy@redhat.com
	g.It("Author:jhajyahy-Medium-38155-Verify when deleting the Provisioning CR, the associated resources are deleted [Disruptive]", func() {
		g.By("Save provisioning-configuration as yaml file")
		filePath, err := oc.AsAdmin().Run("get").Args("provisioning", "provisioning-configuration", "-o=yaml").OutputToFile("prov.yaml")
		o.Expect(err).NotTo(o.HaveOccurred())

		defer func() {
			err := oc.AsAdmin().Run("get").Args("provisioning", "provisioning-configuration").Execute()
			if err != nil {
				errApply := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", filePath).Execute()
				o.Expect(errApply).NotTo(o.HaveOccurred())
				waitForDeployStatus(oc, "metal3", machineAPINamespace, "True")
				cboStatus, err := checkOperator(oc, "baremetal")
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(cboStatus).To(o.BeTrue())
			}
		}()

		g.By("Delete provisioning-configuration")
		deleteErr := oc.AsAdmin().Run("delete").Args("provisioning", "provisioning-configuration").Execute()
		o.Expect(deleteErr).NotTo(o.HaveOccurred())
		waitForPodsWithPrefixDeleted(oc, "metal3", machineAPINamespace)

		g.By("Check metal3 pods, services, secrets and deployment are deleted")
		secrets, secretErr := oc.AsAdmin().Run("get").Args("secrets", "-n", machineAPINamespace, "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(secretErr).NotTo(o.HaveOccurred())
		o.Expect(secrets).ShouldNot(o.ContainSubstring("metal3"))

		allResources, allErr := oc.AsAdmin().Run("get").Args("all", "-n", machineAPINamespace, "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(allErr).NotTo(o.HaveOccurred())
		o.Expect(allResources).ShouldNot(o.ContainSubstring("metal3"))

		g.By("Check cluster-baremetal-operator deployment still exists and is available")
		bmoDeployStatus, bmoErr := oc.AsAdmin().Run("get").Args("deployment", "-n", machineAPINamespace, "cluster-baremetal-operator", "-o=jsonpath={.status.conditions[?(@.type=='Available')].status}").Output()
		o.Expect(bmoErr).NotTo(o.HaveOccurred())
		o.Expect(bmoDeployStatus).To(o.Equal("True"), "BMO deployment should remain available after Provisioning CR deletion")

		g.By("Check cluster baremetal operator still available")
		status, err := checkOperator(oc, "baremetal")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(status).To(o.BeTrue())

		g.By("Recreate provisioning-configuration")
		createErr := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", filePath).Execute()
		o.Expect(createErr).NotTo(o.HaveOccurred())

		g.By("Check metal3 pods, services, secrets and deployment are recreated")
		waitForDeployStatus(oc, "metal3", machineAPINamespace, "True")
		waitForDeployStatus(oc, "metal3-baremetal-operator", machineAPINamespace, "True")
		metal3Secrets, secretErr := oc.AsAdmin().Run("get").Args("secrets", "-n", machineAPINamespace, "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(secretErr).NotTo(o.HaveOccurred())
		o.Expect(metal3Secrets).Should(o.ContainSubstring("metal3"))

		pods, err := oc.AsAdmin().Run("get").Args("pods", "-n", machineAPINamespace, "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		podlist := strings.Fields(pods)
		// Filter to only check metal3-related pods to avoid failures from unrelated pods
		metal3Pods := []string{}
		for _, pod := range podlist {
			if strings.HasPrefix(pod, "metal3-") {
				metal3Pods = append(metal3Pods, pod)
			}
		}
		for _, pod := range metal3Pods {
			podStatus := getPodStatus(oc, machineAPINamespace, pod)
			o.Expect(podStatus).Should(o.Equal("Running"))
		}

		g.By("Check cluster baremetal operator is available")
		cboStatus, err := checkOperator(oc, "baremetal")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cboStatus).To(o.BeTrue())

	})

})
