package v1beta2

import "github.com/chenliu1993/resourcelimiter/api/v1beta1"

// Hub marks this type as a conversion hub.
func (*ResourceLimiter) Hub() {}

// Convert reversely for backward compatibility
func (*ResourceLimiter) ConvertTo(v1beta1.ResourceLimiter)
