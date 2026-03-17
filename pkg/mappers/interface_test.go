package mappers

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nginx-kong-migrator/pkg/generator"
)

func TestApply_Empty(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{})
	var plugins []generator.KongPlugin
	var kongIngresses []generator.KongIngress
	var upstreamPolicies []generator.KongUpstreamPolicy
	Apply(ing, &plugins, &kongIngresses, &upstreamPolicies)
	assert.Empty(t, plugins)
	assert.Empty(t, upstreamPolicies)
}

func TestApply_NilAnnotations(t *testing.T) {
	ing := makeIngress("test", "default", nil)
	var plugins []generator.KongPlugin
	var kongIngresses []generator.KongIngress
	var upstreamPolicies []generator.KongUpstreamPolicy
	// Should not panic
	assert.NotPanics(t, func() {
		Apply(ing, &plugins, &kongIngresses, &upstreamPolicies)
	})
	assert.Empty(t, plugins)
}

func TestApply_FullAnnotationSet(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/limit-rps":             "10",
		"nginx.ingress.kubernetes.io/enable-cors":           "true",
		"nginx.ingress.kubernetes.io/rewrite-target":        "/bar",
		"nginx.ingress.kubernetes.io/ssl-redirect":          "true",
		"nginx.ingress.kubernetes.io/proxy-connect-timeout": "30",
	})
	var plugins []generator.KongPlugin
	var kongIngresses []generator.KongIngress
	var upstreamPolicies []generator.KongUpstreamPolicy
	Apply(ing, &plugins, &kongIngresses, &upstreamPolicies)

	// Plugins: rate-limiting + cors
	pluginNames := make([]string, 0, len(plugins))
	for _, p := range plugins {
		pluginNames = append(pluginNames, p.Plugin)
	}
	assert.Contains(t, pluginNames, "rate-limiting")
	assert.Contains(t, pluginNames, "cors")

	// Kong annotations set
	assert.Equal(t, "/bar", ing.Annotations["konghq.com/rewrite"])
	assert.Equal(t, "301", ing.Annotations["konghq.com/https-redirect-status-code"])
	assert.Equal(t, "30000", ing.Annotations["konghq.com/connect-timeout"])

	// No nginx annotations remaining
	for key := range ing.Annotations {
		assert.False(t, strings.HasPrefix(key, "nginx.ingress.kubernetes.io/"),
			"nginx annotation %s should have been removed", key)
	}

	// Plugin ref on ingress
	pluginsRef := ing.Annotations["konghq.com/plugins"]
	assert.NotEmpty(t, pluginsRef)
}

func TestApply_IdempotentPluginRef(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{})
	addPluginReference(ing, "my-plugin")
	addPluginReference(ing, "my-plugin")
	assert.Equal(t, "my-plugin", ing.Annotations["konghq.com/plugins"],
		"plugin ref should appear only once")
}

func TestApply_NginxAnnotationsAllRemoved(t *testing.T) {
	// All annotations handled by mappers should be removed
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/backend-protocol":   "HTTPS",
		"nginx.ingress.kubernetes.io/proxy-body-size":    "5m",
		"nginx.ingress.kubernetes.io/proxy-buffer-size":  "4k",
		"nginx.ingress.kubernetes.io/server-snippet":     "# comment",
		"nginx.ingress.kubernetes.io/default-backend":    "my-svc",
		"nginx.ingress.kubernetes.io/http2-push-preload": "true",
		"nginx.ingress.kubernetes.io/proxy-http-version": "1.1",
	})
	var plugins []generator.KongPlugin
	var kongIngresses []generator.KongIngress
	var upstreamPolicies []generator.KongUpstreamPolicy
	Apply(ing, &plugins, &kongIngresses, &upstreamPolicies)

	for key := range ing.Annotations {
		assert.False(t, strings.HasPrefix(key, "nginx.ingress.kubernetes.io/"),
			"nginx annotation %s should have been removed after Apply", key)
	}
}

func TestApply_PluginsHaveCorrectNamespace(t *testing.T) {
	ing := makeIngress("myapp", "production", map[string]string{
		"nginx.ingress.kubernetes.io/limit-rps": "5",
	})
	var plugins []generator.KongPlugin
	var kongIngresses []generator.KongIngress
	var upstreamPolicies []generator.KongUpstreamPolicy
	Apply(ing, &plugins, &kongIngresses, &upstreamPolicies)
	require.Len(t, plugins, 1)
	assert.Equal(t, "production", plugins[0].Metadata.Namespace)
	assert.Equal(t, "myapp-rate-limiting", plugins[0].Metadata.Name)
}
