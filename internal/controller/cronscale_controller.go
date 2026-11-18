// Package controller reconciles CronScale objects onto workloads.
package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	platformv1 "github.com/bjo3g15nw7/cron-scale-operator/api/v1alpha1"
	"github.com/bjo3g15nw7/cron-scale-operator/internal/scale"
)

// CronScaleReconciler applies the open window of a CronScale to its target.
type CronScaleReconciler struct {
	client.Client
	// Now is injectable for tests; nil means time.Now.
	Now func() time.Time
}

// Event reasons published on the status conditions.
const (
	reasonApplied = "WindowApplied"
	reasonIdle    = "NoOpenWindow"
	reasonFailed  = "EvaluationFailed"
	reasonSuspend = "Suspended"
)

// Reconcile evaluates windows and patches the target replica count.
func (r *CronScaleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var cs platformv1.CronScale
	if err := r.Get(ctx, req.NamespacedName, &cs); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	now := time.Now
	if r.Now != nil {
		now = r.Now
	}

	if cs.Spec.Suspend {
		return ctrl.Result{}, r.record(ctx, &cs, 0, "", reasonSuspend,
			"Reconciliation frozen by spec.suspend")
	}

	loc, err := time.LoadLocation(cs.Spec.Timezone)
	if err != nil {
		return ctrl.Result{}, r.fail(ctx, &cs, fmt.Errorf("timezone %q: %w", cs.Spec.Timezone, err))
	}

	windows, err := scale.Compile(cs.Spec.Windows, loc)
	if err != nil {
		return ctrl.Result{}, r.fail(ctx, &cs, err)
	}

	winner, err := scale.Select(windows, now())
	if err != nil {
		return ctrl.Result{}, r.fail(ctx, &cs, err)
	}

	replicas := scale.Clamp(cs.Spec.MinReplicas, cs.Spec.MinReplicas, cs.Spec.MaxReplicas)
	window := ""
	reason := reasonIdle
	message := "No window is open; holding minReplicas"
	if winner != nil {
		replicas = scale.Clamp(winner.Replicas, cs.Spec.MinReplicas, cs.Spec.MaxReplicas)
		window = winner.Name
		reason = reasonApplied
		message = fmt.Sprintf("Window %q holds %d replicas", window, replicas)
	}

	if err := r.apply(ctx, &cs, replicas); err != nil {
		return ctrl.Result{}, r.fail(ctx, &cs, err)
	}
	logger.V(1).Info("applied replica decision", "window", window, "replicas", replicas)

	requeue := time.Minute
	if next := scale.Boundary(windows, now()); !next.IsZero() {
		requeue = next.Sub(now()).Round(time.Second)
	}
	return ctrl.Result{RequeueAfter: requeue}, r.record(ctx, &cs, replicas, window, reason, message)
}

// apply patches the target workload through the scale subresource.
func (r *CronScaleReconciler) apply(ctx context.Context, cs *platformv1.CronScale, replicas int32) error {
	target := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: cs.Namespace, Name: cs.Spec.Target.Name},
	}
	patch := client.RawPatch(types.MergePatchType, []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas)))
	if err := r.Patch(ctx, target, patch); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("target %s/%s not found", cs.Namespace, cs.Spec.Target.Name)
		}
		return fmt.Errorf("patch target: %w", err)
	}
	return nil
}

// record publishes the decision on the status.
func (r *CronScaleReconciler) record(ctx context.Context, cs *platformv1.CronScale,
	replicas int32, window, reason, message string) error {
	patch := client.MergeFrom(cs.DeepCopy())
	cs.Status.ActiveWindow = window
	cs.Status.DesiredReplicas = replicas
	cs.Status.ObservedGeneration = cs.Generation
	meta.SetStatusCondition(&cs.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: cs.Generation,
	})
	return r.Status().Patch(ctx, cs, patch)
}

// fail marks the CronScale degraded and returns the error for backoff.
func (r *CronScaleReconciler) fail(ctx context.Context, cs *platformv1.CronScale, cause error) error {
	patch := client.MergeFrom(cs.DeepCopy())
	meta.SetStatusCondition(&cs.Status.Conditions, metav1.Condition{
		Type:               "Degraded",
		Status:             metav1.ConditionTrue,
		Reason:             reasonFailed,
		Message:            cause.Error(),
		ObservedGeneration: cs.Generation,
	})
	cs.Status.ObservedGeneration = cs.Generation
	if err := r.Status().Patch(ctx, cs, patch); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

// SetupWithManager wires the reconciler into the manager.
func (r *CronScaleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1.CronScale{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
