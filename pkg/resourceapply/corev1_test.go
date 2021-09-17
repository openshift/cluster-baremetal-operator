package resourceapply

import (
	"context"
	"reflect"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
	"github.com/openshift/library-go/pkg/operator/events"
)

const (
	metal3AppName           = "metal3"
	baremetalDeploymentName = "metal3"
	cboOwnedAnnotation      = "baremetal.openshift.io/owned"
	cboLabelName            = "baremetal.openshift.io/cluster-baremetal-operator"
	stateService            = "metal3-state"
	httpPortName            = "http"
	baremetalSecretKey      = "password"
	baremetalHttpPort       = "6180"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(metal3iov1alpha1.AddToScheme(scheme))
}

func newFakeMetal3StateService() *corev1.Service {
	port, _ := strconv.Atoi(baremetalHttpPort) // #nosec
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      stateService,
			Namespace: "testNS",
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				cboLabelName: stateService,
			},
			Ports: []corev1.ServicePort{
				{
					Name: httpPortName,
					Port: int32(port),
				},
			},
		},
	}
}

func TestApplyService(t *testing.T) {
	ctx := context.TODO()
	expect := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            stateService,
			Namespace:       "testNS",
			ResourceVersion: "1",
			Labels:          map[string]string{},
			Annotations:     map[string]string{},
			OwnerReferences: []metav1.OwnerReference{},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				cboLabelName: stateService,
			},
			Ports: []corev1.ServicePort{
				{
					Name: httpPortName,
					Port: int32(6180),
				},
			},
		},
	}
	tests := []struct {
		name        string
		existing    *corev1.Service
		required    *corev1.Service
		wantService *corev1.Service
		wantUpdated bool
		wantErr     bool
	}{
		{
			name:        "create",
			required:    newFakeMetal3StateService(),
			wantService: expect,
			wantUpdated: true,
		},
		{
			name: "patch selector",
			existing: func() *corev1.Service {
				ser := newFakeMetal3StateService()
				ser.Spec.Selector["test"] = "yes"
				return ser
			}(),
			required: newFakeMetal3StateService(),
			wantService: func() *corev1.Service {
				exp := expect.DeepCopy()
				exp.APIVersion = "v1"
				exp.Kind = "Service"
				exp.ResourceVersion = "1000"
				return exp
			}(),
			wantUpdated: true,
		},
		{
			name: "no change",
			existing: func() *corev1.Service {
				exp := expect.DeepCopy()
				exp.APIVersion = "v1"
				exp.Kind = "Service"
				exp.ResourceVersion = "324"
				return exp
			}(),
			required: newFakeMetal3StateService(),
			wantService: func() *corev1.Service {
				exp := expect.DeepCopy()
				exp.APIVersion = "v1"
				exp.Kind = "Service"
				exp.ResourceVersion = "324"
				return exp
			}(),
			wantUpdated: false,
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

			gotService, gotUpdated, err := ApplyService(ctx, fc, recorder, tt.required)
			if (err != nil) != tt.wantErr {
				t.Errorf("applyService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotUpdated != tt.wantUpdated {
				t.Errorf("applyService() gotUpdated = %v, want %v", gotUpdated, tt.wantUpdated)
			}
			if !reflect.DeepEqual(gotService, tt.wantService) {
				t.Error(cmp.Diff(tt.wantService, gotService))
			}
		})
	}
}

func TestApplySecret(t *testing.T) {
	ctx := context.TODO()
	required := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            stateService,
			Namespace:       "testNS",
			ResourceVersion: "1",
		},
		Data: map[string][]byte{baremetalSecretKey: []byte("bar")},
	}
	expect := required.DeepCopy()
	expect.ResourceVersion = "1"
	expect.Type = corev1.SecretTypeOpaque
	expect.Labels = map[string]string{}
	expect.Annotations = map[string]string{}
	expect.OwnerReferences = []metav1.OwnerReference{}

	existingDifferent := expect.DeepCopy()
	existingDifferent.Data["test"] = []byte("yes")

	tests := []struct {
		name        string
		existing    *corev1.Secret
		required    *corev1.Secret
		should      ShouldUpdateDataFn
		wantSecret  *corev1.Secret
		wantUpdated bool
		wantErr     bool
	}{
		{
			name:        "create",
			required:    required,
			wantSecret:  expect,
			wantUpdated: true,
		},
		{
			name:     "patch data should update true",
			existing: existingDifferent,
			required: required,
			should:   func(existing *corev1.Secret) (bool, error) { return true, nil },
			wantSecret: func() *corev1.Secret {
				exp := expect.DeepCopy()
				exp.ResourceVersion = "2"
				exp.APIVersion = "v1"
				exp.Kind = "Secret"
				return exp
			}(),
			wantUpdated: true,
		},
		{
			name:     "patch data should update false",
			existing: existingDifferent,
			required: required,
			should:   func(existing *corev1.Secret) (bool, error) { return false, nil },
			wantSecret: func() *corev1.Secret {
				exp := existingDifferent.DeepCopy()
				exp.APIVersion = "v1"
				exp.Kind = "Secret"
				exp.Labels = nil
				exp.Annotations = nil
				exp.OwnerReferences = nil
				return exp
			}(),
			wantUpdated: false,
		},
		{
			name:     "no change",
			existing: required,
			required: required,
			wantSecret: func() *corev1.Secret {
				exp := expect.DeepCopy()
				exp.APIVersion = "v1"
				exp.Kind = "Secret"
				exp.Type = ""
				exp.Labels = nil
				exp.Annotations = nil
				exp.OwnerReferences = nil
				return exp
			}(),
			wantUpdated: false,
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

			gotSecret, gotUpdated, err := ApplySecret(ctx, fc, recorder, tt.required, tt.should)
			if (err != nil) != tt.wantErr {
				t.Errorf("applyService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotUpdated != tt.wantUpdated {
				t.Errorf("applyService() gotUpdated = %v, want %v", gotUpdated, tt.wantUpdated)
			}
			if !reflect.DeepEqual(gotSecret, tt.wantSecret) {
				t.Error(cmp.Diff(tt.wantSecret, gotSecret))
			}
		})
	}
}
