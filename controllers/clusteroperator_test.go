package controllers

import (
	"context"
	"fmt"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	osconfigv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

func TestSetClusterOperatorDisabled(t *testing.T) {
	tCases := []struct {
		name               string
		reason             StatusReason
		msg                string
		progressMsg        string
		expectedConditions []operatorv1.OperatorCondition
	}{
		{
			name:        "Disabled",
			reason:      ReasonUnsupported,
			msg:         "Operator is non-functional",
			progressMsg: "",
			expectedConditions: []operatorv1.OperatorCondition{
				newStatusCondition(operatorv1.OperatorStatusTypeDegraded, operatorv1.ConditionFalse, "", ""),
				newStatusCondition(operatorv1.OperatorStatusTypeAvailable, operatorv1.ConditionTrue, string(ReasonExpected), "Operational"),
				newStatusCondition(OperatorDisabled, operatorv1.ConditionTrue, string(ReasonUnsupported), "Operator is non-functional"),
				newStatusCondition(operatorv1.OperatorStatusTypeProgressing, operatorv1.ConditionFalse, "", ""),
				newStatusCondition(operatorv1.OperatorStatusTypeUpgradeable, operatorv1.ConditionTrue, "", ""),
			},
		},
	}

	reconciler := newFakeProvisioningReconciler(nil, []runtime.Object{&osconfigv1.Infrastructure{}})
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			err := SetClusterOperatorDisabled(context.TODO(), reconciler.osClient, RelatedObjects(ComponentNamespace), []osconfigv1.OperandVersion{})
			if err != nil {
				t.Error(err)
			}
			err = reconciler.updateCOStatus(tc.reason, tc.msg, tc.progressMsg)
			if err != nil {
				t.Error(err)
			}
			co, err := reconciler.osClient.ConfigV1().ClusterOperators().Get(context.TODO(), ClusterOperatorName, metav1.GetOptions{})
			if err != nil {
				t.Error(err)
			}

			diff := getStatusConditionsDiff(tc.expectedConditions, clusterCondtionsToOperatorConditions(co.Status.Conditions))
			if diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func clusterCondtionsToOperatorConditions(coConditions []osconfigv1.ClusterOperatorStatusCondition) []operatorv1.OperatorCondition {
	result := []operatorv1.OperatorCondition{}
	for _, cond := range coConditions {
		result = append(result, operatorv1.OperatorCondition{
			Type:    string(cond.Type),
			Status:  operatorv1.ConditionStatus(cond.Status),
			Reason:  cond.Reason,
			Message: cond.Message,
		})
	}
	return result
}

func TestUpdateCOStatus(t *testing.T) {
	tCases := []struct {
		name               string
		reason             StatusReason
		msg                string
		progressMsg        string
		expectedConditions []operatorv1.OperatorCondition
	}{
		{
			name:        "Progressing",
			reason:      ReasonSyncing,
			msg:         "",
			progressMsg: "syncing metal3 pod",
			expectedConditions: []operatorv1.OperatorCondition{
				newStatusCondition(operatorv1.OperatorStatusTypeDegraded, operatorv1.ConditionFalse, "", ""),
				newStatusCondition(operatorv1.OperatorStatusTypeAvailable, operatorv1.ConditionTrue, string(ReasonSyncing), ""),
				newStatusCondition(OperatorDisabled, operatorv1.ConditionFalse, "", ""),
				newStatusCondition(operatorv1.OperatorStatusTypeProgressing, operatorv1.ConditionTrue, string(ReasonSyncing), "syncing metal3 pod"),
				newStatusCondition(operatorv1.OperatorStatusTypeUpgradeable, operatorv1.ConditionTrue, "", ""),
			},
		},
		{
			name:        "Available",
			reason:      ReasonComplete,
			msg:         "metal3 pod running",
			progressMsg: "",
			expectedConditions: []operatorv1.OperatorCondition{
				newStatusCondition(operatorv1.OperatorStatusTypeDegraded, operatorv1.ConditionFalse, "", ""),
				newStatusCondition(operatorv1.OperatorStatusTypeProgressing, operatorv1.ConditionFalse, string(ReasonComplete), ""),
				newStatusCondition(operatorv1.OperatorStatusTypeAvailable, operatorv1.ConditionTrue, string(ReasonComplete), "metal3 pod running"),
				newStatusCondition(operatorv1.OperatorStatusTypeUpgradeable, operatorv1.ConditionTrue, "", ""),
				newStatusCondition(OperatorDisabled, operatorv1.ConditionFalse, "", ""),
			},
		},
		{
			name:        "bad config",
			reason:      ReasonInvalidConfiguration,
			msg:         "provisioningOSDownloadURL is required but is empty",
			progressMsg: "Unable to apply Provisioning CR: invalid configuration",
			expectedConditions: []operatorv1.OperatorCondition{
				newStatusCondition(operatorv1.OperatorStatusTypeDegraded, operatorv1.ConditionTrue, "InvalidConfiguration", "provisioningOSDownloadURL is required but is empty"),
				newStatusCondition(operatorv1.OperatorStatusTypeProgressing, operatorv1.ConditionTrue, "InvalidConfiguration", "Unable to apply Provisioning CR: invalid configuration"),
				newStatusCondition(operatorv1.OperatorStatusTypeAvailable, operatorv1.ConditionTrue, "", ""),
				newStatusCondition(operatorv1.OperatorStatusTypeUpgradeable, operatorv1.ConditionTrue, "", ""),
				newStatusCondition(OperatorDisabled, operatorv1.ConditionFalse, "", ""),
			},
		},
		{
			name:   "Existing Metal3 Deployment",
			reason: ReasonComplete,
			msg:    "found existing Metal3 deployment",
			expectedConditions: []operatorv1.OperatorCondition{
				newStatusCondition(operatorv1.OperatorStatusTypeDegraded, operatorv1.ConditionFalse, "", ""),
				newStatusCondition(operatorv1.OperatorStatusTypeProgressing, operatorv1.ConditionFalse, string(ReasonComplete), ""),
				newStatusCondition(operatorv1.OperatorStatusTypeAvailable, operatorv1.ConditionTrue, string(ReasonComplete), "found existing Metal3 deployment"),
				newStatusCondition(operatorv1.OperatorStatusTypeUpgradeable, operatorv1.ConditionTrue, "", ""),
				newStatusCondition(OperatorDisabled, operatorv1.ConditionFalse, "", ""),
			},
		},
		{
			name:   "New Metal3 Deployment",
			reason: ReasonComplete,
			msg:    "new Metal3 deployment completed",
			expectedConditions: []operatorv1.OperatorCondition{
				newStatusCondition(operatorv1.OperatorStatusTypeDegraded, operatorv1.ConditionFalse, "", ""),
				newStatusCondition(operatorv1.OperatorStatusTypeProgressing, operatorv1.ConditionFalse, string(ReasonComplete), ""),
				newStatusCondition(operatorv1.OperatorStatusTypeAvailable, operatorv1.ConditionTrue, string(ReasonComplete), "new Metal3 deployment completed"),
				newStatusCondition(operatorv1.OperatorStatusTypeUpgradeable, operatorv1.ConditionTrue, "", ""),
				newStatusCondition(OperatorDisabled, operatorv1.ConditionFalse, "", ""),
			},
		},
	}

	reconciler := newFakeProvisioningReconciler(nil, []runtime.Object{&osconfigv1.Infrastructure{}})
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			reconciler.operatorClient = v1helpers.NewFakeOperatorClient(&operatorv1.OperatorSpec{}, &operatorv1.OperatorStatus{}, nil)
			err := reconciler.resetOperatorConditions()
			if err != nil {
				t.Error(err)
			}
			err = reconciler.updateCOStatus(tc.reason, tc.msg, tc.progressMsg)
			if err != nil {
				t.Error(err)
			}
			_, opStatus, _, err := reconciler.operatorClient.GetOperatorState()
			if err != nil {
				t.Error(err)
			}

			diff := getStatusConditionsDiff(tc.expectedConditions, opStatus.Conditions)
			if diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

// getStatusConditionsDiff this is based on v1helpers.GetStatusDiff except it
// is focused on comparing the conditions better and nothing else.
func getStatusConditionsDiff(oldConditions []operatorv1.OperatorCondition, newConditions []operatorv1.OperatorCondition) string {
	messages := []string{}
	for _, newCondition := range newConditions {
		existingStatusCondition := v1helpers.FindOperatorCondition(oldConditions, newCondition.Type)
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
		if c := v1helpers.FindOperatorCondition(newConditions, oldCondition.Type); c == nil {
			messages = append(messages, fmt.Sprintf("%s was removed", oldCondition.Type))
		}
	}

	return strings.Join(messages, ",")
}
