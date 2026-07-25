package provisioning

import (
	"testing"

	"github.com/stretchr/testify/assert"

	configv1 "github.com/openshift/api/config/v1"
)

func TestShouldHonorClusterTLSProfile(t *testing.T) {
	tests := []struct {
		name     string
		policy   configv1.TLSAdherencePolicy
		expected bool
	}{
		{
			name:     "NoOpinion (empty) returns false",
			policy:   configv1.TLSAdherencePolicyNoOpinion,
			expected: false,
		},
		{
			name:     "LegacyAdheringComponentsOnly returns false",
			policy:   configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly,
			expected: false,
		},
		{
			name:     "StrictAllComponents returns true",
			policy:   configv1.TLSAdherencePolicyStrictAllComponents,
			expected: true,
		},
		{
			name:     "Unknown value returns true (fail-secure)",
			policy:   configv1.TLSAdherencePolicy("FutureValue"),
			expected: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ShouldHonorClusterTLSProfile(tc.policy)
			assert.Equal(t, tc.expected, result)
		})
	}
}

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

func TestTlsGroupsToOpenSSLCurves(t *testing.T) {
	tests := []struct {
		name                string
		groups              []configv1.TLSGroup
		expected            string
		expectedUnsupported []string
	}{
		{
			name: "standard groups pass through",
			groups: []configv1.TLSGroup{
				configv1.TLSGroupX25519,
				configv1.TLSGroupSecP256r1,
				configv1.TLSGroupSecP384r1,
			},
			expected: "X25519:secp256r1:secp384r1",
		},
		{
			name: "all supported groups",
			groups: []configv1.TLSGroup{
				configv1.TLSGroupX25519,
				configv1.TLSGroupSecP256r1,
				configv1.TLSGroupSecP384r1,
				configv1.TLSGroupSecP521r1,
				configv1.TLSGroupX25519MLKEM768,
				configv1.TLSGroupSecP256r1MLKEM768,
				configv1.TLSGroupSecP384r1MLKEM1024,
			},
			expected: "X25519:secp256r1:secp384r1:secp521r1:X25519MLKEM768:SecP256r1MLKEM768:SecP384r1MLKEM1024",
		},
		{
			name: "hybrid PQC groups included",
			groups: []configv1.TLSGroup{
				configv1.TLSGroupX25519,
				configv1.TLSGroupSecP256r1MLKEM768,
				configv1.TLSGroupSecP384r1MLKEM1024,
			},
			expected: "X25519:SecP256r1MLKEM768:SecP384r1MLKEM1024",
		},
		{
			name: "unknown groups filtered out",
			groups: []configv1.TLSGroup{
				configv1.TLSGroupX25519,
				configv1.TLSGroup("FutureGroup"),
			},
			expected:            "X25519",
			expectedUnsupported: []string{"FutureGroup"},
		},
		{
			name:     "empty input",
			groups:   []configv1.TLSGroup{},
			expected: "",
		},
		{
			name:     "nil input",
			groups:   nil,
			expected: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, unsupported := tlsGroupsToOpenSSLCurves(tc.groups)
			assert.Equal(t, tc.expected, result)
			assert.Equal(t, tc.expectedUnsupported, unsupported)
		})
	}
}

