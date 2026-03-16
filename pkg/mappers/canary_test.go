package mappers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nginx-kong-migrator/pkg/generator"
)

func TestMapCanary_WithWeight(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/canary":        "true",
		"nginx.ingress.kubernetes.io/canary-weight": "20",
	})
	var plugins []generator.KongPlugin
	mapCanary(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, "canary", plugins[0].Plugin)
	assert.Equal(t, 20, plugins[0].Config["percentage"])
}

func TestMapCanary_ByHeader(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/canary":           "true",
		"nginx.ingress.kubernetes.io/canary-by-header": "X-Canary",
	})
	var plugins []generator.KongPlugin
	mapCanary(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, "header", plugins[0].Config["hash"])
	assert.Equal(t, "X-Canary", plugins[0].Config["hash_header"])
}

func TestMapCanary_Absent(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{})
	var plugins []generator.KongPlugin
	mapCanary(ing, &plugins)
	assert.Empty(t, plugins)
}
