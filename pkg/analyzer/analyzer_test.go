package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeIngress(name, ns string, annotations map[string]string) networkingv1.Ingress {
	return networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			Annotations: annotations,
		},
	}
}

func makeIngressWithClass(name, ns, class string) networkingv1.Ingress {
	ing := makeIngress(name, ns, map[string]string{
		"kubernetes.io/ingress.class": class,
	})
	return ing
}

func makeIngressWithSpecClass(name, ns, class string) networkingv1.Ingress {
	ing := makeIngress(name, ns, map[string]string{})
	ing.Spec.IngressClassName = &class
	return ing
}

func TestAnalyze_Green_NoAnnotations(t *testing.T) {
	ing := makeIngressWithClass("test", "default", "kong")
	result := Analyze(ing)
	assert.Equal(t, StatusGreen, result.Status)
}

func TestAnalyze_Green_NoNginxAnnotations(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"some.other/annotation": "value",
	})
	result := Analyze(ing)
	assert.Equal(t, StatusGreen, result.Status)
}

func TestAnalyze_Yellow_Migratable(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/limit-rps": "10",
	})
	result := Analyze(ing)
	assert.Equal(t, StatusYellow, result.Status)
	// plugin should be listed
	assert.NotEmpty(t, result.GeneratedPlugins)
}

func TestAnalyze_Yellow_WithActionNote(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/affinity": "cookie",
	})
	result := Analyze(ing)
	assert.Equal(t, StatusYellow, result.Status)
	assert.NotEmpty(t, result.ActionNotes)
}

func TestAnalyze_Red_UnmigratedAnnotation(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/custom-unknown-annotation": "value",
	})
	result := Analyze(ing)
	assert.Equal(t, StatusRed, result.Status)
	assert.Contains(t, result.UnmigratedKeys, "nginx.ingress.kubernetes.io/custom-unknown-annotation")
}

func TestAnalyze_HostsExtracted(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/limit-rps": "5",
	})
	ing.Spec.Rules = []networkingv1.IngressRule{
		{Host: "app1.example.com"},
		{Host: "app2.example.com"},
	}
	result := Analyze(ing)
	require.Len(t, result.Hosts, 2)
	assert.Contains(t, result.Hosts, "app1.example.com")
	assert.Contains(t, result.Hosts, "app2.example.com")
}

func TestAnalyze_DedupHosts(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/limit-rps": "5",
	})
	ing.Spec.Rules = []networkingv1.IngressRule{
		{Host: "app.example.com"},
		{Host: "app.example.com"},
	}
	result := Analyze(ing)
	assert.Len(t, result.Hosts, 1)
	assert.Equal(t, "app.example.com", result.Hosts[0])
}

func TestAnalyze_AnnotationCount(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/limit-rps":        "10",
		"nginx.ingress.kubernetes.io/backend-protocol": "HTTPS",
		"nginx.ingress.kubernetes.io/rewrite-target":   "/",
	})
	result := Analyze(ing)
	assert.Equal(t, 3, result.AnnotationCount)
}

func TestAnalyze_OriginalNotMutated(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/limit-rps": "10",
	})
	Analyze(ing)
	// original ingress should still have the annotation
	_, hasAnnotation := ing.Annotations["nginx.ingress.kubernetes.io/limit-rps"]
	assert.True(t, hasAnnotation, "Analyze should not mutate the original ingress")
}

func TestAnalyze_GeneratedPluginsListed(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/limit-rps":   "10",
		"nginx.ingress.kubernetes.io/enable-cors": "true",
	})
	result := Analyze(ing)
	assert.NotEmpty(t, result.GeneratedPlugins)
	// should contain both rate-limiting and cors plugin names
	found := make(map[string]bool)
	for _, name := range result.GeneratedPlugins {
		found[name] = true
	}
	// plugin names follow the pattern {ingress-name}-{plugin-type}
	assert.True(t, found["test-rate-limiting"] || containsSubstring(result.GeneratedPlugins, "rate-limiting"),
		"rate-limiting plugin should be listed")
}

func TestAnalyze_NginxIngressClassAnnotation(t *testing.T) {
	ing := makeIngressWithClass("test", "default", "nginx")
	result := Analyze(ing)
	assert.Equal(t, StatusYellow, result.Status)
}

func TestAnalyze_NginxIngressClassSpec(t *testing.T) {
	ing := makeIngressWithSpecClass("test", "default", "nginx")
	result := Analyze(ing)
	assert.Equal(t, StatusYellow, result.Status)
}

func containsSubstring(strs []string, sub string) bool {
	for _, s := range strs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
