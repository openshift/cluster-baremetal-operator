package resourceapply

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
)

type ShouldUpdateDataFn func(existing *corev1.Secret) (bool, error)

// ApplySecret merges objectmeta, applies data if the secret does not exist or shouldUpdateDataFn returns false.
func ApplySecret(ctx context.Context, c client.Client, recorder events.Recorder, required *corev1.Secret, shouldUpdateData ShouldUpdateDataFn) (*corev1.Secret, bool, error) {
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      required.Name,
			Namespace: required.Namespace,
		},
	}

	needsDataCopy := func(existing *corev1.Secret) (bool, error) {
		if len(existing.Data) == 0 { // create case
			return true, nil
		}
		// update case
		if shouldUpdateData == nil {
			return false, nil
		}
		return shouldUpdateData(existing)
	}

	result, err := controllerutil.CreateOrPatch(ctx, c, existing, func() error {
		if needsApply, err := needsDataCopy(existing); !needsApply || err != nil {
			return err
		}
		resourcemerge.EnsureObjectMeta(resourcemerge.BoolPtr(false), &existing.ObjectMeta, required.ObjectMeta)
		if required.Data == nil {
			required.Data = map[string][]byte{}
		}
		existing.Data = required.Data
		for k, v := range required.StringData {
			existing.Data[k] = []byte(v)
		}
		existing.StringData = nil
		existing.Type = required.Type
		if existing.Type == "" {
			existing.Type = corev1.SecretTypeOpaque
		}
		return nil
	})
	updated := result != controllerutil.OperationResultNone
	if err != nil || updated {
		reportCreateOrPatchEvent(recorder, existing, result, err)
	}

	return existing, updated, err
}

func ApplyService(ctx context.Context, c client.Client, recorder events.Recorder, required *corev1.Service) (*corev1.Service, bool, error) {
	existing := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      required.Name,
			Namespace: required.Namespace,
		},
	}

	result, err := controllerutil.CreateOrPatch(ctx, c, existing, func() error {
		modified := resourcemerge.BoolPtr(false)
		resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
		if len(existing.Spec.Ports) == 0 && len(existing.Spec.Selector) == 0 {
			// this is a create, we need copy everything.
			existing.Spec = *required.Spec.DeepCopy()
			return nil
		}
		existing.Spec.Selector = required.Spec.Selector
		existing.Spec.Type = required.Spec.Type // if this is different, the update will fail.  Status will indicate it.
		return nil
	})
	if err != nil {
		err = fmt.Errorf("unable to apply %s service: %w", existing.Name, err)
	}
	updated := result != controllerutil.OperationResultNone
	if err != nil || updated {
		reportCreateOrPatchEvent(recorder, existing, result, err)
	}
	return existing, updated, err
}
