package baremetal

import (
	"context"
	"fmt"
	"os/exec"
	"time"

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
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		url := fmt.Sprintf("https://%s/v1/nodes", endpointIP[1])
		out, cmdErr := exec.CommandContext(ctx, "curl", "-i", "-k", url).Output()
		o.Expect(cmdErr).ShouldNot(o.HaveOccurred())
		o.Expect(out).Should(o.ContainSubstring("HTTP/1.1 401 Unauthorized"))
	})

	// author: jhajyahy@redhat.com
	// port=yes - 96.1% pass rate (724 runs last 60 days)
	g.It("Author:jhajyahy-Medium-40560-An unauthenticated user can't do actions in the ironic-api when using http", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		url := fmt.Sprintf("http://%s/v1/nodes", endpointIP[1])
		out, cmdErr := exec.CommandContext(ctx, "curl", url).Output()
		o.Expect(cmdErr).Should(o.HaveOccurred())
		o.Expect(out).ShouldNot(o.ContainSubstring("HTTP/1.1 200 OK"))
		o.Expect(cmdErr.Error()).Should(o.ContainSubstring("exit status 52"))
	})

	// author: jhajyahy@redhat.com
	// port=yes - 96.1% pass rate (724 runs last 60 days)
	g.It("Author:jhajyahy-Medium-40561-An authenticated user can't do actions in the ironic-api when using http", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		url := fmt.Sprintf("http://%s/v1/nodes", endpointIP[1])
		out, cmdErr := exec.CommandContext(ctx, "curl", "-u", userPass, url).Output()
		o.Expect(cmdErr).Should(o.HaveOccurred())
		o.Expect(out).ShouldNot(o.ContainSubstring("HTTP/1.1 200 OK"))
		o.Expect(cmdErr.Error()).Should(o.ContainSubstring("exit status 52"))
	})

	// author: jhajyahy@redhat.com
	// port=yes - 95.9% pass rate (724 runs last 60 days)
	g.It("Author:jhajyahy-Medium-40562-An authenticated user can do actions in the ironic-api when using --insecure flag with https", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		url := fmt.Sprintf("https://%s/v1/nodes", endpointIP[1])
		out, cmdErr := exec.CommandContext(ctx, "curl", "-i", "-k", "-u", userPass, url).Output()
		o.Expect(cmdErr).ShouldNot(o.HaveOccurred())
		o.Expect(out).Should(o.ContainSubstring("HTTP/1.1 200 OK"))
		o.Expect(out).Should(o.ContainSubstring(`"nodes"`), "Expected JSON response with 'nodes' key")
	})
})
