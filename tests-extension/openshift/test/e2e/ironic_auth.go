package baremetal

import (
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[OTP][sig-baremetal] INSTALLER IPI for INSTALLER_GENERAL job on BareMetal", func() {
	defer g.GinkgoRecover()
	var (
		oc           = compat_otp.NewCLI("baremetal-ironic-authentication", compat_otp.KubeConfigPath())
		iaasPlatform string
		endpointIP   []string
		userPass     string
	)
	g.BeforeEach(func() {
		compat_otp.SkipForSNOCluster(oc)
		iaasPlatform = compat_otp.CheckPlatform(oc)
		if !(iaasPlatform == "baremetal") {
			e2e.Logf("Cluster is: %s", iaasPlatform)
			g.Skip("For Non-baremetal cluster , this is not supported!")
		}

		user := getUserFromSecret(oc, machineAPINamespace, "metal3-ironic-password")
		pass := getPassFromSecret(oc, machineAPINamespace, "metal3-ironic-password")
		userPass = user + ":" + pass

		// Get metal3 pod's host IP and use Ironic's default port (6385)
		hostIP, err := oc.AsAdmin().Run("get").Args("pods", "-n", machineAPINamespace, "-l", "baremetal.openshift.io/cluster-baremetal-operator=metal3-state", "-o=jsonpath={.items[0].status.hostIP}").Output()
		o.Expect(err).ShouldNot(o.HaveOccurred())
		o.Expect(hostIP).ShouldNot(o.BeEmpty(), "Failed to get metal3 pod hostIP")
		endpointIP = []string{"", hostIP + ":6385"} // [0] is full match (unused), [1] is the endpoint

	})

	// author: jhajyahy@redhat.com
	// port=yes - 96.1% pass rate (724 runs last 60 days)
	g.It("Author:jhajyahy-Medium-40655-An unauthenticated user can't do actions in the ironic-api when using --insecure flag with https", func() {
		// Get metal3 pod name
		metal3Pod, err := oc.AsAdmin().Run("get").Args("-n", machineAPINamespace, "pods", "-l", "baremetal.openshift.io/cluster-baremetal-operator=metal3-state", "-o=jsonpath={.items[0].metadata.name}").Output()
		o.Expect(err).ShouldNot(o.HaveOccurred())

		// Run curl from inside the metal3 pod (which has network access to Ironic API)
		url := fmt.Sprintf("https://%s/v1/nodes", endpointIP[1])
		curlCmd := fmt.Sprintf("curl -i -k %s", url)
		out, cmdErr := oc.AsAdmin().Run("exec").Args("-n", machineAPINamespace, metal3Pod, "-c", "metal3-ironic", "--", "sh", "-c", curlCmd).Output()
		o.Expect(cmdErr).ShouldNot(o.HaveOccurred())
		o.Expect(out).Should(o.ContainSubstring("HTTP/1.1 401 Unauthorized"))
	})

	// author: jhajyahy@redhat.com
	// port=yes - 96.1% pass rate (724 runs last 60 days)
	g.It("Author:jhajyahy-Medium-40560-An unauthenticated user can't do actions in the ironic-api when using http", func() {
		// Get metal3 pod name
		metal3Pod, err := oc.AsAdmin().Run("get").Args("-n", machineAPINamespace, "pods", "-l", "baremetal.openshift.io/cluster-baremetal-operator=metal3-state", "-o=jsonpath={.items[0].metadata.name}").Output()
		o.Expect(err).ShouldNot(o.HaveOccurred())

		// Run curl from inside the metal3 pod - HTTP should be rejected with 400 Bad Request
		url := fmt.Sprintf("http://%s/v1/nodes", endpointIP[1])
		curlCmd := fmt.Sprintf("curl -i %s", url)
		out, cmdErr := oc.AsAdmin().Run("exec").Args("-n", machineAPINamespace, metal3Pod, "-c", "metal3-ironic", "--", "sh", "-c", curlCmd).Output()
		o.Expect(cmdErr).ShouldNot(o.HaveOccurred())
		o.Expect(out).Should(o.ContainSubstring("HTTP/1.1 400 Bad Request"))
		o.Expect(out).Should(o.ContainSubstring("You're speaking plain HTTP to an SSL-enabled server port"))
	})

	// author: jhajyahy@redhat.com
	// port=yes - 96.1% pass rate (724 runs last 60 days)
	g.It("Author:jhajyahy-Medium-40561-An authenticated user can't do actions in the ironic-api when using http", func() {
		// Get metal3 pod name
		metal3Pod, err := oc.AsAdmin().Run("get").Args("-n", machineAPINamespace, "pods", "-l", "baremetal.openshift.io/cluster-baremetal-operator=metal3-state", "-o=jsonpath={.items[0].metadata.name}").Output()
		o.Expect(err).ShouldNot(o.HaveOccurred())

		// Run curl from inside the metal3 pod - HTTP should be rejected even with credentials
		// Use environment variable to avoid embedding credentials directly in the curl command
		url := fmt.Sprintf("http://%s/v1/nodes", endpointIP[1])
		shellCmd := fmt.Sprintf(`IRONIC_CREDS="%s" && curl -i -u "$IRONIC_CREDS" %s`, userPass, url)
		out, cmdErr := oc.AsAdmin().Run("exec").Args("-n", machineAPINamespace, metal3Pod, "-c", "metal3-ironic", "--", "sh", "-c", shellCmd).Output()
		o.Expect(cmdErr).ShouldNot(o.HaveOccurred())
		o.Expect(out).Should(o.ContainSubstring("HTTP/1.1 400 Bad Request"))
		o.Expect(out).Should(o.ContainSubstring("You're speaking plain HTTP to an SSL-enabled server port"))
	})

	// author: jhajyahy@redhat.com
	// port=yes - 95.9% pass rate (724 runs last 60 days)
	g.It("Author:jhajyahy-Medium-40562-An authenticated user can do actions in the ironic-api when using --insecure flag with https", func() {
		// Get metal3 pod name
		metal3Pod, err := oc.AsAdmin().Run("get").Args("-n", machineAPINamespace, "pods", "-l", "baremetal.openshift.io/cluster-baremetal-operator=metal3-state", "-o=jsonpath={.items[0].metadata.name}").Output()
		o.Expect(err).ShouldNot(o.HaveOccurred())

		// Run curl from inside the metal3 pod - HTTPS with credentials should succeed
		// Use environment variable to avoid embedding credentials directly in the curl command
		url := fmt.Sprintf("https://%s/v1/nodes", endpointIP[1])
		shellCmd := fmt.Sprintf(`IRONIC_CREDS="%s" && curl -i -k -u "$IRONIC_CREDS" %s`, userPass, url)
		out, cmdErr := oc.AsAdmin().Run("exec").Args("-n", machineAPINamespace, metal3Pod, "-c", "metal3-ironic", "--", "sh", "-c", shellCmd).Output()
		o.Expect(cmdErr).ShouldNot(o.HaveOccurred())
		o.Expect(out).Should(o.ContainSubstring("HTTP/1.1 200 OK"))
		o.Expect(out).Should(o.ContainSubstring(`"nodes"`), "Expected JSON response with 'nodes' key")
	})
})
