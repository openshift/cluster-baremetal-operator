package controllers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekube "k8s.io/client-go/kubernetes/fake"

	configv1 "github.com/openshift/api/config/v1"
	osconfigv1 "github.com/openshift/api/config/v1"
	fakeconfigclientset "github.com/openshift/client-go/config/clientset/versioned/fake"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
	"github.com/openshift/cluster-baremetal-operator/pkg/externalclients"
	"github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"
)

func TestUpdateCOStatus(t *testing.T) {
	tCases := []struct {
		name               string
		reason             StatusReason
		msg                string
		progressMsg        string
		expectedConditions []osconfigv1.ClusterOperatorStatusCondition
	}{
		{
			name:        "Disabled",
			reason:      ReasonUnsupported,
			msg:         "Operator is non-functional",
			progressMsg: "",
			expectedConditions: []osconfigv1.ClusterOperatorStatusCondition{
				setStatusCondition(osconfigv1.OperatorDegraded, osconfigv1.ConditionFalse, "", ""),
				setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, string(ReasonExpected), "Operational"),
				setStatusCondition(OperatorDisabled, osconfigv1.ConditionTrue, string(ReasonUnsupported), "Operator is non-functional"),
				setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionFalse, "", ""),
				setStatusCondition(osconfigv1.OperatorUpgradeable, osconfigv1.ConditionTrue, "", ""),
			},
		},
		{
			name:        "Progressing",
			reason:      ReasonSyncing,
			msg:         "",
			progressMsg: "syncing metal3 pod",
			expectedConditions: []osconfigv1.ClusterOperatorStatusCondition{
				setStatusCondition(osconfigv1.OperatorDegraded, osconfigv1.ConditionFalse, "", ""),
				setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, string(ReasonSyncing), ""),
				setStatusCondition(OperatorDisabled, osconfigv1.ConditionFalse, "", ""),
				setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionTrue, string(ReasonSyncing), "syncing metal3 pod"),
				setStatusCondition(osconfigv1.OperatorUpgradeable, osconfigv1.ConditionTrue, "", ""),
			},
		},
		{
			name:        "Available",
			reason:      ReasonComplete,
			msg:         "metal3 pod running",
			progressMsg: "",
			expectedConditions: []osconfigv1.ClusterOperatorStatusCondition{
				setStatusCondition(osconfigv1.OperatorDegraded, osconfigv1.ConditionFalse, "", ""),
				setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionFalse, string(ReasonComplete), ""),
				setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, string(ReasonComplete), "metal3 pod running"),
				setStatusCondition(osconfigv1.OperatorUpgradeable, osconfigv1.ConditionTrue, "", ""),
				setStatusCondition(OperatorDisabled, osconfigv1.ConditionFalse, "", ""),
			},
		},
	}

	for _, tc := range tCases {
		ec := externalclients.NewExternalResourceClient(fakekube.NewSimpleClientset(), fakeconfigclientset.NewSimpleClientset(&osconfigv1.Infrastructure{}))
		reconciler := newFakeProvisioningReconciler(setUpSchemeForReconciler(), ec)
		co, err := reconciler.createClusterOperator()
		if err != nil {
			t.Error(err)
		}

		reconciler.updateCOStatus(co, tc.reason, tc.msg, tc.progressMsg)
		diff := getStatusConditionsDiff(tc.expectedConditions, co.Status.Conditions)
		if diff != "" {
			t.Fatal(diff)
		}
	}
}

