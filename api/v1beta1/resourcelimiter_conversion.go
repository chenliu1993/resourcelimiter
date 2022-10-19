package v1beta1

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	"github.com/chenliu1993/resourcelimiter/api/v1beta2"
)

func (src *ResourceLimiter) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta2.ResourceLimiter)
	return nil
}
