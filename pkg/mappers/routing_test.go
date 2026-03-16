package mappers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapRewrite_Simple(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/rewrite-target": "/foo",
	})
	mapRewrite(ing)
	assert.Equal(t, "/foo", ing.Annotations["konghq.com/rewrite"])
	_, hasNginx := ing.Annotations["nginx.ingress.kubernetes.io/rewrite-target"]
	assert.False(t, hasNginx)
}

func TestMapRewrite_RegexCapture(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/rewrite-target": "/$1",
	})
	mapRewrite(ing)
	assert.Equal(t, "/$1", ing.Annotations["konghq.com/rewrite"])
}

func TestMapRewrite_Absent(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{})
	mapRewrite(ing)
	_, has := ing.Annotations["konghq.com/rewrite"]
	assert.False(t, has)
}

func TestMapProtocol_HTTPS(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/backend-protocol": "HTTPS",
	})
	mapProtocol(ing)
	assert.Equal(t, "HTTPS", ing.Annotations["konghq.com/protocol"])
	_, hasNginx := ing.Annotations["nginx.ingress.kubernetes.io/backend-protocol"]
	assert.False(t, hasNginx)
}

func TestMapProtocol_GRPC(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/backend-protocol": "grpc",
	})
	mapProtocol(ing)
	assert.Equal(t, "grpc", ing.Annotations["konghq.com/protocol"])
}

func TestMapProtocol_Absent(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{})
	mapProtocol(ing)
	_, has := ing.Annotations["konghq.com/protocol"]
	assert.False(t, has)
}

func TestMapSSLRedirect_ForceTrue(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/force-ssl-redirect": "true",
	})
	mapSSLRedirect(ing)
	assert.Equal(t, "301", ing.Annotations["konghq.com/https-redirect-status-code"])
	_, hasForce := ing.Annotations["nginx.ingress.kubernetes.io/force-ssl-redirect"]
	_, hasSSL := ing.Annotations["nginx.ingress.kubernetes.io/ssl-redirect"]
	assert.False(t, hasForce)
	assert.False(t, hasSSL)
}

func TestMapSSLRedirect_SSLRedirectTrue(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/ssl-redirect": "true",
	})
	mapSSLRedirect(ing)
	assert.Equal(t, "301", ing.Annotations["konghq.com/https-redirect-status-code"])
}

func TestMapSSLRedirect_False(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/ssl-redirect": "false",
	})
	mapSSLRedirect(ing)
	_, hasRedirect := ing.Annotations["konghq.com/https-redirect-status-code"]
	assert.False(t, hasRedirect)
	_, hasSSL := ing.Annotations["nginx.ingress.kubernetes.io/ssl-redirect"]
	assert.False(t, hasSSL)
}

func TestMapSSLRedirect_ForceTakesPriority(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/force-ssl-redirect": "true",
		"nginx.ingress.kubernetes.io/ssl-redirect":       "true",
	})
	mapSSLRedirect(ing)
	assert.Equal(t, "301", ing.Annotations["konghq.com/https-redirect-status-code"])
	// only one annotation set (not doubled)
	_, hasForce := ing.Annotations["nginx.ingress.kubernetes.io/force-ssl-redirect"]
	_, hasSSL := ing.Annotations["nginx.ingress.kubernetes.io/ssl-redirect"]
	assert.False(t, hasForce)
	assert.False(t, hasSSL)
}

func TestMapServiceUpstream(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/service-upstream": "true",
	})
	MapServiceUpstream(ing)
	assert.Equal(t, "true", ing.Annotations["konghq.com/service-upstream"])
	_, hasNginx := ing.Annotations["nginx.ingress.kubernetes.io/service-upstream"]
	assert.False(t, hasNginx)
}

func TestMapUseRegex_True(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/use-regex": "true",
	})
	MapUseRegex(ing)
	assert.Equal(t, "100", ing.Annotations["konghq.com/regex-priority"])
	_, hasNginx := ing.Annotations["nginx.ingress.kubernetes.io/use-regex"]
	assert.False(t, hasNginx)
}

func TestMapUseRegex_False(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/use-regex": "false",
	})
	MapUseRegex(ing)
	_, hasKong := ing.Annotations["konghq.com/regex-priority"]
	assert.False(t, hasKong)
	_, hasNginx := ing.Annotations["nginx.ingress.kubernetes.io/use-regex"]
	assert.False(t, hasNginx)
}

func TestMapPriority(t *testing.T) {
	ing := makeIngress("test", "default", map[string]string{
		"nginx.ingress.kubernetes.io/priority": "50",
	})
	MapPriority(ing)
	assert.Equal(t, "50", ing.Annotations["konghq.com/regex-priority"])
	_, hasNginx := ing.Annotations["nginx.ingress.kubernetes.io/priority"]
	assert.False(t, hasNginx)
}