func TestEnsureClusterOperator(t *testing.T) {
	var defaultConditions = []osconfigv1.ClusterOperatorStatusCondition{
		setStatusCondition(
			osconfigv1.OperatorProgressing,
			osconfigv1.ConditionFalse,
			"", "",
		),
		setStatusCondition(
			osconfigv1.OperatorDegraded,
			osconfigv1.ConditionFalse,
			"", "",
		),
		setStatusCondition(
			osconfigv1.OperatorAvailable,
			osconfigv1.ConditionFalse,
			"", "",
		),
		setStatusCondition(
			osconfigv1.OperatorUpgradeable,
			osconfigv1.ConditionTrue,
			"", "",
		),
		setStatusCondition(
			OperatorDisabled,
			osconfigv1.ConditionFalse,
			"", "",
		),
	}

	var conditions = []osconfigv1.ClusterOperatorStatusCondition{
		setStatusCondition(
			osconfigv1.OperatorProgressing,
			osconfigv1.ConditionFalse,
			"", "",
		),
		setStatusCondition(
			osconfigv1.OperatorDegraded,
			osconfigv1.ConditionFalse,
			"", "",
		),
		{
			Type:    "Available",
			Status:  "true",
			Reason:  "",
			Message: "",
		},
	}

	testCases := []struct {
		name       string
		existingCO *osconfigv1.ClusterOperator
		expectedCO *osconfigv1.ClusterOperator
	}{
		{
			name:       "No clusteroperator",
			existingCO: nil,
			expectedCO: &osconfigv1.ClusterOperator{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ClusterOperator",
					APIVersion: "config.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterOperatorName,
					Annotations: map[string]string{
						"exclude.release.openshift.io/internal-openshift-hosted":      "true",
						"include.release.openshift.io/self-managed-high-availability": "true",
						"include.release.openshift.io/single-node-developer":          "true",
					},
				},
				Status: osconfigv1.ClusterOperatorStatus{
					Conditions:     defaultConditions,
					RelatedObjects: relatedObjects(),
					Versions:       []osconfigv1.OperandVersion{{Name: "operator", Version: "test-version"}},
				},
			},
		},
		{
			name: "Get existing clusteroperator",
			existingCO: &osconfigv1.ClusterOperator{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ClusterOperator",
					APIVersion: "config.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterOperatorName,
					Annotations: map[string]string{
						"include.release.openshift.io/self-managed-high-availability": "true",
						"include.release.openshift.io/single-node-developer":          "true",
					},
				},
				Status: osconfigv1.ClusterOperatorStatus{
					Conditions: conditions,
				},
			},
			expectedCO: &osconfigv1.ClusterOperator{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ClusterOperator",
					APIVersion: "config.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterOperatorName,
					Annotations: map[string]string{
						"include.release.openshift.io/self-managed-high-availability": "true",
						"include.release.openshift.io/single-node-developer":          "true",
					},
				},
				Status: osconfigv1.ClusterOperatorStatus{
					Conditions:     conditions,
					RelatedObjects: relatedObjects(),
					Versions:       []osconfigv1.OperandVersion{{Name: "operator", Version: "test-version"}},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var reconciler *ProvisioningReconciler
			if tc.existingCO != nil {
				ec := externalclients.NewExternalResourceClient(
					fakekube.NewSimpleClientset(),
					fakeconfigclientset.NewSimpleClientset(&osconfigv1.Infrastructure{}, tc.existingCO))
				reconciler = newFakeProvisioningReconciler(setUpSchemeForReconciler(), ec)
			} else {
				ec := externalclients.NewExternalResourceClient(
					fakekube.NewSimpleClientset(),
					fakeconfigclientset.NewSimpleClientset(&osconfigv1.Infrastructure{}))
				reconciler = newFakeProvisioningReconciler(setUpSchemeForReconciler(), ec)
			}
			reconciler.ReleaseVersion = "test-version"

			var co = tc.existingCO
			if co == nil {
				var err error
				co, err = reconciler.createClusterOperator()
				if err != nil {
					t.Error(err)
				}
			}

			reconciler.ensureDefaultsClusterOperator(co)

			normalizeTransitionTimes(co.Status, tc.expectedCO.Status)

			if !equality.Semantic.DeepEqual(co, tc.expectedCO) {
				t.Error(cmp.Diff(tc.expectedCO, co))
			}
		})
	}
}

func normalizeTransitionTimes(got, expected osconfigv1.ClusterOperatorStatus) {
	now := metav1.NewTime(time.Now())

	for i := range got.Conditions {
		got.Conditions[i].LastTransitionTime = now
	}

	for i := range expected.Conditions {
		expected.Conditions[i].LastTransitionTime = now
	}
}

// getStatusConditionsDiff this is based on v1helpers.GetStatusDiff except it
// is focused on comparing the conditions better and nothing else.
func getStatusConditionsDiff(oldConditions []configv1.ClusterOperatorStatusCondition, newConditions []configv1.ClusterOperatorStatusCondition) string {
	messages := []string{}
	for _, newCondition := range newConditions {
		existingStatusCondition := v1helpers.FindStatusCondition(oldConditions, newCondition.Type)
		if existingStatusCondition == nil {
			messages = append(messages, fmt.Sprintf("%s set to %s (%q)", newCondition.Type, newCondition.Status, newCondition.Message))
			continue
		}
		if existingStatusCondition.Status != newCondition.Status {
			messages = append(messages, fmt.Sprintf("%s changed from %s to %s (%q)", existingStatusCondition.Type, existingStatusCondition.Status, newCondition.Status, newCondition.Message))
			continue
		}
		if existingStatusCondition.Message != newCondition.Message {
			messages = append(messages, fmt.Sprintf("%s message changed from %q to %q", existingStatusCondition.Type, existingStatusCondition.Message, newCondition.Message))
		}
		if existingStatusCondition.Reason != newCondition.Reason {
			messages = append(messages, fmt.Sprintf("%s reason changed from %q to %q", existingStatusCondition.Type, existingStatusCondition.Reason, newCondition.Reason))
		}
	}
	for _, oldCondition := range oldConditions {
		// This should not happen. It means we removed old condition entirely instead of just changing its status
		if c := v1helpers.FindStatusCondition(newConditions, oldCondition.Type); c == nil {
			messages = append(messages, fmt.Sprintf("%s was removed", oldCondition.Type))
		}
	}

	return strings.Join(messages, ",")
}

