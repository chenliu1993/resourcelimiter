/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"

	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	rlv1beta1 "github.com/chenliu1993/resourcelimiter/api/v1beta1"
	"github.com/chenliu1993/resourcelimiter/pkg/constants"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var (
	// TODO: export this field to users
	concurrentWorkers int = 1
)

// ResourceLimiterReconciler reconciles a ResourceLimiter object
type ResourceLimiterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// Event filter
func eventPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() || e.ObjectOld.GetResourceVersion() != e.ObjectNew.GetResourceVersion()
		},
	}
}

//+kubebuilder:rbac:groups=resources.resourcelimiter.io,resources=resourcelimiters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=resources.resourcelimiter.io,resources=resourcelimiters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=resourcequotas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;create;
//+kubebuilder:rbac:groups=resources.resourcelimiter.io,resources=resourcelimiters/finalizers,verbs=update;delete

func (r *ResourceLimiterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	rl := rlv1beta1.ResourceLimiter{}
	if err := r.Get(ctx, req.NamespacedName, &rl); err != nil {
		// return ctrl.Result{Requeue: true}, client.IgnoreNotFound(err)
		return ctrl.Result{}, err
	}

	newrl := rl.DeepCopy()

	// Add our finalizer if it does not exist
	if !controllerutil.ContainsFinalizer(newrl, constants.DefaultFinalizer) {
		patch := client.MergeFrom(newrl)
		controllerutil.AddFinalizer(newrl, constants.DefaultFinalizer)
		if err := r.Patch(ctx, newrl, patch); err != nil {
			log.WithName("ResourceLimiter").Error(err, "unable to register finalizer")
			return ctrl.Result{}, err
		}
		if err := r.Get(ctx, req.NamespacedName, &rl); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Under deletion
	if !rl.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, newrl)
	}

	return r.reconcile(ctx, newrl)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceLimiterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(ctrlcontroller.Options{MaxConcurrentReconciles: concurrentWorkers}).
		For(&rlv1beta1.ResourceLimiter{}).
		Watches(
			&source.Kind{Type: &corev1.ResourceQuota{}},
			&handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &rlv1beta1.ResourceLimiter{},
			}).
		WithEventFilter(eventPredicate()).
		Complete(r)
}

