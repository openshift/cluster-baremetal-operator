package resourceapply

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/library-go/pkg/operator/events"
	goresourceapply "github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
)

func ApplyDeployment(ctx context.Context, c client.Client, recorder events.Recorder, required *appsv1.Deployment, expectedGeneration int64) (*appsv1.Deployment, bool, error) { //nolint:dupl
	err := goresourceapply.SetSpecHashAnnotation(&required.ObjectMeta, required.Spec)
	if err != nil {
		return nil, false, fmt.Errorf("unable to set SetSpecHashAnnotation on deployment: %w", err)
	}
	existing := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      required.Name,
			Namespace: required.Namespace,
		},
	}

	result, err := controllerutil.CreateOrPatch(ctx, c, existing, func() error {
		modified := false
		resourcemerge.EnsureObjectMeta(&modified, &existing.ObjectMeta, required.ObjectMeta)

		// there was no change to metadata, the generation was right
		if !modified && existing.ObjectMeta.Generation == expectedGeneration {
			return nil
		}

		existing.Spec = *required.Spec.DeepCopy()
		return nil
	})

	updated := result != controllerutil.OperationResultNone
	if updated || err != nil {
		reportCreateOrPatchEvent(recorder, existing, result, err)
	}
	return existing, updated, err
}

func ApplyDaemonSet(ctx context.Context, c client.Client, recorder events.Recorder, required *appsv1.DaemonSet, expectedGeneration int64) (*appsv1.DaemonSet, bool, error) { //nolint:dupl
	err := goresourceapply.SetSpecHashAnnotation(&required.ObjectMeta, required.Spec)
	if err != nil {
		return nil, false, fmt.Errorf("unable to set SetSpecHashAnnotation on daemonSet: %w", err)
	}
	existing := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      required.Name,
			Namespace: required.Namespace,
		},
	}
	result, err := controllerutil.CreateOrPatch(ctx, c, existing, func() error {
		modified := false
		resourcemerge.EnsureObjectMeta(&modified, &existing.ObjectMeta, required.ObjectMeta)
		// there was no change to metadata, the generation was right, and we weren't asked for force the deployment
		if !modified && existing.ObjectMeta.Generation == expectedGeneration {
			return nil
		}

		existing.Spec = *required.Spec.DeepCopy()
		return nil
	})

	updated := result != controllerutil.OperationResultNone
	if updated || err != nil {
		reportCreateOrPatchEvent(recorder, existing, result, err)
	}

	return existing, updated, err
}
