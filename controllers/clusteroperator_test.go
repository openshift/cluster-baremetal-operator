package controllers

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	osconfigv1 "github.com/openshift/api/config/v1"
	fakeconfigclientset "github.com/openshift/client-go/config/clientset/versioned/fake"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
	"github.com/openshift/cluster-baremetal-operator/provisioning"
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

	reconciler := newFakeProvisioningReconciler(setUpSchemeForReconciler(), &osconfigv1.Infrastructure{})

	for _, tc := range tCases {
		co, _ := reconciler.createClusterOperator()
		reconciler.OSClient = fakeconfigclientset.NewSimpleClientset(co)

		err := reconciler.updateCOStatus(tc.reason, tc.msg, tc.progressMsg)
		if err != nil {
			t.Error(err)
		}
		gotCO, _ := reconciler.OSClient.ConfigV1().ClusterOperators().Get(context.Background(), clusterOperatorName, metav1.GetOptions{})

		diff := getStatusConditionsDiff(tc.expectedConditions, gotCO.Status.Conditions)
		if diff != "" {
			t.Fatal(diff)
		}
		_ = reconciler.OSClient.ConfigV1().ClusterOperators().Delete(context.Background(), clusterOperatorName, metav1.DeleteOptions{})
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

	var progressingUpgradeConditions = []osconfigv1.ClusterOperatorStatusCondition{
		setStatusCondition(
			osconfigv1.OperatorProgressing,
			osconfigv1.ConditionTrue,
			string(ReasonSyncing),
			"Upgrading to release version test-version",
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

	testCases := []struct {
		name            string
		existingCO      *osconfigv1.ClusterOperator
		machineConfigCO *osconfigv1.ClusterOperator
		expectedCO      *osconfigv1.ClusterOperator
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
						"capability.openshift.io/name":                                "baremetal",
						"include.release.openshift.io/ibm-cloud-managed":              "true",
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
		{
			name: "Version upgrade sets Progressing",
			existingCO: &osconfigv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterOperatorName,
				},
				Status: osconfigv1.ClusterOperatorStatus{
					Conditions:     defaultConditions,
					RelatedObjects: relatedObjects(),
					Versions:       []osconfigv1.OperandVersion{{Name: "operator", Version: "old-version"}},
				},
			},
			expectedCO: &osconfigv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterOperatorName,
				},
				Status: osconfigv1.ClusterOperatorStatus{
					Conditions:     progressingUpgradeConditions,
					RelatedObjects: relatedObjects(),
					Versions:       []osconfigv1.OperandVersion{{Name: "operator", Version: "old-version"}},
				},
			},
		},
		{
			name: "Version upgrade waits for MCO",
			existingCO: &osconfigv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterOperatorName,
				},
				Status: osconfigv1.ClusterOperatorStatus{
					Conditions:     defaultConditions,
					RelatedObjects: relatedObjects(),
					Versions:       []osconfigv1.OperandVersion{{Name: "operator", Version: "old-version"}},
				},
			},
			machineConfigCO: machineConfigClusterOperatorProgressing(),
			expectedCO: &osconfigv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterOperatorName,
				},
				Status: osconfigv1.ClusterOperatorStatus{
					Conditions:     defaultConditions,
					RelatedObjects: relatedObjects(),
					Versions:       []osconfigv1.OperandVersion{{Name: "operator", Version: "old-version"}},
				},
			},
		},
		{
			name: "First version set does not set Progressing",
			existingCO: &osconfigv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterOperatorName,
				},
				Status: osconfigv1.ClusterOperatorStatus{
					Conditions:     defaultConditions,
					RelatedObjects: relatedObjects(),
					Versions:       []osconfigv1.OperandVersion{},
				},
			},
			expectedCO: &osconfigv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterOperatorName,
				},
				Status: osconfigv1.ClusterOperatorStatus{
					Conditions:     defaultConditions,
					RelatedObjects: relatedObjects(),
					Versions:       []osconfigv1.OperandVersion{{Name: "operator", Version: "test-version"}},
				},
			},
		},
		{
			name: "Same version does not update",
			existingCO: &osconfigv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterOperatorName,
				},
				Status: osconfigv1.ClusterOperatorStatus{
					Conditions:     defaultConditions,
					RelatedObjects: relatedObjects(),
					Versions:       []osconfigv1.OperandVersion{{Name: "operator", Version: "test-version"}},
				},
			},
			expectedCO: &osconfigv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterOperatorName,
				},
				Status: osconfigv1.ClusterOperatorStatus{
					Conditions:     defaultConditions,
					RelatedObjects: relatedObjects(),
					Versions:       []osconfigv1.OperandVersion{{Name: "operator", Version: "test-version"}},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var osClient *fakeconfigclientset.Clientset
			switch {
			case tc.existingCO != nil && tc.machineConfigCO != nil:
				osClient = fakeconfigclientset.NewSimpleClientset(tc.existingCO, tc.machineConfigCO)
			case tc.existingCO != nil:
				osClient = fakeconfigclientset.NewSimpleClientset(tc.existingCO)
			case tc.machineConfigCO != nil:
				osClient = fakeconfigclientset.NewSimpleClientset(tc.machineConfigCO)
			default:
				osClient = fakeconfigclientset.NewSimpleClientset()
			}
			reconciler := newFakeProvisioningReconciler(setUpSchemeForReconciler(), &osconfigv1.Infrastructure{})
			reconciler.OSClient = osClient
			reconciler.ReleaseVersion = "test-version"

			err := reconciler.ensureClusterOperator(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			co, err := reconciler.OSClient.ConfigV1().ClusterOperators().Get(context.Background(), clusterOperatorName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			normalizeTransitionTimes(co.Status, tc.expectedCO.Status)

			if !equality.Semantic.DeepEqual(co, tc.expectedCO) {
				t.Error(cmp.Diff(tc.expectedCO, co))
			}
		})
	}
}

