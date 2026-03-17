package ui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func makeTestIngress(name, ns string, annotations map[string]string) *networkingv1.Ingress {
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			Annotations: annotations,
		},
	}
}

// ── handleIndex ───────────────────────────────────────────────────────────────

func TestHandleIndex_ServesHTML(t *testing.T) {
	s := newTestSrv("v1.2.3")
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	s.handleIndex(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, w.Body.String(), "v1.2.3")
	assert.NotContains(t, w.Body.String(), "__APP_VERSION__")
}

func TestHandleIndex_NotFound(t *testing.T) {
	s := newTestSrv("v1.0.0")
	r := httptest.NewRequest(http.MethodGet, "/other-path", nil)
	w := httptest.NewRecorder()
	s.handleIndex(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── handleIngresses ───────────────────────────────────────────────────────────

func TestHandleIngresses_ReturnsJSON(t *testing.T) {
	ing := makeTestIngress("my-app", "default", map[string]string{
		"kubernetes.io/ingress.class": "kong",
	})
	s := newTestSrv("v1.0.0", ing)
	r := httptest.NewRequest(http.MethodGet, "/api/ingresses", nil)
	w := httptest.NewRecorder()
	s.handleIngresses(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	var analyses []interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &analyses))
	assert.NotEmpty(t, analyses)
}

func TestHandleIngresses_MethodNotAllowed(t *testing.T) {
	s := newTestSrv("v1.0.0")
	r := httptest.NewRequest(http.MethodPost, "/api/ingresses", nil)
	w := httptest.NewRecorder()
	s.handleIngresses(w, r)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHandleIngresses_EmptyNamespace(t *testing.T) {
	s := newTestSrv("v1.0.0")
	r := httptest.NewRequest(http.MethodGet, "/api/ingresses", nil)
	w := httptest.NewRecorder()
	s.handleIngresses(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "[]")
}

// ── handleIngressDownload ─────────────────────────────────────────────────────

func TestHandleIngressDownload_KongFormat(t *testing.T) {
	ing := makeTestIngress("test-ing", "default", map[string]string{
		"nginx.ingress.kubernetes.io/backend-protocol": "HTTPS",
	})
	s := newTestSrv("v1.0.0", ing)
	r := httptest.NewRequest(http.MethodGet, "/api/ingress/default/test-ing/download?format=kong-ingress", nil)
	w := httptest.NewRecorder()
	s.handleIngressDownload(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "yaml")
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
}

func TestHandleIngressDownload_GatewayAPI(t *testing.T) {
	ing := makeTestIngress("test-ing", "default", map[string]string{})
	s := newTestSrv("v1.0.0", ing)
	r := httptest.NewRequest(http.MethodGet, "/api/ingress/default/test-ing/download?format=gateway-api", nil)
	w := httptest.NewRecorder()
	s.handleIngressDownload(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "HTTPRoute")
}

func TestHandleIngressDownload_NotFound(t *testing.T) {
	s := newTestSrv("v1.0.0")
	r := httptest.NewRequest(http.MethodGet, "/api/ingress/default/nonexistent/download", nil)
	w := httptest.NewRecorder()
	s.handleIngressDownload(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleIngressDownload_BadPath(t *testing.T) {
	s := newTestSrv("v1.0.0")
	// Missing /download segment
	r := httptest.NewRequest(http.MethodGet, "/api/ingress/default/test-ing", nil)
	w := httptest.NewRecorder()
	s.handleIngressDownload(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── handleIngressRaw ──────────────────────────────────────────────────────────

func TestHandleIngressRaw_ValidIngress(t *testing.T) {
	ing := makeTestIngress("test-ing", "default", map[string]string{
		"nginx.ingress.kubernetes.io/limit-rps": "10",
	})
	s := newTestSrv("v1.0.0", ing)
	r := httptest.NewRequest(http.MethodGet, "/api/ingress-raw/default/test-ing", nil)
	w := httptest.NewRecorder()
	s.handleIngressRaw(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "yaml")
	assert.NotEmpty(t, w.Body.String())
}

func TestHandleIngressRaw_NotFound(t *testing.T) {
	s := newTestSrv("v1.0.0")
	r := httptest.NewRequest(http.MethodGet, "/api/ingress-raw/default/nonexistent", nil)
	w := httptest.NewRecorder()
	s.handleIngressRaw(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleIngressRaw_BadPath(t *testing.T) {
	s := newTestSrv("v1.0.0")
	// Single segment (no namespace/name split)
	r := httptest.NewRequest(http.MethodGet, "/api/ingress-raw/onlyone", nil)
	w := httptest.NewRecorder()
	s.handleIngressRaw(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── handleMigrate ─────────────────────────────────────────────────────────────

func TestHandleMigrate_Success(t *testing.T) {
	// Use annotation that doesn't create plugins (no dynamic client calls needed)
	ing := makeTestIngress("test-ing", "default", map[string]string{
		"nginx.ingress.kubernetes.io/backend-protocol": "HTTPS",
	})
	s := newTestSrv("v1.0.0", ing)

	body := `{"ingresses":[{"namespace":"default","name":"test-ing"}]}`
	r := httptest.NewRequest(http.MethodPost, "/api/migrate", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleMigrate(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var results []MigrateResult
	require.NoError(t, json.NewDecoder(w.Body).Decode(&results))
	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Empty(t, results[0].Error)
}

func TestHandleMigrate_GreenIngress(t *testing.T) {
	ing := makeTestIngress("green-ing", "default", map[string]string{
		"kubernetes.io/ingress.class": "kong",
	})
	s := newTestSrv("v1.0.0", ing)

	body := `{"ingresses":[{"namespace":"default","name":"green-ing"}]}`
	r := httptest.NewRequest(http.MethodPost, "/api/migrate", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleMigrate(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var results []MigrateResult
	require.NoError(t, json.NewDecoder(w.Body).Decode(&results))
	require.Len(t, results, 1)
	assert.False(t, results[0].Success)
	assert.NotEmpty(t, results[0].Error)
}

func TestHandleMigrate_MethodNotAllowed(t *testing.T) {
	s := newTestSrv("v1.0.0")
	r := httptest.NewRequest(http.MethodGet, "/api/migrate", nil)
	w := httptest.NewRecorder()
	s.handleMigrate(w, r)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHandleMigrate_InvalidBody(t *testing.T) {
	s := newTestSrv("v1.0.0")
	r := httptest.NewRequest(http.MethodPost, "/api/migrate", bytes.NewReader([]byte("{invalid")))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleMigrate(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── handleCopyToNamespace ─────────────────────────────────────────────────────

func TestHandleCopyToNamespace_Success(t *testing.T) {
	ing := makeTestIngress("test-ing", "default", map[string]string{
		"nginx.ingress.kubernetes.io/backend-protocol": "HTTPS",
	})
	s := newTestSrv("v1.0.0", ing)

	body := `{"ingresses":[{"namespace":"default","name":"test-ing"}],"targetNamespace":"new-ns"}`
	r := httptest.NewRequest(http.MethodPost, "/api/copy-to-namespace", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleCopyToNamespace(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var results []CopyToNsResult
	require.NoError(t, json.NewDecoder(w.Body).Decode(&results))
	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
}

func TestHandleCopyToNamespace_NoTargetNs(t *testing.T) {
	s := newTestSrv("v1.0.0")
	body := `{"ingresses":[{"namespace":"default","name":"test"}]}`
	r := httptest.NewRequest(http.MethodPost, "/api/copy-to-namespace", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleCopyToNamespace(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCopyToNamespace_MethodNotAllowed(t *testing.T) {
	s := newTestSrv("v1.0.0")
	r := httptest.NewRequest(http.MethodGet, "/api/copy-to-namespace", nil)
	w := httptest.NewRecorder()
	s.handleCopyToNamespace(w, r)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}
