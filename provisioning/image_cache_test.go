/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package provisioning

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransformURLFQDNTrailingDot(t *testing.T) {
	tCases := []struct {
		name            string
		namespace       string
		inputURL        string
		expectedURL     string
		expectError     bool
		expectedPattern string
	}{
		{
			name:            "Valid HTTP URL",
			namespace:       "openshift-machine-api",
			inputURL:        "http://example.com/images/rhcos-42.80.20190725.1-openstack.qcow2",
			expectedURL:     "http://metal3-state.openshift-machine-api.svc.cluster.local.:6180/images/rhcos-42.80.20190725.1-openstack.qcow2/rhcos-42.80.20190725.1-openstack.qcow2",
			expectError:     false,
			expectedPattern: ".svc.cluster.local.:",
		},
		{
			name:            "Valid HTTPS URL",
			namespace:       "test-namespace",
			inputURL:        "https://example.com/images/test-image.qcow2",
			expectedURL:     "http://metal3-state.test-namespace.svc.cluster.local.:6180/images/test-image.qcow2/test-image.qcow2",
			expectError:     false,
			expectedPattern: ".svc.cluster.local.:",
		},
		{
			name:        "Invalid URL",
			namespace:   "openshift-machine-api",
			inputURL:    "://invalid-url",
			expectError: true,
		},
	}

	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			actualURL, err := transformURL(tc.namespace, tc.inputURL)

			if tc.expectError {
				require.Error(t, err, "Expected an error for input URL: %s", tc.inputURL)
				return
			}

			require.NoError(t, err, "Unexpected error: %v", err)
			assert.Equal(t, tc.expectedURL, actualURL,
				"Transformed URL should match expected value")

			// Verify FQDN contains trailing dot
			assert.Contains(t, actualURL, tc.expectedPattern,
				"FQDN in cache URL should have trailing dot to prevent DNS search domain appending")
		})
	}
}
