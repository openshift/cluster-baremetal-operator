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

package controllers

import (
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	osconfigv1 "github.com/openshift/api/config/v1"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/apis/metal3.io/v1alpha1"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

func init() {
	utilruntime.Must(metal3iov1alpha1.AddToScheme(clientgoscheme.Scheme))
	utilruntime.Must(osconfigv1.AddToScheme(clientgoscheme.Scheme))
}
