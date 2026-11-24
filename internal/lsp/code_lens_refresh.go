package lsp

import (
	"context"
	"time"

	"go.lsp.dev/uri"
)

// noteCartAnalyzed schedules one throttled workspace/codeLens/refresh request
// per cart when the client advertised refresh support and the budget lens is
// enabled.
func (s *Server) noteCartAnalyzed(root uri.URI) {
	if root == "" || !s.cfg.Settings().Limits.CodeLens {
		return
	}
	if !s.codeLensRefreshSupported() || s.ShuttingDown() {
		return
	}

	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	if s.refreshStopped {
		return
	}
	if _, pending := s.refreshTimers[root]; pending {
		return
	}
	delay := s.codeLensRefreshDelay
	if delay <= 0 {
		delay = 250 * time.Millisecond
	}
	s.refreshTimers[root] = time.AfterFunc(delay, func() {
		s.sendCodeLensRefresh(root)
	})
}

func (s *Server) sendCodeLensRefresh(root uri.URI) {
	s.refreshMu.Lock()
	delete(s.refreshTimers, root)
	stopped := s.refreshStopped
	s.refreshMu.Unlock()
	if stopped || !s.codeLensRefreshSupported() || s.ShuttingDown() {
		return
	}

	client := s.Client()
	if client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.CodeLensRefresh(ctx); err != nil {
		s.log.Debug("code lens refresh failed", "root", root.String(), "error", err)
	}
}

func (s *Server) codeLensRefreshSupported() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.codeLensRefresh
}

// stopCodeLensRefresh cancels pending refresh timers during shutdown or exit.
func (s *Server) stopCodeLensRefresh() {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	s.refreshStopped = true
	for root, timer := range s.refreshTimers {
		timer.Stop()
		delete(s.refreshTimers, root)
	}
}
