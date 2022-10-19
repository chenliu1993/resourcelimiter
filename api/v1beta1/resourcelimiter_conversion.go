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
	dst.Spec.Quotas = make(map[string]v1beta2.ResourceLimiterQuota, len(src.Spec.Targets))
	for _, ns := range src.Spec.Targets {
		newQuota := v1beta2.ResourceLimiterQuota{
			CpuRequest: src.Spec.Types[ResourceLimiterType("requests.cpu")],
			CpuLimit:   src.Spec.Types[ResourceLimiterType("limits.cpu")],
			MemRequest: src.Spec.Types[ResourceLimiterType("requests.memory")],
			MemLimit:   src.Spec.Types[ResourceLimiterType("limits.memory")],
		}
		dst.Spec.Quotas[string(ns)] = newQuota
	}
	dst.Spec.Applied = src.Spec.Applied
	return nil
}
