package controllers

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"time"

	rlv1beta2 "github.com/chenliu1993/resourcelimiter/api/v1beta2"
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
		rl := &rlv1beta2.ResourceLimiter{}
		content, err := ioutil.ReadFile(filepath.Join(pwd, "fixtures/fixtures_cr_v1beta2.yaml"))
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
			var existingResourceLimiter1 rlv1beta2.ResourceLimiter
			Eventually(func() string {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rl), &existingResourceLimiter1); err != nil {
					return "notknown"
				}
				return existingResourceLimiter1.Status.State
			}, timeout, interval).Should(Equal(constants.Ready))

			By("By checking all the related quotas")
			resourceQuota := &corev1.ResourceQuota{}
			namespacedName := types.NamespacedName{}
			for _, tgt := range existingResourceLimiter1.Spec.Quotas {

				Eventually(func() bool {
					namespacedName = types.NamespacedName{Name: fmt.Sprintf("rl-quota-%s", tgt.NamespaceName), Namespace: tgt.NamespaceName}
					if err := k8sClient.Get(ctx, namespacedName, resourceQuota); err != nil {
						return false
					}
					return true
				}, timeout, interval).Should(Equal(true))
			}
			By("Should delete ResourceLimter CR and Quotas")
			Expect(k8sClient.Delete(ctx, rl)).Should(Succeed())
			var existingResourceLimiter2 rlv1beta2.ResourceLimiter
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rl), &existingResourceLimiter2); err != nil {
					if apierrors.IsNotFound(err) {
						return true
					}
				}
				return false
			}, timeout, interval).Should(Equal(true))

			By("By checking all the related quotas non-exists")
			resourceQuota = &corev1.ResourceQuota{}
			namespacedName = types.NamespacedName{}
			for _, tgt := range existingResourceLimiter1.Spec.Quotas {

				Eventually(func() bool {
					namespacedName = types.NamespacedName{Name: fmt.Sprintf("rl-quota-%s", tgt.NamespaceName), Namespace: tgt.NamespaceName}
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
		rl := &rlv1beta2.ResourceLimiter{}
		content, err := ioutil.ReadFile(filepath.Join(pwd, "fixtures/fixtures_cr_v1beta2.yaml"))
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
			var existingResourceLimiter1 rlv1beta2.ResourceLimiter
			Eventually(func() string {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rl), &existingResourceLimiter1); err != nil {
					return "notknown"
				}
				return existingResourceLimiter1.Status.State
			}, 2*timeout, interval).Should(Equal(constants.Ready))

			By("By checking all the related quotas")
			resourceQuota := &corev1.ResourceQuota{}
			namespacedName := types.NamespacedName{}
			for _, tgt := range existingResourceLimiter1.Spec.Quotas {

				Eventually(func() bool {
					namespacedName = types.NamespacedName{Name: fmt.Sprintf("rl-quota-%s", tgt.NamespaceName), Namespace: tgt.NamespaceName}
					if err := k8sClient.Get(ctx, namespacedName, resourceQuota); err != nil {
						return false
					}
					return true
				}, timeout, interval).Should(Equal(true))
			}
			By("Should stop ResourceLimter CR and delete Quotas")
			existingResourceLimiter2 := &rlv1beta2.ResourceLimiter{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(rl), existingResourceLimiter2)).Should(Succeed())
			rlstop := existingResourceLimiter2.DeepCopy()
			rlstop.Spec.Applied = false

			Expect(k8sClient.Update(ctx, rlstop)).Should(Succeed())

			var existingResourceLimiter3 rlv1beta2.ResourceLimiter
			Eventually(func() string {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rlstop), &existingResourceLimiter3); err != nil {
					return "notknown"
				}
				return existingResourceLimiter3.Status.State
			}, timeout, interval).Should(Equal(constants.Stopped))

			By("By checking all the related quotas non-exists")
			resourceQuota = &corev1.ResourceQuota{}
			namespacedName = types.NamespacedName{}
			for _, tgt := range existingResourceLimiter3.Spec.Quotas {

				Eventually(func() bool {
					namespacedName = types.NamespacedName{Name: fmt.Sprintf("rl-quota-%s", tgt.NamespaceName), Namespace: tgt.NamespaceName}
					if err := k8sClient.Get(ctx, namespacedName, resourceQuota); err != nil {
						if apierrors.IsNotFound(err) {
							return true
						}
					}
					return false
				}, timeout, interval).Should(Equal(true))
			}

			By("Should delete ResourceLimter CR and Quotas")
			Expect(k8sClient.Delete(ctx, &existingResourceLimiter3)).Should(Succeed())
			var existingResourceLimiter4 rlv1beta2.ResourceLimiter
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rl), &existingResourceLimiter4); err != nil {
					if apierrors.IsNotFound(err) {
						return true
					}
				}
				return false
			}, timeout, interval).Should(Equal(true))
		})
	})

	Context("ResourceLimiter Quota working", func() {
		rl := &rlv1beta2.ResourceLimiter{}
		content, err := ioutil.ReadFile(filepath.Join(pwd, "fixtures/fixtures_cr_v1beta2.yaml"))
		Expect(err).NotTo(HaveOccurred())
		err = yaml.Unmarshal(content, rl)
		Expect(err).NotTo(HaveOccurred())

		podOk := &corev1.Pod{}
		content, err = ioutil.ReadFile(filepath.Join(pwd, "fixtures/fixtures_pod_ok.yaml"))
		Expect(err).NotTo(HaveOccurred())
		err = yaml.Unmarshal(content, podOk)
		Expect(err).NotTo(HaveOccurred())

		podOk1 := &corev1.Pod{}
		content, err = ioutil.ReadFile(filepath.Join(pwd, "fixtures/fixtures_pod_ok_1.yaml"))
		Expect(err).NotTo(HaveOccurred())
		err = yaml.Unmarshal(content, podOk1)
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
			Eventually(func() bool {
				if err := k8sClient.Delete(ctx, podOk1); err != nil {
					return apierrors.IsNotFound(err)
				}
				return false
			}, 5*timeout, interval).Should(Equal(true))
		})

		It("Should create the pod successfully", func() {
			ctx := context.Background()
			By("By creating a new ResourceLimiter")
			Expect(k8sClient.Create(ctx, rl)).Should(Succeed())
			var existingResourceLimiter1 rlv1beta2.ResourceLimiter
			Eventually(func() string {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rl), &existingResourceLimiter1); err != nil {
					return "notknown"
				}
				return existingResourceLimiter1.Status.State
			}, timeout, interval).Should(Equal(constants.Ready))

			By("By checking all the related quotas")
			existingResourceQuota1 := &corev1.ResourceQuota{}
			for _, tgt := range existingResourceLimiter1.Spec.Quotas {

				Eventually(func() bool {
					namespacedName := types.NamespacedName{Name: fmt.Sprintf("rl-quota-%s", tgt.NamespaceName), Namespace: tgt.NamespaceName}
					if err := k8sClient.Get(ctx, namespacedName, existingResourceQuota1); err != nil {
						return false
					}
					return true
				}, timeout, interval).Should(Equal(true))
			}
			By("By createing the target pods")
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

			Expect(k8sClient.Create(ctx, podOk1)).Should(Succeed())
			var existingPod1 corev1.Pod
			Eventually(func() string {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(podOk), &existingPod1); err != nil {
					return "notknown"
				}
				return string(existingPod1.Status.Phase)
			}, 2*timeout, interval).Should(Equal("Running"))

			By("By checking the quota limits")
			existingResourceQuota3 := &corev1.ResourceQuota{}
			Eventually(func() bool {
				namespacedName := types.NamespacedName{Name: fmt.Sprintf("rl-%s-%d", "local-path-storage", 1), Namespace: "local-path-storage"}
				if err := k8sClient.Get(ctx, namespacedName, existingResourceQuota3); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(Equal(true))
			Expect(existingResourceQuota3.Status.Used["limits.cpu"]).Should(Equal(k8sresource.MustParse("200m")))
			Expect(existingResourceQuota3.Status.Used["requests.cpu"]).Should(Equal(k8sresource.MustParse("100m")))
			Expect(existingResourceQuota3.Status.Used["limits.memory"]).Should(Equal(k8sresource.MustParse("100Mi")))
			Expect(existingResourceQuota3.Status.Used["requests.memory"]).Should(Equal(k8sresource.MustParse("90Mi")))
		})
	})

	Context("ResourceLimiter Quota not working", func() {
		rl := &rlv1beta2.ResourceLimiter{}
		content, err := ioutil.ReadFile(filepath.Join(pwd, "fixtures/fixtures_cr_v1beta2.yaml"))
		Expect(err).NotTo(HaveOccurred())
		err = yaml.Unmarshal(content, rl)
		Expect(err).NotTo(HaveOccurred())

		podUnOk := &corev1.Pod{}
		content, err = ioutil.ReadFile(filepath.Join(pwd, "fixtures/fixtures_pod_not_ok.yaml"))
		Expect(err).NotTo(HaveOccurred())
		err = yaml.Unmarshal(content, podUnOk)
		Expect(err).NotTo(HaveOccurred())

		podUnOk1 := &corev1.Pod{}
		content, err = ioutil.ReadFile(filepath.Join(pwd, "fixtures/fixtures_pod_not_ok_1.yaml"))
		Expect(err).NotTo(HaveOccurred())
		err = yaml.Unmarshal(content, podUnOk1)
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
			Eventually(func() bool {
				if err := k8sClient.Delete(ctx, podUnOk1); err != nil {
					return apierrors.IsNotFound(err)
				}
				return false
			}, 5*timeout, interval).Should(Equal(true))
		})

		It("Should create the pod failed", func() {
			By("By creating a new ResourceLimiter")
			Expect(k8sClient.Create(ctx, rl)).Should(Succeed())
			var existingResourceLimiter1 rlv1beta2.ResourceLimiter
			Eventually(func() string {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rl), &existingResourceLimiter1); err != nil {
					return "notknown"
				}
				return existingResourceLimiter1.Status.State
			}, timeout, interval).Should(Equal(constants.Ready))

			By("By checking all the related quotas")
			existingResourceQuota1 := &corev1.ResourceQuota{}
			for _, tgt := range existingResourceLimiter1.Spec.Quotas {

				Eventually(func() bool {
					namespacedName := types.NamespacedName{Name: fmt.Sprintf("rl-quota-%s", tgt.NamespaceName), Namespace: tgt.NamespaceName}
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
			Eventually(func() string {
				if err := k8sClient.Create(ctx, podUnOk1); err != nil {
					return err.Error()
				}
				return ""
			}, timeout, interval).Should(ContainSubstring("forbidden: exceeded quota"))
		})
	})

	Context("ResourceLimiter Quota half working", func() {
		rl := &rlv1beta2.ResourceLimiter{}
		content, err := ioutil.ReadFile(filepath.Join(pwd, "fixtures/fixtures_cr_v1beta2.yaml"))
		Expect(err).NotTo(HaveOccurred())
		err = yaml.Unmarshal(content, rl)
		Expect(err).NotTo(HaveOccurred())

		podOk := &corev1.Pod{}
		content, err = ioutil.ReadFile(filepath.Join(pwd, "fixtures/fixtures_pod_ok_1.yaml"))
		Expect(err).NotTo(HaveOccurred())
		err = yaml.Unmarshal(content, podOk)
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
			Eventually(func() bool {
				if err := k8sClient.Delete(ctx, podOk); err != nil {
					return apierrors.IsNotFound(err)
				}
				return false
			}, 5*timeout, interval).Should(Equal(true))
		})

		It("Should create the pod failed", func() {
			By("By creating a new ResourceLimiter")
			Expect(k8sClient.Create(ctx, rl)).Should(Succeed())
			var existingResourceLimiter1 rlv1beta2.ResourceLimiter
			Eventually(func() string {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rl), &existingResourceLimiter1); err != nil {
					return "notknown"
				}
				return existingResourceLimiter1.Status.State
			}, timeout, interval).Should(Equal(constants.Ready))

			By("By checking all the related quotas")
			existingResourceQuota1 := &corev1.ResourceQuota{}
			for _, tgt := range existingResourceLimiter1.Spec.Quotas {

				Eventually(func() bool {
					namespacedName := types.NamespacedName{Name: fmt.Sprintf("rl-quota-%s", tgt.NamespaceName), Namespace: tgt.NamespaceName}
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

			Expect(k8sClient.Create(ctx, podOk)).Should(Succeed())
			var existingPod1 corev1.Pod
			Eventually(func() string {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(podOk), &existingPod1); err != nil {
					return "notknown"
				}
				return string(existingPod1.Status.Phase)
			}, 2*timeout, interval).Should(Equal("Running"))

			By("By checking the quota limits")
			existingResourceQuota2 := &corev1.ResourceQuota{}
			Eventually(func() bool {
				namespacedName := types.NamespacedName{Name: fmt.Sprintf("rl-quota-%s", "local-path-storage"), Namespace: "local-path-storage"}
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
	Context("ResourceLimiter Status Quota", func() {
		rl := &rlv1beta2.ResourceLimiter{}
		content, err := ioutil.ReadFile(filepath.Join(pwd, "fixtures/fixtures_cr_v1beta2.yaml"))
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

		})

		It("Should show the right status", func() {
			By("By creating a new ResourceLimiter")
			Expect(k8sClient.Create(ctx, rl)).Should(Succeed())
			var existingResourceLimiter1 rlv1beta2.ResourceLimiter
			Eventually(func() string {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rl), &existingResourceLimiter1); err != nil {
					return "notknown"
				}
				return existingResourceLimiter1.Status.State
			}, timeout, interval).Should(Equal(constants.Ready))

			By("By checking all the related quotas")
			existingResourceLimiter2 := &rlv1beta2.ResourceLimiter{}
			Eventually(func() string {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&existingResourceLimiter1), existingResourceLimiter2); err != nil {
					return "notknown"
				}
				return existingResourceLimiter2.Status.State
			}, timeout, interval).Should(Equal(constants.Ready))
			existingResourceQuota2 := &corev1.ResourceQuota{}
			for _, tgt := range existingResourceLimiter2.Spec.Quotas {

				Eventually(func() bool {
					namespacedName := types.NamespacedName{Name: fmt.Sprintf("rl-quota-%s", tgt.NamespaceName), Namespace: tgt.NamespaceName}
					if err := k8sClient.Get(ctx, namespacedName, existingResourceQuota2); err != nil {
						return false
					}
					return true
				}, timeout, interval).Should(Equal(true))
				Expect(reflect.DeepEqual(existingResourceLimiter2.Status.Quotas, []rlv1beta2.ResourceLimiterQuota{
					rlv1beta2.ResourceLimiterQuota{
						NamespaceName: fmt.Sprintf("rl-quota-%s", tgt.NamespaceName),
						CpuRequest:    "0/250m",
						CpuLimit:      "0/500m",
						MemLimit:      "0/150Mi",
						MemRequest:    "0/120Mi",
					},
				})).Should(Equal(true))
			}

			By("By checking all the related quotas after createing the target pod")
			Expect(k8sClient.Create(ctx, podOk)).Should(Succeed())
			var existingPod1 corev1.Pod
			Eventually(func() string {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(podOk), &existingPod1); err != nil {
					return "notknown"
				}
				return string(existingPod1.Status.Phase)
			}, 2*timeout, interval).Should(Equal("Running"))
			existingResourceLimiter3 := &rlv1beta2.ResourceLimiter{}
			Eventually(func() string {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(existingResourceLimiter2), existingResourceLimiter3); err != nil {
					return "notknown"
				}
				return existingResourceLimiter3.Status.State
			}, timeout, interval).Should(Equal(constants.Ready))

			Expect(reflect.DeepEqual(existingResourceLimiter3.Status.Quotas, []rlv1beta2.ResourceLimiterQuota{
				rlv1beta2.ResourceLimiterQuota{
					NamespaceName: "rl-quota-default",
					CpuRequest:    "100m/250m",
					CpuLimit:      "200m/500m",
					MemLimit:      "100Mi/150Mi",
					MemRequest:    "90Mi/120Mi",
				},
			})).Should(Equal(true))

			/*By("By checking all the related quotas after updating the new rls")
			updatedrl := existingResourceLimiter3.DeepCopy()
			updatedrl.Spec.Types[constants.RetrainTypeLimitsCpu] = "600m"
			updatedrl.Spec.Types[constants.RetrainTypeRequestsCpu] = "350m"
			updatedrl.Spec.Types[constants.RetrainTypeLimitsMemory] = "160Mi"
			updatedrl.Spec.Types[constants.RetrainTypeRequestsMemory] = "130Mi"
			Expect(k8sClient.Update(ctx, updatedrl)).Should(Succeed())
			var existingResourceLimiter4 rlv1beta2.ResourceLimiter
			Eventually(func() string {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(updatedrl), &existingResourceLimiter4); err != nil {
					return "notknown"
				}
				return existingResourceLimiter4.Status.State
			}, timeout, interval).Should(Equal(constants.Ready))
			fmt.Println(existingResourceLimiter4)
			Expect(reflect.DeepEqual(existingResourceLimiter4.Status.Quotas["rl-default-0"], rlv1beta2.ResourceLimiterQuotas{
				Namespace:   "default",
				CpuRequests: "100m/350m",
				CpuLimits:   "200m/600m",
				MemLimits:   "100Mi/160Mi",
				MemRequests: "90Mi/130Mi",
			})).Should(Equal(true))*/
			By("By checking the quotas after deleting the pod")
			Eventually(func() bool {
				if err := k8sClient.Delete(ctx, &existingPod1); err != nil {
					return apierrors.IsNotFound(err)
				}
				return false
			}, 5*timeout, interval).Should(Equal(true))
			var existingResourceLimiter5 rlv1beta2.ResourceLimiter
			Eventually(func() string {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(existingResourceLimiter3), &existingResourceLimiter5); err != nil {
					return "notknown"
				}
				return existingResourceLimiter5.Status.State
			}, timeout, interval).Should(Equal(constants.Ready))
			Expect(reflect.DeepEqual(existingResourceLimiter3.Status.Quotas, []rlv1beta2.ResourceLimiterQuota{
				rlv1beta2.ResourceLimiterQuota{
					NamespaceName: "rl-quota-default",
					CpuRequest:    "0/250m",
					CpuLimit:      "0/500m",
					MemLimit:      "0/150Mi",
					MemRequest:    "0/120Mi",
				},
			})).Should(Equal(true))

		})
	})
})
