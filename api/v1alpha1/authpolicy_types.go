package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AuthPolicySpec defines the desired state of AuthPolicy.
//
// +kubebuilder:validation:XValidation:message="acceptedResources must be non-empty when using Ansattporten or ID-Porten",rule="!(self.wellKnownURI in ['https://test.idporten.no/.well-known/openid-configuration', 'https://idporten.no/.well-known/openid-configuration', 'https://test.ansattporten.no/.well-known/openid-configuration', 'https://ansattporten.no/.well-known/openid-configuration']) || (has(self.acceptedResources) && self.acceptedResources.size() > 0)"
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
	// +kubebuilder:validation:Optional
	OAuthCredentials *OAuthCredentials `json:"oAuthCredentials,omitempty"`

	// WellKnownURI specifies the URi to the identity provider's discovery document (also known as well-known endpoint).
	//
	// +kubebuilder:validation:Required
	WellKnownURI string `json:"wellKnownURI"`

	// Deprecated: use .allowedAudiences instead.
	// Audience defines the accepted audience (`aud`) values in the JWT.
	// At least one of the listed audience values must be present in the token's `aud` claim for validation to succeed.
	//
	// +kubebuilder:validation:Deprecated
	// +kubebuilder:validation:Optional
	Audience []string `json:"audience,omitempty"`

	// AllowedAudiences defines the allowed audience (`aud`) values in the JWT.
	// At least one of the listed audience values must be present in the token's `aud` claim for validation to succeed.
	//
	// The normative behaviour for an OAuth / OIDC-compliant identity provider is to validate the presense of one or more client IDs as allowed audiences.
	//
	// +kubebuilder:validation:Optional
	AllowedAudiences []AllowedAudience `json:"allowedAudiences,omitempty"`

	// If set to `true`, the original token will be kept for the upstream request. Defaults to `true`.
	//
	// +kubebuilder:validation:Optional
	ForwardJwt *bool `json:"forwardJwt,omitempty"`

	// OutputClaimsToHeaders specifies a list of operations to copy the claim to HTTP headers on a successfully verified token.
	// The header specified in each operation in the list must be unique. Nested claims of type string/int/bool is supported as well.
	// If the claim is an object or array, it will be added to the header as a base64-encoded JSON string.
	//
	// +kubebuilder:validation:Optional
	OutputClaimToHeaders *[]ClaimToHeader `json:"outputClaimToHeaders,omitempty"`

	// AcceptedResources specifies resource indicators used to request an audience limited access token following [RFC8707](https://datatracker.ietf.org/doc/html/rfc8707).
	// It defines accepted audience resource indicators in the JWT token.
	//
	// The resource indicators specified will be added to the initial authorize request towards the configured identity provider.
	// Each resource indicator must be a valid URI,
	// and the access token returned by the identity provider will set the resource indicators in the `aud` claim in the JWT token.
	// If none of the specified resource indicators is present in the `aud` claim in the JWT, the request will be denied.
	//
	// Please note that this alone is not sufficient to securely restrict access to a resource based on the `aud` claim.
	// Use .allowedAudiences to specify one or more allowed client IDs.
	//
	// +listType=set
	// +kubebuilder:validation:Items.Pattern=`^(https?):\/\/[^\s\/$.?#].[^\s]*$`
	// +kubebuilder:validation:Optional
	AcceptedResources *[]string `json:"acceptedResources,omitempty"`

	// BaselineAuth defines additional JWT authentication, beyond standard JWT verification.
	// Baseline authentication applies to all combinations of paths and methods not explicitly ignored by .ignoreAuthRules.
	//
	// +kubebuilder:validation:Optional
	BaselineAuth *BaselineAuth `json:"baselineAuth,omitempty"`

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

// AllowedAudience defines an audience that is validated against the `aud` claim in the JWT.
// An audience can be defined as a static value or retrieved from a kubernetes resource.
//
// +kubebuilder:validation:XValidation:message="either 'value' or 'valueFrom' must be set",rule="has(self.value) || has(self.valueFrom)"
// +kubebuilder:validation:XValidation:message="one audience cannot be defined from both 'value' and 'valueFrom'",rule="!(has(self.value) && has(self.valueFrom))"
// +kubebuilder:validation:XValidation:message="field 'value' cannot be empty string",rule="!has(self.value) || size(self.value) > 0"
// +kubebuilder:object:generate=true
type AllowedAudience struct {
	// Value specifies a static audience value.
	//
	// +kubebuilder:validation:Optional
	Value *string `json:"value,omitempty"`

	// ValueFrom specifies a reference to a kubernetes resource to retrieve the audience value from.
	//
	// +kubebuilder:validation:Optional
	ValueFrom *ValueFrom `json:"valueFrom,omitempty"`
}

