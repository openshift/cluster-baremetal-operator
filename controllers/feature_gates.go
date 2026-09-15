package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	osconfigv1 "github.com/openshift/api/config/v1"
	osclientset "github.com/openshift/client-go/config/clientset/versioned"
)

const featureGateCRName = "cluster"

// FeatureGates provides access to enabled/disabled OpenShift feature gates for the current release.
type FeatureGates struct {
	Enabled    map[osconfigv1.FeatureGateName]bool
	FeatureSet osconfigv1.FeatureSet
}

// ReadFeatureGates reads the cluster FeatureGate CR and returns the feature gates
// applicable to the given release version. If releaseVersion is empty or does not
// match any entry in the status, an empty FeatureGates is returned.
func ReadFeatureGates(ctx context.Context, osClient osclientset.Interface, releaseVersion string) (FeatureGates, error) {
	featureGateCR, err := osClient.ConfigV1().FeatureGates().Get(ctx, featureGateCRName, metav1.GetOptions{})
	if err != nil {
		return FeatureGates{}, err
	}
	return processFeatureGates(featureGateCR, releaseVersion)
}

// processFeatureGates reads the status of the feature gates for the specified
// release from the FeatureGate CR.
func processFeatureGates(featureGateCR *osconfigv1.FeatureGate, releaseVersion string) (FeatureGates, error) {
	fg := FeatureGates{
		Enabled:    make(map[osconfigv1.FeatureGateName]bool),
		FeatureSet: featureGateCR.Spec.FeatureSet,
	}

	if releaseVersion == "" {
		klog.Warning("Release version is not set; feature gates will be unavailable until the operator restarts")
		return fg, nil
	}

	for _, details := range featureGateCR.Status.FeatureGates {
		if details.Version != releaseVersion {
			continue
		}
		for _, gate := range details.Enabled {
			fg.Enabled[gate.Name] = true
		}
		for _, gate := range details.Disabled {
			fg.Enabled[gate.Name] = false
		}
		return fg, nil
	}

	return fg, fmt.Errorf("no feature gate status found for release version %q", releaseVersion)
}

// FeatureGateWatcher watches the cluster FeatureGate CR for changes and triggers
// a graceful shutdown when the active feature set changes so the operator restarts
// and picks up the new configuration.
type FeatureGateWatcher struct {
	client.Client

	// InitialFeatureSet is the feature set that was active when the operator started.
	InitialFeatureSet osconfigv1.FeatureSet

	// OnChange is called when the active feature set changes. The typical use is
	// to cancel the operator's root context, triggering a graceful shutdown.
	OnChange func(ctx context.Context)
}

// SetupWithManager registers the FeatureGateWatcher controller with the manager.
func (r *FeatureGateWatcher) SetupWithManager(mgr ctrl.Manager) error {
	if err := ctrl.NewControllerManagedBy(mgr).
		Named("featuregatewatcher").
		WithOptions(controller.Options{NeedLeaderElection: new(false)}).
		For(&osconfigv1.FeatureGate{}, builder.WithPredicates(
			predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					return e.Object.GetName() == featureGateCRName
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					return e.ObjectNew.GetName() == featureGateCRName
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					return e.Object.GetName() == featureGateCRName
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return e.Object.GetName() == featureGateCRName
				},
			},
		)).
		WithLogConstructor(func(_ *reconcile.Request) logr.Logger {
			return mgr.GetLogger().WithValues("controller", "featuregatewatcher")
		}).
		Complete(r); err != nil {
		return fmt.Errorf("could not set up feature gate watcher controller: %w", err)
	}
	return nil
}

// Reconcile detects changes to the FeatureGate spec and invokes OnChange when
// the active feature set differs from the one observed at operator startup.
func (r *FeatureGateWatcher) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx, "name", req.Name)

	logger.V(1).Info("Reconciling FeatureGate")
	defer logger.V(1).Info("Finished reconciling FeatureGate")

	fg := &osconfigv1.FeatureGate{}
	if err := r.Get(ctx, req.NamespacedName, fg); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get FeatureGate %s: %w", req.String(), err)
	}

	if fg.Spec.FeatureSet != r.InitialFeatureSet {
		if r.OnChange != nil {
			r.OnChange(ctx)
		}
		r.InitialFeatureSet = fg.Spec.FeatureSet
	}

	return ctrl.Result{}, nil
}
