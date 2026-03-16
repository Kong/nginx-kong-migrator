package mappers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nginx-kong-migrator/pkg/generator"
)

func TestMapAffinity_Cookie(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/affinity": "cookie",
	})
	var plugins []generator.KongPlugin
	var kongIngresses []generator.KongIngress
	var upstreamPolicies []generator.KongUpstreamPolicy
	MapAffinity(ing, &plugins, &kongIngresses, &upstreamPolicies)
	require.Len(t, upstreamPolicies, 1)
	assert.Equal(t, "KongUpstreamPolicy", upstreamPolicies[0].Kind)
	spec, ok := upstreamPolicies[0].Spec["hashOn"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "route", spec["cookie"]) // default cookie name
	_, hasNginx := ing.Annotations["nginx.ingress.kubernetes.io/affinity"]
	assert.False(t, hasNginx)
}

func TestMapAffinity_WithCookieName(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/affinity":            "cookie",
		"nginx.ingress.kubernetes.io/session-cookie-name": "MY_SID",
	})
	var plugins []generator.KongPlugin
	var kongIngresses []generator.KongIngress
	var upstreamPolicies []generator.KongUpstreamPolicy
	MapAffinity(ing, &plugins, &kongIngresses, &upstreamPolicies)
	require.Len(t, upstreamPolicies, 1)
	spec, ok := upstreamPolicies[0].Spec["hashOn"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "MY_SID", spec["cookie"])
}

func TestMapAffinity_Absent(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{})
	var plugins []generator.KongPlugin
	var kongIngresses []generator.KongIngress
	var upstreamPolicies []generator.KongUpstreamPolicy
	MapAffinity(ing, &plugins, &kongIngresses, &upstreamPolicies)
	assert.Empty(t, upstreamPolicies)
}

func TestWarnBufferSettings_NoOp(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/proxy-buffer-size": "4k",
	})
	WarnBufferSettings(ing)
	_, has := ing.Annotations["nginx.ingress.kubernetes.io/proxy-buffer-size"]
	assert.False(t, has, "annotation should be removed")
}

func TestWarnSnippets_NoOp(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/server-snippet": "proxy_pass http://backend;",
	})
	WarnSnippets(ing)
	_, has := ing.Annotations["nginx.ingress.kubernetes.io/server-snippet"]
	assert.False(t, has, "snippet annotation should be removed")
}