// ValueFrom specifies a reference to a kubernetes resource to retrieve a value from.
//
// +kubebuilder:validation:XValidation:message="either 'configMapKeyRef' or 'secretKeyRef' must be set",rule="has(self.configMapKeyRef) || has(self.secretKeyRef)"
// +kubebuilder:validation:XValidation:message="cannot reference both a ConfigMap and a Secret",rule="!(has(self.configMapKeyRef) && has(self.secretKeyRef))"
// +kubebuilder:object:generate=true
type ValueFrom struct {
	// ConfigMapKeyRef specifies a reference to a key in a ConfigMap.
	//
	// +kubebuilder:validation:Optional
	ConfigMapKeyRef *KeyRef `json:"configMapKeyRef,omitempty"`

	// SecretKeyRef specifies a reference to a key in a Secret.
	//
	// +kubebuilder:validation:Optional
	SecretKeyRef *KeyRef `json:"secretKeyRef,omitempty"`
}

// KeyRef specifies a reference to a specific key within a kubernetes resource.
//
// +kubebuilder:object:generate=true
type KeyRef struct {
	// Name specifies the name of the ConfigMap/Secret; must satisfy DNS-1123 subdomain naming.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Key specifies the data entry name within the ConfigMap/Secret; must follow key naming rules.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?$`
	// +kubebuilder:validation:Required
	Key string `json:"key"`
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
	// +kubebuilder:validation:Optional
	LoginPath *string `json:"loginPath,omitempty"`

	// RedirectPath specifies which path to redirect the user to after completing the OIDC flow.
	// If omitted, a default path of /oauth2/callback is used.
	//
	// +kubebuilder:validation:Optional
	RedirectPath *string `json:"redirectPath,omitempty"`

	// LogoutPath specifies which URI to redirect the user to when signing out.
	// This will end the session for the application and also redirect the user
	// to log out towards the configured identity provider (RP-initiated logout).
	// If omitted, a default path of /logout is used.
	//
	// +kubebuilder:validation:Optional
	LogoutPath *string `json:"logoutPath,omitempty"`

	// PostLogoutRedirectURI specifies which URI to redirect the user to after
	// successfully signed out towards the configured identity provider (RP-initiated logout).
	// If omitted, no post_logout_redirect_uri will be used.
	//
	// +kubebuilder:validation:Optional
	PostLogoutRedirectURI *string `json:"postLogoutRedirectUri,omitempty"`

	// Scopes specifies the OAuth2 scopes used during authorization code flow.
	//
	// +kubebuilder:validation:Required
	Scopes []string `json:"scopes"`

	// LoginParams specifies a map of query parameters and their values which will be added in the authorize request made towards the configured identity provider.
	//
	// +kubebuilder:validation:Optional
	LoginParams map[string]string `json:"loginParams,omitempty"`
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
	// +kubebuilder:validation:Required
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
	// +kubebuilder:validation:Required
	Header string `json:"header"`

	// Claim specifies the name of the claim in the JWT token that will be copied to the header.
	//
	// +kubebuilder:validation:Pattern="^[a-zA-Z0-9-._]+$"
	// +kubebuilder:validation:MaxLength=128
	// +kubebuilder:validation:Required
	Claim string `json:"claim"`
}

// BaselineAuth defines additional JWT authentication, beyond standard JWT verification.
//
// +kubebuilder:object:generate=true
// +kubebuilder:validation:XValidation:message="claims must be a non-empty list",rule="has(self.claims) && self.claims.size() > 0"
type BaselineAuth struct {
	// Claims defines conditions based on JWT claims that must be met.
	// These conditions are applied to all paths and methods not explicitly ignored in .ignoreAuthRules,
	// including those covered by other specified AuthRules.
	//
	// The request is permitted if all the specified conditions are satisfied.
	// +kubebuilder:validation:Required
	Claims []Condition `json:"claims"`
}

// RequestAuthRule defines a rule for controlling access to HTTP requests using JWT authentication.
//
// +kubebuilder:object:generate=true
type RequestAuthRule struct {
	RequestMatcher `json:",inline"`

	// When defines additional conditions based on JWT claims that must be met.
	//
	// The request is permitted if all the specified conditions are satisfied.
	// +kubebuilder:validation:Optional
	When *[]Condition `json:"when,omitempty"`

	// DenyRedirect specifies whether a denied request should trigger auto-login (if configured) or not when it is denied due to missing or invalid authentication.
	// Defaults to false, meaning auto-login will be triggered (if configured).
	//
	// +kubebuilder:validation:Optional
	DenyRedirect *bool `json:"denyRedirect,omitempty"`
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
	// +kubebuilder:validation:Required
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
	// +kubebuilder:validation:Optional
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
	// +kubebuilder:validation:Required
	Claim string `json:"claim"`

	// Values specifies a list of allowed values for the claim.
	// If the claim in the JWT contains any of these values (OR logic), the condition is met.
	//
	// +listType=set
	// +kubebuilder:validation:Required
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
	matchers := ap.GetRequireAuthRequestMatchers()
	authorizedPaths := make([]string, 0, len(matchers))
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
