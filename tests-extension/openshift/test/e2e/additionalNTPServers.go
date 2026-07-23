package baremetal

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[OTP][sig-baremetal] IPI BareMetal", func() {
	defer g.GinkgoRecover()
	var (
		oc = compat_otp.NewCLI("additional-ntp-servers", compat_otp.KubeConfigPath())
	)

	g.BeforeEach(func() {
		SkipIfNotBaremetalCluster(oc)
	})

	// author: sgoveas@redhat.com
	g.It("Author:sgoveas-NonPreRelease-Medium-79243-Check Additional NTP servers were added in install-config.yaml [Level0]", func() {
		// Skip if not running on equinix-ocp-metal-qe profile
		clusterProfileName := os.Getenv("CLUSTER_PROFILE_NAME")
		if clusterProfileName != "equinix-ocp-metal-qe" {
			g.Skip("This test requires CLUSTER_PROFILE_NAME=equinix-ocp-metal-qe")
		}

		compat_otp.By("1) Get the internal NTP server")
		ntpHost := "aux-host-internal-name"
		ntpFile := filepath.Join(os.Getenv(clusterProfileDir), ntpHost)
		ntpServersList, err := ioutil.ReadFile(ntpFile)
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("2) Check additionalNTPServer was added to install-config.yaml")
		installConfig, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("configmap", "-n", "kube-system", "cluster-config-v1", "-o=jsonpath={.data.install-config}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		yqCmd := exec.CommandContext(ctx, "yq", ".platform.baremetal.additionalNTPServers")
		yqCmd.Stdin = strings.NewReader(installConfig)
		ntpList, err := yqCmd.Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Parse expected NTP servers from file (one per line)
		expectedServers := strings.Fields(strings.TrimSpace(string(ntpServersList)))
		// Parse actual NTP servers from yq output (YAML array format: "- server\n- server")
		actualNTPOutput := strings.TrimSpace(string(ntpList))

		// Check each expected server is present in the actual output
		for _, expected := range expectedServers {
			expected = strings.TrimSpace(expected)
			if expected == "" {
				continue
			}
			// Check if the server appears in the YAML list (either as "- server" or just "server")
			if !strings.Contains(actualNTPOutput, expected) {
				e2e.Failf("Expected NTP server '%s' not found in install-config.yaml. Got: %s", expected, actualNTPOutput)
			}
		}
	})
})
