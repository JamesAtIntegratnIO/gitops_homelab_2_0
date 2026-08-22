package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Kargo's http step is synchronous, so the handler must answer before doing
// any work. A handler that blocked would put a model round trip inside every
// promotion's critical path.
func TestPromotionOpenedAnswersImmediately(t *testing.T) {
	blocked := make(chan struct{})
	s := &Server{
		Triage:  nil, // never reached: the fake below replaces Run
		Log:     testLogger(t),
		Timeout: time.Minute,
	}
	s.runFn = func(p Promotion) error {
		<-blocked // hold the "work" open
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/promotion-opened",
		bytes.NewReader([]byte(`{"prNumber":42,"stage":"cert-manager"}`)))
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() { s.PromotionOpened(rec, req); close(done) }()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler blocked on the triage goroutine")
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d", rec.Code)
	}
	close(blocked)
	s.Wait()
}

// Kargo retries a step whose response it did not like. A retry must not start
// a second triage of the same pull request.
func TestDuplicateCallsForOnePRCollapse(t *testing.T) {
	started := make(chan struct{}, 4)
	release := make(chan struct{})
	s := &Server{Log: testLogger(t), Timeout: time.Minute}
	s.runFn = func(p Promotion) error {
		started <- struct{}{}
		<-release
		return nil
	}

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/promotion-opened",
			bytes.NewReader([]byte(`{"prNumber":7}`)))
		s.PromotionOpened(httptest.NewRecorder(), req)
	}
	// Give the first goroutine a moment to register.
	time.Sleep(100 * time.Millisecond)
	if len(started) != 1 {
		t.Fatalf("want exactly 1 triage started, got %d", len(started))
	}
	close(release)
	s.Wait()
}

func TestRejectsPayloadWithoutAPRNumber(t *testing.T) {
	s := &Server{Log: testLogger(t), Timeout: time.Minute}
	s.runFn = func(Promotion) error { t.Fatal("should not run"); return nil }

	req := httptest.NewRequest(http.MethodPost, "/v1/promotion-opened",
		bytes.NewReader([]byte(`{"stage":"cert-manager"}`)))
	rec := httptest.NewRecorder()
	s.PromotionOpened(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

// A panic in triage must not take the process down -- one malformed pull
// request should not stop the agent handling the next.
func TestPanicInTriageIsContained(t *testing.T) {
	s := &Server{Log: testLogger(t), Timeout: time.Minute}
	s.runFn = func(Promotion) error { panic("boom") }

	req := httptest.NewRequest(http.MethodPost, "/v1/promotion-opened",
		bytes.NewReader([]byte(`{"prNumber":1}`)))
	s.PromotionOpened(httptest.NewRecorder(), req)
	s.Wait() // would crash the test binary if the recover were missing
}
