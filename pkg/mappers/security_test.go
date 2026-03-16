package mappers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nginx-kong-migrator/pkg/generator"
)

func TestMapIPRestriction_SingleCIDR(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/whitelist-source-range": "10.0.0.0/8",
	})
	var plugins []generator.KongPlugin
	mapIPRestriction(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, "ip-restriction", plugins[0].Plugin)
	allow, ok := plugins[0].Config["allow"].([]string)
	require.True(t, ok)
	assert.Equal(t, []string{"10.0.0.0/8"}, allow)
	_, hasNginx := ing.Annotations["nginx.ingress.kubernetes.io/whitelist-source-range"]
	assert.False(t, hasNginx)
}

func TestMapIPRestriction_MultiCIDR(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/whitelist-source-range": "10.0.0.0/8, 192.168.0.0/16",
	})
	var plugins []generator.KongPlugin
	mapIPRestriction(ing, &plugins)
	require.Len(t, plugins, 1)
	allow, ok := plugins[0].Config["allow"].([]string)
	require.True(t, ok)
	require.Len(t, allow, 2)
	assert.Equal(t, "10.0.0.0/8", allow[0])
	assert.Equal(t, "192.168.0.0/16", allow[1])
}

func TestMapIPRestriction_Absent(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{})
	var plugins []generator.KongPlugin
	mapIPRestriction(ing, &plugins)
	assert.Empty(t, plugins)
}

func TestMapCORS_Enabled(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/enable-cors": "true",
	})
	var plugins []generator.KongPlugin
	mapCORS(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, "cors", plugins[0].Plugin)
	origins, ok := plugins[0].Config["origins"].([]string)
	require.True(t, ok)
	assert.Equal(t, []string{"*"}, origins)
}

func TestMapCORS_CustomOrigins(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/enable-cors":       "true",
		"nginx.ingress.kubernetes.io/cors-allow-origin": "https://a.com, https://b.com",
	})
	var plugins []generator.KongPlugin
	mapCORS(ing, &plugins)
	require.Len(t, plugins, 1)
	origins, ok := plugins[0].Config["origins"].([]string)
	require.True(t, ok)
	require.Len(t, origins, 2)
	assert.Equal(t, "https://a.com", origins[0])
	assert.Equal(t, "https://b.com", origins[1])
}

func TestMapCORS_Methods(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/enable-cors":        "true",
		"nginx.ingress.kubernetes.io/cors-allow-methods": "get,post",
	})
	var plugins []generator.KongPlugin
	mapCORS(ing, &plugins)
	require.Len(t, plugins, 1)
	methods, ok := plugins[0].Config["methods"].([]string)
	require.True(t, ok)
	assert.Contains(t, methods, "GET")
	assert.Contains(t, methods, "POST")
}

func TestMapCORS_Credentials(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/enable-cors":            "true",
		"nginx.ingress.kubernetes.io/cors-allow-credentials": "true",
	})
	var plugins []generator.KongPlugin
	mapCORS(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, true, plugins[0].Config["credentials"])
}

func TestMapCORS_MaxAge(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/enable-cors":  "true",
		"nginx.ingress.kubernetes.io/cors-max-age": "3600",
	})
	var plugins []generator.KongPlugin
	mapCORS(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, 3600, plugins[0].Config["max_age"])
}

func TestMapCORS_Disabled(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/enable-cors": "false",
	})
	var plugins []generator.KongPlugin
	mapCORS(ing, &plugins)
	assert.Empty(t, plugins)
	// Note: when enable-cors="false", the annotation itself is NOT removed by the
	// current implementation (only sub-keys are cleaned up). The enableKey remains.
}

func TestMapCORS_Absent(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{})
	var plugins []generator.KongPlugin
	mapCORS(ing, &plugins)
	assert.Empty(t, plugins)
}

func TestMapCORS_AnnotationsRemoved(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/enable-cors":            "true",
		"nginx.ingress.kubernetes.io/cors-allow-origin":      "https://a.com",
		"nginx.ingress.kubernetes.io/cors-allow-methods":     "get",
		"nginx.ingress.kubernetes.io/cors-allow-headers":     "Authorization",
		"nginx.ingress.kubernetes.io/cors-allow-credentials": "true",
		"nginx.ingress.kubernetes.io/cors-max-age":           "3600",
	})
	var plugins []generator.KongPlugin
	mapCORS(ing, &plugins)
	for _, key := range []string{
		"nginx.ingress.kubernetes.io/enable-cors",
		"nginx.ingress.kubernetes.io/cors-allow-origin",
		"nginx.ingress.kubernetes.io/cors-allow-methods",
		"nginx.ingress.kubernetes.io/cors-allow-headers",
		"nginx.ingress.kubernetes.io/cors-allow-credentials",
		"nginx.ingress.kubernetes.io/cors-max-age",
	} {
		_, has := ing.Annotations[key]
		assert.False(t, has, "annotation %s should be removed", key)
	}
}

// TestMapSecurity_IPRestrictionCalledTwice documents that mapSecurity calls
// mapIPRestriction twice (existing code behavior). Because the annotation is
// removed on the first call, the second call is a no-op and only one plugin
// is created.
func TestMapSecurity_IPRestrictionCalledTwice(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/whitelist-source-range": "10.0.0.0/8",
	})
	var plugins []generator.KongPlugin
	mapSecurity(ing, &plugins)
	// The annotation is removed on first call; second call is a no-op.
	// Exactly 1 ip-restriction plugin + possibly 0 cors plugin = 1 total.
	ipRestrictionCount := 0
	for _, p := range plugins {
		if p.Plugin == "ip-restriction" {
			ipRestrictionCount++
		}
	}
	assert.Equal(t, 1, ipRestrictionCount)
}
