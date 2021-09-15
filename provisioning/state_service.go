package provisioning

import (
	"context"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/cluster-baremetal-operator/pkg/resourceapply"
)

const (
	stateService = "metal3-state"
	httpPortName = "http"
)

func newMetal3StateService(targetNamespace string) *corev1.Service {
	port, _ := strconv.Atoi(baremetalHttpPort) // #nosec
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      stateService,
			Namespace: targetNamespace,
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

func EnsureMetal3StateService(ctx context.Context, info *ProvisioningInfo) (bool, error) {
	metal3StateService := newMetal3StateService(info.Namespace)

	err := controllerutil.SetControllerReference(info.ProvConfig, metal3StateService, info.Scheme)
	if err != nil {
		return false, fmt.Errorf("unable to set controllerReference on service: %w", err)
	}

	var updated bool
	_, updated, err = resourceapply.ApplyService(ctx, info.Client, info.EventRecorder, metal3StateService)
	return updated, err
}

func DeleteMetal3StateService(ctx context.Context, info *ProvisioningInfo) error {
	obj := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: stateService, Namespace: info.Namespace}}
	err := client.IgnoreNotFound(info.Client.Delete(ctx, obj, &client.DeleteOptions{}))
	resourceapply.ReportDeleteEvent(info.EventRecorder, obj, err)
	return err
}