func machineConfigClusterOperatorProgressing() *osconfigv1.ClusterOperator {
	return &osconfigv1.ClusterOperator{
		ObjectMeta: metav1.ObjectMeta{Name: machineConfigClusterOperatorName},
		Status: osconfigv1.ClusterOperatorStatus{
			Conditions: []osconfigv1.ClusterOperatorStatusCondition{
				setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionTrue, string(ReasonSyncing), "Updating nodes"),
			},
		},
	}
}

func TestIsMachineConfigOperatorProgressing(t *testing.T) {
	testCases := []struct {
		name        string
		mco         *osconfigv1.ClusterOperator
		progressing bool
	}{
		{
			name:        "missing machine-config CO",
			progressing: false,
		},
		{
			name:        "machine-config progressing",
			mco:         machineConfigClusterOperatorProgressing(),
			progressing: true,
		},
		{
			name: "machine-config not progressing",
			mco: &osconfigv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{Name: machineConfigClusterOperatorName},
				Status: osconfigv1.ClusterOperatorStatus{
					Conditions: []osconfigv1.ClusterOperatorStatusCondition{
						setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionFalse, string(ReasonComplete), ""),
					},
				},
			},
			progressing: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			objects := []runtime.Object{}
			if tc.mco != nil {
				objects = append(objects, tc.mco)
			}
			reconciler := newFakeProvisioningReconciler(setUpSchemeForReconciler(), &osconfigv1.Infrastructure{})
			reconciler.OSClient = fakeconfigclientset.NewSimpleClientset(objects...)

			progressing, err := reconciler.isMachineConfigOperatorProgressing(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if progressing != tc.progressing {
				t.Fatalf("expected progressing=%v, got %v", tc.progressing, progressing)
			}
		})
	}
}

func TestIsOperatorVersionUpgradePending(t *testing.T) {
	clusterVersion := &osconfigv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{Name: "version"},
		Status: osconfigv1.ClusterVersionStatus{
			Desired: osconfigv1.Release{Version: "cluster-target-version"},
		},
	}

	testCases := []struct {
		name            string
		releaseVersion  string
		reportedVersion string
		clusterVersion  *osconfigv1.ClusterVersion
		pending         bool
		target          string
	}{
		{
			name:            "release version mismatch",
			releaseVersion:  "new-version",
			reportedVersion: "old-version",
			clusterVersion:  clusterVersion,
			pending:         true,
			target:          "new-version",
		},
		{
			name:            "cluster desired version mismatch",
			releaseVersion:  "old-version",
			reportedVersion: "old-version",
			clusterVersion:  clusterVersion,
			pending:         true,
			target:          "cluster-target-version",
		},
		{
			name:            "upgrade complete",
			releaseVersion:  "target-version",
			reportedVersion: "target-version",
			clusterVersion: &osconfigv1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{Name: "version"},
				Status: osconfigv1.ClusterVersionStatus{
					Desired: osconfigv1.Release{Version: "target-version"},
				},
			},
			pending: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			co := &osconfigv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{Name: clusterOperatorName},
				Status: osconfigv1.ClusterOperatorStatus{
					Versions: []osconfigv1.OperandVersion{{Name: "operator", Version: tc.reportedVersion}},
				},
			}
			reconciler := newFakeProvisioningReconciler(setUpSchemeForReconciler(), &osconfigv1.Infrastructure{})
			reconciler.OSClient = fakeconfigclientset.NewSimpleClientset(co, tc.clusterVersion)
			reconciler.ReleaseVersion = tc.releaseVersion

			pending, target, err := reconciler.isOperatorVersionUpgradePending(context.Background(), co)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if pending != tc.pending {
				t.Fatalf("expected pending=%v, got %v", tc.pending, pending)
			}
			if pending && target != tc.target {
				t.Fatalf("expected target=%q, got %q", tc.target, target)
			}
		})
	}
}

