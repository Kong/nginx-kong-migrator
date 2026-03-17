package mappers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nginx-kong-migrator/pkg/generator"
)

func TestMapMaintenanceMode_Default(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/maintenance-mode": "true",
	})
	var plugins []generator.KongPlugin
	MapMaintenanceMode(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, "request-termination", plugins[0].Plugin)
	assert.Equal(t, 503, plugins[0].Config["status_code"])
	_, hasNginx := ing.Annotations["nginx.ingress.kubernetes.io/maintenance-mode"]
	assert.False(t, hasNginx)
}

func TestMapMaintenanceMode_CustomCode(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/maintenance-mode":        "true",
		"nginx.ingress.kubernetes.io/maintenance-status-code": "200",
	})
	var plugins []generator.KongPlugin
	MapMaintenanceMode(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, 200, plugins[0].Config["status_code"])
}

func TestMapMaintenanceMode_Absent(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{})
	var plugins []generator.KongPlugin
	MapMaintenanceMode(ing, &plugins)
	assert.Empty(t, plugins)
}

func TestMapDefaultBackendResponse(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/default-backend-status-code": "404",
		"nginx.ingress.kubernetes.io/default-backend-message":     "not found",
	})
	var plugins []generator.KongPlugin
	MapDefaultBackendResponse(ing, &plugins)
	require.Len(t, plugins, 1)
	assert.Equal(t, "request-termination", plugins[0].Plugin)
	assert.Equal(t, 404, plugins[0].Config["status_code"])
	assert.Equal(t, "not found", plugins[0].Config["message"])
}
