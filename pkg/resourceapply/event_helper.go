package resourceapply

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcehelper"
)

func reportCreateOrPatchEvent(recorder events.Recorder, obj runtime.Object, result controllerutil.OperationResult, originalErr error) {
	gvk := resourcehelper.GuessObjectGroupVersionKind(obj)
	if originalErr == nil {
		recorder.Eventf(fmt.Sprintf("%s%s", gvk.Kind, result), "%s %s because it was required", resourcehelper.FormatResourceForCLIWithNamespace(obj), result)
		return
	}
	recorder.Warningf(fmt.Sprintf("%sCreateOrPatchFailed", gvk.Kind), "Failed to CreateOrPatch %s: %v", resourcehelper.FormatResourceForCLIWithNamespace(obj), originalErr)
}

func ReportDeleteEvent(recorder events.Recorder, obj runtime.Object, originalErr error) {
	gvk := resourcehelper.GuessObjectGroupVersionKind(obj)
	if originalErr == nil {
		recorder.Eventf(fmt.Sprintf("%sDeleted", gvk.Kind), "Deleted %s", resourcehelper.FormatResourceForCLIWithNamespace(obj))
		return
	}
	recorder.Warningf(fmt.Sprintf("%sDeleteFailed", gvk.Kind), "Failed to delete %s: %v", resourcehelper.FormatResourceForCLIWithNamespace(obj), originalErr)
}
