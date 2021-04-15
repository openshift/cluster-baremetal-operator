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

package webhooks

import (
	"encoding/json"
	"strings"
	"testing"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/cluster-baremetal-operator/apis/metal3.io/v1alpha1"
)

func responseContains(out *admissionv1beta1.AdmissionResponse, want string) bool {
	if out.Allowed {
		return want == ""
	}
	if want == "" {
		return out.Allowed
	}

	return strings.Contains(out.Result.Message, want)
}

func TestProvisioningValidateCreate(t *testing.T) {
	tests := []struct {
		name    string
		p       *v1alpha1.Provisioning
		wantErr string
	}{
		{
			name: "name correct",
			p:    &v1alpha1.Provisioning{ObjectMeta: metav1.ObjectMeta{Name: "provisioning-configuration"}},
		},
		{
			name:    "name wrong",
			p:       &v1alpha1.Provisioning{ObjectMeta: metav1.ObjectMeta{Name: "something"}},
			wantErr: "Provisioning object is a singleton and must be named \"provisioning-configuration\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.p.Spec = v1alpha1.ProvisioningSpec{
				ProvisioningInterface:     "",
				ProvisioningIP:            "172.30.20.3",
				ProvisioningNetworkCIDR:   "172.30.20.0/24",
				ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
				ProvisioningNetwork:       "Disabled",
			}

			var err error
			admissionSpec := &admissionv1beta1.AdmissionRequest{Operation: admissionv1beta1.Create}
			admissionSpec.Object.Raw, err = json.Marshal(tt.p)
			if err != nil {
				t.Errorf("marshal() error = %v", err)
			}
			wh := &ProvisioningValidatingWebHook{}

			if resp := wh.Validate(admissionSpec); !responseContains(resp, tt.wantErr) {
				t.Errorf("Provisioning.ValidateCreate() resp = %v, wantErr %v", resp, tt.wantErr)
			}
		})
	}
}
