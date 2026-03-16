package mappers

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nginx-kong-migrator/pkg/generator"

	networkingv1 "k8s.io/api/networking/v1"
)

func TestMapAuth_BasicAuth(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/auth-type":   "basic",
		"nginx.ingress.kubernetes.io/auth-secret": "my-secret",
	})
	var plugins []generator.KongPlugin
	mapAuth(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, "basic-auth", plugins[0].Plugin)
	pluginsRef := ing.Annotations["konghq.com/plugins"]
	assert.True(t, strings.Contains(pluginsRef, "test-basic-auth"))
	_, hasType := ing.Annotations["nginx.ingress.kubernetes.io/auth-type"]
	_, hasSecret := ing.Annotations["nginx.ingress.kubernetes.io/auth-secret"]
	assert.False(t, hasType)
	assert.False(t, hasSecret)
}

func TestMapAuth_BasicAuthNoSecret(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/auth-type": "basic",
	})
	var plugins []generator.KongPlugin
	mapAuth(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, "basic-auth", plugins[0].Plugin)
}

func TestMapAuth_OIDC(t *testing.T) {
	ing := makeIngressWithRules("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/auth-signin": "https://auth.example.com/oauth2/start",
	}, []networkingv1.IngressRule{
		{Host: "example.com"},
	})
	var plugins []generator.KongPlugin
	mapAuth(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, "openid-connect", plugins[0].Plugin)
	_, hasSignin := ing.Annotations["nginx.ingress.kubernetes.io/auth-signin"]
	assert.False(t, hasSignin)
}

func TestMapAuth_OIDCUsesFirstHost(t *testing.T) {
	ing := makeIngressWithRules("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/auth-signin": "https://auth.example.com/oauth2/start",
	}, []networkingv1.IngressRule{
		{Host: "myapp.example.com"},
	})
	var plugins []generator.KongPlugin
	mapAuth(ing, &plugins)
	require.Len(t, plugins, 1)
	redirectURIs, ok := plugins[0].Config["redirect_uri"].([]string)
	require.True(t, ok)
	require.Len(t, redirectURIs, 1)
	assert.Contains(t, redirectURIs[0], "myapp.example.com")
}

func TestMapAuth_None(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{})
	var plugins []generator.KongPlugin
	mapAuth(ing, &plugins)
	assert.Empty(t, plugins)
}
