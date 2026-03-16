package mappers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nginx-kong-migrator/pkg/generator"
)

func TestMapMTLS_Required(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/auth-tls-verify-client": "on",
	})
	var plugins []generator.KongPlugin
	MapMTLS(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, "mtls-auth", plugins[0].Plugin)
	_, hasSkip := plugins[0].Config["skip_consumer_lookup"]
	assert.False(t, hasSkip, "required mTLS should not set skip_consumer_lookup")
	_, hasNginx := ing.Annotations["nginx.ingress.kubernetes.io/auth-tls-verify-client"]
	assert.False(t, hasNginx)
}

func TestMapMTLS_Optional(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/auth-tls-verify-client": "optional",
	})
	var plugins []generator.KongPlugin
	MapMTLS(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, true, plugins[0].Config["skip_consumer_lookup"])
}

func TestMapMTLS_WithDepth(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/auth-tls-verify-client": "on",
		"nginx.ingress.kubernetes.io/auth-tls-verify-depth":  "3",
	})
	var plugins []generator.KongPlugin
	MapMTLS(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, "3", plugins[0].Config["verify_depth"])
}

func TestMapMTLS_Absent(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{})
	var plugins []generator.KongPlugin
	MapMTLS(ing, &plugins)
	assert.Empty(t, plugins)
}
