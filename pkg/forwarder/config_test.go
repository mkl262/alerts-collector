// Copyright Contributors to the Open Cluster Management project

package forwarder

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBasicAuthIsZero(t *testing.T) {
	t.Parallel()

	if !(BasicAuth{}).IsZero() {
		t.Fatal("expected empty BasicAuth to be zero")
	}
	if (BasicAuth{Username: "u"}).IsZero() {
		t.Fatal("expected BasicAuth with username to be non-zero")
	}
	if (BasicAuth{Password: "p"}).IsZero() {
		t.Fatal("expected BasicAuth with password to be non-zero")
	}
	if (BasicAuth{PasswordFile: "f"}).IsZero() {
		t.Fatal("expected BasicAuth with password file to be non-zero")
	}
}

func TestLoadAlertingConfig(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`alertmanagers:
  - static_configs:
      - alertmanager.example.com:9093
    scheme: http
    api_version: v2
    timeout: 5s
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loadAlertingConfig(path)
	if err != nil {
		t.Fatalf("loadAlertingConfig: %v", err)
	}
	if len(cfg.Alertmanagers) != 1 {
		t.Fatalf("expected 1 alertmanager, got %d", len(cfg.Alertmanagers))
	}
	am := cfg.Alertmanagers[0]
	if am.APIVersion != APIv2 {
		t.Fatalf("expected api version %q, got %q", APIv2, am.APIVersion)
	}
	if am.EndpointsConfig.Scheme != "http" {
		t.Fatalf("unexpected scheme: %q", am.EndpointsConfig.Scheme)
	}
	if len(am.EndpointsConfig.StaticAddresses) != 1 || am.EndpointsConfig.StaticAddresses[0] != "alertmanager.example.com:9093" {
		t.Fatalf("unexpected static addresses: %#v", am.EndpointsConfig.StaticAddresses)
	}
}

func TestLoadAlertingConfigMissingFile(t *testing.T) {
	t.Parallel()

	_, err := loadAlertingConfig(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestLoadAlertingConfigInvalidYAML(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte("alertmanagers: ["), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := loadAlertingConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid yaml")
	}
}

func TestCreateHTTPClient(t *testing.T) {
	t.Parallel()

	client, err := createHTTPClient(ClientConfig{}, "test-client")
	if err != nil {
		t.Fatalf("createHTTPClient: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestCreateHTTPClientWithBasicAuth(t *testing.T) {
	t.Parallel()

	client, err := createHTTPClient(ClientConfig{
		BasicAuth: BasicAuth{
			Username: "user",
			Password: "pass",
		},
	}, "test-client-auth")
	if err != nil {
		t.Fatalf("createHTTPClient: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestCreateHTTPClientInvalidProxyURL(t *testing.T) {
	t.Parallel()

	_, err := createHTTPClient(ClientConfig{
		ProxyURL: ":not a url:[[[",
	}, "test-client-proxy")
	if err == nil {
		t.Fatal("expected error for invalid proxy URL")
	}
}
