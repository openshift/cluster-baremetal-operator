package baremetal

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-baremetal] INSTALLER IPI for INSTALLER_GENERAL job on BareMetal", func() {
	defer g.GinkgoRecover()
	var (
		oc           = compat_otp.NewCLI("ironic-tls-certificate", compat_otp.KubeConfigPath())
		iaasPlatform string
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
	// port=yes - 99.7% pass rate (369 runs last 60 days)
	g.It("Author:jhajyahy-High-88555-Verify Ironic TLS certificate contains all required SANs (IPv4 and IPv6)", func() {
		g.By("Extract TLS certificate from metal3-ironic-tls secret")
		certBase64, err := oc.AsAdmin().Run("get").Args("secret", "metal3-ironic-tls", "-n", machineAPINamespace, "-o=jsonpath={.data.tls\\.crt}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(certBase64).NotTo(o.BeEmpty())

		// Decode and save certificate to temp file using native Go
		certFile := "/tmp/ironic-tls-cert.pem"
		certBytes, err := base64.StdEncoding.DecodeString(certBase64)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = os.WriteFile(certFile, certBytes, 0644)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Parse certificate SANs using Go stdlib")
		// Read and parse certificate using Go's x509 package
		pemData, err := os.ReadFile(certFile)
		o.Expect(err).NotTo(o.HaveOccurred())
		block, _ := pem.Decode(pemData)
		o.Expect(block).NotTo(o.BeNil(), "Failed to decode PEM block from certificate")
		cert, err := x509.ParseCertificate(block.Bytes)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Build a combined string representation of SANs for legacy string matching
		var sanParts []string
		for _, dnsName := range cert.DNSNames {
			sanParts = append(sanParts, "DNS:"+dnsName)
		}
		for _, ip := range cert.IPAddresses {
			sanParts = append(sanParts, "IP Address:"+ip.String())
		}
		sanString := strings.Join(sanParts, ", ")

		// Log redacted summary instead of full SANs (avoid exposing infrastructure details)
		sanCount := len(cert.DNSNames) + len(cert.IPAddresses)
		hasServiceHostnames := strings.Contains(sanString, "metal3-state") && strings.Contains(sanString, "openshift-machine-api")
		e2e.Logf("Certificate SANs summary: %d entries found (DNS: %d, IP: %d), service hostnames present: %v",
			sanCount, len(cert.DNSNames), len(cert.IPAddresses), hasServiceHostnames)

		g.By("Verify service hostnames are present in SANs")
		o.Expect(sanString).To(o.ContainSubstring("metal3-state.openshift-machine-api.svc"))
		o.Expect(sanString).To(o.ContainSubstring("metal3-state.openshift-machine-api.svc.cluster.local"))

		g.By("Detect cluster network type (IPv4, IPv6, or dual-stack)")
		isDualStack, ipv4VIPs, ipv6VIPs, err := isDualStackCluster(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		// Log network type without exposing actual IP addresses
		if isDualStack {
			e2e.Logf("Dual-stack cluster detected - IPv4 VIP count: %d, IPv6 VIP count: %d", len(ipv4VIPs), len(ipv6VIPs))
		} else if len(ipv6VIPs) > 0 {
			e2e.Logf("IPv6-only cluster detected - IPv6 VIP count: %d", len(ipv6VIPs))
		} else if len(ipv4VIPs) > 0 {
			e2e.Logf("IPv4-only cluster detected - IPv4 VIP count: %d", len(ipv4VIPs))
		}

		g.By("Get Provisioning CR configuration")
		provNetworkType, err := oc.AsAdmin().Run("get").Args("provisioning", "provisioning-configuration", "-o=jsonpath={.spec.provisioningNetwork}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Provisioning Network Type: %s", provNetworkType)

		provIP, err := oc.AsAdmin().Run("get").Args("provisioning", "provisioning-configuration", "-o=jsonpath={.spec.provisioningIP}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		// Log only presence, not the actual IP
		e2e.Logf("Provisioning IP configured: %v", provIP != "")

		provisioningJSON, err := oc.AsAdmin().Run("get").Args("provisioning", "provisioning-configuration", "-o=json").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		// Parse JSON to get the externalIPs list
		var provisioningObj struct {
			Spec struct {
				ExternalIPs []string `json:"externalIPs"`
			} `json:"spec"`
		}
		err = json.Unmarshal([]byte(provisioningJSON), &provisioningObj)
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to parse provisioning configuration JSON")
		externalIPsList := provisioningObj.Spec.ExternalIPs
		externalIPCount := len(externalIPsList)
		e2e.Logf("External IPs count: %d", externalIPCount)

		virtualMediaExternal, err := oc.AsAdmin().Run("get").Args("provisioning", "provisioning-configuration", "-o=jsonpath={.spec.virtualMediaViaExternalNetwork}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Virtual Media Via External Network: %s", virtualMediaExternal)

		g.By("Verify provisioning IP is in SANs if configured")
		if provIP != "" {
			o.Expect(sanString).To(o.ContainSubstring(provIP))
		}

		g.By("Verify ironic-proxy SANs when provisioning network is disabled or virtual media via external network")
		if provNetworkType == "Disabled" || virtualMediaExternal == "true" {
			o.Expect(sanString).To(o.ContainSubstring("ironic-proxy.openshift-machine-api.svc"))
			o.Expect(sanString).To(o.ContainSubstring("ironic-proxy.openshift-machine-api.svc.cluster.local"))

			g.By("Verify API server internal IPs (IPv4 and IPv6) are in SANs")
			infraJSON, err := oc.AsAdmin().Run("get").Args("infrastructure", "cluster", "-o=json").Output()
			if err == nil && infraJSON != "" {
				// Parse JSON to get apiServerInternalIPs
				var infraObj struct {
					Status struct {
						PlatformStatus struct {
							Baremetal struct {
								APIServerInternalIPs []string `json:"apiServerInternalIPs"`
							} `json:"baremetal"`
						} `json:"platformStatus"`
					} `json:"status"`
				}
				err := json.Unmarshal([]byte(infraJSON), &infraObj)
				apiVIPs := infraObj.Status.PlatformStatus.Baremetal.APIServerInternalIPs
				if err == nil && len(apiVIPs) > 0 {
					e2e.Logf("API Server Internal IP count: %d", len(apiVIPs))

					// Verify each VIP in SANs
					ipv4CheckedCount := 0
					ipv6CheckedCount := 0
					for _, ip := range apiVIPs {
						if strings.Contains(ip, ":") {
							// IPv6 address
							normalizedIP := normalizeIP(ip)
							matched := strings.Contains(strings.ToLower(sanString), strings.ToLower(normalizedIP))
							o.Expect(matched).To(o.BeTrue(), fmt.Sprintf("IPv6 API VIP should be in SANs"))
							ipv6CheckedCount++
						} else {
							// IPv4 address
							o.Expect(sanString).To(o.ContainSubstring(ip), fmt.Sprintf("IPv4 API VIP should be in SANs"))
							ipv4CheckedCount++
						}
					}
					if ipv4CheckedCount > 0 {
						e2e.Logf("Verified %d IPv4 API VIPs in SANs", ipv4CheckedCount)
					}
					if ipv6CheckedCount > 0 {
						e2e.Logf("Verified %d IPv6 API VIPs in SANs", ipv6CheckedCount)
					}
				}
			}
		}

		g.By("Verify external IPs (IPv4 and IPv6) are in SANs if configured")
		if len(externalIPsList) > 0 {
			// Check each external IP in SANs
			extIPv4Count := 0
			extIPv6Count := 0
			for _, ip := range externalIPsList {
				if strings.Contains(ip, ":") {
					// IPv6 address
					normalizedIP := normalizeIP(ip)
					matched := strings.Contains(strings.ToLower(sanString), strings.ToLower(normalizedIP))
					o.Expect(matched).To(o.BeTrue(), fmt.Sprintf("IPv6 external IP should be in SANs"))
					extIPv6Count++
				} else {
					// IPv4 address
					o.Expect(sanString).To(o.ContainSubstring(ip), fmt.Sprintf("IPv4 external IP should be in SANs"))
					extIPv4Count++
				}
			}
			if extIPv4Count > 0 {
				e2e.Logf("Verified %d IPv4 external IPs in SANs", extIPv4Count)
			}
			if extIPv6Count > 0 {
				e2e.Logf("Verified %d IPv6 external IPs in SANs", extIPv6Count)
			}
		}

		g.By("Verify IPv6 addresses in SANs are in canonical form (for DNS SANs)")
		// DNS SANs should use canonical lowercase form (e.g., fd00::1, not FD00:0000::1)
		// IP Address SANs may be in expanded uppercase form (OpenSSL encoding)
		dnsSection := strings.Split(sanString, "IP Address:")[0]
		if strings.Contains(dnsSection, "::") {
			e2e.Logf("IPv6 addresses found in DNS SANs")
			// Check for uppercase IPv6 in DNS section (should be lowercase)
			uppercaseIPv6Re := regexp.MustCompile(`[A-F]+[0-9A-F]*:[0-9A-F:]*`)
			uppercaseMatches := uppercaseIPv6Re.FindAllString(dnsSection, -1)
			for _, match := range uppercaseMatches {
				if strings.Contains(match, ":") {
					e2e.Logf("Warning: Found uppercase IPv6 in DNS SANs: %s (should be lowercase canonical)", match)
				}
			}
		}

		g.By("Verify certificate is not expired")
		ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel2()
		_, err = exec.CommandContext(ctx2, "openssl", "x509", "-in", certFile, "-noout", "-checkend", "0").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Certificate should not be expired")

		// Cleanup using native Go
		if err := os.Remove(certFile); err != nil {
			e2e.Logf("Warning: Failed to cleanup temporary certificate file %s: %v", certFile, err)
		}
	})

	// author: jhajyahy@redhat.com
	// port=unknown - no data in BigQuery last 60 days
	g.It("Author:jhajyahy-High-88556-Verify TLS certificate regenerates when SANs change (IPv4 and IPv6) [Disruptive]", func() {
		certFile := "/tmp/ironic-tls-cert.pem"
		defer func() {
			if err := os.Remove(certFile); err != nil {
				e2e.Logf("Warning: Failed to cleanup temporary certificate file %s: %v", certFile, err)
			}
		}()

		g.By("Get original certificate serial number")
		certBase64, err := oc.AsAdmin().Run("get").Args("secret", "metal3-ironic-tls", "-n", machineAPINamespace, "-o=jsonpath={.data.tls\\.crt}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Decode using native Go
		certBytes, err := base64.StdEncoding.DecodeString(certBase64)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = os.WriteFile(certFile, certBytes, 0644)
		o.Expect(err).NotTo(o.HaveOccurred())

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		originalSerial, err := exec.CommandContext(ctx, "openssl", "x509", "-in", certFile, "-serial", "-noout").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		originalSerialStr := strings.TrimSpace(string(originalSerial))
		e2e.Logf("Original certificate serial: %s", originalSerialStr)

		g.By("Detect cluster network type to choose appropriate test IP")
		isDualStack, ipv4VIPs, ipv6VIPs, _ := isDualStackCluster(oc)
		var testExternalIPs []string

		// Use RFC documentation IP addresses (TEST-NET-1 and IPv6 documentation prefix)
		if len(ipv4VIPs) > 0 {
			testExternalIPs = append(testExternalIPs, "192.0.2.101") // TEST-NET-1 (RFC 5737)
		}
		if isDualStack || len(ipv6VIPs) > 0 {
			testExternalIPs = append(testExternalIPs, "2001:db8::101") // IPv6 documentation prefix (RFC 3849)
			e2e.Logf("Adding IPv6 test external IP for dual-stack/IPv6 cluster")
		}
		e2e.Logf("Test external IP count: %d", len(testExternalIPs))

		g.By("Save original externalIPs configuration")
		originalExternalIPs, err := oc.AsAdmin().Run("get").Args("provisioning", "provisioning-configuration", "-o=jsonpath={.spec.externalIPs}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Original externalIPs: %s", originalExternalIPs)

		// Track if we need to restore
		needsCleanup := false

		defer func() {
			if !needsCleanup {
				e2e.Logf("No cleanup needed - test didn't modify externalIPs")
				return
			}

			g.By("Cleanup: Remove test externalIPs from Provisioning CR")
			e2e.Logf("Original externalIPs: %s", originalExternalIPs)

			// Use JSON patch to remove the externalIPs field, then optionally restore original
			var restoreErr error

			if originalExternalIPs == "" || originalExternalIPs == "[]" || originalExternalIPs == "null" {
				// Original was empty/not set - remove the field entirely
				e2e.Logf("Removing externalIPs field (was not set originally)")
				restoreErr = oc.AsAdmin().Run("patch").Args("provisioning", "provisioning-configuration", "--type=json", "-p", `[{"op": "remove", "path": "/spec/externalIPs"}]`).Execute()

				// If remove fails (field might not exist), that's okay
				if restoreErr != nil {
					e2e.Logf("Remove failed (field may not exist): %v, trying empty array", restoreErr)
					restoreErr = oc.AsAdmin().Run("patch").Args("provisioning", "provisioning-configuration", "--type=merge", "-p", `{"spec":{"externalIPs":[]}}`).Execute()
				}
			} else {
				// Restore original value
				e2e.Logf("Restoring externalIPs to original value: %s", originalExternalIPs)
				patchJSON := fmt.Sprintf(`{"spec":{"externalIPs":%s}}`, originalExternalIPs)
				restoreErr = oc.AsAdmin().Run("patch").Args("provisioning", "provisioning-configuration", "--type=merge", "-p", patchJSON).Execute()
			}

			if restoreErr != nil {
				e2e.Failf("Failed to restore externalIPs configuration: %v - cluster left in mutated state, manual cleanup required!", restoreErr)
			} else {
				e2e.Logf("✓ Successfully restored externalIPs configuration")
			}

			// Wait for operator to reconcile
			e2e.Logf("Waiting 60s for certificate regeneration after cleanup...")
			time.Sleep(60 * time.Second)
		}()

		g.By("Add external IPs to trigger certificate regeneration")
		externalIPsJSON := fmt.Sprintf("[%s]", strings.Join(func() []string {
			var quoted []string
			for _, ip := range testExternalIPs {
				quoted = append(quoted, fmt.Sprintf(`"%s"`, ip))
			}
			return quoted
		}(), ","))

		patchErr := oc.AsAdmin().Run("patch").Args("provisioning", "provisioning-configuration", "--type=merge", "-p", fmt.Sprintf(`{"spec":{"externalIPs":%s}}`, externalIPsJSON)).Execute()
		o.Expect(patchErr).NotTo(o.HaveOccurred())

		// Mark that cleanup is needed
		needsCleanup = true
		e2e.Logf("✓ Successfully added test externalIPs - cleanup will run in defer")

		g.By("Wait for certificate to be regenerated")
		err = wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 120*time.Second, true, func(context.Context) (bool, error) {
			newCertBase64, err := oc.AsAdmin().Run("get").Args("secret", "metal3-ironic-tls", "-n", machineAPINamespace, "-o=jsonpath={.data.tls\\.crt}").Output()
			if err != nil {
				return false, err
			}

			// Decode using native Go
			newCertBytes, err := base64.StdEncoding.DecodeString(newCertBase64)
			if err != nil {
				return false, err
			}
			err = os.WriteFile(certFile, newCertBytes, 0644)
			if err != nil {
				return false, err
			}

			// Get serial with timeout
			serialCtx, serialCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer serialCancel()
			newSerial, err := exec.CommandContext(serialCtx, "openssl", "x509", "-in", certFile, "-serial", "-noout").Output()
			if err != nil {
				return false, err
			}

			newSerialStr := strings.TrimSpace(string(newSerial))
			e2e.Logf("Checking certificate serial: %s", newSerialStr)

			if newSerialStr != originalSerialStr {
				e2e.Logf("Certificate regenerated! New serial: %s", newSerialStr)
				return true, nil
			}

			e2e.Logf("Certificate not yet regenerated, waiting...")
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "Certificate should be regenerated within 120 seconds")

		g.By("Verify new certificate contains the added external IPs (IPv4 and/or IPv6)")
		// Parse certificate to get IPAddresses field
		newCertPEM, err := os.ReadFile(certFile)
		o.Expect(err).NotTo(o.HaveOccurred())
		block, _ := pem.Decode(newCertPEM)
		o.Expect(block).NotTo(o.BeNil(), "Failed to decode PEM block from new certificate")
		newCert, err := x509.ParseCertificate(block.Bytes)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Log redacted summary instead of full SANs
		newSanCount := len(newCert.DNSNames) + len(newCert.IPAddresses)
		e2e.Logf("New certificate SANs summary: %d entries found (DNS: %d, IP: %d)",
			newSanCount, len(newCert.DNSNames), len(newCert.IPAddresses))

		// Verify all test external IPs are present using proper IP comparison
		matchedCount := 0
		for _, testIPStr := range testExternalIPs {
			testIP := net.ParseIP(testIPStr)
			o.Expect(testIP).NotTo(o.BeNil(), "Failed to parse test IP: %s", testIPStr)

			matched := false
			for _, certIP := range newCert.IPAddresses {
				if certIP.Equal(testIP) {
					matched = true
					break
				}
			}
			o.Expect(matched).To(o.BeTrue(), "Test external IP %s should be in certificate SANs", testIPStr)
			if matched {
				matchedCount++
			}
		}
		e2e.Logf("Verified %d test external IPs are present in SANs", matchedCount)
	})

	// author: jhajyahy@redhat.com
	// port=yes - 99.7% pass rate (369 runs last 60 days)
	g.It("Author:jhajyahy-High-88557-Verify BareMetalHosts communicate successfully with Ironic using TLS certificate", func() {
		g.By("Verify all BareMetalHosts are in valid state")
		bmhList, err := oc.AsAdmin().Run("get").Args("baremetalhosts", "-n", machineAPINamespace, "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		if bmhList == "" {
			e2e.Logf("No BareMetalHosts found, skipping test")
			g.Skip("No BareMetalHosts found in the cluster")
		}

		bmhNames := strings.Fields(bmhList)
		e2e.Logf("Found %d BareMetalHosts: %v", len(bmhNames), bmhNames)

		for _, bmhName := range bmhNames {
			g.By(fmt.Sprintf("Checking BareMetalHost: %s", bmhName))

			// Check provisioning state
			state, err := oc.AsAdmin().Run("get").Args("bmh", bmhName, "-n", machineAPINamespace, "-o=jsonpath={.status.provisioning.state}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("BMH %s state: %s", bmhName, state)

			// Valid states: provisioned, available, ready, etc.
			validStates := []string{"provisioned", "available", "ready", "inspecting", "provisioning"}
			isValidState := false
			for _, validState := range validStates {
				if state == validState {
					isValidState = true
					break
				}
			}
			o.Expect(isValidState).To(o.BeTrue(), fmt.Sprintf("BMH %s should be in a valid state, got: %s", bmhName, state))

			// Check for errors
			errorMsg, _ := oc.AsAdmin().Run("get").Args("bmh", bmhName, "-n", machineAPINamespace, "-o=jsonpath={.status.errorMessage}").Output()
			o.Expect(errorMsg).To(o.BeEmpty(), fmt.Sprintf("BMH %s should not have errors: %s", bmhName, errorMsg))

			// Check online status
			online, _ := oc.AsAdmin().Run("get").Args("bmh", bmhName, "-n", machineAPINamespace, "-o=jsonpath={.status.poweredOn}").Output()
			e2e.Logf("BMH %s online status: %s", bmhName, online)
		}

		g.By("Check BMO pod logs for TLS/certificate errors")
		bmoPod, err := oc.AsAdmin().Run("get").Args("pods", "-n", machineAPINamespace, "-l", "baremetal.openshift.io/cluster-baremetal-operator=metal3-baremetal-operator", "-o=jsonpath={.items[0].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(bmoPod).NotTo(o.BeEmpty())
		e2e.Logf("BMO Pod: %s", bmoPod)

		// Get recent logs and check for TLS errors
		logs, err := oc.AsAdmin().Run("logs").Args(bmoPod, "-n", machineAPINamespace, "--tail=200").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Check for TLS/certificate errors
		tlsErrorPatterns := []string{
			"tls.*error",
			"certificate.*error",
			"x509.*error",
			"handshake.*failed",
			"certificate verify failed",
		}

		for _, pattern := range tlsErrorPatterns {
			matched, _ := regexp.MatchString("(?i)"+pattern, logs)
			o.Expect(matched).To(o.BeFalse(), fmt.Sprintf("BMO logs should not contain TLS errors matching pattern: %s", pattern))
		}

		g.By("Verify BareMetalHosts have no lastError in their status")
		for _, bmhName := range bmhNames {
			lastError, _ := oc.AsAdmin().Run("get").Args("bmh", bmhName, "-n", machineAPINamespace, "-o=jsonpath={.status.provisioning.lastError}").Output()
			o.Expect(lastError).To(o.BeEmpty(), fmt.Sprintf("BMH %s should not have lastError: %s", bmhName, lastError))
		}

		g.By("Verify CBO operator is healthy")
		cboStatus, err := checkOperator(oc, "baremetal")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cboStatus).To(o.BeTrue(), "CBO operator should be healthy")

		e2e.Logf("All BareMetalHosts are successfully communicating with Ironic via TLS")
	})

	// author: jhajyahy@redhat.com
	// port=yes - 99.7% pass rate (369 runs last 60 days)
	g.It("Author:jhajyahy-High-88558-Verify Ironic TLS certificate has 1-year validity period", func() {
		certFile := "/tmp/ironic-tls-cert-validity.pem"
		defer func() {
			if err := os.Remove(certFile); err != nil {
				e2e.Logf("Warning: Failed to cleanup temporary certificate file %s: %v", certFile, err)
			}
		}()

		g.By("Extract TLS certificate from metal3-ironic-tls secret")
		certBase64, err := oc.AsAdmin().Run("get").Args("secret", "metal3-ironic-tls", "-n", machineAPINamespace, "-o=jsonpath={.data.tls\\.crt}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(certBase64).NotTo(o.BeEmpty())

		// Decode and save certificate using native Go
		certBytes, err := base64.StdEncoding.DecodeString(certBase64)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = os.WriteFile(certFile, certBytes, 0644)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Get certificate validity dates")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		startDateOutput, err := exec.CommandContext(ctx, "openssl", "x509", "-in", certFile, "-noout", "-startdate").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		// Extract date after "notBefore=" or "="
		startDate := strings.TrimSpace(strings.SplitN(string(startDateOutput), "=", 2)[1])
		e2e.Logf("Certificate Not Before: %s", startDate)

		ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel2()
		endDateOutput, err := exec.CommandContext(ctx2, "openssl", "x509", "-in", certFile, "-noout", "-enddate").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		// Extract date after "notAfter=" or "="
		endDate := strings.TrimSpace(strings.SplitN(string(endDateOutput), "=", 2)[1])
		e2e.Logf("Certificate Not After: %s", endDate)

		g.By("Calculate validity duration in days")
		validityDays, err := getCertificateValidityDays(certFile)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Certificate validity duration: %d days", validityDays)

		g.By("Verify certificate validity is exactly 365 days (1 year)")
		o.Expect(validityDays).To(o.Equal(365), "Certificate should have 365-day validity period as per METAL-1715")

		g.By("Verify certificate is currently valid (not expired)")
		ctx3, cancel3 := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel3()
		_, err = exec.CommandContext(ctx3, "openssl", "x509", "-in", certFile, "-noout", "-checkend", "0").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Certificate should be currently valid")

		g.By("Calculate days remaining until expiration")
		remainingDays, err := getCertificateDaysRemaining(certFile)
		if err == nil {
			e2e.Logf("Days remaining until expiration: %d", remainingDays)
			e2e.Logf("Rotation trigger threshold: 30 days (per METAL-1715)")

			// Verify certificate is not close to expiration (should be recently generated)
			o.Expect(remainingDays).To(o.BeNumerically(">", 30), "Certificate should have more than 30 days remaining")
		}

		e2e.Logf("Certificate validity period verification complete: 365 days ✓")
	})
})
