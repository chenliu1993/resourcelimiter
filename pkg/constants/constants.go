package constants

import (
	rlv1beta1 "github.com/chenliu1993/resourcelimiter/api/v1beta1"
)

const (
	// Guaranteed
	RestrainLevelHard = "hard"
	// Burstable 1/2
	RestrainLevelSoft = "soft"
	// BestEffort
	RestrainLevelNon = "no"
)

const (
	RetrainTypeCpu    rlv1beta1.ResourceLimiterType = "cpu"
	RetrainTypeMemory rlv1beta1.ResourceLimiterType = "memory"
	// RetrainTypeStorage rlv1beta1.ResourceLimiterType = "storage"
	// And maybe more...
)

const (
	IgnoreKubeSystem rlv1beta1.ResourceLimiterNamespace = "kube-system"
)
