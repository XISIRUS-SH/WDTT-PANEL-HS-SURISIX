package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed panelui/index.html
var panelHTML string

const panelSessionCookie = "wdtt_panel_session"
const panelCSRFCookie = "wdtt_panel_csrf"

var panelAllowedCommands = map[string]bool{
	"list": true, "details": true, "create": true, "delete": true, "unbind": true,
	"deactivate": true, "activate": true, "set-label": true, "set-hash": true,
	"set-expiry": true, "set-retention": true, "set-ports": true, "set-password": true,
	"update-client": true, "set-dns": true, "set-limit": true, "set-default-ports": true,
	"set-public-ip": true, "update-settings": true, "update-admin-profile": true,
	"refresh-public-ip": true, "cleanup-expired": true, "cleanup-orphans": true,
	"reset-traffic": true, "merge-client-traffic": true, "restart": true,
	"export-client-state": true, "import-client-state": true,
}

type panelSession struct {
	ExpiresAt time.Time
	CSRF      string
}
type panelSessionStore struct {
	mu sync.RWMutex
	m  map[string]panelSession
}

func newPanelSessionStore() *panelSessionStore {
	return &panelSessionStore{m: make(map[string]panelSession)}
}
func panelToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func (s *panelSessionStore) create() (string, panelSession) {
	x := panelSession{ExpiresAt: time.Now().Add(12 * time.Hour), CSRF: panelToken(24)}
	id := panelToken(32)
	s.mu.Lock()
	s.m[id] = x
	s.mu.Unlock()
	return id, x
}
func (s *panelSessionStore) get(id string) (panelSession, bool) {
	s.mu.RLock()
	x, ok := s.m[id]
	s.mu.RUnlock()
	if !ok || time.Now().After(x.ExpiresAt) {
		if ok {
			s.delete(id)
		}
		return panelSession{}, false
	}
	return x, true
}
func (s *panelSessionStore) delete(id string) { s.mu.Lock(); delete(s.m, id); s.mu.Unlock() }

func (s *panelSessionStore) purgeExpired(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	for id, session := range s.m {
		if now.After(session.ExpiresAt) {
			delete(s.m, id)
			removed++
		}
	}
	return removed
}

type panelServer struct {
	sessions  *panelSessionStore
	wgDev     wgDevice
	loginMu   sync.Mutex
	attempts  map[string][]time.Time
	rateMu    sync.Mutex
	mutations map[string][]time.Time
}

func newPanelServer(wgDev wgDevice) *panelServer {
	return &panelServer{sessions: newPanelSessionStore(), wgDev: wgDev, attempts: make(map[string][]time.Time), mutations: make(map[string][]time.Time)}
}

func (p *panelServer) allowLogin(ip string) bool {
	now := time.Now()
	p.loginMu.Lock()
	defer p.loginMu.Unlock()
	a := p.attempts[ip]
	j := 0
	for _, t := range a {
		if now.Sub(t) < 10*time.Minute {
			a[j] = t
			j++
		}
	}
	a = a[:j]
	if len(a) >= 12 {
		p.attempts[ip] = a
		return false
	}
	a = append(a, now)
	p.attempts[ip] = a
	return true
}

func (p *panelServer) allowMutation(sessionID string) bool {
	now := time.Now()
	p.rateMu.Lock()
	defer p.rateMu.Unlock()
	a := p.mutations[sessionID]
	j := 0
	for _, t := range a {
		if now.Sub(t) < time.Minute {
			a[j] = t
			j++
		}
	}
	a = a[:j]
	if len(a) >= 120 {
		p.mutations[sessionID] = a
		return false
	}
	p.mutations[sessionID] = append(a, now)
	return true
}

func (p *panelServer) clearMutationRate(sessionID string) {
	p.rateMu.Lock()
	delete(p.mutations, sessionID)
	p.rateMu.Unlock()
}

func panelClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
func (p *panelServer) session(r *http.Request) (string, panelSession, bool) {
	c, err := r.Cookie(panelSessionCookie)
	if err != nil {
		return "", panelSession{}, false
	}
	x, ok := p.sessions.get(c.Value)
	return c.Value, x, ok
}

func panelDecodeJSON(w http.ResponseWriter, r *http.Request, dst any, max int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, max)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("ожидался один JSON-объект")
	}
	return nil
}

func panelJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
func panelNoStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.Header().Set("Pragma", "no-cache")
}

