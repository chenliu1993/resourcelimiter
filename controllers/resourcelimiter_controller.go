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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	rlv1beta1 "github.com/chenliu1993/resourcelimiter/api/v1beta1"
	"github.com/chenliu1993/resourcelimiter/pkg/constants"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
)

// ResourceLimiterReconciler reconciles a ResourceLimiter object
type ResourceLimiterReconciler struct {
	client.Client
	KubeClient *kubernetes.Clientset
	Log        logr.Logger
	Scheme     *runtime.Scheme
}

//+kubebuilder:rbac:groups=resources.resourcelimiter.io,resources=resourcelimiters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=resources.resourcelimiter.io,resources=resourcelimiters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=resourcequotas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;create;
//+kubebuilder:rbac:groups=resources.resourcelimiter.io,resources=resourcelimiters/finalizers,verbs=update

func (r *ResourceLimiterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.WithValues("resourcelimiter", "")

	var rl rlv1beta1.ResourceLimiter
	if err := r.Get(ctx, req.NamespacedName, &rl); err != nil {
		// return ctrl.Result{Requeue: true}, client.IgnoreNotFound(err)
		return ctrl.Result{Requeue: true}, err
	}

	targetNs := rl.Spec.Targets
	if len(targetNs) == 0 {
		// Empty lists means all namespaces should be applied
		targetNs = []rlv1beta1.ResourceLimiterNamespace{}
	}

	level := rl.Spec.Level
	if level == "" {
		level = constants.RestrainLevelHard
	}
	types := rl.Spec.Types
	if len(types) == 0 {
		// TODO: storage will be implemented later
		types = map[rlv1beta1.ResourceLimiterType]string{constants.RetrainTypeCpu: "0", constants.RetrainTypeMemory: "0"}
	}

	// Create ResourceQuota per namespace
	var resourceQuota *corev1.ResourceQuota
	for idx, ns := range targetNs {
		if ns == constants.IgnoreKubeSystem {
			continue
		}
		// Make sure namespace exists
		if _, err := r.KubeClient.CoreV1().Namespaces().Get(ctx, string(ns), metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				if _, er := r.KubeClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: string(ns)}}, metav1.CreateOptions{}); er != nil {
					return reconcile.Result{}, er
				}
			} else {
				return reconcile.Result{Requeue: true}, err
			}
		}

		// Generate target resource quota spec
		resourceQuota.Name = fmt.Sprintf("rl-%s-%d", string(ns), idx)
		resourceQuota.Namespace = string(ns)
		if level == constants.RestrainLevelHard {
			resourceQuota.Spec.Hard[corev1.ResourceLimitsCPU] = k8sresource.Quantity{
				Format: k8sresource.Format(types[constants.RetrainTypeCpu]),
			}
			resourceQuota.Spec.Hard[corev1.ResourceRequestsCPU] = k8sresource.Quantity{
				Format: k8sresource.Format(types[constants.RetrainTypeCpu]),
			}
			resourceQuota.Spec.Hard[corev1.ResourceLimitsMemory] = k8sresource.Quantity{
				Format: k8sresource.Format(types[constants.RetrainTypeMemory]),
			}
			resourceQuota.Spec.Hard[corev1.ResourceRequestsMemory] = k8sresource.Quantity{
				Format: k8sresource.Format(types[constants.RetrainTypeMemory]),
			}
		} else if level == constants.RestrainLevelSoft {
			resourceQuota.Spec.Hard[corev1.ResourceRequestsCPU] = k8sresource.Quantity{
				Format: k8sresource.Format(types[constants.RetrainTypeCpu]),
			}
			resourceQuota.Spec.Hard[corev1.ResourceRequestsMemory] = k8sresource.Quantity{
				Format: k8sresource.Format(types[constants.RetrainTypeMemory]),
			}
		} else {
			// "No" means there is no quotas anymore
			if err := r.KubeClient.CoreV1().ResourceQuotas(string(ns)).Delete(ctx, resourceQuota.Name, metav1.DeleteOptions{}); err != nil {
				if apierrors.IsNotFound(err) {
					// For whatever reason, the rl cr is gone, so we ignore it then
					return reconcile.Result{}, nil
				} else {
					// Should warn here
					r.Log.WithValues("resourcelimiter", "").Info(fmt.Sprintf("Delete resource quota %s failed, please check the error msg %s", resourceQuota.Name, err.Error()))
					return reconcile.Result{Requeue: true}, err
				}
			}
		}

		if _, err := r.KubeClient.CoreV1().ResourceQuotas(string(ns)).Get(ctx, resourceQuota.Name, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				_, er := r.KubeClient.CoreV1().ResourceQuotas(string(ns)).Create(ctx, resourceQuota, metav1.CreateOptions{})
				if er != nil {
					return reconcile.Result{Requeue: true}, er
				}
				return reconcile.Result{}, nil
			}
			return reconcile.Result{Requeue: true}, err
		} else {
			if _, er := r.KubeClient.CoreV1().ResourceQuotas(string(ns)).Update(ctx, resourceQuota, metav1.UpdateOptions{}); er != nil {
				return reconcile.Result{Requeue: true}, er
			}
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceLimiterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rlv1beta1.ResourceLimiter{}).
		Complete(r)
}
