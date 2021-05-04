package controllers

//go:generate go run -mod=vendor ../vendor/github.com/go-bindata/go-bindata/go-bindata/ -nometadata -pkg $GOPACKAGE -ignore=bindata.go  ../manifests/0000_31_cluster-baremetal-operator_07_clusteroperator.cr.yaml
//go:generate gofmt -s -l -w bindata.go

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	osconfigv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	osclientset "github.com/openshift/client-go/config/clientset/versioned"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/apis/metal3.io/v1alpha1"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

// StatusReason is a MixedCaps string representing the reason for a
// status condition change.
type StatusReason string

const (
	ClusterOperatorName = "baremetal"

	// OperatorDisabled represents a Disabled ConditionType
	OperatorDisabled string = "Disabled"

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

	// ReasonNotFound indicates that the deployment is not found
	ReasonNotFound StatusReason = "ResourceNotFound"

	// ReasonUnsupported is an unsupported StatusReason
	ReasonUnsupported StatusReason = "UnsupportedPlatform"
)

func RelatedObjects(namespace string) []osconfigv1.ObjectReference {
	return []osconfigv1.ObjectReference{
		{
			Group:    "",
			Resource: "namespaces",
			Name:     namespace,
		},
		{
			Group:     "metal3.io",
			Resource:  "baremetalhosts",
			Name:      "",
			Namespace: namespace,
		},
		{
			Group:    "metal3.io",
			Resource: "provisioning",
			Name:     metal3iov1alpha1.ProvisioningSingletonName,
		},
	}
}

