package mappers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nginx-kong-migrator/pkg/generator"
)

func TestMapBodySize_Megabytes(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/proxy-body-size": "10m",
	})
	var plugins []generator.KongPlugin
	mapBodySize(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, "request-size-limiting", plugins[0].Plugin)
	assert.Equal(t, 10, plugins[0].Config["allowed_payload_size"])
	assert.Equal(t, "megabytes", plugins[0].Config["size_unit"])
	_, hasNginx := ing.Annotations["nginx.ingress.kubernetes.io/proxy-body-size"]
	assert.False(t, hasNginx)
}

func TestMapBodySize_Kilobytes(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/proxy-body-size": "512k",
	})
	var plugins []generator.KongPlugin
	mapBodySize(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, 512, plugins[0].Config["allowed_payload_size"])
	assert.Equal(t, "kilobytes", plugins[0].Config["size_unit"])
}

func TestMapBodySize_Zero(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/proxy-body-size": "0",
	})
	var plugins []generator.KongPlugin
	mapBodySize(ing, &plugins)
	// 0 with no unit → bytes, number=0, plugin is created
	require.Len(t, plugins, 1)
	assert.Equal(t, 0, plugins[0].Config["allowed_payload_size"])
	assert.Equal(t, "bytes", plugins[0].Config["size_unit"])
}

func TestMapClientMaxBodySize_Alias(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/client-max-body-size": "5m",
	})
	var plugins []generator.KongPlugin
	MapClientMaxBodySize(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, "request-size-limiting", plugins[0].Plugin)
	assert.Equal(t, 5, plugins[0].Config["allowed_payload_size"])
	_, hasNginx := ing.Annotations["nginx.ingress.kubernetes.io/client-max-body-size"]
	assert.False(t, hasNginx)
}
