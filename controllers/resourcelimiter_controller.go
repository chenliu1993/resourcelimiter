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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	rlv1beta1 "github.com/chenliu1993/resourcelimiter/api/v1beta1"
	"github.com/chenliu1993/resourcelimiter/pkg/constants"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

// ResourceLimiterReconciler reconciles a ResourceLimiter object
type ResourceLimiterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
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

	// Add our finalizer if it does not exist
	if !controllerutil.ContainsFinalizer(&rl, constants.DefaultFinalizer) {
		patch := client.MergeFrom(rl.DeepCopy())
		controllerutil.AddFinalizer(&rl, constants.DefaultFinalizer)
		if err := r.Patch(ctx, &rl, patch); err != nil {
			log.WithName("ResourceLimiter").Error(err, "unable to register finalizer")
			return ctrl.Result{}, err
		}
	}

	// Under deletion
	if !rl.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &rl)
	}

	return r.reconcile(ctx, &rl)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceLimiterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rlv1beta1.ResourceLimiter{}).
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
			if ns == constants.IgnoreKubeSystem || ns == constants.IgnoreKubePublic {
				continue
			}
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

	patch := client.MergeFrom(rl.DeepCopy())
	controllerutil.RemoveFinalizer(rl, constants.DefaultFinalizer)
	if err := r.Patch(ctx, rl, patch); err != nil {
		log.WithName("ResourceLimiter").Error(err, "unable to register finalizer")
		return ctrl.Result{}, err
	}
	// rlstop := rl.DeepCopy()
	// controllerutil.RemoveFinalizer(rlstop, constants.DefaultFinalizer)
	// if err := r.Update(ctx, rlstop); err != nil {
	// 	return ctrl.Result{}, err
	// }

	return ctrl.Result{}, nil
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
		namespace      corev1.Namespace
		namespacedName k8stypes.NamespacedName
		resourceQuota  = &corev1.ResourceQuota{}
		blockOnOwner   = true
	)

	for idx, ns := range rl.Spec.Targets {
		if ns == constants.IgnoreKubeSystem || ns == constants.IgnoreKubePublic {
			continue
		}
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
			namespacedName = k8stypes.NamespacedName{Namespace: string(ns), Name: fmt.Sprintf("rl-%s-%d", string(ns), idx)}
			log.WithName("ResourceLimiter").Info(fmt.Sprintf("create or update the resource quota %s", fmt.Sprintf("rl-%s-%d", string(ns), idx)))
			if err := r.Get(ctx, namespacedName, resourceQuota); err != nil {
				if apierrors.IsNotFound(err) {
					log.WithName("ResourceLimiter").Info(fmt.Sprintf("create resource quota %s", fmt.Sprintf("rl-%s-%d", string(ns), idx)))
					resourceQuota.Name = fmt.Sprintf("rl-%s-%d", string(ns), idx)
					resourceQuota.Namespace = string(ns)
					resourceQuota.OwnerReferences = []metav1.OwnerReference{
						{
							UID:                rl.UID,
							APIVersion:         constants.ResourceLimiterApiVersion,
							Kind:               constants.ResourceLimiterKind,
							Name:               rl.Name,
							BlockOwnerDeletion: &blockOnOwner,
						},
					}
					resourceQuota.Spec.Hard = map[corev1.ResourceName]k8sresource.Quantity{}
					resourceQuota.Spec.Hard[corev1.ResourceLimitsCPU] = k8sresource.MustParse(rl.Spec.Types[constants.RetrainTypeLimitsCpu])
					resourceQuota.Spec.Hard[corev1.ResourceRequestsCPU] = k8sresource.MustParse(rl.Spec.Types[constants.RetrainTypeRequestsCpu])
					resourceQuota.Spec.Hard[corev1.ResourceLimitsMemory] = k8sresource.MustParse(rl.Spec.Types[constants.RetrainTypeLimitsMemory])
					resourceQuota.Spec.Hard[corev1.ResourceRequestsMemory] = k8sresource.MustParse(rl.Spec.Types[constants.RetrainTypeRequestsMemory])
					if er := r.Create(ctx, resourceQuota); er != nil {
						log.WithName("ResourceLimiter").Error(er, fmt.Sprintf("create the quopta %s failed", resourceQuota.Name))
						return ctrl.Result{}, er
					}
					log.WithName("ResourceLimiter").Info(fmt.Sprintf("create resource quota %s successfully", resourceQuota.Name))
					if err := r.updateStatus(ctx, rl, constants.Ready); err != nil {
						return ctrl.Result{}, err
					}
					return ctrl.Result{}, nil
				}
				log.WithName("ResourceLimiter").Error(err, fmt.Sprintf("get the quota %s failed", fmt.Sprintf("rl-%s-%d", string(ns), idx)))
				return ctrl.Result{}, err
			} else {
				resourceQuota.Spec.Hard = map[corev1.ResourceName]k8sresource.Quantity{}
				resourceQuota.Spec.Hard[corev1.ResourceLimitsCPU] = k8sresource.MustParse(rl.Spec.Types[constants.RetrainTypeLimitsCpu])
				resourceQuota.Spec.Hard[corev1.ResourceRequestsCPU] = k8sresource.MustParse(rl.Spec.Types[constants.RetrainTypeRequestsCpu])
				resourceQuota.Spec.Hard[corev1.ResourceLimitsMemory] = k8sresource.MustParse(rl.Spec.Types[constants.RetrainTypeLimitsMemory])
				resourceQuota.Spec.Hard[corev1.ResourceRequestsMemory] = k8sresource.MustParse(rl.Spec.Types[constants.RetrainTypeRequestsMemory])
				if er := r.Update(ctx, resourceQuota); er != nil {
					return ctrl.Result{}, er
				}
				log.WithName("ResourceLimiter").Info(fmt.Sprintf("update resource quota %s successfully", resourceQuota.Name))
				if err := r.updateStatus(ctx, rl, constants.Ready); err != nil {
					return ctrl.Result{}, err
				}
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
			log.WithName("ResourceLimiter").Info(fmt.Sprintf("resource quota %s deleted", fmt.Sprintf("rl-%s-%d", string(ns), idx)))

			if err := r.updateStatus(ctx, rl, constants.Stopped); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *ResourceLimiterReconciler) updateStatus(ctx context.Context, rl *rlv1beta1.ResourceLimiter, targetState string) error {
	rl.Status.State = targetState
	return r.Status().Update(ctx, rl.DeepCopy())
}
