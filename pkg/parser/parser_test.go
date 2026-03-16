package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFixture(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ingresses.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

const singleIngress = `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-ingress
  namespace: default
spec:
  rules:
  - host: example.com
`

const multiDocIngress = `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: first
  namespace: default
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: second
  namespace: default
`

const ingressWithConfigMap = `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
data:
  key: value
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: real-ingress
  namespace: default
`

func TestParseIngress_Single(t *testing.T) {
	path := writeFixture(t, singleIngress)
	ingresses, err := ParseIngress(path)
	require.NoError(t, err)
	require.Len(t, ingresses, 1)
	assert.Equal(t, "my-ingress", ingresses[0].Name)
}

func TestParseIngress_MultiDoc(t *testing.T) {
	path := writeFixture(t, multiDocIngress)
	ingresses, err := ParseIngress(path)
	require.NoError(t, err)
	require.Len(t, ingresses, 2)
	assert.Equal(t, "first", ingresses[0].Name)
	assert.Equal(t, "second", ingresses[1].Name)
}

func TestParseIngress_SkipsNonIngress(t *testing.T) {
	path := writeFixture(t, ingressWithConfigMap)
	ingresses, err := ParseIngress(path)
	require.NoError(t, err)
	require.Len(t, ingresses, 1)
	assert.Equal(t, "real-ingress", ingresses[0].Name)
}

func TestParseIngress_WrongAPIVersion(t *testing.T) {
	content := `apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: old-ingress
`
	path := writeFixture(t, content)
	ingresses, err := ParseIngress(path)
	require.NoError(t, err)
	assert.Empty(t, ingresses)
}

func TestParseIngress_EmptyFile(t *testing.T) {
	path := writeFixture(t, "")
	ingresses, err := ParseIngress(path)
	require.NoError(t, err)
	assert.Empty(t, ingresses)
}

func TestParseIngress_FileNotFound(t *testing.T) {
	_, err := ParseIngress("/nonexistent/path/file.yaml")
	assert.Error(t, err)
}

func TestParseIngress_InvalidYAMLDoc(t *testing.T) {
	content := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: valid
---
{invalid yaml: [unclosed
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: also-valid
`
	path := writeFixture(t, content)
	ingresses, err := ParseIngress(path)
	require.NoError(t, err)
	// valid docs should still be parsed; invalid doc is skipped
	assert.GreaterOrEqual(t, len(ingresses), 1)
}

func TestParseIngress_EmptyDocSeparators(t *testing.T) {
	content := "---\n---\n---\n"
	path := writeFixture(t, content)
	ingresses, err := ParseIngress(path)
	require.NoError(t, err)
	assert.Empty(t, ingresses)
}
