package resourceapply

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/openshift/library-go/pkg/operator/events"
)

func TestApplyDeployment(t *testing.T) {
	dep1 := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      baremetalDeploymentName,
			Namespace: "testNS",
			Annotations: map[string]string{
				cboOwnedAnnotation: "",
			},
			Labels: map[string]string{
				"k8s-app":    metal3AppName,
				cboLabelName: stateService,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"k8s-app":    metal3AppName,
					cboLabelName: stateService,
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
		},
	}
	dep2 := dep1.DeepCopy()
	dep2.Spec.Replicas = pointer.Int32Ptr(2)

	dep3 := dep1.DeepCopy()
	dep3.Generation = 3

	ctx := context.TODO()
	tests := []struct { //nolint dupl
		name               string
		existing           *appsv1.Deployment
		expectedGeneration int64
		required           *appsv1.Deployment
		wantDeployment     *appsv1.Deployment
		wantUpdated        bool
		wantErr            bool
	}{
		{
			name:     "create",
			required: dep2.DeepCopy(),
			wantDeployment: func() *appsv1.Deployment {
				d := dep2.DeepCopy()
				d.ResourceVersion = "1"
				d.Annotations["operator.openshift.io/spec-hash"] = "c20ebbe295b75967078c1dfd91ee78ec779055a31684bc1c567a66c90760a398"
				return d
			}(),
			wantUpdated: true,
		},
		{
			name:     "patch",
			existing: dep1.DeepCopy(),
			required: dep2.DeepCopy(),
			wantDeployment: func() *appsv1.Deployment {
				d := dep2.DeepCopy()
				d.Kind = "Deployment"
				d.APIVersion = "apps/v1"
				d.ResourceVersion = "1000"
				d.Annotations["operator.openshift.io/spec-hash"] = "c20ebbe295b75967078c1dfd91ee78ec779055a31684bc1c567a66c90760a398"
				return d
			}(),
			wantUpdated: true,
		},
		{
			name: "no change",
			existing: func() *appsv1.Deployment {
				d := dep3.DeepCopy()
				d.Kind = "Deployment"
				d.APIVersion = "apps/v1"
				d.ResourceVersion = "1000"
				d.Annotations["operator.openshift.io/spec-hash"] = "b5b911841fc5536440bcf323b0bcb2591510724ffc751f3f03e3bbf9107868f7"
				return d
			}(),
			required: dep1.DeepCopy(),
			wantDeployment: func() *appsv1.Deployment {
				d := dep3.DeepCopy()
				d.Kind = "Deployment"
				d.APIVersion = "apps/v1"
				d.ResourceVersion = "1000"
				d.Annotations["operator.openshift.io/spec-hash"] = "b5b911841fc5536440bcf323b0bcb2591510724ffc751f3f03e3bbf9107868f7"
				return d
			}(),
			expectedGeneration: 3,
			wantUpdated:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := events.NewLoggingEventRecorder("tests")
			fcb := fakeclient.NewClientBuilder().WithScheme(scheme)
			if tt.existing != nil {
				fcb.WithObjects(tt.existing)
			}
			fc := fcb.Build()
			gotDeployment, gotUpdated, err := ApplyDeployment(ctx, fc, recorder, tt.required, tt.expectedGeneration)
			if (err != nil) != tt.wantErr {
				t.Errorf("applyDeployment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotUpdated != tt.wantUpdated {
				t.Errorf("applyDeployment() got = %v, want %v", gotUpdated, tt.wantUpdated)
			}
			if !reflect.DeepEqual(gotDeployment, tt.wantDeployment) {
				t.Error(cmp.Diff(tt.wantDeployment, gotDeployment))
			}
		})
	}
}

func TestApplyDaemonSet(t *testing.T) {
	dep1 := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      baremetalDeploymentName,
			Namespace: "testNS",
			Annotations: map[string]string{
				cboOwnedAnnotation: "",
			},
			Labels: map[string]string{
				"k8s-app":    metal3AppName,
				cboLabelName: stateService,
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"k8s-app":    metal3AppName,
					cboLabelName: stateService,
				},
			},
		},
	}
	dep2 := dep1.DeepCopy()
	dep2.Spec.MinReadySeconds = 54

	dep3 := dep1.DeepCopy()
	dep3.Generation = 3

	ctx := context.TODO()
	tests := []struct { //nolint dupl
		name               string
		existing           *appsv1.DaemonSet
		expectedGeneration int64
		required           *appsv1.DaemonSet
		wantDaemonSet      *appsv1.DaemonSet
		wantUpdated        bool
		wantErr            bool
	}{
		{
			name:     "create",
			required: dep2.DeepCopy(),
			wantDaemonSet: func() *appsv1.DaemonSet {
				d := dep2.DeepCopy()
				d.ResourceVersion = "1"
				d.Annotations["operator.openshift.io/spec-hash"] = "909f8edcae535fea7ed89dfa14f4791a0f93248191622a6ce9c6e69984da864f"
				return d
			}(),
			wantUpdated: true,
		},
		{
			name:     "patch",
			existing: dep1.DeepCopy(),
			required: dep2.DeepCopy(),
			wantDaemonSet: func() *appsv1.DaemonSet {
				d := dep2.DeepCopy()
				d.Kind = "DaemonSet"
				d.APIVersion = "apps/v1"
				d.ResourceVersion = "1000"
				d.Annotations["operator.openshift.io/spec-hash"] = "909f8edcae535fea7ed89dfa14f4791a0f93248191622a6ce9c6e69984da864f"
				return d
			}(),
			wantUpdated: true,
		},
		{
			name: "no change",
			existing: func() *appsv1.DaemonSet {
				d := dep3.DeepCopy()
				d.Kind = "DaemonSet"
				d.APIVersion = "apps/v1"
				d.ResourceVersion = "1000"
				d.Annotations["operator.openshift.io/spec-hash"] = "2037037c77b73ce0cd27a2710bdfe16f4bca52551aebade71be16ac6e4bea9d5"
				return d
			}(),
			required: dep1.DeepCopy(),
			wantDaemonSet: func() *appsv1.DaemonSet {
				d := dep3.DeepCopy()
				d.Kind = "DaemonSet"
				d.APIVersion = "apps/v1"
				d.ResourceVersion = "1000"
				d.Annotations["operator.openshift.io/spec-hash"] = "2037037c77b73ce0cd27a2710bdfe16f4bca52551aebade71be16ac6e4bea9d5"
				return d
			}(),
			expectedGeneration: 3,
			wantUpdated:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := events.NewLoggingEventRecorder("tests")
			fcb := fakeclient.NewClientBuilder().WithScheme(scheme)
			if tt.existing != nil {
				fcb.WithObjects(tt.existing)
			}
			fc := fcb.Build()
			gotDaemonSet, gotUpdated, err := ApplyDaemonSet(ctx, fc, recorder, tt.required, tt.expectedGeneration)
			if (err != nil) != tt.wantErr {
				t.Errorf("applyDaemonSet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotUpdated != tt.wantUpdated {
				t.Errorf("applyDaemonSet() got = %v, want %v", gotUpdated, tt.wantUpdated)
			}
			if !reflect.DeepEqual(gotDaemonSet, tt.wantDaemonSet) {
				t.Error(cmp.Diff(tt.wantDaemonSet, gotDaemonSet))
			}
		})
	}
}
