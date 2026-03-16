package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeIngress(name, ns string, annotations map[string]string) *networkingv1.Ingress {
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			Annotations: annotations,
		},
	}
}

func makeIngressRule(host, path string, pathType networkingv1.PathType, svcName string, svcPort int32) networkingv1.IngressRule {
	return networkingv1.IngressRule{
		Host: host,
		IngressRuleValue: networkingv1.IngressRuleValue{
			HTTP: &networkingv1.HTTPIngressRuleValue{
				Paths: []networkingv1.HTTPIngressPath{
					{
						Path:     path,
						PathType: &pathType,
						Backend: networkingv1.IngressBackend{
							Service: &networkingv1.IngressServiceBackend{
								Name: svcName,
								Port: networkingv1.ServiceBackendPort{Number: svcPort},
							},
						},
					},
				},
			},
		},
	}
}

func TestConvertToHTTPRoute_Basic(t *testing.T) {
	ing := makeIngress("my-app", "default", map[string]string{})
	ing.Spec.Rules = []networkingv1.IngressRule{
		makeIngressRule("example.com", "/", networkingv1.PathTypePrefix, "my-svc", 80),
	}
	route := ConvertToHTTPRoute(ing, "kong", "kong-system")
	assert.Equal(t, "gateway.networking.k8s.io/v1", route.APIVersion)
	assert.Equal(t, "HTTPRoute", route.Kind)
	assert.Equal(t, "my-app", route.Metadata.Name)
	require.Len(t, route.Spec.ParentRefs, 1)
	assert.Equal(t, "kong", route.Spec.ParentRefs[0].Name)
	assert.Contains(t, route.Spec.Hostnames, "example.com")
	require.Len(t, route.Spec.Rules, 1)
	require.Len(t, route.Spec.Rules[0].BackendRefs, 1)
	assert.Equal(t, "my-svc", route.Spec.Rules[0].BackendRefs[0].Name)
}

func TestConvertToHTTPRoute_MultiHosts(t *testing.T) {
	ing := makeIngress("multi", "default", map[string]string{})
	ing.Spec.Rules = []networkingv1.IngressRule{
		makeIngressRule("app1.example.com", "/", networkingv1.PathTypePrefix, "svc1", 80),
		makeIngressRule("app2.example.com", "/", networkingv1.PathTypePrefix, "svc2", 80),
		makeIngressRule("app1.example.com", "/api", networkingv1.PathTypePrefix, "svc1", 80),
	}
	route := ConvertToHTTPRoute(ing, "kong", "kong-system")
	assert.Len(t, route.Spec.Hostnames, 2)
}

func TestConvertToHTTPRoute_WithRewrite(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/rewrite-target": "/bar",
	})
	ing.Spec.Rules = []networkingv1.IngressRule{
		makeIngressRule("example.com", "/foo", networkingv1.PathTypePrefix, "svc", 80),
	}
	route := ConvertToHTTPRoute(ing, "kong", "kong-system")
	require.Len(t, route.Spec.Rules, 1)
	filters := route.Spec.Rules[0].Filters
	require.NotEmpty(t, filters)
	found := false
	for _, f := range filters {
		if f.Type == "URLRewrite" {
			found = true
			require.NotNil(t, f.URLRewrite)
			require.NotNil(t, f.URLRewrite.Path)
			assert.Equal(t, "ReplaceFullPath", f.URLRewrite.Path.Type)
			require.NotNil(t, f.URLRewrite.Path.ReplaceFullPath)
			assert.Equal(t, "/bar", *f.URLRewrite.Path.ReplaceFullPath)
		}
	}
	assert.True(t, found, "URLRewrite filter should be present")
}

func TestConvertToHTTPRoute_WithCORS_NoOrigins(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/enable-cors": "true",
	})
	ing.Spec.Rules = []networkingv1.IngressRule{
		makeIngressRule("example.com", "/", networkingv1.PathTypePrefix, "svc", 80),
	}
	route := ConvertToHTTPRoute(ing, "kong", "kong-system")
	require.Len(t, route.Spec.Rules, 1)
	found := false
	for _, f := range route.Spec.Rules[0].Filters {
		if f.Type == "ResponseHeaderModifier" {
			found = true
			require.NotNil(t, f.ResponseHeaderModifier)
			// When no origin specified, defaults to "*"
			for _, h := range f.ResponseHeaderModifier.Add {
				if h.Name == "Access-Control-Allow-Origin" {
					assert.Equal(t, "*", h.Value)
				}
			}
		}
	}
	assert.True(t, found, "ResponseHeaderModifier filter should be present")
}

func TestConvertToHTTPRoute_WithCORS_Origins(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/enable-cors":       "true",
		"nginx.ingress.kubernetes.io/cors-allow-origin": "https://a.com",
	})
	ing.Spec.Rules = []networkingv1.IngressRule{
		makeIngressRule("example.com", "/", networkingv1.PathTypePrefix, "svc", 80),
	}
	route := ConvertToHTTPRoute(ing, "kong", "kong-system")
	for _, f := range route.Spec.Rules[0].Filters {
		if f.Type == "ResponseHeaderModifier" {
			for _, h := range f.ResponseHeaderModifier.Add {
				if h.Name == "Access-Control-Allow-Origin" {
					assert.Equal(t, "https://a.com", h.Value)
				}
			}
		}
	}
}