func TestUpdateCOProgressingStatusDuringUpgrade(t *testing.T) {
	clusterVersion := &osconfigv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{Name: "version"},
		Status: osconfigv1.ClusterVersionStatus{
			Desired: osconfigv1.Release{Version: "new-version"},
		},
	}

	testCases := []struct {
		name              string
		rolloutInProgress bool
		deploymentState   appsv1.DeploymentConditionType
		bmoState          appsv1.DeploymentConditionType
		machineConfigCO   *osconfigv1.ClusterOperator
		expectProgressing bool
		expectReportedVer string
	}{
		{
			name:              "rollout in progress",
			rolloutInProgress: true,
			deploymentState:   appsv1.DeploymentAvailable,
			bmoState:          appsv1.DeploymentAvailable,
			expectProgressing: true,
			expectReportedVer: "old-version",
		},
		{
			name:              "operands rolled out",
			rolloutInProgress: false,
			deploymentState:   appsv1.DeploymentAvailable,
			bmoState:          appsv1.DeploymentAvailable,
			expectProgressing: false,
			expectReportedVer: "new-version",
		},
		{
			name:              "operands not ready",
			rolloutInProgress: false,
			deploymentState:   appsv1.DeploymentProgressing,
			bmoState:          appsv1.DeploymentAvailable,
			expectProgressing: true,
			expectReportedVer: "old-version",
		},
		{
			name:              "while MCO progressing",
			rolloutInProgress: true,
			machineConfigCO:   machineConfigClusterOperatorProgressing(),
			deploymentState:   appsv1.DeploymentAvailable,
			bmoState:          appsv1.DeploymentAvailable,
			expectProgressing: false,
			expectReportedVer: "old-version",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			co := &osconfigv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{Name: clusterOperatorName},
				Status: osconfigv1.ClusterOperatorStatus{
					Conditions: defaultStatusConditions(),
					Versions:   []osconfigv1.OperandVersion{{Name: "operator", Version: "old-version"}},
				},
			}

			reconciler := newFakeProvisioningReconciler(setUpSchemeForReconciler(), &osconfigv1.Infrastructure{})
			objects := []runtime.Object{co, clusterVersion}
			if tc.machineConfigCO != nil {
				objects = append(objects, tc.machineConfigCO)
			}
			reconciler.OSClient = fakeconfigclientset.NewSimpleClientset(objects...)
			reconciler.ReleaseVersion = "new-version"

			err := reconciler.updateCOProgressingStatus(
				context.Background(),
				tc.rolloutInProgress,
				tc.deploymentState,
				tc.bmoState,
				provisioning.DaemonSetAvailable,
				provisioning.DaemonSetAvailable,
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			gotCO, err := reconciler.OSClient.ConfigV1().ClusterOperators().Get(context.Background(), clusterOperatorName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			progressing := v1helpers.FindStatusCondition(gotCO.Status.Conditions, osconfigv1.OperatorProgressing)
			if tc.expectProgressing {
				if progressing == nil || progressing.Status != osconfigv1.ConditionTrue {
					t.Fatalf("expected Progressing=True, got %#v", progressing)
				}
			} else if progressing == nil || progressing.Status != osconfigv1.ConditionFalse {
				t.Fatalf("expected Progressing=False, got %#v", progressing)
			}
			if got := getReportedOperatorVersion(gotCO); got != tc.expectReportedVer {
				t.Fatalf("expected reported version %q, got %q", tc.expectReportedVer, got)
			}
		})
	}
}

