package mappers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"nginx-kong-migrator/pkg/generator"
)

func TestMapTimeouts_Connect(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/proxy-connect-timeout": "30",
	})
	var plugins []generator.KongPlugin
	mapTimeouts(ing, &plugins)
	assert.Equal(t, "30000", ing.Annotations["konghq.com/connect-timeout"])
	_, hasNginx := ing.Annotations["nginx.ingress.kubernetes.io/proxy-connect-timeout"]
	assert.False(t, hasNginx)
}

func TestMapTimeouts_Read(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/proxy-read-timeout": "60",
	})
	var plugins []generator.KongPlugin
	mapTimeouts(ing, &plugins)
	assert.Equal(t, "60000", ing.Annotations["konghq.com/read-timeout"])
	_, hasNginx := ing.Annotations["nginx.ingress.kubernetes.io/proxy-read-timeout"]
	assert.False(t, hasNginx)
}

func TestMapTimeouts_Write(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/proxy-send-timeout": "120",
	})
	var plugins []generator.KongPlugin
	mapTimeouts(ing, &plugins)
	assert.Equal(t, "120000", ing.Annotations["konghq.com/write-timeout"])
	_, hasNginx := ing.Annotations["nginx.ingress.kubernetes.io/proxy-send-timeout"]
	assert.False(t, hasNginx)
}

func TestMapTimeouts_WithS(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/proxy-connect-timeout": "30s",
	})
	var plugins []generator.KongPlugin
	mapTimeouts(ing, &plugins)
	assert.Equal(t, "30000", ing.Annotations["konghq.com/connect-timeout"])
}

func TestMapTimeouts_InvalidValue(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/proxy-connect-timeout": "abc",
	})
	var plugins []generator.KongPlugin
	mapTimeouts(ing, &plugins)
	_, hasKong := ing.Annotations["konghq.com/connect-timeout"]
	assert.False(t, hasKong)
	// nginx key is NOT removed on parse failure
	_, hasNginx := ing.Annotations["nginx.ingress.kubernetes.io/proxy-connect-timeout"]
	assert.True(t, hasNginx)
}

func TestMapTimeouts_AllThree(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/proxy-connect-timeout": "10",
		"nginx.ingress.kubernetes.io/proxy-read-timeout":    "20",
		"nginx.ingress.kubernetes.io/proxy-send-timeout":    "30",
	})
	var plugins []generator.KongPlugin
	mapTimeouts(ing, &plugins)
	assert.Equal(t, "10000", ing.Annotations["konghq.com/connect-timeout"])
	assert.Equal(t, "20000", ing.Annotations["konghq.com/read-timeout"])
	assert.Equal(t, "30000", ing.Annotations["konghq.com/write-timeout"])
}

func TestMapTimeouts_Absent(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{})
	var plugins []generator.KongPlugin
	mapTimeouts(ing, &plugins)
	_, hasConnect := ing.Annotations["konghq.com/connect-timeout"]
	_, hasRead := ing.Annotations["konghq.com/read-timeout"]
	_, hasWrite := ing.Annotations["konghq.com/write-timeout"]
	assert.False(t, hasConnect)
	assert.False(t, hasRead)
	assert.False(t, hasWrite)
}
