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

package main

import (
	"testing"

	"k8s.io/apimachinery/pkg/fields"

	"github.com/openshift/cluster-baremetal-operator/controllers"
	"github.com/openshift/cluster-baremetal-operator/provisioning"
)

func TestSecretCacheNamespaces(t *testing.T) {
	namespaces := secretCacheNamespaces()
	if len(namespaces) != 2 {
		t.Fatalf("expected two Secret cache namespaces, got %d", len(namespaces))
	}
	if namespaces[controllers.ComponentNamespace].FieldSelector != nil {
		t.Fatal("operator namespace Secrets must not be filtered")
	}
	selector := namespaces[provisioning.OpenshiftConfigNamespace].FieldSelector
	if selector == nil {
		t.Fatal("pull-secret cache must have a field selector")
	}
	if !selector.Matches(fields.Set{"metadata.name": provisioning.PullSecretName}) ||
		selector.Matches(fields.Set{"metadata.name": "other-secret"}) {
		t.Fatalf("unexpected pull-secret field selector: %s", selector)
	}
}

func TestDefaultCacheNamespaces(t *testing.T) {
	namespaces := defaultCacheNamespaces()
	if len(namespaces) != 1 {
		t.Fatalf("expected one default cache namespace, got %d", len(namespaces))
	}
	if _, ok := namespaces[controllers.ComponentNamespace]; !ok {
		t.Fatal("operator namespace must be in the default cache")
	}
}
