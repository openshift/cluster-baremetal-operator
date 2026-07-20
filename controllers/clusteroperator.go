package controllers

//go:generate go run -mod=vendor ../vendor/github.com/go-bindata/go-bindata/go-bindata/ -nometadata -pkg $GOPACKAGE -ignore=bindata.go  ../manifests/0000_31_cluster-baremetal-operator_07_clusteroperator.cr.yaml
//go:generate gofmt -s -l -w bindata.go

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"

	osconfigv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"
)

// StatusReason is a MixedCaps string representing the reason for a
// status condition change.
type StatusReason string

const (
	clusterOperatorName              = "baremetal"
	machineConfigClusterOperatorName = "machine-config"

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
// Also update the manifest directly so that must-gather will contain this information
// even if cbo fails early.(See BZ 1961844 for the reasoning.)
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
		{
			Group:     "metal3.io",
			Resource:  "hostfirmwarecomponents",
			Name:      "",
			Namespace: ComponentNamespace,
		},
		{
			Group:     "metal3.io",
			Resource:  "dataimages",
			Name:      "",
			Namespace: ComponentNamespace,
		},
		{
			Group:     "metal3.io",
			Resource:  "hostupdatepolicies",
			Name:      "",
			Namespace: ComponentNamespace,
		},
		{
			Group:    "rbac.authorization.k8s.io",
			Resource: "clusterroles",
			Name:     "cluster-baremetal-operator",
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

func getReportedOperatorVersion(co *osconfigv1.ClusterOperator) string {
	for _, version := range co.Status.Versions {
		if version.Name == "operator" {
			return version.Version
		}
	}
	return ""
}

func (r *ProvisioningReconciler) getClusterDesiredVersion(ctx context.Context) (string, error) {
	cv, err := r.OSClient.ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return cv.Status.Desired.Version, nil
}

// isOperatorVersionUpgradePending reports whether the baremetal ClusterOperator
// still needs to finish an upgrade. This compares the reported operator version
// against both the local RELEASE_VERSION and the cluster desired version so
// upgrades are surfaced even before the new CBO pod is rolled out.
func (r *ProvisioningReconciler) isOperatorVersionUpgradePending(ctx context.Context, co *osconfigv1.ClusterOperator) (bool, string, error) {
	reported := getReportedOperatorVersion(co)
	if reported == "" {
		return false, "", nil
	}

	if r.ReleaseVersion != "" && reported != r.ReleaseVersion {
		return true, r.ReleaseVersion, nil
	}

	desired, err := r.getClusterDesiredVersion(ctx)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return false, "", nil
		}
		return false, "", err
	}
	if desired != "" && reported != desired {
		return true, desired, nil
	}

	return false, "", nil
}

func upgradeProgressMessage(targetVersion string) string {
	return fmt.Sprintf("Upgrading to release version %s", targetVersion)
}

func (r *ProvisioningReconciler) isMachineConfigOperatorProgressing(ctx context.Context) (bool, error) {
	mco, err := r.OSClient.ConfigV1().ClusterOperators().Get(ctx, machineConfigClusterOperatorName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return v1helpers.IsStatusConditionTrue(mco.Status.Conditions, osconfigv1.OperatorProgressing), nil
}

func setOperatorVersionUpgradeProgressing(co *osconfigv1.ClusterOperator, reported, target string, clusterUpgrade bool) {
	if clusterUpgrade {
		klog.InfoS("Cluster version upgrade pending, setting Progressing=True",
			"reportedVersion", reported,
			"targetVersion", target)
	} else {
		klog.InfoS("Version upgrade pending, setting Progressing=True",
			"reportedVersion", reported,
			"targetVersion", target)
	}
	v1helpers.SetStatusCondition(&co.Status.Conditions,
		setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionTrue,
			string(ReasonSyncing),
			upgradeProgressMessage(target)),
		clock.RealClock{})
}

func (r *ProvisioningReconciler) reconcileOperatorVersionStatus(ctx context.Context, co *osconfigv1.ClusterOperator, justCreated bool) (bool, error) {
	reported := getReportedOperatorVersion(co)
	if r.ReleaseVersion != "" && reported != r.ReleaseVersion {
		if justCreated || reported == "" {
			co.Status.Versions = operandVersions(r.ReleaseVersion)
			return true, nil
		}
		updated, err := r.setVersionUpgradeProgressingIfAllowed(ctx, co, reported, r.ReleaseVersion, false)
		if err != nil {
			return false, err
		}
		return updated, nil
	}

	if justCreated {
		return false, nil
	}

	pending, target, err := r.isOperatorVersionUpgradePending(ctx, co)
	if err != nil {
		return false, err
	}
	if !pending {
		return false, nil
	}

	return r.setVersionUpgradeProgressingIfAllowed(ctx, co, reported, target, true)
}

