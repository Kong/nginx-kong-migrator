package mappers

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nginx-kong-migrator/pkg/generator"
)

func TestMapRateLimit_RPSOnly(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/limit-rps": "10",
	})
	var plugins []generator.KongPlugin
	mapRateLimit(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, "rate-limiting", plugins[0].Plugin)
	assert.Equal(t, 10, plugins[0].Config["second"])
	assert.Equal(t, "ip", plugins[0].Config["limit_by"])
	_, hasNginx := ing.Annotations["nginx.ingress.kubernetes.io/limit-rps"]
	assert.False(t, hasNginx)
}

func TestMapRateLimit_RPMOnly(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/limit-rpm": "100",
	})
	var plugins []generator.KongPlugin
	mapRateLimit(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, 100, plugins[0].Config["minute"])
	_, hasSecond := plugins[0].Config["second"]
	assert.False(t, hasSecond)
}

func TestMapRateLimit_Both(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/limit-rps": "5",
		"nginx.ingress.kubernetes.io/limit-rpm": "100",
	})
	var plugins []generator.KongPlugin
	mapRateLimit(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, 5, plugins[0].Config["second"])
	assert.Equal(t, 100, plugins[0].Config["minute"])
}

func TestMapRateLimit_NoAnnotations(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{})
	var plugins []generator.KongPlugin
	mapRateLimit(ing, &plugins)
	assert.Empty(t, plugins)
}

func TestMapRateLimit_InvalidValue(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/limit-rps": "abc",
	})
	var plugins []generator.KongPlugin
	mapRateLimit(ing, &plugins)
	assert.Empty(t, plugins)
}

func TestMapRateLimit_LimitByHeader(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/limit-rps":       "5",
		"nginx.ingress.kubernetes.io/limit-by":        "header",
		"nginx.ingress.kubernetes.io/limit-by-header": "X-User",
	})
	var plugins []generator.KongPlugin
	mapRateLimit(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, "header", plugins[0].Config["limit_by"])
	assert.Equal(t, "X-User", plugins[0].Config["header_name"])
}

func TestMapRateLimit_LimitByInvalidValue(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/limit-rps": "5",
		"nginx.ingress.kubernetes.io/limit-by":  "unknown",
	})
	var plugins []generator.KongPlugin
	mapRateLimit(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, "ip", plugins[0].Config["limit_by"])
}

func TestMapRateLimit_ErrorCode(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/limit-rps":        "5",
		"nginx.ingress.kubernetes.io/limit-error-code": "429",
	})
	var plugins []generator.KongPlugin
	mapRateLimit(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, 429, plugins[0].Config["error_code"])
}

func TestMapRateLimit_ErrorCodeOutOfRange(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/limit-rps":        "5",
		"nginx.ingress.kubernetes.io/limit-error-code": "999",
	})
	var plugins []generator.KongPlugin
	mapRateLimit(ing, &plugins)
	require.Len(t, plugins, 1)
	_, hasCode := plugins[0].Config["error_code"]
	assert.False(t, hasCode)
}

func TestMapRateLimit_ErrorMessage(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/limit-rps":           "5",
		"nginx.ingress.kubernetes.io/limit-error-message": "slow down",
	})
	var plugins []generator.KongPlugin
	mapRateLimit(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, "slow down", plugins[0].Config["error_message"])
}

func TestMapRateLimit_AnnotationsRemoved(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/limit-rps":           "5",
		"nginx.ingress.kubernetes.io/limit-rpm":           "100",
		"nginx.ingress.kubernetes.io/limit-by":            "ip",
		"nginx.ingress.kubernetes.io/limit-error-code":    "429",
		"nginx.ingress.kubernetes.io/limit-error-message": "slow down",
	})
	var plugins []generator.KongPlugin
	mapRateLimit(ing, &plugins)
	for _, key := range []string{
		"nginx.ingress.kubernetes.io/limit-rps",
		"nginx.ingress.kubernetes.io/limit-rpm",
		"nginx.ingress.kubernetes.io/limit-by",
		"nginx.ingress.kubernetes.io/limit-error-code",
		"nginx.ingress.kubernetes.io/limit-error-message",
	} {
		_, has := ing.Annotations[key]
		assert.False(t, has, "annotation %s should be removed", key)
	}
}

func TestMapRateLimit_PluginReference(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/limit-rps": "5",
	})
	var plugins []generator.KongPlugin
	mapRateLimit(ing, &plugins)
	require.Len(t, plugins, 1)
	pluginsRef := ing.Annotations["konghq.com/plugins"]
	assert.True(t, strings.Contains(pluginsRef, "test-rate-limiting"))
}
