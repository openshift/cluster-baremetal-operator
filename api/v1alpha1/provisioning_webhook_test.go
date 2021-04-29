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

package v1alpha1

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func errorContains(out error, want string) bool {
	if out == nil {
		return want == ""
	}
	if want == "" {
		return false
	}
	return strings.Contains(out.Error(), want)
}

func TestProvisioningValidateCreate(t *testing.T) {
	tests := []struct {
		name    string
		p       *Provisioning
		wantErr string
	}{
		{
			name: "name correct",
			p:    &Provisioning{ObjectMeta: metav1.ObjectMeta{Name: "provisioning-configuration"}},
		},
		{
			name:    "name wrong",
			p:       &Provisioning{ObjectMeta: metav1.ObjectMeta{Name: "something"}},
			wantErr: "Provisioning object is a singleton and must be named \"provisioning-configuration\"",
		},
	}
	enabledFeatures = EnabledFeatures{
		ProvisioningNetwork: map[ProvisioningNetwork]bool{
			ProvisioningNetworkDisabled:  true,
			ProvisioningNetworkUnmanaged: true,
			ProvisioningNetworkManaged:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.p.Spec = *disabledProvisioning().build()
			if err := tt.p.ValidateCreate(); !errorContains(err, tt.wantErr) {
				t.Errorf("Provisioning.ValidateCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