func TestUpdateCOStatusDegraded(t *testing.T) {
	baremetalCR := &metal3iov1alpha1.Provisioning{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Provisioning",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: metal3iov1alpha1.ProvisioningSingletonName,
		},
	}

	tCases := []struct {
		name               string
		spec               metal3iov1alpha1.ProvisioningSpec
		expectedConditions []osconfigv1.ClusterOperatorStatusCondition
	}{
		{
			name: "Incorrect Config",
			spec: metal3iov1alpha1.ProvisioningSpec{
				ProvisioningInterface:     "eth0",
				ProvisioningIP:            "172.30.20.3",
				ProvisioningNetworkCIDR:   "172.30.20.0/24",
				ProvisioningDHCPRange:     "172.30.20.11,172.30.20.101",
				ProvisioningOSDownloadURL: "",
				ProvisioningNetwork:       "Managed",
			},
			expectedConditions: []osconfigv1.ClusterOperatorStatusCondition{
				setStatusCondition(osconfigv1.OperatorDegraded, osconfigv1.ConditionTrue, "InvalidConfiguration", "provisioningOSDownloadURL is required but is empty"),
				setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionTrue, "InvalidConfiguration", "Unable to apply Provisioning CR: invalid configuration"),
				setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, "", ""),
				setStatusCondition(osconfigv1.OperatorUpgradeable, osconfigv1.ConditionTrue, "", ""),
				setStatusCondition(OperatorDisabled, osconfigv1.ConditionFalse, "", ""),
			},
		},
	}

	for _, tc := range tCases {
		ec := externalclients.NewExternalResourceClient(
			fakekube.NewSimpleClientset(),
			fakeconfigclientset.NewSimpleClientset(&osconfigv1.Infrastructure{}))
		reconciler := newFakeProvisioningReconciler(setUpSchemeForReconciler(), ec)
		co, err := reconciler.createClusterOperator()
		if err != nil {
			t.Error(err)
		}
		baremetalCR.Spec = tc.spec
		if err := baremetalCR.ValidateBaremetalProvisioningConfig(); err != nil {
			reconciler.updateCOStatus(co, ReasonInvalidConfiguration, err.Error(), "Unable to apply Provisioning CR: invalid configuration")
		}

		diff := getStatusConditionsDiff(tc.expectedConditions, co.Status.Conditions)
		if diff != "" {
			t.Fatal(diff)
		}
	}
}

func TestUpdateCOStatusAvailable(t *testing.T) {
	tCases := []struct {
		name               string
		msg                string
		expectedConditions []osconfigv1.ClusterOperatorStatusCondition
	}{
		{
			name: "Existing Metal3 Deployment",
			msg:  "found existing Metal3 deployment",
			expectedConditions: []osconfigv1.ClusterOperatorStatusCondition{
				setStatusCondition(osconfigv1.OperatorDegraded, osconfigv1.ConditionFalse, "", ""),
				setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionFalse, string(ReasonComplete), ""),
				setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, string(ReasonComplete), "found existing Metal3 deployment"),
				setStatusCondition(osconfigv1.OperatorUpgradeable, osconfigv1.ConditionTrue, "", ""),
				setStatusCondition(OperatorDisabled, osconfigv1.ConditionFalse, "", ""),
			},
		},
		{
			name: "New Metal3 Deployment",
			msg:  "new Metal3 deployment completed",
			expectedConditions: []osconfigv1.ClusterOperatorStatusCondition{
				setStatusCondition(osconfigv1.OperatorDegraded, osconfigv1.ConditionFalse, "", ""),
				setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionFalse, string(ReasonComplete), ""),
				setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, string(ReasonComplete), "new Metal3 deployment completed"),
				setStatusCondition(osconfigv1.OperatorUpgradeable, osconfigv1.ConditionTrue, "", ""),
				setStatusCondition(OperatorDisabled, osconfigv1.ConditionFalse, "", ""),
			},
		},
	}

	for _, tc := range tCases {
		ec := externalclients.NewExternalResourceClient(fakekube.NewSimpleClientset(), fakeconfigclientset.NewSimpleClientset(&osconfigv1.Infrastructure{}))
		reconciler := newFakeProvisioningReconciler(setUpSchemeForReconciler(), ec)
		co, err := reconciler.createClusterOperator()
		if err != nil {
			t.Error(err)
		}
		reconciler.updateCOStatus(co, ReasonComplete, tc.msg, "")

		diff := getStatusConditionsDiff(tc.expectedConditions, co.Status.Conditions)
		if diff != "" {
			t.Fatal(diff)
		}
	}
}