func TestTlsGroupsToGoNames(t *testing.T) {
	tests := []struct {
		name                string
		groups              []configv1.TLSGroup
		expected            []string
		expectedUnsupported []string
	}{
		{
			name: "standard groups mapped to Go names",
			groups: []configv1.TLSGroup{
				configv1.TLSGroupX25519,
				configv1.TLSGroupSecP256r1,
				configv1.TLSGroupSecP384r1,
				configv1.TLSGroupSecP521r1,
				configv1.TLSGroupX25519MLKEM768,
			},
			expected: []string{"X25519", "CurveP256", "CurveP384", "CurveP521", "X25519MLKEM768"},
		},
		{
			name: "forward-compat groups filtered out",
			groups: []configv1.TLSGroup{
				configv1.TLSGroupX25519,
				configv1.TLSGroupSecP256r1MLKEM768,
				configv1.TLSGroupSecP384r1MLKEM1024,
			},
			expected:            []string{"X25519"},
			expectedUnsupported: []string{"SecP256r1MLKEM768", "SecP384r1MLKEM1024"},
		},
		{
			name:     "empty input",
			groups:   []configv1.TLSGroup{},
			expected: nil,
		},
		{
			name:     "nil input",
			groups:   nil,
			expected: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, unsupported := tlsGroupsToGoNames(tc.groups)
			assert.Equal(t, tc.expected, result)
			assert.Equal(t, tc.expectedUnsupported, unsupported)
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

func TestTlsProfileToApacheEnvVarsCurves(t *testing.T) {
	t.Run("curves env vars set when Groups is populated", func(t *testing.T) {
		profile := configv1.TLSProfileSpec{
			Ciphers:       []string{"ECDHE-RSA-AES128-GCM-SHA256"},
			MinTLSVersion: configv1.VersionTLS12,
			Groups: []configv1.TLSGroup{
				configv1.TLSGroupX25519,
				configv1.TLSGroupSecP256r1,
				configv1.TLSGroupSecP384r1,
			},
		}

		envVars := tlsProfileToApacheEnvVars(profile)
		envMap := map[string]string{}
		for _, e := range envVars {
			envMap[e.Name] = e.Value
		}

		assert.Equal(t, "X25519:secp256r1:secp384r1", envMap["IRONIC_TLS_CURVES"])
		assert.Equal(t, "X25519:secp256r1:secp384r1", envMap["IRONIC_VMEDIA_CURVES"])
	})

	t.Run("curves env vars NOT set when Groups is nil", func(t *testing.T) {
		profile := configv1.TLSProfileSpec{
			Ciphers:       []string{"ECDHE-RSA-AES128-GCM-SHA256"},
			MinTLSVersion: configv1.VersionTLS12,
		}

		envVars := tlsProfileToApacheEnvVars(profile)
		envMap := map[string]string{}
		for _, e := range envVars {
			envMap[e.Name] = e.Value
		}

		assert.NotContains(t, envMap, "IRONIC_TLS_CURVES")
		assert.NotContains(t, envMap, "IRONIC_VMEDIA_CURVES")
	})

	t.Run("curves env vars NOT set when Groups is empty", func(t *testing.T) {
		profile := configv1.TLSProfileSpec{
			Ciphers:       []string{"ECDHE-RSA-AES128-GCM-SHA256"},
			MinTLSVersion: configv1.VersionTLS12,
			Groups:        []configv1.TLSGroup{},
		}

		envVars := tlsProfileToApacheEnvVars(profile)
		envMap := map[string]string{}
		for _, e := range envVars {
			envMap[e.Name] = e.Value
		}

		assert.NotContains(t, envMap, "IRONIC_TLS_CURVES")
		assert.NotContains(t, envMap, "IRONIC_VMEDIA_CURVES")
	})

	t.Run("hybrid PQC groups included in curves env vars", func(t *testing.T) {
		profile := configv1.TLSProfileSpec{
			Ciphers:       []string{"ECDHE-RSA-AES128-GCM-SHA256"},
			MinTLSVersion: configv1.VersionTLS12,
			Groups: []configv1.TLSGroup{
				configv1.TLSGroupSecP256r1MLKEM768,
				configv1.TLSGroupSecP384r1MLKEM1024,
			},
		}

		envVars := tlsProfileToApacheEnvVars(profile)
		envMap := map[string]string{}
		for _, e := range envVars {
			envMap[e.Name] = e.Value
		}

		assert.Equal(t, "SecP256r1MLKEM768:SecP384r1MLKEM1024", envMap["IRONIC_TLS_CURVES"])
		assert.Equal(t, "SecP256r1MLKEM768:SecP384r1MLKEM1024", envMap["IRONIC_VMEDIA_CURVES"])
	})
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

func TestTlsProfileToBMOArgsCurves(t *testing.T) {
	t.Run("curve preferences set when Groups is populated", func(t *testing.T) {
		profile := configv1.TLSProfileSpec{
			Ciphers:       []string{"ECDHE-RSA-AES128-GCM-SHA256"},
			MinTLSVersion: configv1.VersionTLS12,
			Groups: []configv1.TLSGroup{
				configv1.TLSGroupX25519,
				configv1.TLSGroupSecP256r1,
				configv1.TLSGroupSecP384r1,
			},
		}

		args := tlsProfileToBMOArgs(profile)

		// Find --tls-curve-preferences
		found := false
		for i, a := range args {
			if a == "--tls-curve-preferences" {
				found = true
				assert.Equal(t, "X25519,CurveP256,CurveP384", args[i+1])
				break
			}
		}
		assert.True(t, found, "--tls-curve-preferences should be present")
	})

	t.Run("curve preferences NOT set when Groups is nil", func(t *testing.T) {
		profile := configv1.TLSProfileSpec{
			Ciphers:       []string{"ECDHE-RSA-AES128-GCM-SHA256"},
			MinTLSVersion: configv1.VersionTLS12,
		}

		args := tlsProfileToBMOArgs(profile)

		for _, a := range args {
			assert.NotEqual(t, "--tls-curve-preferences", a)
		}
	})

	t.Run("curve preferences NOT set when Groups is empty", func(t *testing.T) {
		profile := configv1.TLSProfileSpec{
			Ciphers:       []string{"ECDHE-RSA-AES128-GCM-SHA256"},
			MinTLSVersion: configv1.VersionTLS12,
			Groups:        []configv1.TLSGroup{},
		}

		args := tlsProfileToBMOArgs(profile)

		for _, a := range args {
			assert.NotEqual(t, "--tls-curve-preferences", a)
		}
	})

	t.Run("curve preferences with TLS 1.3 still includes curves", func(t *testing.T) {
		profile := configv1.TLSProfileSpec{
			Ciphers:       []string{"TLS_AES_128_GCM_SHA256"},
			MinTLSVersion: configv1.VersionTLS13,
			Groups: []configv1.TLSGroup{
				configv1.TLSGroupX25519,
				configv1.TLSGroupSecP256r1,
			},
		}

		args := tlsProfileToBMOArgs(profile)

		// Cipher suites should not be present (TLS 1.3)
		for _, a := range args {
			assert.NotEqual(t, "--tls-cipher-suites", a)
		}

		// But curve preferences should still be present
		found := false
		for i, a := range args {
			if a == "--tls-curve-preferences" {
				found = true
				assert.Equal(t, "X25519,CurveP256", args[i+1])
				break
			}
		}
		assert.True(t, found, "--tls-curve-preferences should be present even with TLS 1.3")
	})

	t.Run("forward-compat groups filtered out", func(t *testing.T) {
		profile := configv1.TLSProfileSpec{
			Ciphers:       []string{"ECDHE-RSA-AES128-GCM-SHA256"},
			MinTLSVersion: configv1.VersionTLS12,
			Groups: []configv1.TLSGroup{
				configv1.TLSGroupX25519,
				configv1.TLSGroupSecP256r1MLKEM768,
				configv1.TLSGroupSecP384r1MLKEM1024,
			},
		}

		args := tlsProfileToBMOArgs(profile)

		for i, a := range args {
			if a == "--tls-curve-preferences" {
				// Forward-compat groups filtered — only X25519 remains
				assert.Equal(t, "X25519", args[i+1])
				return
			}
		}
		t.Fatal("--tls-curve-preferences not found in args")
	})
}
