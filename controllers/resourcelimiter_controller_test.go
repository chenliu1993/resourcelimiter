package controllers

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	rlv1beta1 "github.com/chenliu1993/resourcelimiter/api/v1beta1"
	"github.com/chenliu1993/resourcelimiter/pkg/constants"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	timeout  = 10 * time.Second
	interval = 1 * time.Second
)

var _ = Describe("ResourceLimiter controller", func() {
	pwd, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())

	Context("ResourceLimiter LifeCycle 1", func() {
		rl := &rlv1beta1.ResourceLimiter{}
		content, err := ioutil.ReadFile(filepath.Join(pwd, "fixtures/fixtures_cr.yaml"))
		Expect(err).NotTo(HaveOccurred())
		err = yaml.Unmarshal(content, rl)
		Expect(err).NotTo(HaveOccurred())
		It("Should create ResourceLimter CR and Quotas", func() {
			By("By creating a new ResourceLimiter")
			ctx := context.Background()

			Expect(k8sClient.Create(ctx, rl)).Should(Succeed())
			var existingResourceLimiter1 rlv1beta1.ResourceLimiter
			Eventually(func() string {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rl), &existingResourceLimiter1); err != nil {
					return "notknown"
				}
				return existingResourceLimiter1.Status.State
			}, timeout, interval).Should(Equal(constants.Ready))

			By("By checking all the related quotas")
			resourceQuota := &corev1.ResourceQuota{}
			namespacedName := types.NamespacedName{}
			for idx, ns := range rl.Spec.Targets {
				if ns == constants.IgnoreKubePublic || ns == constants.IgnoreKubeSystem {
					continue
				}

				Eventually(func() bool {
					namespacedName = types.NamespacedName{Name: fmt.Sprintf("rl-%s-%d", string(ns), idx), Namespace: string(ns)}
					if err := k8sClient.Get(ctx, namespacedName, resourceQuota); err != nil {
						// fmt.Fprintf(GinkgoWriter, "%v", err)
						return false
					}
					return true
				}, timeout, interval).Should(Equal(true))
			}
		})

		It("Should delete ResourceLimter CR and Quotas", func() {
			By("By deleting a ResourceLimiter")
			ctx := context.Background()

			Expect(k8sClient.Delete(ctx, rl)).Should(Succeed())
			var existingResourceLimiter rlv1beta1.ResourceLimiter
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rl), &existingResourceLimiter); err != nil {
					if apierrors.IsNotFound(err) {
						return true
					}
				}
				return false
			}, timeout, interval).Should(Equal(true))

			By("By checking all the related quotas non-exists")
			resourceQuota := &corev1.ResourceQuota{}
			namespacedName := types.NamespacedName{}
			for idx, ns := range rl.Spec.Targets {
				if ns == constants.IgnoreKubePublic || ns == constants.IgnoreKubeSystem {
					continue
				}

				Eventually(func() bool {
					namespacedName = types.NamespacedName{Name: fmt.Sprintf("rl-%s-%d", string(ns), idx), Namespace: string(ns)}
					if err := k8sClient.Get(ctx, namespacedName, resourceQuota); err != nil {
						if apierrors.IsNotFound(err) {
							return true
						}
					}
					return false
				}, timeout, interval).Should(Equal(true))
			}
		})
	})

	Context("ResourceLimiter LifeCycle 2", func() {
		rl := &rlv1beta1.ResourceLimiter{}
		content, err := ioutil.ReadFile(filepath.Join(pwd, "fixtures/fixtures_cr.yaml"))
		Expect(err).NotTo(HaveOccurred())
		err = yaml.Unmarshal(content, rl)
		Expect(err).NotTo(HaveOccurred())
		It("Should create ResourceLimter CR and Quotas", func() {
			By("By creating a new ResourceLimiter")
			ctx := context.Background()

			Expect(k8sClient.Create(ctx, rl)).Should(Succeed())
			var existingResourceLimiter1 rlv1beta1.ResourceLimiter
			Eventually(func() string {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rl), &existingResourceLimiter1); err != nil {
					return "notknown"
				}
				return existingResourceLimiter1.Status.State
			}, timeout, interval).Should(Equal(constants.Ready))

			By("By checking all the related quotas")
			resourceQuota := &corev1.ResourceQuota{}
			namespacedName := types.NamespacedName{}
			for idx, ns := range rl.Spec.Targets {
				if ns == constants.IgnoreKubePublic || ns == constants.IgnoreKubeSystem {
					continue
				}

				Eventually(func() bool {
					namespacedName = types.NamespacedName{Name: fmt.Sprintf("rl-%s-%d", string(ns), idx), Namespace: string(ns)}
					if err := k8sClient.Get(ctx, namespacedName, resourceQuota); err != nil {
						return false
					}
					return true
				}, timeout, interval).Should(Equal(true))
			}
		})

		It("Should stop ResourceLimter CR and delete Quotas", func() {
			By("By stopping a ResourceLimiter")
			existingrl := &rlv1beta1.ResourceLimiter{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(rl), existingrl)).Should(Succeed())
			rlstop := existingrl.DeepCopy()
			rlstop.Spec.Applied = false
			ctx := context.Background()

			Expect(k8sClient.Update(ctx, rlstop)).Should(Succeed())

			var existingResourceLimiter rlv1beta1.ResourceLimiter
			Eventually(func() string {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rlstop), &existingResourceLimiter); err != nil {
					return "notknown"
				}
				return existingResourceLimiter.Status.State
			}, timeout, interval).Should(Equal(constants.Stopped))

			By("By checking all the related quotas non-exists")
			resourceQuota := &corev1.ResourceQuota{}
			namespacedName := types.NamespacedName{}
			for idx, ns := range rlstop.Spec.Targets {
				if ns == constants.IgnoreKubePublic || ns == constants.IgnoreKubeSystem {
					continue
				}

				Eventually(func() bool {
					namespacedName = types.NamespacedName{Name: fmt.Sprintf("rl-%s-%d", string(ns), idx), Namespace: string(ns)}
					if err := k8sClient.Get(ctx, namespacedName, resourceQuota); err != nil {
						if apierrors.IsNotFound(err) {
							return true
						}
					}
					return false
				}, timeout, interval).Should(Equal(true))
			}
		})
	})
})
