package mappers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nginx-kong-migrator/pkg/generator"
)

func TestMapCaching_Enabled(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/proxy-cache": "true",
	})
	var plugins []generator.KongPlugin
	mapCaching(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, "proxy-cache", plugins[0].Plugin)
	assert.Equal(t, "memory", plugins[0].Config["strategy"])
	assert.Equal(t, 300, plugins[0].Config["cache_ttl"])
	_, hasNginx := ing.Annotations["nginx.ingress.kubernetes.io/proxy-cache"]
	assert.False(t, hasNginx)
}

// TestMapCaching_CustomTTL documents that the current implementation always sets
// cache_ttl=300 regardless of the proxy-cache-valid value (MVP simplification).
func TestMapCaching_CustomTTL(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/proxy-cache":       "true",
		"nginx.ingress.kubernetes.io/proxy-cache-valid": "600s",
	})
	var plugins []generator.KongPlugin
	mapCaching(ing, &plugins)
	require.Len(t, plugins, 1)
	// MVP: TTL is always 300 regardless of proxy-cache-valid value
	assert.Equal(t, 300, plugins[0].Config["cache_ttl"])
	_, hasValid := ing.Annotations["nginx.ingress.kubernetes.io/proxy-cache-valid"]
	assert.False(t, hasValid)
}

func TestMapCaching_Absent(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{})
	var plugins []generator.KongPlugin
	mapCaching(ing, &plugins)
	assert.Empty(t, plugins)
}
