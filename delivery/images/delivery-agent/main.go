// delivery-agent triages automated dependency-bump pull requests.
//
// Kargo calls it when a pull request opens. It reads the pre-merge gate,
// explains a red one, and fixes the cases the rendered diff proves -- a chart
// default that flipped, a pin that must move with another, a port a policy
// still names. Everything else it hands to a human.
//
// The model never edits a file. It returns a structured verdict and a proposed
// edit set; this process applies them, behind an allowlist, a from-value check
// and a corroboration check. So "never edit the gate" and "never invent a
// version" are properties of the code, not requests in a prompt. See
// docs/safety-model.md and docs/prompt-contract.md.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/JamesAtIntegratnIO/delivery-kit/delivery-agent/edits"
	"github.com/JamesAtIntegratnIO/delivery-kit/delivery-agent/gitprovider"
	"github.com/JamesAtIntegratnIO/delivery-kit/delivery-agent/llm"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC)

	cfg, err := LoadConfig()
	if err != nil {
		logger.Fatalf("configuration: %v", err)
	}

	var model llm.Provider
	switch cfg.LLMProvider {
	case "openai":
		model = &llm.OpenAI{
			BaseURL: cfg.LLMBaseURL, APIKey: cfg.LLMKey, Model: cfg.LLMModel,
			ReasoningEffort: cfg.LLMReasoningEffort, Timeout: cfg.LLMTimeout,
		}
	case "anthropic":
		model = &llm.Anthropic{
			BaseURL: cfg.LLMBaseURL, APIKey: cfg.LLMKey, Model: cfg.LLMModel,
			Timeout: cfg.LLMTimeout,
		}
	}

	git := &gitprovider.GitHub{
		APIBase: cfg.GitAPIBase, Owner: cfg.GitOwner, Repo: cfg.GitRepo,
		Token: cfg.GitToken, AuthorName: cfg.AuthorName, AuthorEmail: cfg.AuthorEmail,
	}

	t := &Triage{
		Git: git, LLM: model,
		Policy:      edits.Policy{Allow: cfg.AllowPaths, Deny: cfg.DenyPaths},
		CheckName:   cfg.CheckName,
		MaxAttempts: cfg.MaxAttempts,
		GateWait:    cfg.GateWait,
		GatePoll:    cfg.GatePoll,
		CloneRoot:   cfg.CloneRoot,
		RepoURL:     cfg.GitRepoURL,
		Log:         func(f string, a ...any) { logger.Printf(f, a...) },
	}

	srv := &Server{Triage: t, Log: logger, Timeout: cfg.LLMTimeout + 5*time.Minute}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/promotion-opened", srv.PromotionOpened)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	httpSrv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Printf("delivery-agent listening on %s (model %s, repo %s/%s, allow %v)",
			cfg.Addr, model.Name(), cfg.GitOwner, cfg.GitRepo, cfg.AllowPaths)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("serving: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Print("shutting down; waiting for in-flight triage")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
	srv.Wait()
	logger.Print("stopped")
}

// Server accepts Kargo's call and gets out of the way.
type Server struct {
	Triage  *Triage
	Log     *log.Logger
	Timeout time.Duration

	wg sync.WaitGroup
	// inFlight collapses duplicate calls for the same pull request. Kargo
	// retries a step whose response it did not like, and a retry must not
	// start a second triage of the same PR.
	mu       sync.Mutex
	inFlight map[int]bool

	// runFn is the work the handler dispatches. Defaults to Triage.Run; tests
	// substitute it so the handler's concurrency behaviour can be exercised
	// without a git host or a model behind it.
	runFn func(Promotion) error
}

func (s *Server) run(ctx context.Context, p Promotion) error {
	if s.runFn != nil {
		return s.runFn(p)
	}
	return s.Triage.Run(ctx, p)
}

// PromotionOpened answers 202 immediately and does the work on a goroutine.
//
// This is not an optimisation. Kargo's `http` promotion step is synchronous,
// so a handler that blocked would put a model round trip -- minutes, on a
// local model -- inside the critical path of every promotion.
func (s *Server) PromotionOpened(w http.ResponseWriter, r *http.Request) {
	var p Promotion
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&p); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if p.PRNumber <= 0 {
		http.Error(w, "prNumber is required", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	if s.inFlight == nil {
		s.inFlight = map[int]bool{}
	}
	if s.inFlight[p.PRNumber] {
		s.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"already in progress"}`))
		return
	}
	s.inFlight[p.PRNumber] = true
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"accepted"}`))

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			s.mu.Lock()
			delete(s.inFlight, p.PRNumber)
			s.mu.Unlock()
			if rec := recover(); rec != nil {
				s.Log.Printf("PR %d: triage panicked: %v", p.PRNumber, rec)
			}
		}()

		// Detached from the request context on purpose: the response has
		// already gone back to Kargo, so cancelling with it would abort every
		// triage immediately.
		ctx, cancel := context.WithTimeout(context.Background(), s.Timeout)
		defer cancel()

		start := time.Now()
		if err := s.run(ctx, p); err != nil {
			s.Log.Printf("PR %d: triage failed after %s: %v", p.PRNumber, time.Since(start).Round(time.Second), err)
			return
		}
		s.Log.Printf("PR %d: triage done in %s", p.PRNumber, time.Since(start).Round(time.Second))
	}()
}

// Wait blocks until in-flight triage finishes.
func (s *Server) Wait() { s.wg.Wait() }