// newStatusCondition initalizes and returns a OperatorCondition
func newStatusCondition(conditionType string, conditionStatus operatorv1.ConditionStatus, reason string, message string) operatorv1.OperatorCondition {
	return operatorv1.OperatorCondition{
		Type:               conditionType,
		Status:             conditionStatus,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// newCOStatusCondition initalizes and returns a ClusterOperatorStatusCondition
func newCOStatusCondition(conditionType osconfigv1.ClusterStatusConditionType, conditionStatus osconfigv1.ConditionStatus, reason string, message string) osconfigv1.ClusterOperatorStatusCondition {
	return osconfigv1.ClusterOperatorStatusCondition{
		Type:               conditionType,
		Status:             conditionStatus,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// createClusterOperator creates the ClusterOperator
func createClusterOperator(ctx context.Context, osClient osclientset.Interface) (*osconfigv1.ClusterOperator, error) {
	b, err := Asset("../manifests/0000_31_cluster-baremetal-operator_07_clusteroperator.cr.yaml")
	if err != nil {
		return nil, err
	}

	codecs := serializer.NewCodecFactory(clientgoscheme.Scheme)
	obj, _, err := codecs.UniversalDeserializer().Decode(b, nil, nil)
	if err != nil {
		return nil, err
	}

	defaultCO, ok := obj.(*osconfigv1.ClusterOperator)
	if !ok {
		return nil, fmt.Errorf("could not convert deserialized asset into ClusterOperoator")
	}

	return osClient.ConfigV1().ClusterOperators().Create(ctx, defaultCO, metav1.CreateOptions{})
}

// SetClusterOperatorDisabled makes sure that the CO exists
func SetClusterOperatorDisabled(ctx context.Context, osClient osclientset.Interface, relatedObjects []osconfigv1.ObjectReference, versions []osconfigv1.OperandVersion) error {
	co, err := osClient.ConfigV1().ClusterOperators().Get(ctx, ClusterOperatorName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		co, err = createClusterOperator(ctx, osClient)
	}
	if err != nil {
		return err
	}
	co.Status.RelatedObjects = relatedObjects
	co.Status.Versions = versions
	co.Status.Conditions = []osconfigv1.ClusterOperatorStatusCondition{
		newCOStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionFalse, "", ""),
		newCOStatusCondition(osconfigv1.OperatorDegraded, osconfigv1.ConditionFalse, "", ""),
		newCOStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, string(ReasonExpected), "Operational"),
		newCOStatusCondition(osconfigv1.OperatorUpgradeable, osconfigv1.ConditionTrue, "", ""),
		newCOStatusCondition(osconfigv1.ClusterStatusConditionType(OperatorDisabled), osconfigv1.ConditionTrue, string(ReasonUnsupported), "Operator is non-functional"),
	}

	_, err = osClient.ConfigV1().ClusterOperators().UpdateStatus(ctx, co, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("Failed to updateStatus on baremetal clusteroperator %w", err)
	}
	return nil
}

// resetOperatorConditions resets status conditions to the defaults.
func (r *ProvisioningController) resetOperatorConditions() error {
	_, _, err := v1helpers.UpdateStatus(r.operatorClient,
		v1helpers.UpdateConditionFn(newStatusCondition(operatorv1.OperatorStatusTypeProgressing, operatorv1.ConditionFalse, "", "")),
		v1helpers.UpdateConditionFn(newStatusCondition(operatorv1.OperatorStatusTypeDegraded, operatorv1.ConditionFalse, "", "")),
		v1helpers.UpdateConditionFn(newStatusCondition(operatorv1.OperatorStatusTypeAvailable, operatorv1.ConditionFalse, "", "")),
		v1helpers.UpdateConditionFn(newStatusCondition(operatorv1.OperatorStatusTypeUpgradeable, operatorv1.ConditionTrue, "", "")),
		v1helpers.UpdateConditionFn(newStatusCondition(OperatorDisabled, operatorv1.ConditionFalse, "", "")),
	)
	return err
}

func (r *ProvisioningController) updateCOStatus(newReason StatusReason, msg, progressMsg string) error {
	var err error
	switch newReason {
	case ReasonSyncing:
		_, _, err = v1helpers.UpdateStatus(r.operatorClient,
			v1helpers.UpdateConditionFn(newStatusCondition(operatorv1.OperatorStatusTypeAvailable, operatorv1.ConditionTrue, string(newReason), msg)),
			v1helpers.UpdateConditionFn(newStatusCondition(operatorv1.OperatorStatusTypeProgressing, operatorv1.ConditionTrue, string(newReason), progressMsg)),
		)
	case ReasonComplete:
		_, _, err = v1helpers.UpdateStatus(r.operatorClient,
			v1helpers.UpdateConditionFn(newStatusCondition(operatorv1.OperatorStatusTypeAvailable, operatorv1.ConditionTrue, string(newReason), msg)),
			v1helpers.UpdateConditionFn(newStatusCondition(operatorv1.OperatorStatusTypeProgressing, operatorv1.ConditionFalse, string(newReason), progressMsg)),
		)
	case ReasonInvalidConfiguration, ReasonDeployTimedOut, ReasonNotFound:
		_, _, err = v1helpers.UpdateStatus(r.operatorClient,
			v1helpers.UpdateConditionFn(newStatusCondition(operatorv1.OperatorStatusTypeDegraded, operatorv1.ConditionTrue, string(newReason), msg)),
			v1helpers.UpdateConditionFn(newStatusCondition(operatorv1.OperatorStatusTypeAvailable, operatorv1.ConditionTrue, string(ReasonEmpty), "")),
			v1helpers.UpdateConditionFn(newStatusCondition(operatorv1.OperatorStatusTypeProgressing, operatorv1.ConditionTrue, string(newReason), progressMsg)),
		)
	case ReasonDeploymentCrashLooping:
		_, _, err = v1helpers.UpdateStatus(r.operatorClient,
			v1helpers.UpdateConditionFn(newStatusCondition(operatorv1.OperatorStatusTypeDegraded, operatorv1.ConditionTrue, string(newReason), msg)),
			v1helpers.UpdateConditionFn(newStatusCondition(operatorv1.OperatorStatusTypeAvailable, operatorv1.ConditionFalse, string(newReason), msg)),
			v1helpers.UpdateConditionFn(newStatusCondition(operatorv1.OperatorStatusTypeProgressing, operatorv1.ConditionFalse, string(newReason), progressMsg)),
		)
	}

	return err
}
