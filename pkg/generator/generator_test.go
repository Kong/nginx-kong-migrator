package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeTestIngress(name, ns string) networkingv1.Ingress {
	return networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}
}

func makeTestPlugin(name, ns, pluginType string) KongPlugin {
	return KongPlugin{
		Metadata: ObjectMeta{Name: name, Namespace: ns},
		Plugin:   pluginType,
		Config:   map[string]interface{}{"key": "value"},
	}
}

func TestWriteOutput_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "output.yaml")
	ing := makeTestIngress("test", "default")
	err := WriteOutput([]networkingv1.Ingress{ing}, nil, nil, nil, outFile)
	require.NoError(t, err)
	_, err = os.Stat(outFile)
	assert.NoError(t, err)
}

func TestWriteOutput_YAMLFormat(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "output.yaml")
	ing := makeTestIngress("test", "default")
	plugin := makeTestPlugin("test-rate-limiting", "default", "rate-limiting")
	err := WriteOutput([]networkingv1.Ingress{ing}, []KongPlugin{plugin}, nil, nil, outFile)
	require.NoError(t, err)

	data, err := os.ReadFile(outFile)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "---")
	assert.Contains(t, content, "KongPlugin")
	assert.Contains(t, content, "configuration.konghq.com/v1")
}

func TestWriteOutput_EmptyInputs(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "output.yaml")
	ing := makeTestIngress("empty", "default")
	err := WriteOutput([]networkingv1.Ingress{ing}, nil, nil, nil, outFile)
	require.NoError(t, err)

	data, err := os.ReadFile(outFile)
	require.NoError(t, err)
	// Should contain the ingress YAML but no KongPlugin
	content := string(data)
	assert.NotEmpty(t, content)
	assert.NotContains(t, content, "KongPlugin")
}

func TestWriteGatewayAPIOutput_Basic(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "gateway.yaml")
	ing := makeTestIngress("test", "default")
	err := WriteGatewayAPIOutput([]networkingv1.Ingress{ing}, nil, outFile, "kong", "kong-system")
	require.NoError(t, err)

	data, err := os.ReadFile(outFile)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "HTTPRoute")
	assert.Contains(t, content, "gateway.networking.k8s.io/v1")
}

func TestWriteGatewayAPIOutput_WithPlugins(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "gateway.yaml")
	ing := makeTestIngress("test", "default")
	plugin := makeTestPlugin("test-cors", "default", "cors")
	err := WriteGatewayAPIOutput([]networkingv1.Ingress{ing}, []KongPlugin{plugin}, outFile, "kong", "kong-system")
	require.NoError(t, err)

	data, err := os.ReadFile(outFile)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "HTTPRoute")
	assert.Contains(t, content, "KongPlugin")
}
