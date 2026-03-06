package provisioning

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/cert"

	"github.com/openshift/library-go/pkg/crypto"
)

// generateTestCertWithLifetime creates a test certificate with a specific lifetime
// for testing expiration and rotation boundary conditions.
func generateTestCertWithLifetime(lifetime time.Duration) ([]byte, error) {
	caConfig, err := crypto.MakeSelfSignedCAConfig("test-ca", lifetime)
	if err != nil {
		return nil, err
	}
	ca := crypto.CA{
		Config:          caConfig,
		SerialGenerator: &crypto.RandomSerialGenerator{},
	}
	config, err := ca.MakeServerCert(sets.New("localhost"), lifetime)
	if err != nil {
		return nil, err
	}
	certBytes, _, err := config.GetPEMBytes()
	return certBytes, err
}

// containsIP checks if the given IP list contains an IP that matches the target,
// handling the difference between 4-byte and 16-byte IPv4 representations.
func containsIP(ips []net.IP, target string) bool {
	targetIP := net.ParseIP(target)
	for _, ip := range ips {
		if ip.Equal(targetIP) {
			return true
		}
	}
	return false
}

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
	tlsCert, err := generateTlsCertificate(sets.New("localhost"))
	if err != nil {
		t.Errorf("Unexpected error while generating a certificate: %s", err)
	} else {
		assert.NotEqual(t, tlsCert.certificate, "", "empty certificate")
		assert.NotEqual(t, tlsCert.privateKey, "", "empty private key")
	}

	expired, err := isTlsCertificateExpired(tlsCert.certificate)
	if err != nil {
		t.Errorf("Unexpected error while checking a certificate: %s", err)
	} else {
		assert.False(t, expired, "new certificate is already expired")
	}
}

func TestGenerateTlsCertificateWithHost(t *testing.T) {
	tlsCert, err := generateTlsCertificate(sets.New("127.0.0.1"))
	if err != nil {
		t.Errorf("Unexpected error while generating a certificate: %s", err)
	} else {
		assert.NotEqual(t, tlsCert.certificate, "", "empty certificate")
		assert.NotEqual(t, tlsCert.privateKey, "", "empty private key")
	}

	expired, err := isTlsCertificateExpired(tlsCert.certificate)
	if err != nil {
		t.Errorf("Unexpected error while checking a certificate: %s", err)
	} else {
		assert.False(t, expired, "new certificate is already expired")
	}
}

func TestGenerateTlsCertificateEmptyHosts(t *testing.T) {
	_, err := generateTlsCertificate(sets.New[string]())
	require.Error(t, err, "expected error for empty hosts")
	assert.Contains(t, err.Error(), "at least one SAN host is required")
}

func TestGenerateTlsCertificateServiceHostnames(t *testing.T) {
	hosts := sets.New(
		"metal3-state.openshift-machine-api.svc",
		"metal3-state.openshift-machine-api.svc.cluster.local",
		"localhost",
	)

	tlsCert, err := generateTlsCertificate(hosts)
	require.NoError(t, err)

	certs, err := cert.ParseCertsPEM(tlsCert.certificate)
	require.NoError(t, err)
	require.NotEmpty(t, certs, "expected at least one certificate")

	// The server certificate is the first one in the chain
	serverCert := certs[0]

	assert.Contains(t, serverCert.DNSNames, "metal3-state.openshift-machine-api.svc",
		"certificate should contain short service hostname")
	assert.Contains(t, serverCert.DNSNames, "metal3-state.openshift-machine-api.svc.cluster.local",
		"certificate should contain FQDN service hostname")
	assert.Contains(t, serverCert.DNSNames, "localhost",
		"certificate should contain localhost")
}

func TestGenerateTlsCertificateWithProvisioningIP(t *testing.T) {
	hosts := sets.New(
		"metal3-state.openshift-machine-api.svc",
		"metal3-state.openshift-machine-api.svc.cluster.local",
		"172.22.0.3",
	)

	tlsCert, err := generateTlsCertificate(hosts)
	require.NoError(t, err)

	certs, err := cert.ParseCertsPEM(tlsCert.certificate)
	require.NoError(t, err)
	require.NotEmpty(t, certs)

	serverCert := certs[0]

	assert.True(t, containsIP(serverCert.IPAddresses, "172.22.0.3"),
		"certificate should contain provisioning IP")
	assert.Contains(t, serverCert.DNSNames, "metal3-state.openshift-machine-api.svc",
		"certificate should contain short service hostname")
	assert.Contains(t, serverCert.DNSNames, "metal3-state.openshift-machine-api.svc.cluster.local",
		"certificate should contain FQDN service hostname")
}

