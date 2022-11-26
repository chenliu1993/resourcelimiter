package main

import (
	"encoding/json"
	"fmt"
	"reflect"

	rlv1beta1 "github.com/chenliu1993/resourcelimiter/api/v1beta1"
	rlv1beta2 "github.com/chenliu1993/resourcelimiter/api/v1beta2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("Tests for conversion webhooks", func() {
	Context("Convert v1beta1 into v1beta2", func() {
		It("Should convert into v1beta2 successfully", func() {
			inputResourceLimiterv1beta1 := rlv1beta1.ResourceLimiter{
				TypeMeta: metav1.TypeMeta{
					Kind: "ResourceLimiter",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "resourcelimiter-v1beta1",
				},
				Spec: rlv1beta1.ResourceLimiterSpec{
					Applied: true,
					Targets: []rlv1beta1.ResourceLimiterNamespace{
						"default",
						"fixtures",
					},
					Types: map[rlv1beta1.ResourceLimiterType]string{
						"cpu_limits":   "200m",
						"cpu_requests": "150m",
						"mem_limits":   "1000Mi",
						"mem_requests": "200Mi",
					},
				},
			}

			outputResourceLimiterV1beta2 := rlv1beta2.ResourceLimiter{
				TypeMeta: metav1.TypeMeta{
					Kind: "ResourceLimiter",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "resourcelimiter-v1beta1",
				},
				Spec: rlv1beta2.ResourceLimiterSpec{
					Applied: true,
					Quotas: []rlv1beta2.ResourceLimiterQuota{
						{
							NamespaceName: "default",
							CpuRequest:    "150m",
							CpuLimit:      "200m",
							MemLimit:      "1000Mi",
							MemRequest:    "200Mi",
						},
						{
							NamespaceName: "fixtures",
							CpuRequest:    "150m",
							CpuLimit:      "200m",
							MemLimit:      "1000Mi",
							MemRequest:    "200Mi",
						},
					},
				},
			}

			output, err := json.Marshal(inputResourceLimiterv1beta1)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(output)).NotTo(Equal(0))

			fmt.Println(string(output))

			cr := &v1beta1.ConversionReview{
				TypeMeta: metav1.TypeMeta{
					Kind: "v1beta1",
				},
				Request: &v1beta1.ConversionRequest{
					DesiredAPIVersion: "resources.resourcelimiter.io/v1beta2",
					Objects: []runtime.RawExtension{
						{
							Raw: output,
						},
					},
				},
			}

			response := doConversion(cr.Request)
			Expect(response.Result.Status).To(ContainSubstring("Success"))

			Expect(len(response.ConvertedObjects)).To(Equal(1))

			Expect(reflect.DeepEqual(response.ConvertedObjects[0].Object.(*rlv1beta2.ResourceLimiter), outputResourceLimiterV1beta2))
		})

		It("Should convert into v1beta1 successfully", func() {
			inputResourceLimiterv1beta2 := rlv1beta2.ResourceLimiter{
				TypeMeta: metav1.TypeMeta{
					Kind: "ResourceLimiter",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "resourcelimiter-v1beta2",
				},
				Spec: rlv1beta2.ResourceLimiterSpec{
					Applied: true,
					Quotas: []rlv1beta2.ResourceLimiterQuota{
						{
							NamespaceName: "default",
							CpuRequest:    "150m",
							CpuLimit:      "200m",
							MemLimit:      "1000Mi",
							MemRequest:    "200Mi",
						},
						{
							NamespaceName: "fixtures",
							CpuRequest:    "150m",
							CpuLimit:      "200m",
							MemLimit:      "1000Mi",
							MemRequest:    "200Mi",
						},
					},
				},
			}

			outputResourceLimiterV1beta1 := rlv1beta1.ResourceLimiter{
				TypeMeta: metav1.TypeMeta{
					Kind: "ResourceLimiter",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "resourcelimiter-v1beta2",
				},
				Spec: rlv1beta1.ResourceLimiterSpec{
					Applied: true,
					Targets: []rlv1beta1.ResourceLimiterNamespace{
						"default",
						"fixtures",
					},
					Types: map[rlv1beta1.ResourceLimiterType]string{
						"cpu_limits":   "200m",
						"cpu_requests": "150m",
						"mem_limits":   "1000Mi",
						"mem_requests": "200Mi",
					},
				},
			}

			output, err := json.Marshal(inputResourceLimiterv1beta2)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(output)).NotTo(Equal(0))

			fmt.Println(string(output))

			cr := &v1beta1.ConversionReview{
				TypeMeta: metav1.TypeMeta{
					Kind: "v1beta1",
				},
				Request: &v1beta1.ConversionRequest{
					DesiredAPIVersion: "resources.resourcelimiter.io/v1beta1",
					Objects: []runtime.RawExtension{
						{
							Raw: output,
						},
					},
				},
			}

			response := doConversion(cr.Request)
			Expect(response.Result.Status).To(ContainSubstring("Success"))

			Expect(len(response.ConvertedObjects)).To(Equal(1))

			Expect(reflect.DeepEqual(response.ConvertedObjects[0].Object.(*rlv1beta1.ResourceLimiter), outputResourceLimiterV1beta1))
		})
	})
})
