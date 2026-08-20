package baremetal

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[OTP][sig-baremetal] IPI BareMetal", func() {
	defer g.GinkgoRecover()
	var (
		oc = compat_otp.NewCLI("readonly-filesystem", compat_otp.KubeConfigPath())
	)

	g.BeforeEach(func() {
		SkipIfNotBaremetalCluster(oc)
	})

	// author: sgoveas@redhat.com
	g.It("Author:sgoveas-NonPreRelease-Medium-87309-Verify machine-api containers have read-only filesystem", func() {
		namespace := "openshift-machine-api"
		failedContainers := []string{}

		compat_otp.By("1) Get all pods in openshift-machine-api namespace")
		podNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", namespace, "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get pods in namespace %s", namespace)

		pods := strings.Fields(podNames)
		o.Expect(len(pods)).To(o.BeNumerically(">", 0), "No pods found in namespace %s", namespace)
		e2e.Logf("Found %d pods in namespace %s", len(pods), namespace)

		compat_otp.By("2) Check each container in each pod for read-only filesystem")
		for _, podName := range pods {
			e2e.Logf("==========")
			e2e.Logf("Checking pod: %s", podName)
			e2e.Logf("==========")

			// Get list of containers in the pod
			containerNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podName, "-n", namespace, "-o=jsonpath={.spec.containers[*].name}").Output()
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get containers for pod %s", podName)

			containers := strings.Fields(containerNames)
			e2e.Logf("Pod %s has %d containers: %v", podName, len(containers), containers)

			// Test each container
			for _, containerName := range containers {
				e2e.Logf("---%s---", containerName)

				// Try to create a test file in the container
				testFile := "/testfile"
				touchOutput, touchErr := oc.AsAdmin().Run("rsh").Args("-n", namespace, "--container", containerName, podName, "touch", testFile).Output()

				if touchErr == nil {
					// Touch succeeded - filesystem is writable (FAIL)
					e2e.Logf("FAIL: %s container in %s pod has writable filesystem", containerName, podName)
					failedContainers = append(failedContainers, fmt.Sprintf("%s/%s", podName, containerName))
				} else {
					// Touch failed - check if it's a read-only filesystem error
					errorText := strings.ToLower(touchErr.Error() + " " + touchOutput)
					if strings.Contains(errorText, "read-only file system") || strings.Contains(errorText, "readonly") {
						// Genuine read-only filesystem error (PASS)
						e2e.Logf("PASS: %s container in %s pod has read-only filesystem", containerName, podName)
					} else {
						// Other error (exec/connection/binary missing) - treat as test failure
						e2e.Logf("FAIL: %s container in %s pod - unexpected error (not RO filesystem): %v, output: %s", containerName, podName, touchErr, touchOutput)
						failedContainers = append(failedContainers, fmt.Sprintf("%s/%s (error: %v)", podName, containerName, touchErr))
					}
				}
			}
		}

		compat_otp.By("3) Verify all containers have read-only filesystem")
		o.Expect(failedContainers).Should(o.BeEmpty(), "These containers have writable filesystem: %v", failedContainers)
	})

	// author: sgoveas@redhat.com
	g.It("Author:sgoveas-NonPreRelease-Medium-87310-Verify machine-os-images initContainer has readOnlyRootFilesystem set to true", func() {
		namespace := "openshift-machine-api"
		targetContainer := "machine-os-images"
		foundContainer := false

		compat_otp.By("1) Get all pods in openshift-machine-api namespace")
		podNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", namespace, "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get pods in namespace %s", namespace)

		pods := strings.Fields(podNames)
		o.Expect(len(pods)).To(o.BeNumerically(">", 0), "No pods found in namespace %s", namespace)
		e2e.Logf("Found %d pods in namespace %s", len(pods), namespace)

		compat_otp.By("2) Check initContainers for machine-os-images container")
		for _, podName := range pods {
			e2e.Logf("Checking pod: %s", podName)

			// Get initContainer names
			initContainerNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podName, "-n", namespace, "-o=jsonpath={.spec.initContainers[*].name}").Output()
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get initContainers for pod %s in namespace %s", podName, namespace)

			if initContainerNames == "" {
				e2e.Logf("Pod %s has no initContainers, skipping", podName)
				continue
			}

			initContainers := strings.Fields(initContainerNames)
			e2e.Logf("Pod %s has initContainers: %v", podName, initContainers)

			// Check if machine-os-images container exists
			containerFound := false
			for _, name := range initContainers {
				if name == targetContainer {
					containerFound = true
					break
				}
			}

			if !containerFound {
				e2e.Logf("Pod %s does not have %s initContainer, skipping", podName, targetContainer)
				continue
			}

			foundContainer = true
			e2e.Logf("Found %s initContainer in pod %s", targetContainer, podName)

			compat_otp.By(fmt.Sprintf("3) Verify readOnlyRootFilesystem is set to true for %s in pod %s", targetContainer, podName))

			// Get readOnlyRootFilesystem value
			readOnlyValue, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podName, "-n", namespace, "-o=jsonpath={.spec.initContainers[?(@.name==\""+targetContainer+"\")].securityContext.readOnlyRootFilesystem}").Output()
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get readOnlyRootFilesystem for %s in pod %s", targetContainer, podName)

			e2e.Logf("Container %s readOnlyRootFilesystem value: %s", targetContainer, readOnlyValue)

			o.Expect(readOnlyValue).To(o.Equal("true"), "readOnlyRootFilesystem is not set to true for %s initContainer in pod %s", targetContainer, podName)
			e2e.Logf("PASS: %s initContainer in pod %s has readOnlyRootFilesystem=true", targetContainer, podName)
		}

		compat_otp.By("4) Verify that machine-os-images initContainer was found")
		o.Expect(foundContainer).To(o.BeTrue(), "Did not find %s initContainer in any pod in namespace %s", targetContainer, namespace)
	})
})
