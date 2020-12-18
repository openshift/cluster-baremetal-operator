package provisioning

import (
	"context"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
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

func EnsureMetal3StateService(info *ProvisioningInfo) (updated bool, err error) {
	metal3StateService := newMetal3StateService(info.Namespace)

	err = controllerutil.SetControllerReference(info.ProvConfig, metal3StateService, info.Scheme)
	if err != nil {
		err = fmt.Errorf("unable to set controllerReference on service: %w", err)
		return
	}

	_, updated, err = resourceapply.ApplyService(info.Client.CoreV1(),
		info.EventRecorder, metal3StateService)
	if err != nil {
		err = fmt.Errorf("unable to apply Metal3-state service: %w", err)
	}
	return
}

func DeleteMetal3StateService(info *ProvisioningInfo) error {
	return client.IgnoreNotFound(info.Client.CoreV1().Services(info.Namespace).Delete(context.Background(), stateService, metav1.DeleteOptions{}))
}
