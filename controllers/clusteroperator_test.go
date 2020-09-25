package controllers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	osconfigv1 "github.com/openshift/api/config/v1"
	fakeconfigclientset "github.com/openshift/client-go/config/clientset/versioned/fake"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
	provisioning "github.com/openshift/cluster-baremetal-operator/provisioning"
	"github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"
)

func TestUpdateCOStatusDisabled(t *testing.T) {
	tCases := []struct {
		name               string
		expectedConditions []osconfigv1.ClusterOperatorStatusCondition
		expected           bool
	}{
		{
			name: "Incorrect Condition",
			expectedConditions: []osconfigv1.ClusterOperatorStatusCondition{
				setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionFalse, "", ""),
				setStatusCondition(OperatorDisabled, osconfigv1.ConditionTrue, "", ""),
			},
			expected: false,
		},
		{
			name: "Correct Condition",
			expectedConditions: []osconfigv1.ClusterOperatorStatusCondition{
				setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, "", ""),
				setStatusCondition(OperatorDisabled, osconfigv1.ConditionTrue, "", ""),
			},
			expected: true,
		},
	}

	reconciler := newFakeProvisioningReconciler(setUpSchemeForReconciler(), &osconfigv1.Infrastructure{})
	co, _ := reconciler.createClusterOperator()
	reconciler.OSClient = fakeconfigclientset.NewSimpleClientset(co)

	for _, tc := range tCases {
		reconciler.updateCOStatusDisabled()
		gotCO, _ := reconciler.OSClient.ConfigV1().ClusterOperators().Get(context.Background(), clusterOperatorName, metav1.GetOptions{})

		for _, expectedCondition := range tc.expectedConditions {
			ok := v1helpers.IsStatusConditionPresentAndEqual(
				gotCO.Status.Conditions, expectedCondition.Type, expectedCondition.Status,
			)
			if !ok {
				assert.Regexp(t, tc.expected, ok)
			}
		}
	}
}

func TestGetOrCreateClusterOperator(t *testing.T) {
	var namespace = "openshift-machine-api"

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
				},
				Status: osconfigv1.ClusterOperatorStatus{
					Conditions: defaultConditions,
					RelatedObjects: []osconfigv1.ObjectReference{
						{
							Group:    "",
							Resource: "namespaces",
							Name:     namespace,
						},
					},
				},
			},
		},
		{
			name: "Get existing clusteroperator",
			existingCO: &osconfigv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterOperatorName,
				},
				Status: osconfigv1.ClusterOperatorStatus{
					Conditions: conditions,
				},
			},
			expectedCO: &osconfigv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterOperatorName,
				},
				Status: osconfigv1.ClusterOperatorStatus{
					Conditions: conditions,
				},
			},
		},
	}

	for _, tc := range testCases {
		var osClient *fakeconfigclientset.Clientset
		if tc.existingCO != nil {
			osClient = fakeconfigclientset.NewSimpleClientset(tc.existingCO)
		} else {
			osClient = fakeconfigclientset.NewSimpleClientset()
		}
		reconciler := newFakeProvisioningReconciler(setUpSchemeForReconciler(), &osconfigv1.Infrastructure{})
		reconciler.OSClient = osClient

		co, err := reconciler.getOrCreateClusterOperator()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		normalizeTransitionTimes(co.Status, tc.expectedCO.Status)

		if !equality.Semantic.DeepEqual(co, tc.expectedCO) {
			t.Errorf("got: %v, expected: %v", co, tc.expectedCO)
		}
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

func TestUpdateCOStatusDegraded(t *testing.T) {
	baremetalCR := &metal3iov1alpha1.Provisioning{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Provisioning",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: BaremetalProvisioningCR,
		},
	}

	tCases := []struct {
		name                       string
		spec                       metal3iov1alpha1.ProvisioningSpec
		degradedReason             string
		expectedConditions         []osconfigv1.ClusterOperatorStatusCondition
		expectedDegradedMessage    string
		expectedProgressingMessage string
		expectedMatch              bool
	}{
		{
			name: "Incorrect Config",
			spec: metal3iov1alpha1.ProvisioningSpec{
				ProvisioningInterface:     "eth0",
				ProvisioningIP:            "172.30.20.3",
				ProvisioningNetworkCIDR:   "172.30.20.0/24",
				ProvisioningDHCPRange:     "172.30.20.11, 172.30.20.101",
				ProvisioningOSDownloadURL: "",
				ProvisioningNetwork:       "Managed",
			},
			degradedReason: "Incorrect Config",
			expectedConditions: []osconfigv1.ClusterOperatorStatusCondition{
				setStatusCondition(osconfigv1.OperatorDegraded, osconfigv1.ConditionTrue, "Incorrect Config", "Operator is Degraded"),
				setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionTrue, "ProvisioningOSDownloadURL is required but is empty", "Operator is Degraded while Progressing"),
				setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionFalse, "", ""),
				setStatusCondition(OperatorDisabled, osconfigv1.ConditionFalse, "", ""),
			},
			expectedDegradedMessage:    "Operator is Degraded",
			expectedProgressingMessage: "Operator is Degraded while Progressing",
			expectedMatch:              true,
		},
	}

	reconciler := newFakeProvisioningReconciler(setUpSchemeForReconciler(), &osconfigv1.Infrastructure{})
	co, _ := reconciler.createClusterOperator()
	reconciler.OSClient = fakeconfigclientset.NewSimpleClientset(co)

	for _, tc := range tCases {
		baremetalCR.Spec = tc.spec
		if err := provisioning.ValidateBaremetalProvisioningConfig(baremetalCR); err != nil {
			reconciler.updateCOStatusDegraded(tc.degradedReason, err.Error())
		}
		gotCO, _ := reconciler.OSClient.ConfigV1().ClusterOperators().Get(context.Background(), clusterOperatorName, metav1.GetOptions{})

		for _, expectedCondition := range tc.expectedConditions {
			ok := v1helpers.IsStatusConditionPresentAndEqual(
				gotCO.Status.Conditions, expectedCondition.Type, expectedCondition.Status,
			)
			if !ok {
				assert.Regexp(t, tc.expectedMatch, ok)
			}
		}
		for _, condition := range gotCO.Status.Conditions {
			if condition.Type == osconfigv1.OperatorDegraded {
				assert.Regexp(t, condition.Reason, tc.degradedReason)
				assert.Regexp(t, condition.Message, tc.expectedDegradedMessage)
			}
			if condition.Type == osconfigv1.OperatorProgressing {
				assert.Contains(t, condition.Reason, "ProvisioningOSDownloadURL")
				assert.Regexp(t, condition.Message, tc.expectedProgressingMessage)
			}
		}
	}
}
