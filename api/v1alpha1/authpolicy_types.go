package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AuthPolicySpec defines the desired state of AuthPolicy.
type AuthPolicySpec struct {
	// Whether to enable JWT validation.
	// If enabled, incoming JWTs will be validated against the issuer specified in the app registration and the generated audience.
	//
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`

	// AutoLogin specifies the required configuration needed to log in users.
	//
	// +kubebuilder:validation:Optional
	AutoLogin *AutoLogin `json:"autoLogin,omitempty"`

	// OAuthCredentials specifies a reference to a kubernetes secret in the same namespace holding OAuth credentials used for authentication.
	//
	// +kubebuilder:validation:Required
	OAuthCredentials *OAuthCredentials `json:"oAuthCredentials,omitempty"`

	// WellKnownURI specifies the URi to the identity provider's discovery document (also known as well-known endpoint).
	//
	// +kubebuilder:validation:Required
	WellKnownURI string `json:"wellKnownURI"`

	// Audience defines the accepted audience (`aud`) values in the JWT.
	// At least one of the listed audience values must be present in the token's `aud` claim for validation to succeed.
	//
	// +kubebuilder:validation:Optional
	Audience []string `json:"audience,omitempty"`

	// If set to `true`, the original token will be kept for the upstream request. Defaults to `true`.
	// +kubebuilder:default=true
	ForwardJwt bool `json:"forwardJwt,omitempty"`

	// OutputClaimsToHeaders specifies a list of operations to copy the claim to HTTP headers on a successfully verified token.
	// The header specified in each operation in the list must be unique. Nested claims of type string/int/bool is supported as well.
	//
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

	// The Selector specifies which workload the defined auth policy should be applied to.
	// +kubebuilder:validation:Required
	Selector WorkloadSelector `json:"selector"`
}

// AutoLogin specifies the required configuration needed to log in users.
//
// +kubebuilder:object:generate=true
type AutoLogin struct {
	// Whether to enable auto login.
	// If enabled, users accessing authenticated endpoints will be redirected to log in towards the configured identity provider.
	//
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`

	// LoginPath specifies a list of URI paths that should trigger the auto-login behavior.
	// When a request matches any of these paths, the user will be redirected to log in if not already authenticated.
	//
	// +kubebuilder:validation:Pattern=`^/.*$`
	LoginPath *string `json:"loginPath,omitempty"`

	// RedirectPath specifies which path to redirect the user to after completing the OIDC flow.
	//
	// +kubebuilder:validation:Required
	RedirectPath string `json:"redirectPath"`

	// LogoutPath specifies which URI to redirect the user to when signing out.
	// This will end the session for the application, but not end the session towards the configured identity provider.
	// This feature will hopefully soon be available in later releases of Istio, ref. [envoy/envoyproxy](https://github.com/envoyproxy/envoy/commit/c12fefc11f7adc9cd404287eb674ba838c2c8bd0).
	//
	// +kubebuilder:validation:Required
	LogoutPath string `json:"logoutPath"`

	// Scopes specifies the OAuth2 scopes used during authorization code flow.
	//
	// +kubebuilder:validation:Required
	Scopes []string `json:"scopes"`
}

// OAuthCredentials specifies the kubernetes secret holding OAuth credentials used for authentication.
//
// +kubebuilder:object:generate=true
type OAuthCredentials struct {
	// SecretRef specifies the name of the kubernetes secret.
	//
	// +kubebuilder:validation:Required
	SecretRef string `json:"secretRef"`

	// ClientSecretKey specifies the data key to access the client secret.
	//
	// +kubebuilder:validation:Required
	ClientSecretKey string `json:"clientSecretKey"`

	// ClientIDKey specifies the data key to access the client ID.
	//
	// +kubebuilder:validation:Required
	ClientIDKey string `json:"clientIDKey"`
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
	// Paths specify a set of URI paths that this rule applies to.
	// Each path must be a valid URI path, starting with '/' and not ending with '/'.
	//
	// +listType=set
	// +kubebuilder:validation:Items:Pattern=`^/.*$`
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

// AuthPolicyStatus defines the observed state of AuthPolicy.
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
	PhaseInvalid Phase = "Invalid"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`

// AuthPolicy is the Schema for the authpolicies API.
type AuthPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AuthPolicySpec   `json:"spec,omitempty"`
	Status AuthPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AuthPolicyList contains a list of AuthPolicy.
type AuthPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AuthPolicy `json:"items"`
}

func GetAcceptedHTTPMethods() []string {
	return []string{
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
	if ap.Spec.AuthRules != nil {
		requireAuthRequestMatchers = append(requireAuthRequestMatchers, GetRequestMatchers(ap.Spec.AuthRules)...)
	}
	return requireAuthRequestMatchers
}

func (ap *AuthPolicy) GetIgnoreAuthRequestMatchers() []RequestMatcher {
	var ignoreAuthRequestMatchers []RequestMatcher
	if ap.Spec.IgnoreAuthRules != nil {
		ignoreAuthRequestMatchers = append(ignoreAuthRequestMatchers, *ap.Spec.IgnoreAuthRules...)
	}
	return ignoreAuthRequestMatchers
}

func (ap *AuthPolicy) GetAuthorizedPaths() []string {
	var authorizedPaths []string
	for _, requestMatcher := range ap.GetRequireAuthRequestMatchers() {
		authorizedPaths = append(authorizedPaths, requestMatcher.Paths...)
	}
	return authorizedPaths
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

func (ap *AuthPolicy) GetPaths() []string {
	var paths []string
	if ap.Spec.AuthRules != nil {
		for _, authRule := range *ap.Spec.AuthRules {
			paths = append(paths, authRule.Paths...)
		}
	}
	if ap.Spec.IgnoreAuthRules != nil {
		for _, ignoreAuthRule := range *ap.Spec.IgnoreAuthRules {
			paths = append(paths, ignoreAuthRule.Paths...)
		}
	}
	return paths
}
