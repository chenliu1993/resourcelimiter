package v1beta1

import (
	"errors"

	"sigs.k8s.io/controller-runtime/pkg/conversion"

	"github.com/chenliu1993/resourcelimiter/api/v1beta2"
)

func (src *ResourceLimiter) ConvertTo(dstRaw conversion.Hub) error {
	dst, ok := dstRaw.(*v1beta2.ResourceLimiter)
	if !ok {
		return errors.New("the dst type is wroong")
	}
	dst.Spec.Quotas = make([]v1beta2.ResourceLimiterQuota, len(src.Spec.Targets))
	for _, ns := range src.Spec.Targets {
		newQuota := v1beta2.ResourceLimiterQuota{
			NamespaceName: string(ns),
			CpuRequest:    src.Spec.Types[ResourceLimiterType("requests.cpu")],
			CpuLimit:      src.Spec.Types[ResourceLimiterType("limits.cpu")],
			MemRequest:    src.Spec.Types[ResourceLimiterType("requests.memory")],
			MemLimit:      src.Spec.Types[ResourceLimiterType("limits.memory")],
		}
		dst.Spec.Quotas = append(dst.Spec.Quotas, newQuota)
	}
	dst.Spec.Applied = src.Spec.Applied
	dst.ObjectMeta.Name = src.ObjectMeta.Name
	return nil
}

// Convert reversely for backward compatibility
func (dst *ResourceLimiter) ConvertFrom(srcRaw conversion.Hub) error {
	src, ok := srcRaw.(*v1beta2.ResourceLimiter)
	if !ok {
		return errors.New("the dst type is wroong")
	}
	dst.Spec.Targets = []ResourceLimiterNamespace{}
	dst.Spec.Types = map[ResourceLimiterType]string{}

	dst.Spec.Applied = src.Spec.Applied

	// Reply on map key will never be duplicated
	// And make the last set one as served quotas
	if len(src.Spec.Quotas) == 0 {
		return errors.New("the quotas field is 0")
	}

	for _, v := range src.Spec.Quotas {
		dst.Spec.Targets = append(dst.Spec.Targets, ResourceLimiterNamespace(v.NamespaceName))
		dst.Spec.Types[ResourceLimiterType("limits.cpu")] = v.CpuLimit
		dst.Spec.Types[ResourceLimiterType("requests.cpu")] = v.CpuRequest
		dst.Spec.Types[ResourceLimiterType("limits.memory")] = v.MemLimit
		dst.Spec.Types[ResourceLimiterType("requests.memory")] = v.MemRequest
	}

	return nil
}
