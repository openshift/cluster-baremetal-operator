package baremetal

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-baremetal] INSTALLER IPI for INSTALLER_GENERAL job on BareMetal", func() {
	defer g.GinkgoRecover()
	var (
		oc           = compat_otp.NewCLI("baremetal-deployment-sanity", compat_otp.KubeConfigPath())
		iaasPlatform string
	)
	g.BeforeEach(func() {
		compat_otp.SkipForSNOCluster(oc)
		iaasPlatform = compat_otp.CheckPlatform(oc)
		if !(iaasPlatform == "baremetal") {
			e2e.Logf("Cluster is: %s", iaasPlatform)
			g.Skip("For Non-baremetal cluster , this is not supported!")
		}
	})
	// author: jhajyahy@redhat.com
	// port=yes - 99.7% pass rate (724 runs last 60 days)
	g.It("Author:jhajyahy-Medium-29146-Verify that all clusteroperators are Available", func() {
		g.By("Running oc get clusteroperators")
		res, err := checkOperatorsRunning(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(res).To(o.BeTrue())
	})

	// author: jhajyahy@redhat.com
	// port=yes - 100.0% pass rate (724 runs last 60 days)
	g.It("Author:jhajyahy-Medium-29719-Verify that all nodes are up and running", func() {
		g.By("Running oc get nodes")
		res, err := checkNodesRunning(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(res).To(o.BeTrue())

	})

	// author: jhajyahy@redhat.com
	// port=yes - 99.7% pass rate (724 runs last 60 days)
	g.It("Author:jhajyahy-Medium-32361-Verify that deployment exists and is not empty", func() {
		g.By("Create new namespace")
		oc.SetupProject()
		ns32361 := oc.Namespace()

		g.By("Create deployment")
		deployCreationErr := oc.Run("create").Args("deployment", "deploy32361", "-n", ns32361, "--image", "quay.io/openshifttest/hello-openshift@sha256:4200f438cf2e9446f6bcff9d67ceea1f69ed07a2f83363b7fb52529f7ddd8a83").Execute()
		o.Expect(deployCreationErr).NotTo(o.HaveOccurred())

		g.By("Check deployment status is available")
		waitForDeployStatus(oc, "deploy32361", ns32361, "True")
		status, err := oc.AsAdmin().Run("get").Args("deployment", "-n", ns32361, "deploy32361", "-o=jsonpath={.status.conditions[?(@.type=='Available')].status}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("\nDeployment %s Status is %s\n", "deploy32361", status)
		o.Expect(status).To(o.Equal("True"))

		g.By("Check pod is in Running state")
		podName := getPodName(oc, ns32361)
		podStatus := getPodStatus(oc, ns32361, podName)
		o.Expect(podStatus).To(o.Equal("Running"))
	})

	// author: jhajyahy@redhat.com
	// port=yes - 99.9% pass rate (724 runs last 60 days)
	g.It("Author:jhajyahy-Medium-34195-Verify all pods replicas are running on workers only", func() {
		g.By("Create new namespace")
		oc.SetupProject()
		ns34195 := oc.Namespace()

		g.By("Determine appropriate replica count based on cluster topology")
		// Query control plane topology to handle SNO, compact, and standard clusters
		topologyOutput, err := oc.AsAdmin().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.controlPlaneTopology}").Output()
		replicasNum := 3 // safe default for standard clusters

		if err == nil && topologyOutput != "" {
			switch topologyOutput {
			case "SingleReplica":
				// SNO cluster - single node, limited capacity
				replicasNum = 1
			case "HighlyAvailable":
				// Standard cluster - use schedulable nodes count
				schedulableNodes, nodeErr := oc.AsAdmin().Run("get").Args("nodes", "-o=jsonpath={.items[?(@.spec.taints[*].effect!='NoSchedule')].metadata.name}").Output()
				if nodeErr == nil && schedulableNodes != "" {
					nodeList := strings.Fields(schedulableNodes)
					if len(nodeList) > 0 {
						replicasNum = len(nodeList) + 1
					}
				} else {
					// Fallback to worker nodes count if schedulable query fails
					workerNodes, workerErr := compat_otp.GetClusterNodesBy(oc, "worker")
					if workerErr == nil && len(workerNodes) > 0 {
						replicasNum = len(workerNodes) + 1
					}
				}
			default:
				// External or unknown topology - use safe default
				e2e.Logf("Unknown topology %s, using default replica count: %d", topologyOutput, replicasNum)
			}
		} else {
			e2e.Logf("Unable to query control plane topology, using default replica count: %d", replicasNum)
		}

		g.By(fmt.Sprintf("Create deployment with %d replicas", replicasNum))
		deployCreationErr := oc.Run("create").Args("deployment", "deploy34195", "-n", ns34195, fmt.Sprintf("--replicas=%d", replicasNum), "--image", "quay.io/openshifttest/hello-openshift@sha256:4200f438cf2e9446f6bcff9d67ceea1f69ed07a2f83363b7fb52529f7ddd8a83").Execute()
		o.Expect(deployCreationErr).NotTo(o.HaveOccurred())
		waitForDeployStatus(oc, "deploy34195", ns34195, "True")

		g.By("Check deployed pods number is as expected")
		pods, err := oc.AsAdmin().Run("get").Args("pods", "-n", ns34195, "--field-selector=status.phase=Running", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		podList := strings.Fields(pods)
		o.Expect(len(podList)).To(o.Equal(replicasNum))

		g.By("Check pods are deployed on worker nodes only")
		for _, pod := range podList {
			podNodeName, err := compat_otp.GetPodNodeName(oc, ns34195, pod)
			o.Expect(err).NotTo(o.HaveOccurred())
			res := compat_otp.IsWorkerNode(oc, podNodeName)
			if !res {
				e2e.Logf("\nPod %s was deployed on non worker node  %s\n", pod, podNodeName)
			}
			o.Expect(res).To(o.BeTrue())
		}
	})

	// author: jhajyahy@redhat.com
	// port=yes - 99.7% pass rate (724 runs last 60 days)
	g.It("Author:jhajyahy-Medium-39126-Verify maximum CPU usage limit hasn't reached on each of the nodes", func() {
		g.By("Running oc get nodes")
		cpuExceededNodes := []string{}
		sampling_time, err := getClusterUptime(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		nodeNames, nodeErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(nodeErr).NotTo(o.HaveOccurred(), "Failed to execute oc get nodes")
		nodes := strings.Fields(nodeNames)
		for _, node := range nodes {
			cpuUsage := getNodeCpuUsage(oc, node, sampling_time)
			if cpuUsage > maxCpuUsageAllowed {
				cpuExceededNodes = append(cpuExceededNodes, node)
				e2e.Logf("\ncpu usage of exceeded node: %s is %.2f%%", node, cpuUsage)
			}
		}
		o.Expect(cpuExceededNodes).Should(o.BeEmpty(), "These nodes exceed max CPU usage allowed: %s", cpuExceededNodes)
	})

	// author: jhajyahy@redhat.com
	// port=yes - 99.6% pass rate (724 runs last 60 days)
	g.It("Author:jhajyahy-Medium-39125-Verify that every node memory is sufficient", func() {
		g.By("Running oc get nodes")
		outOfMemoryNodes := []string{}
		nodeNames, nodeErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(nodeErr).NotTo(o.HaveOccurred(), "Failed to execute oc get nodes")
		nodes := strings.Fields(nodeNames)
		for _, node := range nodes {
			availMem := getNodeavailMem(oc, node)
			e2e.Logf("\nAvailable mem of Node %s is %d", node, availMem)
			if availMem < minRequiredMemoryInBytes {
				outOfMemoryNodes = append(outOfMemoryNodes, node)
				e2e.Logf("\nNode %s does not meet minimum required memory %d Bytes ", node, minRequiredMemoryInBytes)

			}
		}
		o.Expect(outOfMemoryNodes).Should(o.BeEmpty(), "These nodes does not meet minimum required memory: %s", outOfMemoryNodes)
	})
})
