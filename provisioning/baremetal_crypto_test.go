package provisioning

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/util/cert"
)

func TestGenerateRandomPassword(t *testing.T) {
	pwd1, err := generateRandomPassword()
	if err != nil {
		t.Errorf("Unexpected error while generating random password: %s", err)
	}
	if pwd1 == "" {
		t.Errorf("Expected a valid string but got null")
	}
	pwd2, err := generateRandomPassword()
	if err != nil {
		t.Errorf("Unexpected error while re-generating random password: %s", err)
	} else {
		assert.False(t, pwd1 == pwd2, "regenerated random password should not match pervious one")
	}
}

func TestGenerateTlsCertificate(t *testing.T) {
	cert, err := generateTlsCertificate("", "test-namespace")
	if err != nil {
		t.Errorf("Unexpected error while generating a certificate: %s", err)
	} else {
		assert.NotEqual(t, cert.certificate, "", "empty certificate")
		assert.NotEqual(t, cert.privateKey, "", "empty private key")
	}

	expired, err := isTlsCertificateExpired(cert.certificate)
	if err != nil {
		t.Errorf("Unexpected error while checking a certificate: %s", err)
	} else {
		assert.False(t, expired, "new certificate is already expired")
	}
}

func TestGenerateTlsCertificateWithHost(t *testing.T) {
	cert, err := generateTlsCertificate("127.0.0.1", "test-namespace")
	if err != nil {
		t.Errorf("Unexpected error while generating a certificate: %s", err)
	} else {
		assert.NotEqual(t, cert.certificate, "", "empty certificate")
		assert.NotEqual(t, cert.privateKey, "", "empty private key")
	}

	expired, err := isTlsCertificateExpired(cert.certificate)
	if err != nil {
		t.Errorf("Unexpected error while checking a certificate: %s", err)
	} else {
		assert.False(t, expired, "new certificate is already expired")
	}
}

func TestGenerateTlsCertificateSANs(t *testing.T) {
	testCases := []struct {
		name             string
		provisioningIP   string
		namespace        string
		expectedDNSNames []string
		expectedIPs      []string
	}{
		{
			name:           "ironic",
			provisioningIP: "192.168.1.100",
			namespace:      "openshift-machine-api",
			expectedDNSNames: []string{
				"metal3-ironic.openshift-machine-api.svc",
				"metal3-ironic.openshift-machine-api.svc.cluster.local",
			},
			expectedIPs: []string{"192.168.1.100"},
		},
		{
			name:           "localhost",
			provisioningIP: "",
			namespace:      "test-ns",
			expectedDNSNames: []string{
				"metal3-ironic.test-ns.svc",
				"metal3-ironic.test-ns.svc.cluster.local",
			},
			expectedIPs: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tlsCert, err := generateTlsCertificate(tc.provisioningIP, tc.namespace)
			assert.NoError(t, err, "Unexpected error while generating certificate")

			certs, err := cert.ParseCertsPEM(tlsCert.certificate)
			assert.NoError(t, err, "Failed to parse certificate")
			// The certificate chain includes both CA and server cert
			assert.GreaterOrEqual(t, len(certs), 1, "Expected at least one certificate")

			// The server certificate is the first one in the chain
			x509Cert := certs[0]

			// Note: library-go adds IP addresses to DNS SANs for compatibility
			// with Python, Windows, and other libraries (see crypto.go:1066-1071)
			expectedDNSWithIPs := append([]string{}, tc.expectedDNSNames...)
			if tc.expectedIPs != nil {
				expectedDNSWithIPs = append(expectedDNSWithIPs, tc.expectedIPs...)
			}

			// Verify DNS names (includes IPs for compatibility)
			assert.ElementsMatch(t, expectedDNSWithIPs, x509Cert.DNSNames, "DNS SANs mismatch")

			// Verify IP addresses
			if tc.expectedIPs != nil {
				actualIPs := make([]string, len(x509Cert.IPAddresses))
				for i, ip := range x509Cert.IPAddresses {
					actualIPs[i] = ip.String()
				}
				assert.ElementsMatch(t, tc.expectedIPs, actualIPs, "IP SANs mismatch")
			} else {
				assert.Len(t, x509Cert.IPAddresses, 0, "Expected no IP SANs")
			}
		})
	}
}