func TestConvertToHTTPRoute_WithCanary(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/canary":        "true",
		"nginx.ingress.kubernetes.io/canary-weight": "30",
	})
	ing.Spec.Rules = []networkingv1.IngressRule{
		makeIngressRule("example.com", "/", networkingv1.PathTypePrefix, "my-svc", 80),
	}
	route := ConvertToHTTPRoute(ing, "kong", "kong-system")
	require.Len(t, route.Spec.Rules, 1)
	require.Len(t, route.Spec.Rules[0].BackendRefs, 2)
	// Check weights: primary=70, canary=30
	var primaryWeight, canaryWeight int32
	for _, ref := range route.Spec.Rules[0].BackendRefs {
		if ref.Name == "my-svc" {
			primaryWeight = *ref.Weight
		} else {
			canaryWeight = *ref.Weight
		}
	}
	assert.Equal(t, int32(70), primaryWeight)
	assert.Equal(t, int32(30), canaryWeight)
}

func TestConvertToHTTPRoute_PathTypeExact(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{})
	ing.Spec.Rules = []networkingv1.IngressRule{
		makeIngressRule("example.com", "/exact", networkingv1.PathTypeExact, "svc", 80),
	}
	route := ConvertToHTTPRoute(ing, "kong", "kong-system")
	require.Len(t, route.Spec.Rules, 1)
	require.NotNil(t, route.Spec.Rules[0].Matches[0].Path)
	assert.Equal(t, "Exact", route.Spec.Rules[0].Matches[0].Path.Type)
}

func TestConvertToHTTPRoute_PathTypePrefix(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{})
	ing.Spec.Rules = []networkingv1.IngressRule{
		makeIngressRule("example.com", "/prefix", networkingv1.PathTypePrefix, "svc", 80),
	}
	route := ConvertToHTTPRoute(ing, "kong", "kong-system")
	require.Len(t, route.Spec.Rules, 1)
	assert.Equal(t, "PathPrefix", route.Spec.Rules[0].Matches[0].Path.Type)
}

func TestConvertToHTTPRoute_EmptyPath(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{})
	pathType := networkingv1.PathTypePrefix
	ing.Spec.Rules = []networkingv1.IngressRule{
		{
			Host: "example.com",
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path:     "",
							PathType: &pathType,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: "svc",
									Port: networkingv1.ServiceBackendPort{Number: 80},
								},
							},
						},
					},
				},
			},
		},
	}
	route := ConvertToHTTPRoute(ing, "kong", "kong-system")
	require.Len(t, route.Spec.Rules, 1)
	path := route.Spec.Rules[0].Matches[0].Path
	require.NotNil(t, path)
	assert.Equal(t, "PathPrefix", path.Type)
	assert.Equal(t, "/", *path.Value)
}

func TestParseWeight_Valid(t *testing.T) {
	w, err := parseWeight("30")
	require.NoError(t, err)
	assert.Equal(t, int32(30), w)
}

func TestParseWeight_Zero(t *testing.T) {
	w, err := parseWeight("0")
	require.NoError(t, err)
	assert.Equal(t, int32(0), w)
}

func TestParseWeight_Over100(t *testing.T) {
	_, err := parseWeight("101")
	assert.Error(t, err)
}

func TestParseWeight_Negative(t *testing.T) {
	_, err := parseWeight("-1")
	assert.Error(t, err)
}

func TestParseWeight_NonNumeric(t *testing.T) {
	_, err := parseWeight("abc")
	assert.Error(t, err)
}

func TestConvertPathMatch_ExactType(t *testing.T) {
	pathType := networkingv1.PathTypeExact
	path := networkingv1.HTTPIngressPath{
		Path:     "/exact",
		PathType: &pathType,
	}
	match := convertPathMatch(path)
	assert.Equal(t, "Exact", match.Type)
	assert.Equal(t, "/exact", *match.Value)
}

func TestConvertPathMatch_PrefixType(t *testing.T) {
	pathType := networkingv1.PathTypePrefix
	path := networkingv1.HTTPIngressPath{
		Path:     "/prefix",
		PathType: &pathType,
	}
	match := convertPathMatch(path)
	assert.Equal(t, "PathPrefix", match.Type)
}

func TestConvertPathMatch_EmptyPath(t *testing.T) {
	pathType := networkingv1.PathTypePrefix
	path := networkingv1.HTTPIngressPath{
		Path:     "",
		PathType: &pathType,
	}
	match := convertPathMatch(path)
	assert.Equal(t, "PathPrefix", match.Type)
	require.NotNil(t, match.Value)
	assert.Equal(t, "/", *match.Value)
}
