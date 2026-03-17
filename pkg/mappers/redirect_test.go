package mappers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nginx-kong-migrator/pkg/generator"
)

func TestMapPermanentRedirect(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/permanent-redirect": "https://new.example.com",
	})
	var plugins []generator.KongPlugin
	MapPermanentRedirect(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, "redirect", plugins[0].Plugin)
	assert.Equal(t, 301, plugins[0].Config["status_code"])
	assert.Equal(t, "https://new.example.com", plugins[0].Config["location"])
	_, hasNginx := ing.Annotations["nginx.ingress.kubernetes.io/permanent-redirect"]
	assert.False(t, hasNginx)
}

func TestMapTemporalRedirect(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/temporal-redirect": "https://tmp.example.com",
	})
	var plugins []generator.KongPlugin
	MapTemporalRedirect(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, "redirect", plugins[0].Plugin)
	assert.Equal(t, 302, plugins[0].Config["status_code"])
	assert.Equal(t, "https://tmp.example.com", plugins[0].Config["location"])
}

func TestMapBotDetection_Allow(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/bot-detection-allow": "GoogleBot",
	})
	var plugins []generator.KongPlugin
	MapBotDetection(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, "bot-detection", plugins[0].Plugin)
	allow, ok := plugins[0].Config["allow"].([]string)
	require.True(t, ok)
	assert.Contains(t, allow, "GoogleBot")
}

func TestMapBotDetection_Absent(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{})
	var plugins []generator.KongPlugin
	MapBotDetection(ing, &plugins)
	assert.Empty(t, plugins)
}
