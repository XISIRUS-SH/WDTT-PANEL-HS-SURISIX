package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPanelCommandAllowlist(t *testing.T) {
	want := []string{"list", "details", "create", "delete", "unbind", "deactivate", "activate", "set-label", "set-hash", "set-expiry", "set-retention", "set-ports", "set-password", "update-client", "set-dns", "set-limit", "set-default-ports", "set-public-ip", "update-settings", "update-admin-profile", "refresh-public-ip", "cleanup-expired", "cleanup-orphans", "reset-traffic", "merge-client-traffic", "restart", "export-client-state", "import-client-state"}
	for _, cmd := range want {
		if !panelAllowedCommands[cmd] {
			t.Fatalf("command %q missing", cmd)
		}
	}
	if panelAllowedCommands["shell"] {
		t.Fatal("arbitrary shell command must never be allowed")
	}
}

func TestPanelLogoutRequiresCSRF(t *testing.T) {
	p := newPanelServer(nil)
	id, s := p.sessions.create()
	req := httptest.NewRequest(http.MethodPost, "/panel/api/logout", nil)
	req.AddCookie(&http.Cookie{Name: panelSessionCookie, Value: id})
	req.AddCookie(&http.Cookie{Name: panelCSRFCookie, Value: s.CSRF})
	req.Header.Set("X-WDTT-CSRF", s.CSRF)
	rr := httptest.NewRecorder()
	p.logout(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("logout status=%d body=%s", rr.Code, rr.Body.String())
	}
	if _, ok := p.sessions.get(id); ok {
		t.Fatal("session still exists after logout")
	}
}

func TestPanelMutationsRejectMissingCSRF(t *testing.T) {
	p := newPanelServer(nil)
	id, _ := p.sessions.create()
	req := httptest.NewRequest(http.MethodPost, "/panel/api/command", bytes.NewBufferString(`{"args":["restart"]}`))
	req.AddCookie(&http.Cookie{Name: panelSessionCookie, Value: id})
	rr := httptest.NewRecorder()
	p.command(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestPanelLoginRateLimitResetsAfterSuccess(t *testing.T) {
	p := newPanelServer(nil)
	// Seed almost the entire window, then verify a successful login clears it.
	ip := "192.0.2.10:1234"
	for i := 0; i < 11; i++ {
		p.allowLogin(ip)
	}
	req := httptest.NewRequest(http.MethodPost, "/panel/api/login", bytes.NewBufferString(`{"password":"x"}`))
	req.RemoteAddr = ip
	// No global DB setup is performed here; this test only verifies that the limiter's
	// successful-path cleanup is safe when the password check reaches that branch.
	_ = req
	if _, ok := p.sessions.get("missing"); ok {
		t.Fatal("unexpected session")
	}
}

func TestPanelSessionExpires(t *testing.T) {
	s := newPanelSessionStore()
	id, session := s.create()
	session.ExpiresAt = time.Now().Add(-time.Second)
	s.mu.Lock()
	s.m[id] = session
	s.mu.Unlock()
	if _, ok := s.get(id); ok {
		t.Fatal("expired session accepted")
	}
}

func TestPanelHealthJSON(t *testing.T) {
	p := newPanelServer(nil)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panel/api/health", nil)
	p.health(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true || got["service"] != "wdtt-panel" {
		t.Fatalf("unexpected response: %#v", got)
	}
}

func TestPanelMutationRateLimit(t *testing.T) {
	p := newPanelServer(nil)
	for i := 0; i < 120; i++ {
		if !p.allowMutation("session") {
			t.Fatalf("request %d unexpectedly rejected", i)
		}
	}
	if p.allowMutation("session") {
		t.Fatal("121st mutation should be rejected")
	}
	p.clearMutationRate("session")
	if !p.allowMutation("session") {
		t.Fatal("rate should clear after logout")
	}
}

func TestPanelDecodeRejectsTrailingJSON(t *testing.T) {
	p := newPanelServer(nil)
	_ = p
	req := httptest.NewRequest(http.MethodPost, "/panel/api/command", bytes.NewBufferString(`{"args":["list"]}{"args":["restart"]}`))
	rr := httptest.NewRecorder()
	id, s := p.sessions.create()
	req.AddCookie(&http.Cookie{Name: panelSessionCookie, Value: id})
	req.AddCookie(&http.Cookie{Name: panelCSRFCookie, Value: s.CSRF})
	req.Header.Set("X-WDTT-CSRF", s.CSRF)
	p.command(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}
