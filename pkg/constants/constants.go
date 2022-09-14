package constants

import (
	rlv1beta1 "github.com/chenliu1993/resourcelimiter/api/v1beta1"
)

const (
	RetrainTypeLimitsCpu    rlv1beta1.ResourceLimiterType = "limits.cpu"
	RetrainTypeLimitsMemory rlv1beta1.ResourceLimiterType = "limits.memory"

	RetrainTypeRequestsCpu    rlv1beta1.ResourceLimiterType = "requests.cpu"
	RetrainTypeRequestsMemory rlv1beta1.ResourceLimiterType = "requests.memory"
	// RetrainTypeStorage rlv1beta1.ResourceLimiterType = "storage"
	// And maybe more...
)

const (
	IgnoreKubeSystem rlv1beta1.ResourceLimiterNamespace = "kube-system"
	IgnoreKubePublic rlv1beta1.ResourceLimiterNamespace = "kube-public"
)

const (
	DefaultFinalizer = "resourcelimiter.finalizer"
)

const (
	Ready = "ready"
	// Terminating = "terminating"
	Stopped = "stopped"
)

const (
	ResourceLimiterApiVersion = "resources.resourcelimiter.io/v1beta1"
	ResourceLimiterKind       = "ResourceLimiter"
)
