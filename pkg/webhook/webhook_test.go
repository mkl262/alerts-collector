// Copyright Contributors to the Open Cluster Management project

package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"

	"github.com/morvencao/alerts-collector/pkg/forwarder"
)

func TestServeReturnsOKWhenSomeAlertmanagersUnavailable(t *testing.T) {
	t.Parallel()

	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer ok.Close()

	fail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer fail.Close()

	wh := newTestWebhook(t, ok.URL, fail.URL)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(sampleWebhookBody()))
	wh.Serve(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body=%s)", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v (body=%s)", err, rec.Body.String())
	}
	if resp.Status != http.StatusOK || resp.Message != "success" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestServeReturnsErrorWhenAllAlertmanagersUnavailable(t *testing.T) {
	t.Parallel()

	failA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer failA.Close()

	failB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer failB.Close()

	wh := newTestWebhook(t, failA.URL, failB.URL)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(sampleWebhookBody()))
	wh.Serve(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d (body=%s)", http.StatusInternalServerError, rec.Code, rec.Body.String())
	}

	var resp response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v (body=%s)", err, rec.Body.String())
	}
	if resp.Status != http.StatusInternalServerError {
		t.Fatalf("unexpected response status field: %+v", resp)
	}
	if !strings.Contains(resp.Message, "failed to send") {
		t.Fatalf("expected failure message, got %+v", resp)
	}
}

func TestServeRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	wh := newTestWebhook(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader("{not-json"))
	wh.Serve(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d (body=%s)", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestHealthz(t *testing.T) {
	t.Parallel()

	wh := newTestWebhook(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	wh.Healthz(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "OK!" {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestNewWebhookTLSMissingCerts(t *testing.T) {
	t.Parallel()

	_, err := NewWebhook(&Options{
		TLS:      true,
		CertFile: filepath.Join(t.TempDir(), "missing.crt"),
		KeyFile:  filepath.Join(t.TempDir(), "missing.key"),
		Logger:   log.NewNopLogger(),
	})
	if err == nil {
		t.Fatal("expected error when TLS certs are missing")
	}
}

func TestRunAndShutdown(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	fwder, err := forwarder.NewForwarder(log.NewNopLogger(), writeAlertmanagersConfig(t))
	if err != nil {
		t.Fatalf("NewForwarder: %v", err)
	}
	wh, err := NewWebhook(&Options{
		Port:      port,
		Logger:    log.NewNopLogger(),
		Forwarder: fwder,
	})
	if err != nil {
		t.Fatalf("NewWebhook: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- wh.Run()
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, getErr := http.Get(fmt.Sprintf("http://127.0.0.1:%d/healthz", port))
		if getErr == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for webhook server")
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err := wh.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	select {
	case runErr := <-errCh:
		if runErr == nil {
			t.Fatal("expected ListenAndServe error after shutdown")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Run to return")
	}
}

func newTestWebhook(t *testing.T, rawURLs ...string) *Webhook {
	t.Helper()

	fwder, err := forwarder.NewForwarder(log.NewNopLogger(), writeAlertmanagersConfig(t, rawURLs...))
	if err != nil {
		t.Fatalf("NewForwarder: %v", err)
	}

	wh, err := NewWebhook(&Options{
		Logger:    log.NewNopLogger(),
		Forwarder: fwder,
	})
	if err != nil {
		t.Fatalf("NewWebhook: %v", err)
	}
	return wh
}

func writeAlertmanagersConfig(t *testing.T, rawURLs ...string) string {
	t.Helper()

	var b strings.Builder
	b.WriteString("alertmanagers:\n")
	for _, raw := range rawURLs {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse url %q: %v", raw, err)
		}
		fmt.Fprintf(&b, "  - static_configs:\n")
		fmt.Fprintf(&b, "      - %s\n", u.Host)
		fmt.Fprintf(&b, "    scheme: %s\n", u.Scheme)
		if u.Path != "" && u.Path != "/" {
			fmt.Fprintf(&b, "    path_prefix: %s\n", strings.TrimSuffix(u.Path, "/"))
		}
		fmt.Fprintf(&b, "    api_version: v2\n")
		fmt.Fprintf(&b, "    timeout: 2s\n")
	}

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func sampleWebhookBody() string {
	return fmt.Sprintf(`{
		"status": "firing",
		"alerts": [{
			"status": "firing",
			"labels": {"alertname": "TestAlert", "severity": "warning"},
			"annotations": {},
			"startsAt": %q,
			"endsAt": "0001-01-01T00:00:00Z",
			"generatorURL": "http://example.com"
		}],
		"groupLabels": {"alertname": "TestAlert"},
		"commonLabels": {"alertname": "TestAlert", "severity": "warning"},
		"commonAnnotations": {},
		"externalURL": "http://alertmanager.example.com"
	}`, time.Date(2021, 4, 24, 7, 11, 41, 0, time.UTC).Format(time.RFC3339))
}
