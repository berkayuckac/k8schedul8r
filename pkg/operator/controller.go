package operator

import (
	"context"
	"fmt"
	"log"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"github.com/berkayuckac/k8schedul8r/pkg/config"
	"github.com/berkayuckac/k8schedul8r/pkg/model"
	"github.com/berkayuckac/k8schedul8r/pkg/scheduler"
)

type ScheduledResourceReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
	scheduler *scheduler.Scheduler
	provider  *config.CRDProvider
}

func (r *ScheduledResourceReconciler) SetupWithManager(mgr ctrl.Manager, sched *scheduler.Scheduler, provider *config.CRDProvider) error {
	r.Client = mgr.GetClient()
	r.Scheme = mgr.GetScheme()
	r.Recorder = mgr.GetEventRecorderFor("k8schedul8r-controller")
	r.scheduler = sched
	r.provider = provider

	return ctrl.NewControllerManagedBy(mgr).
		For(&model.ScheduledResource{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 5, // Allow multiple reconciles in parallel
		}).
		Complete(r)
}

func (r *ScheduledResourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.Printf("Reconciling ScheduledResource %s/%s", req.Namespace, req.Name)

	// Get the ScheduledResource
	var scheduledResource model.ScheduledResource
	if err := r.Get(ctx, req.NamespacedName, &scheduledResource); err != nil {
		if errors.IsNotFound(err) {
			// Resource was deleted, remove from provider cache
			r.provider.DeleteResource(req.Namespace, req.Name)
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// Convert to model.Resource
	resource := model.Resource{
		Name:      scheduledResource.Name,
		Namespace: scheduledResource.Namespace,
		Target: model.Target{
			Name:       scheduledResource.Spec.Target.Name,
			Kind:       scheduledResource.Spec.Target.Kind,
			APIVersion: scheduledResource.Spec.Target.APIVersion,
		},
		OriginalReplicas: scheduledResource.Spec.OriginalReplicas,
		Windows:          convertWindows(scheduledResource.Spec.Windows),
	}

	// Validate the resource
	if err := resource.Validate(); err != nil {
		r.Recorder.Event(&scheduledResource, "Warning", "ValidationFailed", err.Error())
		return ctrl.Result{}, err
	}

	// Update the provider's cache
	r.provider.UpdateResource(resource)

	// Trigger immediate scaling check
	now := time.Now().Unix()
	desiredReplicas := resource.GetDesiredReplicas(now)

	if err := r.scheduler.ScaleResource(ctx, &resource, desiredReplicas); err != nil {
		r.Recorder.Event(&scheduledResource, "Warning", "ScalingFailed",
			fmt.Sprintf("Failed to scale resource: %v", err))
		return ctrl.Result{}, err
	}

	r.Recorder.Event(&scheduledResource, "Normal", "Scaled",
		fmt.Sprintf("Successfully scaled %s %s/%s to %d replicas",
			resource.Target.Kind, resource.Namespace, resource.Target.Name, desiredReplicas))

	// Requeue after a minute to ensure we keep checking the schedule
	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

func convertWindows(windows []model.Window) []model.ScalingWindow {
	result := make([]model.ScalingWindow, len(windows))
	for i, w := range windows {
		result[i] = model.ScalingWindow{
			StartTime: w.StartTime,
			EndTime:   w.EndTime,
			Replicas:  w.Replicas,
		}
	}
	return result
}
