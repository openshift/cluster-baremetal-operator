package controllers

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	osconfigv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"
)

// StatusReason is a MixedCaps string representing the reason for a
// status condition change.
type StatusReason string

const (
	clusterOperatorName = "baremetal"

	// OperatorDisabled represents a Disabled ClusterStatusConditionTypes
	OperatorDisabled osconfigv1.ClusterStatusConditionType = "Disabled"

	// ReasonEmpty is an empty StatusReason
	ReasonEmpty StatusReason = ""

	// ReasonComplete the deployment of required resources is complete
	ReasonComplete StatusReason = "DeployComplete"

	// ReasonSyncing means that resources are deploying
	ReasonSyncing StatusReason = "SyncingResources"

	// ReasonInvalidConfiguration indicates that the configuration is invalid
	ReasonInvalidConfiguration StatusReason = "InvalidConfiguration"

	// ReasonDeployTimedOut indicates that the deployment timedOut
	ReasonDeployTimedOut StatusReason = "DeployTimedOut"

	// ReasonDeploymentCrashLooping indicates that the deployment is crashlooping
	ReasonDeploymentCrashLooping StatusReason = "DeploymentCrashLooping"

	// ReasonUnsupported is an unsupported StatusReason
	ReasonUnsupported StatusReason = "UnsupportedPlatform"
)

// defaultStatusConditions returns the default set of status conditions for the
// ClusterOperator resource used on first creation of the ClusterOperator.
func defaultStatusConditions() []osconfigv1.ClusterOperatorStatusCondition {
	return []osconfigv1.ClusterOperatorStatusCondition{
		setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionFalse, "", ""),
		setStatusCondition(osconfigv1.OperatorDegraded, osconfigv1.ConditionFalse, "", ""),
		setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionFalse, "", ""),
		setStatusCondition(osconfigv1.OperatorUpgradeable, osconfigv1.ConditionTrue, "", ""),
		setStatusCondition(OperatorDisabled, osconfigv1.ConditionFalse, "", ""),
	}
}

// relatedObjects returns the current list of ObjectReference's for the
// ClusterOperator objects's status.
func relatedObjects() []osconfigv1.ObjectReference {
	return []osconfigv1.ObjectReference{
		{
			Group:    "",
			Resource: "namespaces",
			Name:     ComponentNamespace,
		},
		{
			Group:     "metal3.io",
			Resource:  "baremetalhosts",
			Name:      "baremetalhosts.metal3.io",
			Namespace: ComponentNamespace,
		},
	}
}

// operandVersions returns the current list of OperandVersions for the
// ClusterOperator objects's status.
func (r *ProvisioningReconciler) operandVersions() []osconfigv1.OperandVersion {
	operandVersions := []osconfigv1.OperandVersion{}

	if r.ReleaseVersion != "" {
		operandVersions = append(operandVersions, osconfigv1.OperandVersion{
			Name:    "operator",
			Version: r.ReleaseVersion,
		})
	}

	return operandVersions
}

// createClusterOperator creates the ClusterOperator and updates its status.
func (r *ProvisioningReconciler) createClusterOperator() (*osconfigv1.ClusterOperator, error) {
	defaultCO := &osconfigv1.ClusterOperator{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterOperator",
			APIVersion: "config.openshift.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterOperatorName,
		},
		Status: osconfigv1.ClusterOperatorStatus{
			Conditions:     defaultStatusConditions(),
			RelatedObjects: relatedObjects(),
			Versions:       r.operandVersions(),
		},
	}

	co, err := r.OSClient.ConfigV1().ClusterOperators().Create(context.Background(), defaultCO, metav1.CreateOptions{})
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to create ClusterOperator %s",
			clusterOperatorName))
	}
	r.Log.V(1).Info("created ClusterOperator", "name", clusterOperatorName)

	co.Status = defaultCO.Status
	return r.OSClient.ConfigV1().ClusterOperators().UpdateStatus(context.Background(), co, metav1.UpdateOptions{})
}

// getOrCreateClusterOperator gets the existing CO, failing which it creates a new CO.
func (r *ProvisioningReconciler) getOrCreateClusterOperator() (*osconfigv1.ClusterOperator, error) {
	existing, err := r.OSClient.ConfigV1().ClusterOperators().Get(context.Background(), clusterOperatorName, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return r.createClusterOperator()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get clusterOperator %q: %v", clusterOperatorName, err)
	}

	return existing, nil
}

// setStatusCondition initalizes and returns a ClusterOperatorStatusCondition
func setStatusCondition(conditionType osconfigv1.ClusterStatusConditionType,
	conditionStatus osconfigv1.ConditionStatus, reason string,
	message string) osconfigv1.ClusterOperatorStatusCondition {
	return osconfigv1.ClusterOperatorStatusCondition{
		Type:               conditionType,
		Status:             conditionStatus,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

//syncStatus applies the new condition to the CBO ClusterOperator object.
func (r *ProvisioningReconciler) syncStatus(co *osconfigv1.ClusterOperator, conds []osconfigv1.ClusterOperatorStatusCondition) error {
	for _, c := range conds {
		v1helpers.SetStatusCondition(&co.Status.Conditions, c)
	}

	if len(co.Status.Versions) < 1 {
		r.Log.Info("updating ClusterOperator Status Versions field")
		co.Status.Versions = r.operandVersions()
	}

	_, err := r.OSClient.ConfigV1().ClusterOperators().UpdateStatus(context.Background(), co, metav1.UpdateOptions{})
	return err
}

func (r *ProvisioningReconciler) updateCOStatus(newReason StatusReason, msg, progressMsg string) error {

	co, err := r.getOrCreateClusterOperator()
	if err != nil {
		r.Log.Error(err, "failed to get or create ClusterOperator")
		return err
	}
	conds := defaultStatusConditions()
	switch newReason {
	case ReasonUnsupported:
		v1helpers.SetStatusCondition(&conds, setStatusCondition(OperatorDisabled, osconfigv1.ConditionTrue, string(newReason), msg))
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, string(ReasonEmpty), ""))
	case ReasonSyncing:
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, string(newReason), msg))
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionTrue, string(newReason), progressMsg))
	case ReasonComplete:
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, string(newReason), msg))
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionFalse, string(newReason), progressMsg))
	case ReasonInvalidConfiguration, ReasonDeployTimedOut:
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorDegraded, osconfigv1.ConditionTrue, string(newReason), msg))
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, string(ReasonEmpty), ""))
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionTrue, string(newReason), progressMsg))
	case ReasonDeploymentCrashLooping:
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorDegraded, osconfigv1.ConditionTrue, string(newReason), msg))
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionFalse, string(newReason), msg))
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionFalse, string(newReason), progressMsg))
	}

	return r.syncStatus(co, conds)
}
