// Copyright Contributors to the Open Cluster Management project

package forwarder

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/alertmanager/template"
)

func TestForwardSucceedsWhenSomeAlertmanagersUnavailable(t *testing.T) {
	t.Parallel()

	var okHits, failHits atomic.Uint64
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		okHits.Add(1)
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			t.Errorf("read body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ok.Close()

	fail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		failHits.Add(1)
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			t.Errorf("read body: %v", err)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer fail.Close()

	fwder := newTestForwarder(t, ok.URL, fail.URL)
	err := fwder.Forward(context.Background(), sampleAlerts())
	if err != nil {
		t.Fatalf("expected success when at least one alertmanager accepts alerts, got: %v", err)
	}
	if okHits.Load() != 1 {
		t.Fatalf("expected 1 hit on healthy alertmanager, got %d", okHits.Load())
	}
	if failHits.Load() != 1 {
		t.Fatalf("expected 1 hit on unavailable alertmanager, got %d", failHits.Load())
	}
}

func TestForwardFailsWhenAllAlertmanagersUnavailable(t *testing.T) {
	t.Parallel()

	failA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer failA.Close()

	failB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer failB.Close()

	fwder := newTestForwarder(t, failA.URL, failB.URL)
	err := fwder.Forward(context.Background(), sampleAlerts())
	if err == nil {
		t.Fatal("expected error when all alertmanagers are unavailable")
	}
}

func TestForwardSucceedsWhenOneEndpointIsUnreachable(t *testing.T) {
	t.Parallel()

	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ok.Close()

	fwder := newTestForwarder(t, ok.URL, "http://127.0.0.1:1")
	err := fwder.Forward(context.Background(), sampleAlerts())
	if err != nil {
		t.Fatalf("expected success when one endpoint is unreachable but another succeeds, got: %v", err)
	}
}

func newTestForwarder(t *testing.T, rawURLs ...string) *Forwarder {
	t.Helper()

	ams := make([]*Alertmanager, 0, len(rawURLs))
	for _, raw := range rawURLs {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse url %q: %v", raw, err)
		}
		ams = append(ams, &Alertmanager{
			logger:    log.NewNopLogger(),
			endpoints: []*url.URL{u},
			client:    &http.Client{Timeout: 2 * time.Second},
			timeout:   2 * time.Second,
			version:   APIv2,
		})
	}

	return &Forwarder{
		logger:        log.NewNopLogger(),
		alertmanagers: ams,
		versions:      []APIVersion{APIv2},
	}
}

func sampleAlerts() template.Alerts {
	return template.Alerts{{
		Status:   "firing",
		Labels:   template.KV{"alertname": "TestAlert", "severity": "warning"},
		StartsAt: time.Date(2021, 4, 24, 7, 11, 41, 0, time.UTC),
	}}
}
