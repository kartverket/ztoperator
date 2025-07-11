//go:build !ignore_autogenerated

/*
Copyright 2025.

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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AuthPolicy) DeepCopyInto(out *AuthPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AuthPolicy.
func (in *AuthPolicy) DeepCopy() *AuthPolicy {
	if in == nil {
		return nil
	}
	out := new(AuthPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AuthPolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AuthPolicyList) DeepCopyInto(out *AuthPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]AuthPolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AuthPolicyList.
func (in *AuthPolicyList) DeepCopy() *AuthPolicyList {
	if in == nil {
		return nil
	}
	out := new(AuthPolicyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AuthPolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AuthPolicySpec) DeepCopyInto(out *AuthPolicySpec) {
	*out = *in
	if in.AutoLogin != nil {
		in, out := &in.AutoLogin, &out.AutoLogin
		*out = new(AutoLogin)
		(*in).DeepCopyInto(*out)
	}
	if in.OAuthCredentials != nil {
		in, out := &in.OAuthCredentials, &out.OAuthCredentials
		*out = new(OAuthCredentials)
		**out = **in
	}
	if in.Audience != nil {
		in, out := &in.Audience, &out.Audience
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.ForwardJwt != nil {
		in, out := &in.ForwardJwt, &out.ForwardJwt
		*out = new(bool)
		**out = **in
	}
	if in.OutputClaimToHeaders != nil {
		in, out := &in.OutputClaimToHeaders, &out.OutputClaimToHeaders
		*out = new([]ClaimToHeader)
		if **in != nil {
			in, out := *in, *out
			*out = make([]ClaimToHeader, len(*in))
			copy(*out, *in)
		}
	}
	if in.AcceptedResources != nil {
		in, out := &in.AcceptedResources, &out.AcceptedResources
		*out = new([]string)
		if **in != nil {
			in, out := *in, *out
			*out = make([]string, len(*in))
			copy(*out, *in)
		}
	}
	if in.AuthRules != nil {
		in, out := &in.AuthRules, &out.AuthRules
		*out = new([]RequestAuthRule)
		if **in != nil {
			in, out := *in, *out
			*out = make([]RequestAuthRule, len(*in))
			for i := range *in {
				(*in)[i].DeepCopyInto(&(*out)[i])
			}
		}
	}
	if in.IgnoreAuthRules != nil {
		in, out := &in.IgnoreAuthRules, &out.IgnoreAuthRules
		*out = new([]RequestMatcher)
		if **in != nil {
			in, out := *in, *out
			*out = make([]RequestMatcher, len(*in))
			for i := range *in {
				(*in)[i].DeepCopyInto(&(*out)[i])
			}
		}
	}
	in.Selector.DeepCopyInto(&out.Selector)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AuthPolicySpec.
func (in *AuthPolicySpec) DeepCopy() *AuthPolicySpec {
	if in == nil {
		return nil
	}
	out := new(AuthPolicySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AuthPolicyStatus) DeepCopyInto(out *AuthPolicyStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AuthPolicyStatus.
func (in *AuthPolicyStatus) DeepCopy() *AuthPolicyStatus {
	if in == nil {
		return nil
	}
	out := new(AuthPolicyStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AutoLogin) DeepCopyInto(out *AutoLogin) {
	*out = *in
	if in.LoginPath != nil {
		in, out := &in.LoginPath, &out.LoginPath
		*out = new(string)
		**out = **in
	}
	if in.Scopes != nil {
		in, out := &in.Scopes, &out.Scopes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AutoLogin.
func (in *AutoLogin) DeepCopy() *AutoLogin {
	if in == nil {
		return nil
	}
	out := new(AutoLogin)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClaimToHeader) DeepCopyInto(out *ClaimToHeader) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClaimToHeader.
func (in *ClaimToHeader) DeepCopy() *ClaimToHeader {
	if in == nil {
		return nil
	}
	out := new(ClaimToHeader)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Condition) DeepCopyInto(out *Condition) {
	*out = *in
	if in.Values != nil {
		in, out := &in.Values, &out.Values
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Condition.
func (in *Condition) DeepCopy() *Condition {
	if in == nil {
		return nil
	}
	out := new(Condition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OAuthCredentials) DeepCopyInto(out *OAuthCredentials) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OAuthCredentials.
func (in *OAuthCredentials) DeepCopy() *OAuthCredentials {
	if in == nil {
		return nil
	}
	out := new(OAuthCredentials)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RequestAuthRule) DeepCopyInto(out *RequestAuthRule) {
	*out = *in
	in.RequestMatcher.DeepCopyInto(&out.RequestMatcher)
	if in.When != nil {
		in, out := &in.When, &out.When
		*out = new([]Condition)
		if **in != nil {
			in, out := *in, *out
			*out = make([]Condition, len(*in))
			for i := range *in {
				(*in)[i].DeepCopyInto(&(*out)[i])
			}
		}
	}
	if in.DenyRedirect != nil {
		in, out := &in.DenyRedirect, &out.DenyRedirect
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RequestAuthRule.
func (in *RequestAuthRule) DeepCopy() *RequestAuthRule {
	if in == nil {
		return nil
	}
	out := new(RequestAuthRule)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RequestMatcher) DeepCopyInto(out *RequestMatcher) {
	*out = *in
	if in.Paths != nil {
		in, out := &in.Paths, &out.Paths
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Methods != nil {
		in, out := &in.Methods, &out.Methods
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RequestMatcher.
func (in *RequestMatcher) DeepCopy() *RequestMatcher {
	if in == nil {
		return nil
	}
	out := new(RequestMatcher)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkloadSelector) DeepCopyInto(out *WorkloadSelector) {
	*out = *in
	if in.MatchLabels != nil {
		in, out := &in.MatchLabels, &out.MatchLabels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkloadSelector.
func (in *WorkloadSelector) DeepCopy() *WorkloadSelector {
	if in == nil {
		return nil
	}
	out := new(WorkloadSelector)
	in.DeepCopyInto(out)
	return out
}
