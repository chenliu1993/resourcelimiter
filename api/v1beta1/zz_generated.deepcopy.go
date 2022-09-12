//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by controller-gen. DO NOT EDIT.

package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceLimiter) DeepCopyInto(out *ResourceLimiter) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceLimiter.
func (in *ResourceLimiter) DeepCopy() *ResourceLimiter {
	if in == nil {
		return nil
	}
	out := new(ResourceLimiter)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ResourceLimiter) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceLimiterList) DeepCopyInto(out *ResourceLimiterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ResourceLimiter, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceLimiterList.
func (in *ResourceLimiterList) DeepCopy() *ResourceLimiterList {
	if in == nil {
		return nil
	}
	out := new(ResourceLimiterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ResourceLimiterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceLimiterQuotas) DeepCopyInto(out *ResourceLimiterQuotas) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceLimiterQuotas.
func (in *ResourceLimiterQuotas) DeepCopy() *ResourceLimiterQuotas {
	if in == nil {
		return nil
	}
	out := new(ResourceLimiterQuotas)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceLimiterSpec) DeepCopyInto(out *ResourceLimiterSpec) {
	*out = *in
	if in.Targets != nil {
		in, out := &in.Targets, &out.Targets
		*out = make([]ResourceLimiterNamespace, len(*in))
		copy(*out, *in)
	}
	if in.Types != nil {
		in, out := &in.Types, &out.Types
		*out = make(map[ResourceLimiterType]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceLimiterSpec.
func (in *ResourceLimiterSpec) DeepCopy() *ResourceLimiterSpec {
	if in == nil {
		return nil
	}
	out := new(ResourceLimiterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceLimiterStatus) DeepCopyInto(out *ResourceLimiterStatus) {
	*out = *in
	if in.Quotas != nil {
		in, out := &in.Quotas, &out.Quotas
		*out = make(map[string]ResourceLimiterQuotas, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceLimiterStatus.
func (in *ResourceLimiterStatus) DeepCopy() *ResourceLimiterStatus {
	if in == nil {
		return nil
	}
	out := new(ResourceLimiterStatus)
	in.DeepCopyInto(out)
	return out
}