func (r *ProvisioningReconciler) setVersionUpgradeProgressingIfAllowed(ctx context.Context, co *osconfigv1.ClusterOperator, reported, target string, clusterUpgrade bool) (bool, error) {
	mcoProgressing, err := r.isMachineConfigOperatorProgressing(ctx)
	if err != nil {
		return false, err
	}
	if mcoProgressing {
		return false, nil
	}
	setOperatorVersionUpgradeProgressing(co, reported, target, clusterUpgrade)
	return true, nil
}

// ensureClusterOperator makes sure that the CO exists
func (r *ProvisioningReconciler) ensureClusterOperator(ctx context.Context) error {
	co, err := r.OSClient.ConfigV1().ClusterOperators().Get(ctx, clusterOperatorName, metav1.GetOptions{})
	justCreated := false
	if k8serrors.IsNotFound(err) {
		co, err = r.createClusterOperator()
		justCreated = true
	}
	if err != nil {
		return err
	}

	needsUpdate := false
	if !equality.Semantic.DeepEqual(co.Status.RelatedObjects, relatedObjects()) {
		needsUpdate = true
		co.Status.RelatedObjects = relatedObjects()
	}
	if len(co.Status.Conditions) == 0 {
		needsUpdate = true
		co.Status.Conditions = defaultStatusConditions()
	}

	versionNeedsUpdate, err := r.reconcileOperatorVersionStatus(ctx, co, justCreated)
	if err != nil {
		return err
	}
	needsUpdate = needsUpdate || versionNeedsUpdate

	if needsUpdate {
		_, err = r.OSClient.ConfigV1().ClusterOperators().UpdateStatus(ctx, co, metav1.UpdateOptions{})
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

// getStatusConditionsDiff this is based on v1helpers.GetStatusDiff except it
// is focused on comparing the conditions better and nothing else.
func getStatusConditionsDiff(oldConditions []osconfigv1.ClusterOperatorStatusCondition, newConditions []osconfigv1.ClusterOperatorStatusCondition) string {
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

// syncStatus applies the new condition to the CBO ClusterOperator object.
func (r *ProvisioningReconciler) syncStatus(co *osconfigv1.ClusterOperator, conds []osconfigv1.ClusterOperatorStatusCondition, reason StatusReason) error {
	diff := getStatusConditionsDiff(co.Status.Conditions, conds)

	for _, c := range conds {
		v1helpers.SetStatusCondition(&co.Status.Conditions, c, clock.RealClock{})
	}

	if reason == ReasonComplete {
		klog.Info("upgrade complete, updating ClusterOperator Status Versions field")
		co.Status.Versions = operandVersions(r.ReleaseVersion)
	} else if len(co.Status.Versions) < 1 && r.ReleaseVersion != "" {
		klog.Info("initializing ClusterOperator Status Versions field")
		co.Status.Versions = operandVersions(r.ReleaseVersion)
	}

	if diff != "" {
		klog.InfoS("new CO status", "diff", diff)
	}

	_, err := r.OSClient.ConfigV1().ClusterOperators().UpdateStatus(context.Background(), co, metav1.UpdateOptions{})
	return err
}

func (r *ProvisioningReconciler) updateCOStatus(newReason StatusReason, msg, progressMsg string) error {
	co, err := r.OSClient.ConfigV1().ClusterOperators().Get(context.Background(), clusterOperatorName, metav1.GetOptions{})
	if err != nil {
		klog.ErrorS(err, "failed to get or create ClusterOperator")
		return fmt.Errorf("failed to get clusterOperator %q: %v", clusterOperatorName, err)
	}
	conds := defaultStatusConditions()
	clk := clock.RealClock{}
	switch newReason {
	case ReasonUnsupported:
		v1helpers.SetStatusCondition(&conds, setStatusCondition(OperatorDisabled, osconfigv1.ConditionTrue, string(newReason), msg), clk)
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, string(ReasonExpected), "Operational"), clk)
	case ReasonSyncing:
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, string(newReason), msg), clk)
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionTrue, string(newReason), progressMsg), clk)
	case ReasonComplete, ReasonProvisioningCRNotFound:
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, string(newReason), msg), clk)
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionFalse, string(newReason), progressMsg), clk)
	case ReasonResourceNotFound:
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorDegraded, osconfigv1.ConditionFalse, string(newReason), ""), clk)
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionFalse, string(ReasonEmpty), ""), clk)
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionTrue, string(newReason), progressMsg), clk)
	case ReasonInvalidConfiguration, ReasonDeployTimedOut:
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorDegraded, osconfigv1.ConditionTrue, string(newReason), msg), clk)
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, string(ReasonEmpty), ""), clk)
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionTrue, string(newReason), progressMsg), clk)
	case ReasonDeploymentCrashLooping:
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorDegraded, osconfigv1.ConditionTrue, string(newReason), msg), clk)
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionFalse, string(newReason), msg), clk)
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionFalse, string(newReason), progressMsg), clk)
	}

	return r.syncStatus(co, conds, newReason)
}
