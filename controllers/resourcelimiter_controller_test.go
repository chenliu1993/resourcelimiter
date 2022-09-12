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
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
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

		ctx := context.Background()

		JustAfterEach(func() {
			Eventually(func() bool {
				if err := k8sClient.Delete(ctx, rl); err != nil {
					return apierrors.IsNotFound(err)
				}
				return false
			}, timeout, interval).Should(Equal(true))
		})

		It("Should create ResourceLimter CR and Quotas and delete all quotas when cr got deleted", func() {
			By("By creating a new ResourceLimiter")

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
			By("Should delete ResourceLimter CR and Quotas")
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
			resourceQuota = &corev1.ResourceQuota{}
			namespacedName = types.NamespacedName{}
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
		ctx := context.Background()

		JustAfterEach(func() {
			Eventually(func() bool {
				if err := k8sClient.Delete(ctx, rl); err != nil {
					return apierrors.IsNotFound(err)
				}
				return false
			}, timeout, interval).Should(Equal(true))
		})

		It("Should create ResourceLimter CR and Quotas", func() {
			By("By creating a new ResourceLimiter")

			Expect(k8sClient.Create(ctx, rl)).Should(Succeed())
			var existingResourceLimiter1 rlv1beta1.ResourceLimiter
			Eventually(func() string {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rl), &existingResourceLimiter1); err != nil {
					return "notknown"
				}
				return existingResourceLimiter1.Status.State
			}, 2*timeout, interval).Should(Equal(constants.Ready))

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
			By("Should stop ResourceLimter CR and delete Quotas")
			existingrl := &rlv1beta1.ResourceLimiter{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(rl), existingrl)).Should(Succeed())
			rlstop := existingrl.DeepCopy()
			rlstop.Spec.Applied = false

			Expect(k8sClient.Update(ctx, rlstop)).Should(Succeed())

			var existingResourceLimiter rlv1beta1.ResourceLimiter
			Eventually(func() string {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rlstop), &existingResourceLimiter); err != nil {
					return "notknown"
				}
				return existingResourceLimiter.Status.State
			}, timeout, interval).Should(Equal(constants.Stopped))

			By("By checking all the related quotas non-exists")
			resourceQuota = &corev1.ResourceQuota{}
			namespacedName = types.NamespacedName{}
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

			By("Should delete ResourceLimter CR and Quotas")
			Expect(k8sClient.Delete(ctx, &existingResourceLimiter)).Should(Succeed())
			var existingResourceLimiter2 rlv1beta1.ResourceLimiter
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rl), &existingResourceLimiter2); err != nil {
					if apierrors.IsNotFound(err) {
						return true
					}
				}
				return false
			}, timeout, interval).Should(Equal(true))
		})
	})

	Context("ResourceLimiter Quota working", func() {
		rl := &rlv1beta1.ResourceLimiter{}
		content, err := ioutil.ReadFile(filepath.Join(pwd, "fixtures/fixtures_cr.yaml"))
		Expect(err).NotTo(HaveOccurred())
		err = yaml.Unmarshal(content, rl)
		Expect(err).NotTo(HaveOccurred())

		podOk := &corev1.Pod{}
		content, err = ioutil.ReadFile(filepath.Join(pwd, "fixtures/fixtures_pod_ok.yaml"))
		Expect(err).NotTo(HaveOccurred())
		err = yaml.Unmarshal(content, podOk)
		Expect(err).NotTo(HaveOccurred())

		ctx := context.Background()

		JustAfterEach(func() {
			Eventually(func() bool {
				if err := k8sClient.Delete(ctx, rl); err != nil {
					return apierrors.IsNotFound(err)
				}
				return false
			}, timeout, interval).Should(Equal(true))
			Eventually(func() bool {
				if err := k8sClient.Delete(ctx, podOk); err != nil {
					return apierrors.IsNotFound(err)
				}
				return false
			}, 5*timeout, interval).Should(Equal(true))
		})

		It("Should create the pod successfully", func() {
			ctx := context.Background()
			By("By creating a new ResourceLimiter")
			Expect(k8sClient.Create(ctx, rl)).Should(Succeed())
			var existingResourceLimiter1 rlv1beta1.ResourceLimiter
			Eventually(func() string {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rl), &existingResourceLimiter1); err != nil {
					return "notknown"
				}
				return existingResourceLimiter1.Status.State
			}, timeout, interval).Should(Equal(constants.Ready))

			By("By checking all the related quotas")
			existingResourceQuota1 := &corev1.ResourceQuota{}
			for idx, ns := range rl.Spec.Targets {
				if ns == constants.IgnoreKubePublic || ns == constants.IgnoreKubeSystem {
					continue
				}
				Eventually(func() bool {
					namespacedName := types.NamespacedName{Name: fmt.Sprintf("rl-%s-%d", string(ns), idx), Namespace: string(ns)}
					if err := k8sClient.Get(ctx, namespacedName, existingResourceQuota1); err != nil {
						return false
					}
					return true
				}, timeout, interval).Should(Equal(true))
			}
			By("By createing the target pod")
			Expect(k8sClient.Create(ctx, podOk)).Should(Succeed())
			var existingPod corev1.Pod
			Eventually(func() string {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(podOk), &existingPod); err != nil {
					return "notknown"
				}
				return string(existingPod.Status.Phase)
			}, 2*timeout, interval).Should(Equal("Running"))

			By("By checking the quota limits")
			existingResourceQuota2 := &corev1.ResourceQuota{}
			Eventually(func() bool {
				namespacedName := types.NamespacedName{Name: fmt.Sprintf("rl-%s-%d", "default", 0), Namespace: "default"}
				if err := k8sClient.Get(ctx, namespacedName, existingResourceQuota2); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(Equal(true))
			Expect(existingResourceQuota2.Status.Used["limits.cpu"]).Should(Equal(k8sresource.MustParse("200m")))
			Expect(existingResourceQuota2.Status.Used["requests.cpu"]).Should(Equal(k8sresource.MustParse("100m")))
			Expect(existingResourceQuota2.Status.Used["limits.memory"]).Should(Equal(k8sresource.MustParse("100Mi")))
			Expect(existingResourceQuota2.Status.Used["requests.memory"]).Should(Equal(k8sresource.MustParse("90Mi")))
		})
	})

	Context("ResourceLimiter Quota not working", func() {
		rl := &rlv1beta1.ResourceLimiter{}
		content, err := ioutil.ReadFile(filepath.Join(pwd, "fixtures/fixtures_cr.yaml"))
		Expect(err).NotTo(HaveOccurred())
		err = yaml.Unmarshal(content, rl)
		Expect(err).NotTo(HaveOccurred())

		podUnOk := &corev1.Pod{}
		content, err = ioutil.ReadFile(filepath.Join(pwd, "fixtures/fixtures_pod_not_ok.yaml"))
		Expect(err).NotTo(HaveOccurred())
		err = yaml.Unmarshal(content, podUnOk)
		Expect(err).NotTo(HaveOccurred())

		ctx := context.Background()

		JustAfterEach(func() {
			Eventually(func() bool {
				if err := k8sClient.Delete(ctx, rl); err != nil {
					return apierrors.IsNotFound(err)
				}
				return false
			}, timeout, interval).Should(Equal(true))
			Eventually(func() bool {
				if err := k8sClient.Delete(ctx, podUnOk); err != nil {
					return apierrors.IsNotFound(err)
				}
				return false
			}, 5*timeout, interval).Should(Equal(true))
		})

		It("Should create the pod failed", func() {
			ctx := context.Background()
			By("By creating a new ResourceLimiter")
			Expect(k8sClient.Create(ctx, rl)).Should(Succeed())
			var existingResourceLimiter1 rlv1beta1.ResourceLimiter
			Eventually(func() string {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rl), &existingResourceLimiter1); err != nil {
					return "notknown"
				}
				return existingResourceLimiter1.Status.State
			}, timeout, interval).Should(Equal(constants.Ready))

			By("By checking all the related quotas")
			existingResourceQuota1 := &corev1.ResourceQuota{}
			for idx, ns := range rl.Spec.Targets {
				if ns == constants.IgnoreKubePublic || ns == constants.IgnoreKubeSystem {
					continue
				}
				Eventually(func() bool {
					namespacedName := types.NamespacedName{Name: fmt.Sprintf("rl-%s-%d", string(ns), idx), Namespace: string(ns)}
					if err := k8sClient.Get(ctx, namespacedName, existingResourceQuota1); err != nil {
						return false
					}
					return true
				}, timeout, interval).Should(Equal(true))
			}
			By("By createing the target pod")
			Eventually(func() string {
				if err := k8sClient.Create(ctx, podUnOk); err != nil {
					return err.Error()
				}
				return ""
			}, timeout, interval).Should(ContainSubstring("forbidden: exceeded quota"))
		})
	})
})