func (p *panelServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/panel/api/login", p.login)
	mux.HandleFunc("/panel/api/logout", p.logout)
	mux.HandleFunc("/panel/api/health", p.health)
	mux.HandleFunc("/panel/api/session", p.sessionAPI)
	mux.HandleFunc("/panel/api/list", p.list)
	mux.HandleFunc("/panel/api/command", p.command)
	mux.HandleFunc("/panel/api/outbound/status", p.outboundStatus)
	mux.HandleFunc("/panel/api/outbound/diagnostics", p.outboundDiagnostics)
	mux.HandleFunc("/panel/api/outbound/direct", p.outboundDirect)
	mux.HandleFunc("/panel/api/outbound/local-proxy", p.outboundLocalProxy)
	mux.HandleFunc("/panel/api/outbound/local-proxy/check", p.outboundLocalProxyCheck)
	mux.HandleFunc("/panel/api/outbound/local-proxy/remove", p.outboundLocalProxyRemove)
	mux.HandleFunc("/panel/api/outbound/external-proxy", p.outboundExternalProxy)
	mux.HandleFunc("/panel/api/outbound/external-proxy/check", p.outboundExternalProxyCheck)
	mux.HandleFunc("/panel/api/outbound/warp/check", p.outboundWarpCheck)
	mux.HandleFunc("/panel/api/outbound/warp/delete", p.outboundWarpDelete)
	mux.HandleFunc("/panel/api/outbound/wg/delete", p.outboundWGDelete)
	mux.HandleFunc("/panel/api/captcha", p.captcha)
	mux.HandleFunc("/panel/", p.static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := panelToken(8)
		w.Header().Set("X-WDTT-Request-ID", requestID)
		panelNoStore(w)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; frame-src 'self' https://vk.com https://*.vk.com; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'")
		mux.ServeHTTP(w, r)
	})
}

func (p *panelServer) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		panelJSON(w, 405, map[string]any{"ok": false, "message": "method not allowed"})
		return
	}
	if !p.allowLogin(panelClientIP(r)) {
		panelJSON(w, 429, map[string]any{"ok": false, "message": "слишком много попыток входа"})
		return
	}
	var in struct {
		Password string `json:"password"`
	}
	if err := panelDecodeJSON(w, r, &in, 16<<10); err != nil {
		panelJSON(w, 400, map[string]any{"ok": false, "message": "некорректный JSON"})
		return
	}
	dbMutex.Lock()
	expected := db.MainPassword
	dbMutex.Unlock()
	if subtle.ConstantTimeCompare([]byte(in.Password), []byte(expected)) != 1 {
		panelJSON(w, 401, map[string]any{"ok": false, "message": "неверный пароль"})
		return
	}
	p.loginMu.Lock()
	delete(p.attempts, panelClientIP(r))
	p.loginMu.Unlock()
	id, s := p.sessions.create()
	http.SetCookie(w, &http.Cookie{Name: panelSessionCookie, Value: id, Path: "/panel", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, Expires: s.ExpiresAt})
	http.SetCookie(w, &http.Cookie{Name: panelCSRFCookie, Value: s.CSRF, Path: "/panel", HttpOnly: false, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, Expires: s.ExpiresAt})
	panelJSON(w, 200, map[string]any{"ok": true, "expires_at": s.ExpiresAt.Unix()})
}
func (p *panelServer) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		panelJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "message": "method not allowed"})
		return
	}
	if _, ok := p.require(w, r, true); !ok {
		return
	}
	if c, err := r.Cookie(panelSessionCookie); err == nil {
		p.sessions.delete(c.Value)
		p.clearMutationRate(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: panelSessionCookie, Value: "", Path: "/panel", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	http.SetCookie(w, &http.Cookie{Name: panelCSRFCookie, Value: "", Path: "/panel", MaxAge: -1, SameSite: http.SameSiteStrictMode})
	panelJSON(w, 200, map[string]any{"ok": true})
}

func (p *panelServer) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		panelJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "message": "method not allowed"})
		return
	}
	panelJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "wdtt-panel"})
}

