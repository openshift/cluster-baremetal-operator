package controllers

//go:generate go run -mod=vendor ../vendor/github.com/go-bindata/go-bindata/go-bindata/ -nometadata -pkg $GOPACKAGE -ignore=bindata.go  ../manifests/0000_31_cluster-baremetal-operator_07_clusteroperator.cr.yaml
//go:generate gofmt -s -l -w bindata.go

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog/v2"

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

	// ReasonExpected indicates that the operator is behaving as expected
	ReasonExpected StatusReason = "AsExpected"

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

	// ReasonResourceNotFound indicates that the deployment is not found
	ReasonResourceNotFound StatusReason = "ResourceNotFound"

	// ReasonProvisioningCRNotFound indicates that the provsioning CR is not found
	ReasonProvisioningCRNotFound StatusReason = "WaitingForProvisioningCR"

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
			Name:      "",
			Namespace: ComponentNamespace,
		},
		{
			Group:    "metal3.io",
			Resource: "provisioning",
			Name:     "",
		},
		{
			Group:     "metal3.io",
			Resource:  "hostfirmwaresettings",
			Name:      "",
			Namespace: ComponentNamespace,
		},
		{
			Group:     "metal3.io",
			Resource:  "firmwareschemas",
			Name:      "",
			Namespace: ComponentNamespace,
		},
		{
			Group:     "metal3.io",
			Resource:  "preprovisioningimages",
			Name:      "",
			Namespace: ComponentNamespace,
		},
		{
			Group:     "metal3.io",
			Resource:  "bmceventsubscriptions",
			Name:      "",
			Namespace: ComponentNamespace,
		},
	}
}

// operandVersions returns the current list of OperandVersions for the
// ClusterOperator objects's status.
func operandVersions(version string) []osconfigv1.OperandVersion {
	operandVersions := []osconfigv1.OperandVersion{}

	if version != "" {
		operandVersions = append(operandVersions, osconfigv1.OperandVersion{
			Name:    "operator",
			Version: version,
		})
	}

	return operandVersions
}

// createClusterOperator creates the ClusterOperator and updates its status.
func (r *ProvisioningReconciler) createClusterOperator() (*osconfigv1.ClusterOperator, error) {
	b, err := Asset("../manifests/0000_31_cluster-baremetal-operator_07_clusteroperator.cr.yaml")
	if err != nil {
		return nil, err
	}

	codecs := serializer.NewCodecFactory(r.Scheme)
	obj, _, err := codecs.UniversalDeserializer().Decode(b, nil, nil)
	if err != nil {
		return nil, err
	}

	defaultCO, ok := obj.(*osconfigv1.ClusterOperator)
	if !ok {
		return nil, fmt.Errorf("could not convert deserialized asset into ClusterOperoator")
	}

	return r.OSClient.ConfigV1().ClusterOperators().Create(context.Background(), defaultCO, metav1.CreateOptions{})
}

// ensureClusterOperator makes sure that the CO exists
func (r *ProvisioningReconciler) ensureClusterOperator() error {
	co, err := r.OSClient.ConfigV1().ClusterOperators().Get(context.Background(), clusterOperatorName, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		co, err = r.createClusterOperator()
	}
	if err != nil {
		return err
	}

	needsUpdate := false
	if !equality.Semantic.DeepEqual(co.Status.RelatedObjects, relatedObjects()) {
		needsUpdate = true
		co.Status.RelatedObjects = relatedObjects()
	}
	if !equality.Semantic.DeepEqual(co.Status.Versions, operandVersions(r.ReleaseVersion)) {
		needsUpdate = true
		co.Status.Versions = operandVersions(r.ReleaseVersion)
	}
	if len(co.Status.Conditions) == 0 {
		needsUpdate = true
		co.Status.Conditions = defaultStatusConditions()
	}

	if needsUpdate {
		_, err = r.OSClient.ConfigV1().ClusterOperators().UpdateStatus(context.Background(), co, metav1.UpdateOptions{})
	}
	return err
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
		klog.Info("updating ClusterOperator Status Versions field")
		co.Status.Versions = operandVersions(r.ReleaseVersion)
	}

	_, err := r.OSClient.ConfigV1().ClusterOperators().UpdateStatus(context.Background(), co, metav1.UpdateOptions{})
	return err
}

func (r *ProvisioningReconciler) updateCOStatus(newReason StatusReason, msg, progressMsg string) error {
	klog.InfoS("new CO status", "reason", newReason, "processMessage", progressMsg, "message", msg)
	co, err := r.OSClient.ConfigV1().ClusterOperators().Get(context.Background(), clusterOperatorName, metav1.GetOptions{})
	if err != nil {
		klog.ErrorS(err, "failed to get or create ClusterOperator")
		return fmt.Errorf("failed to get clusterOperator %q: %v", clusterOperatorName, err)
	}
	conds := defaultStatusConditions()
	switch newReason {
	case ReasonUnsupported:
		v1helpers.SetStatusCondition(&conds, setStatusCondition(OperatorDisabled, osconfigv1.ConditionTrue, string(newReason), msg))
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, string(ReasonExpected), "Operational"))
	case ReasonSyncing:
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, string(newReason), msg))
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionTrue, string(newReason), progressMsg))
	case ReasonComplete, ReasonProvisioningCRNotFound:
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, string(newReason), msg))
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionFalse, string(newReason), progressMsg))
	case ReasonInvalidConfiguration, ReasonDeployTimedOut, ReasonResourceNotFound:
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
