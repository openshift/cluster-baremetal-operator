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
	"fmt"
	"net/http"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/apis/metal3.io/v1alpha1"
	"github.com/openshift/generic-admission-server/pkg/apiserver"
)

type ProvisioningValidatingWebHook struct {
}

// +kubebuilder:rbac:groups=flowcontrol.apiserver.k8s.io,resources=prioritylevelconfigurations;flowschemas,verbs=get;list;watch
// +kubebuilder:rbac:groups=flowcontrol.apiserver.k8s.io,resources=flowschemas/status,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups=admission.metal3.io,resources=provisionings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=get;list;watch;create;update;patch;delete

var _ apiserver.AdmissionHook = &ProvisioningValidatingWebHook{}

func (a *ProvisioningValidatingWebHook) ValidatingResource() (plural schema.GroupVersionResource, singular string) {
	return schema.GroupVersionResource{
		Group:    "admission.metal3.io",
		Version:  metal3iov1alpha1.GroupVersion.Version,
		Resource: "provisioningvalidators",
	}, "provisioningvalidator"
}

func (a *ProvisioningValidatingWebHook) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	klog.Infof("initializing provisioning admission webhook")
	return nil
}

func withError(err error) *admissionv1beta1.AdmissionResponse {
	return &admissionv1beta1.AdmissionResponse{
		Allowed: false,
		Result: &metav1.Status{
			Status: metav1.StatusFailure, Code: http.StatusBadRequest, Reason: metav1.StatusReasonBadRequest,
			Message: err.Error(),
		},
	}
}

func (a *ProvisioningValidatingWebHook) Validate(admissionSpec *admissionv1beta1.AdmissionRequest) *admissionv1beta1.AdmissionResponse {
	klog.Info("validating admission webhook", admissionSpec.Operation, admissionSpec.RequestResource.String())
	prov := &metal3iov1alpha1.Provisioning{}
	err := json.Unmarshal(admissionSpec.Object.Raw, prov)
	if err != nil {
		return withError(err)
	}

	if admissionSpec.Operation == admissionv1beta1.Create {
		if prov.Name != metal3iov1alpha1.ProvisioningSingletonName {
			return withError(fmt.Errorf("Provisioning object is a singleton and must be named \"%s\"", metal3iov1alpha1.ProvisioningSingletonName))
		}
	}

	err = prov.ValidateBaremetalProvisioningConfig()
	if err != nil {
		return withError(err)
	}
	return &admissionv1beta1.AdmissionResponse{
		Allowed: true,
	}
}
