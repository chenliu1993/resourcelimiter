package main

import (
	"encoding/json"
	"net/http"
	"reflect"

	rlv1beta1 "github.com/chenliu1993/resourcelimiter/api/v1beta1"
	rlv1beta2 "github.com/chenliu1993/resourcelimiter/api/v1beta2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("ResourceLimiter Webhooks", func() {
	Context("Mutate Webhook Check", func() {
		mockWebhookServer := WebhookServer{
			server: &http.Server{},
		}
		It("Should mutate into the desired content", func() {
			appliedResourceLimiterWithFalseQuantity := rlv1beta2.ResourceLimiter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-empty-format-quantity",
				},
				Spec: rlv1beta2.ResourceLimiterSpec{
					Applied: true,
					Quotas: []rlv1beta2.ResourceLimiterQuota{
						{
							NamespaceName: "default",
							CpuRequest:    "",
							CpuLimit:      "100m",
							MemLimit:      "200Mi",
							MemRequest:    "100Mi",
						},
					},
				},
			}
			desiredResourceLimiter := rlv1beta2.ResourceLimiter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-empty-format-quantity",
				},
				Spec: rlv1beta2.ResourceLimiterSpec{
					Applied: true,
					Quotas: []rlv1beta2.ResourceLimiterQuota{
						{
							NamespaceName: "default",
							CpuRequest:    "1",
							CpuLimit:      "2",
							MemLimit:      "200Mi",
							MemRequest:    "150Mi",
						},
					},
				},
			}

			output, err := json.Marshal(appliedResourceLimiterWithFalseQuantity)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(output)).NotTo(Equal(0))

			ar := admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Kind: metav1.GroupVersionKind{
						Version: "v1beta2",
					},
					Object: runtime.RawExtension{
						Raw: output,
					},
				},
			}
			response := mockWebhookServer.mutate(&ar)
			Expect(response.Allowed).To(Equal(true))
			Expect(len(response.Patch)).NotTo(Equal(0))

			// Apply and get the target
			err = k8sClient.Create(ctx, &appliedResourceLimiterWithFalseQuantity)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Patch(ctx, &appliedResourceLimiterWithFalseQuantity, client.RawPatch(types.JSONPatchType, response.Patch))
			Expect(err).NotTo(HaveOccurred())

			patchedResourceLimiter := &rlv1beta2.ResourceLimiter{}
			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&appliedResourceLimiterWithFalseQuantity), patchedResourceLimiter)
			Expect(err).NotTo(HaveOccurred())
			Expect(reflect.DeepEqual(patchedResourceLimiter.Spec.Quotas, desiredResourceLimiter.Spec.Quotas)).To(Equal(true))
		})
	})
	Context("Validate Webhook Check", func() {
		mockWebhookServer := WebhookServer{
			server: &http.Server{},
		}
		It("Should validate the right ResourceLimiter v1beta2", func() {
			appliedResourceLimiterWithFalseQuantity := rlv1beta2.ResourceLimiter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-wrong-format-quantity",
				},
				Spec: rlv1beta2.ResourceLimiterSpec{
					Applied: true,
					Quotas: []rlv1beta2.ResourceLimiterQuota{
						{
							NamespaceName: "default",
							CpuRequest:    "1cpu",
							CpuLimit:      "100m",
							MemLimit:      "200Mi",
							MemRequest:    "100Mi",
						},
					},
				},
			}

			output, err := json.Marshal(appliedResourceLimiterWithFalseQuantity)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(output)).NotTo(Equal(0))

			ar := admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Kind: metav1.GroupVersionKind{
						Kind:    "ResourceLimiter",
						Version: "v1beta2",
					},
					Object: runtime.RawExtension{
						Raw: output,
					},
				},
			}
			_ = mockWebhookServer.validate(&ar)
			// Expect(response.Allowed).To(Equal(false))
			Expect(resFormErr).To(HaveOccurred())
		})

		It("Should validate the right ResourceLimiter v1beta1", func() {
			appliedResourceLimiterWithFalseQuantity := rlv1beta1.ResourceLimiter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-wrong-format-quantity",
				},
				Spec: rlv1beta1.ResourceLimiterSpec{
					Applied: true,
					Types: map[rlv1beta1.ResourceLimiterType]string{
						"cpu_limits":   "200m",
						"cpu_requests": "100m",
						"mem_requests": " 150Giga",
						"mem_limits":   "200Mi",
					},
					Targets: []rlv1beta1.ResourceLimiterNamespace{
						"default",
					},
				},
			}

			output, err := json.Marshal(appliedResourceLimiterWithFalseQuantity)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(output)).NotTo(Equal(0))

			ar := admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Kind: metav1.GroupVersionKind{
						Kind:    "ResourceLimiter",
						Version: "v1beta1",
					},
					Object: runtime.RawExtension{
						Raw: output,
					},
				},
			}
			_ = mockWebhookServer.validate(&ar)
			Expect(resFormErr).To(HaveOccurred())
		})

		It("Should validate the right pod", func() {
			appliedPod := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-no-resources",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:      "test-without-resources",
							Resources: corev1.ResourceRequirements{},
						},
					},
				},
			}
			output, err := json.Marshal(appliedPod)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(output)).NotTo(Equal(0))

			ar := admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Kind: metav1.GroupVersionKind{
						Kind: "Pod",
					},
					Object: runtime.RawExtension{
						Raw: output,
					},
				},
			}
			response := mockWebhookServer.validate(&ar)
			Expect(response.Allowed).To(Equal(false))
		})
		It("Should validate the right deployment", func() {
			appliedDeployment := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-wrong-deployment",
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"resources": "no",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"resources": "no",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:      "test-without-resources",
									Resources: corev1.ResourceRequirements{},
								},
							},
						},
					},
				},
			}

			output, err := json.Marshal(appliedDeployment)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(output)).NotTo(Equal(0))

			ar := admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Kind: metav1.GroupVersionKind{
						Kind: "Deployment",
					},
					Object: runtime.RawExtension{
						Raw: output,
					},
				},
			}
			response := mockWebhookServer.validate(&ar)
			Expect(response.Allowed).To(Equal(false))
		})

		It("Should validate the right daemonset", func() {
			appliedDaemonset := appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-wrong-daemonset",
				},
				Spec: appsv1.DaemonSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"resources": "no",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"resources": "no",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:      "test-without-resources",
									Resources: corev1.ResourceRequirements{},
								},
							},
						},
					},
				},
			}

			output, err := json.Marshal(appliedDaemonset)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(output)).NotTo(Equal(0))

			ar := admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Kind: metav1.GroupVersionKind{
						Kind: "Daemonset",
					},
					Object: runtime.RawExtension{
						Raw: output,
					},
				},
			}
			response := mockWebhookServer.validate(&ar)
			Expect(response.Allowed).To(Equal(false))
		})
	})
})
