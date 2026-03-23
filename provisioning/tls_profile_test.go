package provisioning

import (
	"testing"

	"github.com/stretchr/testify/assert"

	configv1 "github.com/openshift/api/config/v1"
)

func TestTlsVersionToApacheSSLProtocol(t *testing.T) {
	tests := []struct {
		name       string
		minVersion configv1.TLSProtocolVersion
		expected   string
	}{
		{
			name:       "TLS 1.0",
			minVersion: configv1.VersionTLS10,
			expected:   "-ALL +TLSv1 +TLSv1.1 +TLSv1.2 +TLSv1.3",
		},
		{
			name:       "TLS 1.1",
			minVersion: configv1.VersionTLS11,
			expected:   "-ALL +TLSv1.1 +TLSv1.2 +TLSv1.3",
		},
		{
			name:       "TLS 1.2",
			minVersion: configv1.VersionTLS12,
			expected:   "-ALL +TLSv1.2 +TLSv1.3",
		},
		{
			name:       "TLS 1.3",
			minVersion: configv1.VersionTLS13,
			expected:   "-ALL +TLSv1.3",
		},
		{
			name:       "empty defaults to TLS 1.2",
			minVersion: "",
			expected:   "-ALL +TLSv1.2 +TLSv1.3",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tlsVersionToApacheSSLProtocol(tc.minVersion)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSplitCiphers(t *testing.T) {
	tests := []struct {
		name          string
		ciphers       []string
		expectedTLS12 []string
		expectedTLS13 []string
	}{
		{
			name: "mixed ciphers",
			ciphers: []string{
				"TLS_AES_128_GCM_SHA256",
				"ECDHE-ECDSA-AES128-GCM-SHA256",
				"TLS_CHACHA20_POLY1305_SHA256",
				"ECDHE-RSA-AES256-GCM-SHA384",
			},
			expectedTLS12: []string{
				"ECDHE-ECDSA-AES128-GCM-SHA256",
				"ECDHE-RSA-AES256-GCM-SHA384",
			},
			expectedTLS13: []string{
				"TLS_AES_128_GCM_SHA256",
				"TLS_CHACHA20_POLY1305_SHA256",
			},
		},
		{
			name:          "only TLS 1.2 ciphers",
			ciphers:       []string{"ECDHE-ECDSA-AES128-GCM-SHA256"},
			expectedTLS12: []string{"ECDHE-ECDSA-AES128-GCM-SHA256"},
			expectedTLS13: nil,
		},
		{
			name:          "only TLS 1.3 ciphers",
			ciphers:       []string{"TLS_AES_128_GCM_SHA256"},
			expectedTLS12: nil,
			expectedTLS13: []string{"TLS_AES_128_GCM_SHA256"},
		},
		{
			name:          "empty",
			ciphers:       []string{},
			expectedTLS12: nil,
			expectedTLS13: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tls12, tls13 := splitCiphers(tc.ciphers)
			assert.Equal(t, tc.expectedTLS12, tls12)
			assert.Equal(t, tc.expectedTLS13, tls13)
		})
	}
}

func TestTlsProfileToApacheEnvVars(t *testing.T) {
	tests := []struct {
		name         string
		profile      configv1.TLSProfileSpec
		expectedEnvs map[string]string
	}{
		{
			name:    "Intermediate profile",
			profile: *configv1.TLSProfiles[configv1.TLSProfileIntermediateType],
			expectedEnvs: map[string]string{
				"IRONIC_SSL_PROTOCOL":        "-ALL +TLSv1.2 +TLSv1.3",
				"IRONIC_VMEDIA_SSL_PROTOCOL": "-ALL +TLSv1.2 +TLSv1.3",
			},
		},
		{
			name:    "Modern profile (TLS 1.3 only)",
			profile: *configv1.TLSProfiles[configv1.TLSProfileModernType],
			expectedEnvs: map[string]string{
				"IRONIC_SSL_PROTOCOL":        "-ALL +TLSv1.3",
				"IRONIC_VMEDIA_SSL_PROTOCOL": "-ALL +TLSv1.3",
			},
		},
		{
			name:    "Old profile",
			profile: *configv1.TLSProfiles[configv1.TLSProfileOldType],
			expectedEnvs: map[string]string{
				"IRONIC_SSL_PROTOCOL":        "-ALL +TLSv1 +TLSv1.1 +TLSv1.2 +TLSv1.3",
				"IRONIC_VMEDIA_SSL_PROTOCOL": "-ALL +TLSv1 +TLSv1.1 +TLSv1.2 +TLSv1.3",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			envVars := tlsProfileToApacheEnvVars(tc.profile)
			envMap := map[string]string{}
			for _, e := range envVars {
				envMap[e.Name] = e.Value
			}
			for k, v := range tc.expectedEnvs {
				assert.Equal(t, v, envMap[k], "env var %s", k)
			}
		})
	}
}

func TestTlsProfileToApacheEnvVarsCipherSplit(t *testing.T) {
	profile := *configv1.TLSProfiles[configv1.TLSProfileIntermediateType]

	envVars := tlsProfileToApacheEnvVars(profile)
	envMap := map[string]string{}
	for _, e := range envVars {
		envMap[e.Name] = e.Value
	}

	// Intermediate profile has both TLS 1.2 and TLS 1.3 ciphers
	assert.Contains(t, envMap, "IRONIC_TLS_12_CIPHERS")
	assert.Contains(t, envMap, "IRONIC_TLS_13_CIPHERS")
	assert.NotContains(t, envMap["IRONIC_TLS_12_CIPHERS"], "TLS_AES")
	assert.Contains(t, envMap["IRONIC_TLS_13_CIPHERS"], "TLS_AES_128_GCM_SHA256")
}

func TestTlsProfileToBMOArgs(t *testing.T) {
	tests := []struct {
		name            string
		profile         configv1.TLSProfileSpec
		expectVersion   string
		expectNoCiphers bool
	}{
		{
			name:          "Intermediate profile",
			profile:       *configv1.TLSProfiles[configv1.TLSProfileIntermediateType],
			expectVersion: "TLS12",
		},
		{
			name:            "Modern profile (TLS 1.3 only)",
			profile:         *configv1.TLSProfiles[configv1.TLSProfileModernType],
			expectVersion:   "TLS13",
			expectNoCiphers: true,
		},
		{
			name:          "Old profile clamps to TLS12",
			profile:       *configv1.TLSProfiles[configv1.TLSProfileOldType],
			expectVersion: "TLS12",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := tlsProfileToBMOArgs(tc.profile)

			// First two args should be --tls-min-version <version>
			assert.GreaterOrEqual(t, len(args), 2)
			assert.Equal(t, "--tls-min-version", args[0])
			assert.Equal(t, tc.expectVersion, args[1])

			hasCiphers := false
			for _, a := range args {
				if a == "--tls-cipher-suites" {
					hasCiphers = true
					break
				}
			}
			if tc.expectNoCiphers {
				assert.False(t, hasCiphers, "TLS 1.3 should not set cipher suites")
			} else {
				assert.True(t, hasCiphers, "TLS 1.2 should set cipher suites")
			}
		})
	}
}

func TestTlsProfileToBMOArgsIANAConversion(t *testing.T) {
	profile := configv1.TLSProfileSpec{
		Ciphers:       []string{"ECDHE-RSA-AES128-GCM-SHA256", "TLS_AES_128_GCM_SHA256"},
		MinTLSVersion: configv1.VersionTLS12,
	}

	args := tlsProfileToBMOArgs(profile)

	// Find the cipher suites arg
	for i, a := range args {
		if a == "--tls-cipher-suites" {
			// The IANA name for ECDHE-RSA-AES128-GCM-SHA256 should be present
			assert.Contains(t, args[i+1], "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256")
			// TLS 1.3 ciphers should NOT be in the BMO cipher list
			assert.NotContains(t, args[i+1], "TLS_AES_128_GCM_SHA256")
			return
		}
	}
	t.Fatal("--tls-cipher-suites not found in args")
}