func TestUpdateCOProgressingStatusClusterVersionUpgrade(t *testing.T) {
	co := &osconfigv1.ClusterOperator{
		ObjectMeta: metav1.ObjectMeta{Name: clusterOperatorName},
		Status: osconfigv1.ClusterOperatorStatus{
			Conditions: defaultStatusConditions(),
			Versions:   []osconfigv1.OperandVersion{{Name: "operator", Version: "old-version"}},
		},
	}
	clusterVersion := &osconfigv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{Name: "version"},
		Status: osconfigv1.ClusterVersionStatus{
			Desired: osconfigv1.Release{Version: "new-version"},
		},
	}

	reconciler := newFakeProvisioningReconciler(setUpSchemeForReconciler(), &osconfigv1.Infrastructure{})
	reconciler.OSClient = fakeconfigclientset.NewSimpleClientset(co, clusterVersion)
	reconciler.ReleaseVersion = "old-version"

	err := reconciler.updateCOProgressingStatus(
		context.Background(),
		false,
		appsv1.DeploymentAvailable,
		appsv1.DeploymentAvailable,
		provisioning.DaemonSetAvailable,
		provisioning.DaemonSetAvailable,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotCO, err := reconciler.OSClient.ConfigV1().ClusterOperators().Get(context.Background(), clusterOperatorName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	progressing := v1helpers.FindStatusCondition(gotCO.Status.Conditions, osconfigv1.OperatorProgressing)
	if progressing == nil || progressing.Status != osconfigv1.ConditionTrue {
		t.Fatalf("expected Progressing=True while waiting for newer CBO pod, got %#v", progressing)
	}
	if got := getReportedOperatorVersion(gotCO); got != "old-version" {
		t.Fatalf("expected reported version to remain %q, got %q", "old-version", got)
	}
}

func TestUpdateCOStatusSetsVersionOnComplete(t *testing.T) {
	reconciler := newFakeProvisioningReconciler(setUpSchemeForReconciler(), &osconfigv1.Infrastructure{})
	co, _ := reconciler.createClusterOperator()
	co.Status.Versions = []osconfigv1.OperandVersion{{Name: "operator", Version: "old-version"}}
	reconciler.OSClient = fakeconfigclientset.NewSimpleClientset(co)
	reconciler.ReleaseVersion = "new-version"

	if err := reconciler.updateCOStatus(ReasonComplete, "metal3 pod is running", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotCO, err := reconciler.OSClient.ConfigV1().ClusterOperators().Get(context.Background(), clusterOperatorName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := getReportedOperatorVersion(gotCO); got != "new-version" {
		t.Fatalf("expected version %q after complete, got %q", "new-version", got)
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
				ProvisioningIP:            "172.30.20.11",
				ProvisioningNetworkCIDR:   "172.30.20.0/24",
				ProvisioningDHCPRange:     "172.30.20.11,172.30.20.101",
				ProvisioningOSDownloadURL: "",
				ProvisioningNetwork:       "Managed",
			},
			expectedConditions: []osconfigv1.ClusterOperatorStatusCondition{
				setStatusCondition(osconfigv1.OperatorDegraded, osconfigv1.ConditionTrue, "InvalidConfiguration", "invalid provisioningIP \"172.30.20.11\", value must be outside of the provisioningDHCPRange \"172.30.20.11,172.30.20.101\""),
				setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionTrue, "InvalidConfiguration", "Unable to apply Provisioning CR: invalid configuration"),
				setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, "", ""),
				setStatusCondition(osconfigv1.OperatorUpgradeable, osconfigv1.ConditionTrue, "", ""),
				setStatusCondition(OperatorDisabled, osconfigv1.ConditionFalse, "", ""),
			},
		},
	}

	reconciler := newFakeProvisioningReconciler(setUpSchemeForReconciler(), &osconfigv1.Infrastructure{})
	co, _ := reconciler.createClusterOperator()
	reconciler.OSClient = fakeconfigclientset.NewSimpleClientset(co)

	for _, tc := range tCases {
		baremetalCR.Spec = tc.spec
		if err := baremetalCR.ValidateBaremetalProvisioningConfig(metal3iov1alpha1.EnabledFeatures{
			ProvisioningNetwork: map[metal3iov1alpha1.ProvisioningNetwork]bool{
				metal3iov1alpha1.ProvisioningNetworkDisabled:  true,
				metal3iov1alpha1.ProvisioningNetworkUnmanaged: true,
				metal3iov1alpha1.ProvisioningNetworkManaged:   true,
			},
		}); err != nil {
			err = reconciler.updateCOStatus(ReasonInvalidConfiguration, err.Error(), "Unable to apply Provisioning CR: invalid configuration")
			if err != nil {
				t.Error(err)
			}
		}
		gotCO, _ := reconciler.OSClient.ConfigV1().ClusterOperators().Get(context.Background(), clusterOperatorName, metav1.GetOptions{})

		diff := getStatusConditionsDiff(tc.expectedConditions, gotCO.Status.Conditions)
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
	reconciler := newFakeProvisioningReconciler(setUpSchemeForReconciler(), &osconfigv1.Infrastructure{})
	co, _ := reconciler.createClusterOperator()
	reconciler.OSClient = fakeconfigclientset.NewSimpleClientset(co)

	for _, tc := range tCases {
		err := reconciler.updateCOStatus(ReasonComplete, tc.msg, "")
		if err != nil {
			t.Error(err)
		}
		gotCO, _ := reconciler.OSClient.ConfigV1().ClusterOperators().Get(context.Background(), clusterOperatorName, metav1.GetOptions{})

		diff := getStatusConditionsDiff(tc.expectedConditions, gotCO.Status.Conditions)
		if diff != "" {
			t.Fatal(diff)
		}
	}
}