func TestGenerateTlsCertificateWithExternalIPs(t *testing.T) {
	hosts := sets.New(
		"metal3-state.openshift-machine-api.svc",
		"metal3-state.openshift-machine-api.svc.cluster.local",
		"172.22.0.3",
		"10.0.0.5",
		"10.0.0.6",
	)

	tlsCert, err := generateTlsCertificate(hosts)
	require.NoError(t, err)

	certs, err := cert.ParseCertsPEM(tlsCert.certificate)
	require.NoError(t, err)
	require.NotEmpty(t, certs)

	serverCert := certs[0]

	assert.True(t, containsIP(serverCert.IPAddresses, "172.22.0.3"),
		"certificate should contain provisioning IP")
	assert.True(t, containsIP(serverCert.IPAddresses, "10.0.0.5"),
		"certificate should contain first external IP")
	assert.True(t, containsIP(serverCert.IPAddresses, "10.0.0.6"),
		"certificate should contain second external IP")
}

func TestGenerateTlsCertificateWithAPIVIPs(t *testing.T) {
	hosts := sets.New(
		"metal3-state.openshift-machine-api.svc",
		"metal3-state.openshift-machine-api.svc.cluster.local",
		"localhost",
		"192.168.1.100",
	)

	tlsCert, err := generateTlsCertificate(hosts)
	require.NoError(t, err)

	certs, err := cert.ParseCertsPEM(tlsCert.certificate)
	require.NoError(t, err)
	require.NotEmpty(t, certs)

	serverCert := certs[0]

	assert.True(t, containsIP(serverCert.IPAddresses, "192.168.1.100"),
		"certificate should contain API VIP")
	assert.Contains(t, serverCert.DNSNames, "localhost",
		"certificate should contain localhost")
	assert.Contains(t, serverCert.DNSNames, "metal3-state.openshift-machine-api.svc",
		"certificate should contain short service hostname")
}

func TestGenerateTlsCertificateAllSANs(t *testing.T) {
	hosts := sets.New(
		"metal3-state.openshift-machine-api.svc",
		"metal3-state.openshift-machine-api.svc.cluster.local",
		"172.22.0.3",
		"10.0.0.5",
		"192.168.1.100",
		"192.168.1.101",
	)

	tlsCert, err := generateTlsCertificate(hosts)
	require.NoError(t, err)

	certs, err := cert.ParseCertsPEM(tlsCert.certificate)
	require.NoError(t, err)
	require.NotEmpty(t, certs)

	serverCert := certs[0]

	// Verify DNS SANs
	assert.Contains(t, serverCert.DNSNames, "metal3-state.openshift-machine-api.svc",
		"certificate should contain short service hostname")
	assert.Contains(t, serverCert.DNSNames, "metal3-state.openshift-machine-api.svc.cluster.local",
		"certificate should contain FQDN service hostname")

	// Verify IP SANs
	assert.True(t, containsIP(serverCert.IPAddresses, "172.22.0.3"),
		"certificate should contain provisioning IP")
	assert.True(t, containsIP(serverCert.IPAddresses, "10.0.0.5"),
		"certificate should contain external IP")
	assert.True(t, containsIP(serverCert.IPAddresses, "192.168.1.100"),
		"certificate should contain first API VIP")
	assert.True(t, containsIP(serverCert.IPAddresses, "192.168.1.101"),
		"certificate should contain second API VIP")

	// Verify the certificate is not expired
	expired, err := isTlsCertificateExpired(tlsCert.certificate)
	require.NoError(t, err)
	assert.False(t, expired, "new certificate should not be expired")
}

func TestCertificateValidityPeriod(t *testing.T) {
	tlsCert, err := generateTlsCertificate(sets.New("localhost"))
	require.NoError(t, err)

	certs, err := cert.ParseCertsPEM(tlsCert.certificate)
	require.NoError(t, err)
	require.NotEmpty(t, certs)

	serverCert := certs[0]
	validity := serverCert.NotAfter.Sub(serverCert.NotBefore)
	expectedValidity := 365 * 24 * time.Hour

	assert.InDelta(t, expectedValidity.Seconds(), validity.Seconds(), 60,
		"certificate validity should be approximately 1 year (365 days)")
}

func TestCertificateRotationBoundary(t *testing.T) {
	cases := []struct {
		name            string
		certLifetime    time.Duration
		expectRotation  bool
	}{
		{
			name:           "rotation-triggered-at-29-days",
			certLifetime:   29 * 24 * time.Hour,
			expectRotation: true,
		},
		{
			name:           "rotation-triggered-at-30-days",
			certLifetime:   30 * 24 * time.Hour,
			expectRotation: true,
		},
		{
			name:           "no-rotation-at-31-days",
			certLifetime:   31 * 24 * time.Hour,
			expectRotation: false,
		},
		{
			name:           "no-rotation-at-180-days",
			certLifetime:   180 * 24 * time.Hour,
			expectRotation: false,
		},
		{
			name:           "no-rotation-freshly-generated",
			certLifetime:   365 * 24 * time.Hour,
			expectRotation: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			certPEM, err := generateTestCertWithLifetime(tc.certLifetime)
			require.NoError(t, err)

			needsRotation, err := isTlsCertificateExpired(certPEM)
			require.NoError(t, err)
			assert.Equal(t, tc.expectRotation, needsRotation,
				"cert with %d-day lifetime: expected rotation=%v", int(tc.certLifetime.Hours()/24), tc.expectRotation)
		})
	}
}