func (r *ResourceLimiterReconciler) reconcileDelete(ctx context.Context, rl *rlv1beta1.ResourceLimiter) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	if !controllerutil.ContainsFinalizer(rl, constants.DefaultFinalizer) {
		return ctrl.Result{}, fmt.Errorf(fmt.Sprintf("no finalizer found on %s resourcelimiter CR", rl.Name))
	}

	log.WithName("ResourceLimiter").Info(fmt.Sprintf("start delete related resources according to %s resourcelimiter CR", rl.Name))
	var (
		resourceQuota corev1.ResourceQuota
		namespace     corev1.Namespace
		// reused
		namespacedName k8stypes.NamespacedName
	)

	if rl.Status.State != constants.Stopped {
		for idx, ns := range rl.Spec.Targets {
			// Check if namespace exists
			namespacedName = k8stypes.NamespacedName{Namespace: "", Name: string(ns)}
			if err := r.Get(ctx, namespacedName, &namespace); err != nil {
				if apierrors.IsNotFound(err) {
					log.WithName("ResourceLimiter").Info(fmt.Sprintf("namespace %s not found, continue deleting", string(ns)))
					continue
				}
				return ctrl.Result{}, err
			}

			namespacedName = k8stypes.NamespacedName{Namespace: string(ns), Name: fmt.Sprintf("rl-%s-%d", string(ns), idx)}
			resourceQuota = corev1.ResourceQuota{}
			if err := r.Get(ctx, namespacedName, &resourceQuota); err != nil {
				return ctrl.Result{}, err
			}

			if err := r.Delete(ctx, &resourceQuota); err != nil {
				log.WithName("ResourceLimiter").Error(err, fmt.Sprintf("unable to delete quota %s", fmt.Sprintf("rl-%s-%d", string(ns), idx)))
				return ctrl.Result{}, err
			}
			log.WithName("ResourceLimiter").Info(fmt.Sprintf("resource quota %s deleted", fmt.Sprintf("rl-%s-%d", string(ns), idx)))
		}
	}

	newrl := rl.DeepCopy()
	patch := client.MergeFrom(newrl.DeepCopy())
	controllerutil.RemoveFinalizer(newrl, constants.DefaultFinalizer)
	if err := r.Patch(ctx, newrl, patch); err != nil {
		log.WithName("ResourceLimiter").Error(err, fmt.Sprintf("unable to register finalizer FOR %s", newrl.Name))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func setHard(resourceQuota *corev1.ResourceQuota, types map[rlv1beta1.ResourceLimiterType]string) {
	resourceQuota.Spec.Hard[corev1.ResourceLimitsCPU] = k8sresource.MustParse(types[constants.RetrainTypeLimitsCpu])
	resourceQuota.Spec.Hard[corev1.ResourceRequestsCPU] = k8sresource.MustParse(types[constants.RetrainTypeRequestsCpu])
	resourceQuota.Spec.Hard[corev1.ResourceLimitsMemory] = k8sresource.MustParse(types[constants.RetrainTypeLimitsMemory])
	resourceQuota.Spec.Hard[corev1.ResourceRequestsMemory] = k8sresource.MustParse(types[constants.RetrainTypeRequestsMemory])
}

func (r *ResourceLimiterReconciler) reconcile(ctx context.Context, rl *rlv1beta1.ResourceLimiter) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	if len(rl.Spec.Targets) == 0 {
		// Empty lists means all namespaces should be applied
		rl.Spec.Targets = []rlv1beta1.ResourceLimiterNamespace{}
	}

	if len(rl.Spec.Types) == 0 {
		// TODO: other types will be implemented later
		rl.Spec.Types = map[rlv1beta1.ResourceLimiterType]string{constants.RetrainTypeLimitsCpu: "2", constants.RetrainTypeLimitsMemory: "200Mi",
			constants.RetrainTypeRequestsCpu: "1", constants.RetrainTypeRequestsMemory: "150Mi"}
	}

	// Create ResourceQuota per namespace
	var (
		namespace                                                      corev1.Namespace
		namespacedName                                                 k8stypes.NamespacedName
		resourceQuota                                                  = &corev1.ResourceQuota{}
		rlquotas                                                       = map[string]rlv1beta1.ResourceLimiterQuotas{}
		curCpuLimits, curCpuRequests, curMemLimits, curMemRequests     k8sresource.Quantity
		nextCpuLimits, nextCpuRequests, nextMemLimits, nextMemRequests k8sresource.Quantity
	)

	for idx, ns := range rl.Spec.Targets {
		// Make sure namespace exists
		namespacedName = k8stypes.NamespacedName{Namespace: string(ns), Name: string(ns)}
		if err := r.Get(ctx, namespacedName, &namespace); err != nil {
			if apierrors.IsNotFound(err) {
				log.WithName("ResourceLimiter").Error(err, fmt.Sprintf("namespace %s for resource quota not found, please create it first", string(ns)))
			} else {
				log.WithName("ResourceLimiter").Error(err, fmt.Sprintf("get namespace %s for resource quota failed", string(ns)))
			}
			return ctrl.Result{}, err
		}

		// Generate target resource quota spec
		resourceQuota = &corev1.ResourceQuota{}
		if rl.Spec.Applied {
			nextCpuLimits = k8sresource.MustParse(rl.Spec.Types[constants.RetrainTypeLimitsCpu])
			nextCpuRequests = k8sresource.MustParse(rl.Spec.Types[constants.RetrainTypeRequestsCpu])
			nextMemLimits = k8sresource.MustParse(rl.Spec.Types[constants.RetrainTypeLimitsMemory])
			nextMemRequests = k8sresource.MustParse(rl.Spec.Types[constants.RetrainTypeRequestsMemory])
			namespacedName = k8stypes.NamespacedName{Namespace: string(ns), Name: fmt.Sprintf("rl-%s-%d", string(ns), idx)}
			log.WithName("ResourceLimiter").Info(fmt.Sprintf("create or update the resource quota %s", fmt.Sprintf("rl-%s-%d", string(ns), idx)))
			if err := r.Get(ctx, namespacedName, resourceQuota); err != nil {
				if apierrors.IsNotFound(err) {
					log.WithName("ResourceLimiter").Info(fmt.Sprintf("create resource quota %s", fmt.Sprintf("rl-%s-%d", string(ns), idx)))
					resourceQuota.Name = fmt.Sprintf("rl-%s-%d", string(ns), idx)
					resourceQuota.Namespace = string(ns)
					if err := controllerutil.SetControllerReference(rl, resourceQuota, r.Scheme); err != nil {
						log.WithName("ResourceLimiter").Error(err, "Set ResourceLimiter as the owner and controller")
						return ctrl.Result{}, err
					}
					resourceQuota.Spec.Hard = map[corev1.ResourceName]k8sresource.Quantity{}
					setHard(resourceQuota, rl.Spec.Types)
					rlquotas[fmt.Sprintf("rl-%s-%d", string(ns), idx)] = rlv1beta1.ResourceLimiterQuotas{
						Namespace:   string(ns),
						CpuLimits:   fmt.Sprintf("0/%s", nextCpuLimits.String()),
						CpuRequests: fmt.Sprintf("0/%s", nextCpuRequests.String()),
						MemLimits:   fmt.Sprintf("0/%s", nextMemLimits.String()),
						MemRequests: fmt.Sprintf("0/%s", nextMemRequests.String()),
					}
					if er := r.Create(ctx, resourceQuota); er != nil {
						log.WithName("ResourceLimiter").Error(er, fmt.Sprintf("create the quopta %s failed", resourceQuota.Name))
						return ctrl.Result{}, er
					}
					log.WithName("ResourceLimiter").Info(fmt.Sprintf("create resource quota %s successfully", resourceQuota.Name))
					//if err := r.updateStatus(ctx, rl, rlv1beta1.ResourceLimiterStatus{State: constants.Ready, Quotas: rlquotas}); err != nil {
					//	return ctrl.Result{}, err
					//}
					return ctrl.Result{}, nil
				}
				log.WithName("ResourceLimiter").Error(err, fmt.Sprintf("get the quota %s failed", fmt.Sprintf("rl-%s-%d", string(ns), idx)))
				return ctrl.Result{}, err
			} else {
				currl := resourceQuota.DeepCopy()
				curCpuLimits = currl.Status.Used[corev1.ResourceName(constants.RetrainTypeLimitsCpu)]
				curCpuRequests = currl.Status.Used[corev1.ResourceName(constants.RetrainTypeRequestsCpu)]
				curMemLimits = currl.Status.Used[corev1.ResourceName(constants.RetrainTypeLimitsMemory)]
				curMemRequests = currl.Status.Used[corev1.ResourceName(constants.RetrainTypeRequestsMemory)]
				resourceQuota.Spec.Hard = map[corev1.ResourceName]k8sresource.Quantity{}
				setHard(resourceQuota, rl.Spec.Types)
				rlquotas[fmt.Sprintf("rl-%s-%d", string(ns), idx)] = rlv1beta1.ResourceLimiterQuotas{
					Namespace:   string(ns),
					CpuLimits:   fmt.Sprintf("%s/%s", curCpuLimits.String(), nextCpuLimits.String()),
					CpuRequests: fmt.Sprintf("%s/%s", curCpuRequests.String(), nextCpuRequests.String()),
					MemLimits:   fmt.Sprintf("%s/%s", curMemLimits.String(), nextMemLimits.String()),
					MemRequests: fmt.Sprintf("%s/%s", curMemRequests.String(), nextMemRequests.String()),
				}
				if er := r.Update(ctx, resourceQuota); er != nil {
					return ctrl.Result{}, er
				}

				log.WithName("ResourceLimiter").Info(fmt.Sprintf("update resource quota %s successfully", resourceQuota.Name))
			}
		} else {
			// "No" means there is no quotas anymore, but the rl should be lefted
			log.WithName("ResourceLimiter").Info(fmt.Sprintf("delete related resources according to %s resourcelimiter CR", rl.Name))
			namespacedName = k8stypes.NamespacedName{Namespace: string(ns), Name: fmt.Sprintf("rl-%s-%d", string(ns), idx)}
			if err := r.Get(ctx, namespacedName, resourceQuota); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return ctrl.Result{}, err
			}

			if err := r.Delete(ctx, resourceQuota); err != nil {
				log.WithName("ResourceLimiter").Error(err, fmt.Sprintf("unable to delete quota %s", resourceQuota.Name))
				return ctrl.Result{}, err
			}
		}
	}
	if rl.Spec.Applied {
		if err := r.updateStatus(ctx, rl, rlv1beta1.ResourceLimiterStatus{State: constants.Ready, Quotas: rlquotas}); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, r.updateStatus(ctx, rl, rlv1beta1.ResourceLimiterStatus{State: constants.Stopped, Quotas: map[string]rlv1beta1.ResourceLimiterQuotas{}})
}

func (r *ResourceLimiterReconciler) updateStatus(ctx context.Context, rl *rlv1beta1.ResourceLimiter, status rlv1beta1.ResourceLimiterStatus) error {
	rl.Status.State = status.State
	// We do a full-update
	rl.Status.Quotas = make(map[string]rlv1beta1.ResourceLimiterQuotas, len(status.Quotas))
	for k, v := range status.Quotas {
		rl.Status.Quotas[k] = v
	}
	return r.Status().Update(ctx, rl.DeepCopy())
}
