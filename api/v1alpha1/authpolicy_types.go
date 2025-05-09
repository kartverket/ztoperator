package v1alpha1

import (
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"istio.io/api/security/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AuthPolicySpec defines the desired state of AuthPolicy.
type AuthPolicySpec struct {
	// Rules specifies how incoming requests should be allowed or denied based on the presence and validation of accompanying JWTs.
	// +kubebuilder:validation:Required
	Rules []RequestAuth `json:"rules"`

	// The Selector specifies which workload the defined auth policy should be applied to.
	// +kubebuilder:validation:Required
	Selector WorkloadSelector `json:"selector"`
}

type WorkloadSelector struct {
	// One or more labels that indicate a specific set of pods/VMs
	// on which a policy should be applied. The scope of label search is restricted to
	// the configuration namespace in which the resource is present.
	// +kubebuilder:validation:XValidation:message="wildcard not allowed in label key match",rule="self.all(key, !key.contains('*'))"
	// +kubebuilder:validation:XValidation:message="key must not be empty",rule="self.all(key, key.size() != 0)"
	// +kubebuilder:validation:MaxProperties=4096
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

// RequestAuth specifies how incoming JWTs should be validated.
//
// +kubebuilder:object:generate=true
type RequestAuth struct {
	// Whether to enable JWT validation.
	// If enabled, incoming JWTs will be validated against the issuer specified in the app registration and the generated audience.
	//
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`

	// IssuerUri specifies the expected issuer (`iss`) claim in the JWT.
	// This should match the issuer identifier used when the token was generated.
	//
	// +kubebuilder:validation:Required
	IssuerUri string `json:"issuerURI"`

	// JwksUri is the URI where the JSON Web Key Set (JWKS) can be fetched.
	// It is used to validate the signature of incoming JWTs.
	//
	// +kubebuilder:validation:Required
	JwksUri string `json:"jwksURI"`

	// Audience defines the accepted audience (`aud`) values in the JWT.
	// At least one of the listed audience values must be present in the token's `aud` claim for validation to succeed.
	//
	// +kubebuilder:validation:Required
	Audience []string `json:"audience"`

	// If set to `true`, the original token will be kept for the upstream request. Defaults to `true`.
	// +kubebuilder:default=true
	ForwardJwt bool `json:"forwardJwt,omitempty"`

	// FromCookies denotes the cookies from which the auth policy will look for a JWT.
	//
	// +kubebuilder:validation:Optional
	FromCookies *[]string `json:"fromCookies,omitempty"`

	// OutputClaimsToHeaders specifies a list of operations to copy the claim to HTTP headers on a successfully verified token.
	// The header specified in each operation in the list must be unique. Nested claims of type string/int/bool is supported as well.
	// ```
	//
	//	outputClaimToHeaders:
	//	- header: x-my-company-jwt-group
	//	  claim: my-group
	//	- header: x-test-environment-flag
	//	  claim: test-flag
	//	- header: x-jwt-claim-group
	//	  claim: nested.key.group
	//
	// ```
	// +kubebuilder:validation:Optional
	OutputClaimToHeaders *[]ClaimToHeader `json:"outputClaimToHeaders,omitempty"`

	// AcceptedResources is used as a validation field following [RFC8707](https://datatracker.ietf.org/doc/html/rfc8707).
	// It defines accepted audience resource indicators in the JWT token.
	//
	// Each resource indicator must be a valid URI, and the indicator must be present as the `aud` claim in the JWT token.
	//
	// +kubebuilder:validation:Optional
	// +listType=set
	// +kubebuilder:validation:Items.Pattern=`^(https?):\/\/[^\s\/$.?#].[^\s]*$`
	AcceptedResources *[]string `json:"acceptedResources,omitempty"`

	// AuthRules defines rules for allowing HTTP requests based on conditions
	// that must be met based on JWT claims.
	//
	// API endpoints not covered by AuthRules and/or IgnoreAuthRules requires an authenticated JWT by default.
	//
	// +kubebuilder:validation:Optional
	AuthRules *[]RequestAuthRule `json:"authRules,omitempty"`

	// IgnoreAuthRules defines request matchers for HTTP requests that do not require JWT authentication.
	//
	// API endpoints not covered by AuthRules or IgnoreAuthRules require an authenticated JWT by default.
	//
	// +kubebuilder:validation:Optional
	IgnoreAuthRules *[]RequestMatcher `json:"ignoreAuthRules,omitempty"`
}

// ClaimToHeader specifies a list of operations to copy the claim to HTTP headers on a successfully verified token.
// The header specified in each operation in the list must be unique. Nested claims of type string/int/bool is supported as well.
//
// +kubebuilder:object:generate=true
type ClaimToHeader struct {
	// Header specifies the name of the HTTP header to which the claim value will be copied.
	//
	// +kubebuilder:validation:Pattern="^[a-zA-Z0-9-]+$"
	// +kubebuilder:validation:MaxLength=64
	Header string `json:"header"`

	// Claim specifies the name of the claim in the JWT token that will be copied to the header.
	//
	// +kubebuilder:validation:Pattern="^[a-zA-Z0-9-._]+$"
	// +kubebuilder:validation:MaxLength=128
	Claim string `json:"claim"`
}

// RequestAuthRule defines a rule for controlling access to HTTP requests using JWT authentication.
//
// +kubebuilder:object:generate=true
type RequestAuthRule struct {
	RequestMatcher `json:",inline"`

	// When defines additional conditions based on JWT claims that must be met.
	//
	// The request is permitted if at least one of the specified conditions is satisfied.
	When []Condition `json:"when"`
}

// RequestMatcher defines paths and methods to match incoming HTTP requests.
//
// +kubebuilder:object:generate=true
type RequestMatcher struct {
	// Paths specifies a set of URI paths that this rule applies to.
	// Each path must be a valid URI path, starting with '/' and not ending with '/'.
	// The wildcard '*' is allowed only at the end of the path.
	//
	// +listType=set
	// +kubebuilder:validation:Items.Pattern=`^/[a-zA-Z0-9\-._~!$&'()+,;=:@%/]*(\*)?$`
	Paths []string `json:"paths"`

	// Methods specifies HTTP methods that applies for the defined paths.
	// If omitted, all methods are permitted.
	//
	// Allowed methods:
	// - GET
	// - POST
	// - PUT
	// - PATCH
	// - DELETE
	// - HEAD
	// - OPTIONS
	// - TRACE
	// - CONNECT
	//
	// +listType=set
	// +kubebuilder:validation:Items:Enum=GET,POST,PUT,PATCH,DELETE,HEAD,OPTIONS,TRACE,CONNECT
	Methods []string `json:"methods,omitempty"`
}

// Condition represents a rule that evaluates JWT claims to determine access control.
//
// This type allows defining conditions that check whether a specific claim in
// the JWT token contains one of the expected values.
//
// If multiple conditions are specified, all must be met (AND logic) for the request to be allowed.
//
// +kubebuilder:object:generate=true
type Condition struct {
	// Claim specifies the name of the JWT claim to check.
	//
	Claim string `json:"claim"`

	// Values specifies a list of allowed values for the claim.
	// If the claim in the JWT contains any of these values (OR logic), the condition is met.
	//
	// +listType=set
	Values []string `json:"values"`
}

// AuthPolicyStatus defines the observed state of AuthPolicy
type AuthPolicyStatus struct {
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	Phase              Phase              `json:"phase,omitempty"`
	Message            string             `json:"message,omitempty"`
	Ready              bool               `json:"ready"`
}

type Phase string

const (
	PhasePending Phase = "Pending"
	PhaseReady   Phase = "Ready"
	PhaseFailed  Phase = "Failed"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`

// AuthPolicy is the Schema for the authpolicies API
type AuthPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AuthPolicySpec   `json:"spec,omitempty"`
	Status AuthPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AuthPolicyList contains a list of AuthPolicy
type AuthPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AuthPolicy `json:"items"`
}

var AcceptedHttpMethods = []string{
	"GET",
	"POST",
	"PUT",
	"PATCH",
	"DELETE",
	"HEAD",
	"OPTIONS",
	"TRACE",
	"CONNECT",
}

func init() {
	SchemeBuilder.Register(&AuthPolicy{}, &AuthPolicyList{})
}

func (ap *AuthPolicy) InitializeStatus() {
	if ap.Status.Conditions == nil {
		ap.Status.Conditions = []metav1.Condition{}
	}
	ap.Status.ObservedGeneration = ap.GetGeneration()
	ap.Status.Ready = false
	ap.Status.Phase = PhasePending
}

func (ap *AuthPolicy) GetRequireAuthRequestMatchers() []RequestMatcher {
	var requireAuthRequestMatchers []RequestMatcher
	for _, rule := range ap.Spec.Rules {
		if rule.AuthRules != nil {
			requireAuthRequestMatchers = append(requireAuthRequestMatchers, GetRequestMatchers(rule.AuthRules)...)
		}
	}
	return requireAuthRequestMatchers
}

func (ap *AuthPolicy) GetIgnoreAuthRequestMatchers() []RequestMatcher {
	var ignoreAuthRequestMatchers []RequestMatcher
	for _, rule := range ap.Spec.Rules {
		if rule.IgnoreAuthRules != nil {
			ignoreAuthRequestMatchers = append(ignoreAuthRequestMatchers, *rule.IgnoreAuthRules...)
		}
	}
	return ignoreAuthRequestMatchers
}

func GetRequestMatchers(requestAuthRules *[]RequestAuthRule) []RequestMatcher {
	var requestMatchers []RequestMatcher
	if requestAuthRules != nil {
		for _, authRule := range *requestAuthRules {
			requestMatchers = append(requestMatchers, authRule.RequestMatcher)
		}
	}
	return requestMatchers
}

func ResolveAuthPolicy(authPolicy *AuthPolicy) *AuthPolicy {
	//authPolicy.Spec.Rules = ignorePathsFromOtherRules(authPolicy.Spec.Rules)
	return authPolicy
}

func ignorePathsFromOtherRules(rules []RequestAuth) []RequestAuth {
	for index, jwtRule := range rules {
		if jwtRule.AuthRules == nil {
			var empty []RequestAuthRule
			jwtRule.AuthRules = &empty
		}
		if jwtRule.IgnoreAuthRules == nil {
			var empty []RequestMatcher
			jwtRule.IgnoreAuthRules = &empty
		}
		requireAuthRequestMatchers := GetRequestMatchers(jwtRule.AuthRules)
		ignoredRequestMatchers := flattenOnPaths(*jwtRule.IgnoreAuthRules)
		authorizedRequestMatchers := flattenOnPaths(requireAuthRequestMatchers)
		for otherIndex, otherJwtRule := range rules {
			if index != otherIndex {
				otherRequireAuthRequestMatchers := GetRequestMatchers(otherJwtRule.AuthRules)
				otherAuthorizedRequestMatchers := flattenOnPaths(otherRequireAuthRequestMatchers)
				for otherPath, otherRequestMapper := range otherAuthorizedRequestMatchers {
					if !slices.Contains(maps.Keys(ignoredRequestMatchers), otherPath) &&
						!slices.Contains(maps.Keys(authorizedRequestMatchers), otherPath) {
						*jwtRule.IgnoreAuthRules = append(*jwtRule.IgnoreAuthRules, RequestMatcher{
							Paths:   otherRequestMapper.Operation.Paths,
							Methods: otherRequestMapper.Operation.Methods,
						})
					}
				}
			}
		}
		rules[index] = jwtRule
	}
	return rules
}

func flattenOnPaths(requestMatchers []RequestMatcher) map[string]*v1beta1.Rule_To {
	requestMatchersMap := make(map[string]*v1beta1.Rule_To)
	if requestMatchers != nil {
		for _, requestMatcher := range requestMatchers {
			for _, path := range requestMatcher.Paths {
				if existingMatcher, exists := requestMatchersMap[path]; exists {
					// Combine methods if the path key already exists and remove duplicates
					uniqueMethods := make(map[string]struct{})
					for _, method := range append(existingMatcher.Operation.Methods, requestMatcher.Methods...) {
						uniqueMethods[method] = struct{}{}
					}
					existingMatcher.Operation.Methods = maps.Keys(uniqueMethods)
					requestMatchersMap[path] = existingMatcher
				} else {
					methods := requestMatcher.Methods
					if len(methods) == 0 {
						methods = AcceptedHttpMethods
					}
					requestMatchersMap[path] = &v1beta1.Rule_To{
						Operation: &v1beta1.Operation{
							Paths:   []string{path},
							Methods: methods,
						},
					}
				}
			}
		}
	}
	return requestMatchersMap
}