func (p *panelServer) require(w http.ResponseWriter, r *http.Request, mutate bool) (panelSession, bool) {
	sessionID, s, ok := p.session(r)
	if !ok {
		panelJSON(w, 401, map[string]any{"ok": false, "message": "сессия истекла"})
		return panelSession{}, false
	}
	if mutate {
		c, err := r.Cookie(panelCSRFCookie)
		if err != nil || c.Value != s.CSRF || r.Header.Get("X-WDTT-CSRF") != s.CSRF {
			panelJSON(w, 403, map[string]any{"ok": false, "message": "CSRF-проверка не пройдена"})
			return panelSession{}, false
		}
		if !p.allowMutation(sessionID) {
			panelJSON(w, http.StatusTooManyRequests, map[string]any{"ok": false, "message": "слишком много изменяющих запросов"})
			return panelSession{}, false
		}
	}
	return s, true
}
func (p *panelServer) sessionAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		panelJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "message": "method not allowed"})
		return
	}
	s, ok := p.require(w, r, false)
	if !ok {
		return
	}
	dbMutex.Lock()
	info := buildAdminServerInfo(dbFileDir(), db)
	dbMutex.Unlock()
	panelJSON(w, 200, map[string]any{"ok": true, "session_expires_at": s.ExpiresAt.Unix(), "server": info})
}
func dbFileDir() string { return filepath.Dir(dbFile) }

func (p *panelServer) list(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		panelJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "message": "method not allowed"})
		return
	}
	if _, ok := p.require(w, r, false); !ok {
		return
	}
	dbMutex.Lock()
	defer dbMutex.Unlock()
	response, err := executeAdminCommand(dbFileDir(), db, []string{"list"}, p.wgDev, true)
	if err != nil {
		response = adminErrorResponse(err)
	}
	panelJSON(w, 200, response)
}

func (p *panelServer) command(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		panelJSON(w, 405, map[string]any{"ok": false, "message": "method not allowed"})
		return
	}
	if _, ok := p.require(w, r, true); !ok {
		return
	}
	var in struct {
		Args []string `json:"args"`
	}
	if err := panelDecodeJSON(w, r, &in, 64<<10); err != nil || len(in.Args) == 0 {
		panelJSON(w, 400, map[string]any{"ok": false, "message": "args required"})
		return
	}
	if !panelAllowedCommands[in.Args[0]] {
		panelJSON(w, 400, map[string]any{"ok": false, "message": "команда не разрешена"})
		return
	}
	dbMutex.Lock()
	defer dbMutex.Unlock()
	response, err := executeAdminCommand(dbFileDir(), db, in.Args, p.wgDev, true)
	if err != nil {
		response = adminErrorResponse(err)
	}
	panelJSON(w, 200, response)
}

func (p *panelServer) outboundRun(w http.ResponseWriter, r *http.Request, script string, timeout time.Duration) {
	if _, ok := p.require(w, r, true); !ok {
		return
	}
	out, err := runBotScript(script, timeout)
	panelJSON(w, 200, map[string]any{"ok": err == nil, "output": out, "message": func() string {
		if err != nil {
			return err.Error()
		}
		return "Готово"
	}()})
}
func (p *panelServer) outboundStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := p.require(w, r, false); !ok {
		return
	}
	out, err := runBotScript(outboundStatusScript(), 25*time.Second)
	panelJSON(w, 200, map[string]any{"ok": err == nil, "output": out})
}
func (p *panelServer) outboundDiagnostics(w http.ResponseWriter, r *http.Request) {
	if _, ok := p.require(w, r, false); !ok {
		return
	}
	out, err := runBotScript(outboundDiagnosticsScript(), 35*time.Second)
	panelJSON(w, 200, map[string]any{"ok": err == nil, "output": out})
}
func (p *panelServer) outboundDirect(w http.ResponseWriter, r *http.Request) {
	p.outboundRun(w, r, outboundDisableScript(), 45*time.Second)
}

func (p *panelServer) outboundLocalProxy(w http.ResponseWriter, r *http.Request) {
	if _, ok := p.require(w, r, true); !ok {
		return
	}
	var in struct {
		Login    string `json:"login"`
		Password string `json:"password"`
		Port     int    `json:"port"`
	}
	if err := panelDecodeJSON(w, r, &in, 32<<10); err != nil {
		panelJSON(w, 400, map[string]any{"ok": false, "message": "некорректный JSON"})
		return
	}
	if in.Port < 1 || in.Port > 65535 {
		panelJSON(w, 400, map[string]any{"ok": false, "message": "порт должен быть 1..65535"})
		return
	}
	out, err := runBotScript(localProxyInstallScript(strings.TrimSpace(in.Login), in.Password, in.Port), 180*time.Second)
	panelJSON(w, 200, map[string]any{"ok": err == nil, "output": out, "message": func() string {
		if err != nil {
			return err.Error()
		}
		return "Локальный прокси установлен"
	}()})
}

func (p *panelServer) outboundLocalProxyCheck(w http.ResponseWriter, r *http.Request) {
	if _, ok := p.require(w, r, false); !ok {
		return
	}
	out, err := runBotScript(localProxyCheckScript(), 30*time.Second)
	panelJSON(w, 200, map[string]any{"ok": err == nil, "output": out})
}

