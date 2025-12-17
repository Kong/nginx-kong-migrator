package generator

// Gateway API resources
// Based on gateway.networking.k8s.io/v1

// HTTPRoute represents a Kubernetes Gateway API HTTPRoute resource
type HTTPRoute struct {
	APIVersion string        `json:"apiVersion"`
	Kind       string        `json:"kind"`
	Metadata   ObjectMeta    `json:"metadata"`
	Spec       HTTPRouteSpec `json:"spec"`
}

// HTTPRouteSpec defines the desired state of HTTPRoute
type HTTPRouteSpec struct {
	ParentRefs []ParentReference `json:"parentRefs,omitempty"`
	Hostnames  []string          `json:"hostnames,omitempty"`
	Rules      []HTTPRouteRule   `json:"rules,omitempty"`
}

// ParentReference identifies the parent Gateway
type ParentReference struct {
	Group     *string `json:"group,omitempty"`
	Kind      *string `json:"kind,omitempty"`
	Namespace *string `json:"namespace,omitempty"`
	Name      string  `json:"name"`
	Port      *int32  `json:"port,omitempty"`
}

// HTTPRouteRule defines routing rules
type HTTPRouteRule struct {
	Matches     []HTTPRouteMatch  `json:"matches,omitempty"`
	Filters     []HTTPRouteFilter `json:"filters,omitempty"`
	BackendRefs []HTTPBackendRef  `json:"backendRefs,omitempty"`
}

// HTTPRouteMatch defines request matching criteria
type HTTPRouteMatch struct {
	Path    *HTTPPathMatch    `json:"path,omitempty"`
	Headers []HTTPHeaderMatch `json:"headers,omitempty"`
	Method  *string           `json:"method,omitempty"`
}

// HTTPPathMatch defines path matching
type HTTPPathMatch struct {
	Type  string  `json:"type"` // Exact, PathPrefix, RegularExpression
	Value *string `json:"value,omitempty"`
}

// HTTPHeaderMatch defines header matching
type HTTPHeaderMatch struct {
	Type  string `json:"type"` // Exact, RegularExpression
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HTTPRouteFilter defines request/response filters
type HTTPRouteFilter struct {
	Type                   string                `json:"type"`
	RequestHeaderModifier  *HeaderModifier       `json:"requestHeaderModifier,omitempty"`
	ResponseHeaderModifier *HeaderModifier       `json:"responseHeaderModifier,omitempty"`
	RequestRedirect        *HTTPRequestRedirect  `json:"requestRedirect,omitempty"`
	URLRewrite             *HTTPURLRewriteFilter `json:"urlRewrite,omitempty"`
	ExtensionRef           *LocalObjectReference `json:"extensionRef,omitempty"`
}

// HeaderModifier defines header modifications
type HeaderModifier struct {
	Set    []HTTPHeader `json:"set,omitempty"`
	Add    []HTTPHeader `json:"add,omitempty"`
	Remove []string     `json:"remove,omitempty"`
}

// HTTPHeader represents an HTTP header
type HTTPHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HTTPRequestRedirect defines redirect behavior
type HTTPRequestRedirect struct {
	Scheme     *string `json:"scheme,omitempty"`
	Hostname   *string `json:"hostname,omitempty"`
	Port       *int32  `json:"port,omitempty"`
	StatusCode *int    `json:"statusCode,omitempty"`
}

// HTTPURLRewriteFilter defines URL rewriting
type HTTPURLRewriteFilter struct {
	Hostname *string           `json:"hostname,omitempty"`
	Path     *HTTPPathModifier `json:"path,omitempty"`
}

// HTTPPathModifier defines path modification
type HTTPPathModifier struct {
	Type               string  `json:"type"` // ReplaceFullPath, ReplacePrefixMatch
	ReplaceFullPath    *string `json:"replaceFullPath,omitempty"`
	ReplacePrefixMatch *string `json:"replacePrefixMatch,omitempty"`
}

// HTTPBackendRef defines backend service reference
type HTTPBackendRef struct {
	BackendRef `json:",inline"`
	Filters    []HTTPRouteFilter `json:"filters,omitempty"`
}

// BackendRef identifies a backend service
type BackendRef struct {
	Group     *string `json:"group,omitempty"`
	Kind      *string `json:"kind,omitempty"`
	Name      string  `json:"name"`
	Namespace *string `json:"namespace,omitempty"`
	Port      *int32  `json:"port,omitempty"`
	Weight    *int32  `json:"weight,omitempty"`
}

// LocalObjectReference identifies a local object
type LocalObjectReference struct {
	Group *string `json:"group,omitempty"`
	Kind  string  `json:"kind"`
	Name  string  `json:"name"`
}

// Gateway represents a Kubernetes Gateway API Gateway resource
type Gateway struct {
	APIVersion string      `json:"apiVersion"`
	Kind       string      `json:"kind"`
	Metadata   ObjectMeta  `json:"metadata"`
	Spec       GatewaySpec `json:"spec"`
}

// GatewaySpec defines the desired state of Gateway
type GatewaySpec struct {
	GatewayClassName string     `json:"gatewayClassName"`
	Listeners        []Listener `json:"listeners,omitempty"`
}

// Listener defines a listener on the Gateway
type Listener struct {
	Name     string  `json:"name"`
	Hostname *string `json:"hostname,omitempty"`
	Port     int32   `json:"port"`
	Protocol string  `json:"protocol"` // HTTP, HTTPS, TCP, TLS
	TLS      *TLS    `json:"tls,omitempty"`
}

// TLS defines TLS configuration for a listener
type TLS struct {
	Mode            *string           `json:"mode,omitempty"` // Terminate, Passthrough
	CertificateRefs []SecretReference `json:"certificateRefs,omitempty"`
}

// SecretReference identifies a Secret
type SecretReference struct {
	Group     *string `json:"group,omitempty"`
	Kind      *string `json:"kind,omitempty"`
	Name      string  `json:"name"`
	Namespace *string `json:"namespace,omitempty"`
}
