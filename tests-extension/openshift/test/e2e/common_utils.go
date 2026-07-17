package baremetal

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	clusterProfileDir = "CLUSTER_PROFILE_DIR"
	proxyFile         = "proxy"
)

// SkipIfNotBaremetalCluster skips the test if the cluster is SNO or not baremetal platform
// This is a common helper for baremetal tests that need both checks
func SkipIfNotBaremetalCluster(oc *exutil.CLI) {
	compat_otp.SkipForSNOCluster(oc)
	iaasPlatform := compat_otp.CheckPlatform(oc)
	if iaasPlatform != "baremetal" {
		e2e.Logf("Cluster is: %s", iaasPlatform)
		g.Skip("This is not supported for non-baremetal cluster!")
	}
}

// getCertificateValidityDays calculates the validity period of a certificate in days
// Returns the number of days between notBefore and notAfter dates
func getCertificateValidityDays(certFilePath string) (int, error) {
	certPEM, err := ioutil.ReadFile(certFilePath)
	if err != nil {
		return 0, fmt.Errorf("failed to read certificate file: %w", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return 0, fmt.Errorf("failed to decode PEM block from certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return 0, fmt.Errorf("failed to parse certificate: %w", err)
	}

	validityDays := int(cert.NotAfter.Sub(cert.NotBefore).Hours() / 24)
	return validityDays, nil
}

// getCertificateDaysRemaining calculates how many days remain until certificate expiration
// Returns negative value if certificate is already expired
func getCertificateDaysRemaining(certFilePath string) (int, error) {
	certPEM, err := ioutil.ReadFile(certFilePath)
	if err != nil {
		return 0, fmt.Errorf("failed to read certificate file: %w", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return 0, fmt.Errorf("failed to decode PEM block from certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return 0, fmt.Errorf("failed to parse certificate: %w", err)
	}

	remainingDays := int(time.Until(cert.NotAfter).Hours() / 24)
	return remainingDays, nil
}

// isIPv6 checks if a given string is a valid IPv6 address
func isIPv6(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	return addr.Is6()
}

// normalizeIP normalizes an IP address (IPv4 or IPv6) to canonical form.
// For IPv6: lowercase, zero-compressed (e.g., "2001:db8::1").
// For IPv4: dotted decimal (e.g., "192.168.1.1").
// Returns the normalized address or original string if parsing fails.
func normalizeIP(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ip // Return original if parse fails
	}
	return addr.String()
}

// isDualStackCluster checks if the cluster has both IPv4 and IPv6 API VIPs
func isDualStackCluster(oc *exutil.CLI) (bool, []string, []string, error) {
	apiVIPsJSON, err := oc.AsAdmin().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.baremetal.apiServerInternalIPs}").Output()
	if err != nil {
		return false, nil, nil, err
	}

	if apiVIPsJSON == "" || apiVIPsJSON == "[]" {
		return false, nil, nil, nil
	}

	var ipv4Addrs []string
	var ipv6Addrs []string

	// Parse JSON array
	var ips []string
	jsonErr := json.Unmarshal([]byte(apiVIPsJSON), &ips)
	if jsonErr != nil {
		return false, nil, nil, jsonErr
	}

	for _, ip := range ips {
		if isIPv6(ip) {
			ipv6Addrs = append(ipv6Addrs, ip)
		} else {
			ipv4Addrs = append(ipv4Addrs, ip)
		}
	}

	isDualStack := len(ipv4Addrs) > 0 && len(ipv6Addrs) > 0
	return isDualStack, ipv4Addrs, ipv6Addrs, nil
}

func setProxyEnv() {
	// Get proxy settings from file
	proxyFilePath := filepath.Join(os.Getenv(clusterProfileDir), proxyFile)
	if _, err := os.Stat(proxyFilePath); os.IsNotExist(err) {
		e2e.Logf("Proxy file does not exist at %s", proxyFilePath)
		return
	}

	proxyData, err := ioutil.ReadFile(proxyFilePath)
	if err != nil {
		e2e.Logf("Failed to read proxy file: %v", err)
		return
	}

	proxyURL := strings.TrimSpace(string(proxyData))
	if proxyURL != "" {
		if err := os.Setenv("HTTP_PROXY", proxyURL); err != nil {
			e2e.Failf("Failed to set HTTP_PROXY: %v", err)
		}
		if err := os.Setenv("HTTPS_PROXY", proxyURL); err != nil {
			e2e.Failf("Failed to set HTTPS_PROXY: %v", err)
		}
		if err := os.Setenv("NO_PROXY", "localhost,127.0.0.1,.svc,.cluster.local"); err != nil {
			e2e.Failf("Failed to set NO_PROXY: %v", err)
		}
		e2e.Logf("Proxy environment variables set: HTTP_PROXY=%s", proxyURL)
	}
}

func unsetProxyEnv() {
	os.Unsetenv("HTTP_PROXY")
	os.Unsetenv("HTTPS_PROXY")
	os.Unsetenv("NO_PROXY")
	e2e.Logf("Proxy environment variables unset")
}

func CopyToFile(fromPath string, toFilename string) string {
	// check if source file is regular file
	srcFileStat, err := os.Stat(fromPath)
	if err != nil {
		e2e.Failf("get source file %s stat failed: %v", fromPath, err)
	}
	if !srcFileStat.Mode().IsRegular() {
		e2e.Failf("source file %s is not a regular file", fromPath)
	}

	// open source file
	source, err := os.Open(fromPath)
	if err != nil {
		e2e.Failf("open source file %s failed: %v", fromPath, err)
	}
	defer source.Close()

	// open dest file
	saveTo := filepath.Join(e2e.TestContext.OutputDir, toFilename)
	dest, err := os.Create(saveTo)
	if err != nil {
		e2e.Failf("open destination file %s failed: %v", saveTo, err)
	}
	defer dest.Close()

	// copy from source to dest
	_, err = io.Copy(dest, source)
	if err != nil {
		e2e.Failf("copy file from %s to %s failed: %v", fromPath, saveTo, err)
	}

	return saveTo
}