func (p *panelServer) outboundLocalProxyRemove(w http.ResponseWriter, r *http.Request) {
	p.outboundRun(w, r, localProxyRemoveScript(true), 45*time.Second)
}

func (p *panelServer) outboundExternalProxy(w http.ResponseWriter, r *http.Request) {
	if _, ok := p.require(w, r, true); !ok {
		return
	}
	var in struct {
		Kind     string `json:"kind"`
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := panelDecodeJSON(w, r, &in, 32<<10); err != nil {
		panelJSON(w, 400, map[string]any{"ok": false, "message": "некорректный JSON"})
		return
	}
	kind := strings.ToLower(strings.TrimSpace(in.Kind))
	if kind != "http" && kind != "https" && kind != "socks5" {
		panelJSON(w, 400, map[string]any{"ok": false, "message": "тип прокси: http, https или socks5"})
		return
	}
	if strings.TrimSpace(in.Host) == "" || in.Port < 1 || in.Port > 65535 {
		panelJSON(w, 400, map[string]any{"ok": false, "message": "укажите host и корректный порт"})
		return
	}
	out, err := runBotScript(externalProxyEnableScript(kind, strings.TrimSpace(in.Host), in.Port, in.Login, in.Password), 180*time.Second)
	panelJSON(w, 200, map[string]any{"ok": err == nil, "output": out, "message": func() string {
		if err != nil {
			return err.Error()
		}
		return "Внешний прокси включён"
	}()})
}

func (p *panelServer) outboundExternalProxyCheck(w http.ResponseWriter, r *http.Request) {
	if _, ok := p.require(w, r, true); !ok {
		return
	}
	var in struct {
		Kind     string `json:"kind"`
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := panelDecodeJSON(w, r, &in, 32<<10); err != nil {
		panelJSON(w, 400, map[string]any{"ok": false, "message": "некорректный JSON"})
		return
	}
	kind := strings.ToLower(strings.TrimSpace(in.Kind))
	if kind != "http" && kind != "https" && kind != "socks5" {
		panelJSON(w, 400, map[string]any{"ok": false, "message": "тип прокси: http, https или socks5"})
		return
	}
	out, err := runBotScript(externalProxyCheckScript(kind, strings.TrimSpace(in.Host), in.Port, in.Login, in.Password), 30*time.Second)
	panelJSON(w, 200, map[string]any{"ok": err == nil, "output": out})
}
func (p *panelServer) outboundWarpCheck(w http.ResponseWriter, r *http.Request) {
	p.outboundRun(w, r, freeWarpCheckScript(false), 45*time.Second)
}
func (p *panelServer) outboundWarpDelete(w http.ResponseWriter, r *http.Request) {
	p.outboundRun(w, r, deleteFreeWarpScript(), 45*time.Second)
}
func (p *panelServer) outboundWGDelete(w http.ResponseWriter, r *http.Request) {
	p.outboundRun(w, r, deleteImportedWGScript(), 45*time.Second)
}
func (p *panelServer) captcha(w http.ResponseWriter, r *http.Request) {
	if _, ok := p.require(w, r, false); !ok {
		return
	}
	if r.Method != http.MethodGet {
		panelJSON(w, 405, map[string]any{"ok": false})
		return
	}
	panelJSON(w, 200, map[string]any{"ok": true, "message": "CAPTCHA решается только вручную в браузере"})
}

func (p *panelServer) static(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	if r.URL.Path != "/panel/" && r.URL.Path != "/panel" {
		http.NotFound(w, r)
		return
	}
	t := template.Must(template.New("panel").Parse(panelHTML))
	_ = t.Execute(w, map[string]any{"Title": "WDTT Panel", "Version": wdttServerVersion})
}

func startWebPanel(ctxDone <-chan struct{}, listen string, wgDev wgDevice) (func(), error) {
	if strings.TrimSpace(listen) == "" || listen == "-" {
		return func() {}, nil
	}
	impl := newPanelServer(wgDev)
	srv := &http.Server{Addr: listen, Handler: impl.handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 120 * time.Second}
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return nil, fmt.Errorf("web panel listen %s: %w", listen, err)
	}
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctxDone:
				return
			case now := <-ticker.C:
				impl.sessions.purgeExpired(now)
			}
		}
	}()
	go func() {
		<-ctxDone
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[PANEL] %v", err)
		}
	}()
	log.Printf("[PANEL] Web panel: http://%s/panel/", listen)
	return func() { _ = srv.Close() }, nil
}
