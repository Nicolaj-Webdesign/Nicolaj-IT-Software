// Nicolaj Fernwartung – Server
//
// Ein einziges Programm: Web-Dashboard + Schnittstelle für die Agents auf den PCs.
// Start:  nicolaj-rmm-server            (HTTPS auf Port 8443, Daten im Ordner ./data)
package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var serverVersion = "dev" // wird beim Bauen gesetzt (-X main.serverVersion=…)

//go:embed web
var webFS embed.FS

//go:embed assets
var assetsFS embed.FS

type App struct {
	store       *Store
	hub         *Hub
	dataDir     string
	tls         bool
	fingerprint string
	port        string
	limiter     *RateLimiter
	setupCode   string
	srv         *http.Server
	behindProxy bool   // hinter Coolify/Traefik/Caddy: Proxy-Header vertrauen, Cookies nur über HTTPS
	publicURL   string // z. B. https://rmm.nicolaj-webdesign.de
	container   bool   // läuft in Docker: Updates kommen per neuem Image
}

// trustProxy: Client-IP aus X-Real-IP / X-Forwarded-For lesen (nur hinter einem Proxy aktivieren!).
var trustProxy bool

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func envBool(k string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(k))) {
	case "1", "true", "yes", "ja", "on":
		return true
	}
	return false
}

func main() {
	// Alle Einstellungen gehen auch über Umgebungsvariablen (praktisch für Docker/Coolify).
	addr := flag.String("addr", envOr("NRMM_ADDR", ":8443"), "Adresse und Port, z. B. :8443 (NRMM_ADDR)")
	dataDir := flag.String("data", envOr("NRMM_DATA", "data"), "Ordner für Datenbank, Zertifikat und Einrichtungscode (NRMM_DATA)")
	noTLS := flag.Bool("no-tls", envBool("NRMM_NO_TLS"), "ohne HTTPS starten – nur hinter einem Reverse-Proxy (NRMM_NO_TLS)")
	behindProxy := flag.Bool("behind-proxy", envBool("NRMM_BEHIND_PROXY"), "läuft hinter Coolify/Traefik/Caddy (NRMM_BEHIND_PROXY)")
	publicURL := flag.String("public-url", envOr("NRMM_PUBLIC_URL", ""), "öffentliche Adresse, z. B. https://rmm.nicolaj-webdesign.de (NRMM_PUBLIC_URL)")
	flag.Parse()
	trustProxy = *behindProxy

	if err := os.MkdirAll(*dataDir, 0o700); err != nil {
		log.Fatal(err)
	}
	if f, err := os.CreateTemp(*dataDir, ".schreibtest-*"); err != nil {
		log.Fatalf("Datenordner %s ist nicht beschreibbar (%v).\n"+
			"In Docker/Coolify: als Volume einhängen oder auf dem Host  chown -R 10001 <ordner>  ausführen.", *dataDir, err)
	} else {
		f.Close()
		os.Remove(f.Name())
	}
	if u := strings.TrimSpace(*publicURL); u != "" && !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
		log.Fatalf("NRMM_PUBLIC_URL muss mit https:// beginnen, z. B. https://rmm.nicolaj-webdesign.de (ist: %q)", u)
	}
	store, err := OpenStore(filepath.Join(*dataDir, "db.json"))
	if err != nil {
		log.Fatal(err)
	}
	app := &App{store: store, dataDir: *dataDir, tls: !*noTLS, limiter: NewRateLimiter(),
		behindProxy: *behindProxy, publicURL: strings.TrimRight(*publicURL, "/"), container: envBool("NRMM_CONTAINER")}
	app.hub = NewHub()
	if _, p, err := net.SplitHostPort(*addr); err == nil {
		app.port = p
	}
	if !app.hasAdmin() {
		app.setupCode = app.ensureSetupCode()
	}
	app.recoverAfterRestart()
	store.Update(func(d *DBData) { // ältere Geräte bekommen nachträglich eine PC-ID
		for _, dev := range d.Devices {
			if dev.PCID == "" {
				dev.PCID = d.newPCID()
			}
		}
	})
	go app.janitor()
	go app.githubLoop()

	var certFile, keyFile string
	if app.tls {
		certFile, keyFile, app.fingerprint, err = ensureCert(*dataDir)
		if err != nil {
			log.Fatal("Zertifikat: ", err)
		}
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           app.withSecurityHeaders(app.routes()),
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
		ErrorLog:          log.New(filteredLog{}, "", 0),
	}

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		<-ch
		fmt.Println("\nBeende … Daten werden gespeichert.")
		store.Flush()
		os.Exit(0)
	}()

	app.srv = srv
	app.printBanner()
	// Nach einem Selbst-Update kann der alte Prozess den Port noch kurz belegen: bis zu 20 s warten.
	var ln net.Listener
	for i := 0; i < 40; i++ {
		if ln, err = net.Listen("tcp", *addr); err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		log.Fatal(err)
	}
	if app.tls {
		err = srv.ServeTLS(ln, certFile, keyFile)
	} else {
		err = srv.Serve(ln)
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func (a *App) printBanner() {
	scheme := "https"
	if !a.tls {
		scheme = "http"
	}
	fmt.Printf("\n  nicolaj▍ Fernwartung – Server %s\n\n", serverVersion)
	fmt.Println("  Dashboard im Browser öffnen:")
	for _, u := range a.serverURLs(scheme) {
		fmt.Println("    " + u)
	}
	if a.behindProxy && a.publicURL == "" {
		fmt.Println("\n  Hinweis: NRMM_PUBLIC_URL ist nicht gesetzt (z. B. https://rmm.nicolaj-webdesign.de).")
	}
	if a.tls {
		fmt.Println("\n  Beim ersten Öffnen zeigt der Browser eine Zertifikatswarnung –")
		fmt.Println("  das ist normal (eigenes Zertifikat). Prüfe den Fingerabdruck:")
		fmt.Println("    SHA-256 " + fmtFingerprint(a.fingerprint))
	}
	if a.setupCode != "" {
		fmt.Println("\n  Einrichtungscode für das erste Konto:  " + a.setupCode)
	}
	fmt.Println()
}

func (a *App) serverURLs(scheme string) []string {
	if a.publicURL != "" {
		return []string{a.publicURL}
	}
	port := a.port
	if port == "" {
		port = "8443"
	}
	var out []string
	for _, ip := range localIPv4s() {
		out = append(out, fmt.Sprintf("%s://%s:%s", scheme, ip, port))
	}
	return append(out, fmt.Sprintf("%s://localhost:%s", scheme, port))
}

// filteredLog blendet die harmlosen TLS-Meldungen aus, die Browser bei eigenem Zertifikat erzeugen.
type filteredLog struct{}

func (filteredLog) Write(p []byte) (int, error) {
	if bytes.Contains(p, []byte("TLS handshake error")) {
		return len(p), nil
	}
	return os.Stderr.Write(p)
}

// ---------- Routen ----------

type adminHandler func(w http.ResponseWriter, r *http.Request, user string)
type agentHandler func(w http.ResponseWriter, r *http.Request, deviceID, role string)

func (a *App) routes() *http.ServeMux {
	m := http.NewServeMux()

	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") })
	m.HandleFunc("GET /{$}", a.static("web/index.html", "text/html; charset=utf-8"))
	m.HandleFunc("GET /app.js", a.static("web/app.js", "text/javascript; charset=utf-8"))
	m.HandleFunc("GET /app.css", a.static("web/app.css", "text/css; charset=utf-8"))
	m.HandleFunc("GET /logo.svg", a.static("web/logo.svg", "image/svg+xml"))
	m.HandleFunc("GET /favicon.svg", a.static("web/favicon.svg", "image/svg+xml"))

	// Schnittstelle für die Agents
	m.HandleFunc("POST /api/agent/enroll", a.agentEnroll)
	m.HandleFunc("GET /api/agent/events", a.agentAuth(a.agentEvents))
	m.HandleFunc("POST /api/agent/result", a.agentAuth(a.agentResult))
	m.HandleFunc("POST /api/agent/inventory", a.agentAuth(a.agentInventory))
	m.HandleFunc("POST /api/agent/screen-status", a.agentAuth(a.agentScreenStatus))
	m.HandleFunc("POST /api/agent/frame", a.agentAuth(a.agentFrame))
	m.HandleFunc("POST /api/agent/goodbye", a.agentAuth(a.agentGoodbye))
	m.HandleFunc("GET /api/agent/update", a.agentAuth(a.agentDownloadUpdate))

	// Anmeldung
	m.HandleFunc("GET /api/state", a.apiState)
	m.HandleFunc("POST /api/setup", a.csrf(a.apiSetup))
	m.HandleFunc("POST /api/login", a.csrf(a.apiLogin))
	m.HandleFunc("POST /api/logout", a.admin(a.apiLogout))
	m.HandleFunc("POST /api/account/password", a.admin(a.apiChangePassword))
	m.HandleFunc("POST /api/account/totp/begin", a.admin(a.apiTotpBegin))
	m.HandleFunc("POST /api/account/totp/enable", a.admin(a.apiTotpEnable))
	m.HandleFunc("POST /api/account/totp/disable", a.admin(a.apiTotpDisable))

	// Dashboard
	m.HandleFunc("GET /api/events", a.admin(a.apiEvents))
	m.HandleFunc("GET /api/server-info", a.admin(a.apiServerInfo))
	m.HandleFunc("GET /api/devices", a.admin(a.apiDevices))
	m.HandleFunc("GET /api/devices/{id}", a.admin(a.apiDevice))
	m.HandleFunc("PATCH /api/devices/{id}", a.admin(a.apiDeviceUpdate))
	m.HandleFunc("DELETE /api/devices/{id}", a.admin(a.apiDeviceDelete))
	m.HandleFunc("GET /api/devices/{id}/commands", a.admin(a.apiDeviceCommands))
	m.HandleFunc("POST /api/devices/{id}/commands", a.admin(a.apiCreateCommand))
	m.HandleFunc("POST /api/commands/{id}/cancel", a.admin(a.apiCancelCommand))
	m.HandleFunc("POST /api/devices/{id}/screen/start", a.admin(a.apiScreenStart))
	m.HandleFunc("POST /api/devices/{id}/screen/stop", a.admin(a.apiScreenStop))
	m.HandleFunc("POST /api/devices/{id}/screen/config", a.admin(a.apiScreenConfig))
	m.HandleFunc("POST /api/devices/{id}/screen/input", a.admin(a.apiScreenInput))
	m.HandleFunc("GET /api/devices/{id}/screen/stream", a.admin(a.apiScreenStream))
	m.HandleFunc("GET /api/enroll-codes", a.admin(a.apiEnrollCodes))
	m.HandleFunc("POST /api/enroll-codes", a.admin(a.apiCreateEnrollCode))
	m.HandleFunc("DELETE /api/enroll-codes/{code}", a.admin(a.apiDeleteEnrollCode))
	m.HandleFunc("GET /api/enroll-codes/{code}/installer", a.admin(a.apiInstaller))
	m.HandleFunc("GET /download/nicolaj-agent.exe", a.admin(a.apiDownloadAgent))
	m.HandleFunc("GET /api/audit", a.admin(a.apiAudit))
	m.HandleFunc("GET /api/updates", a.admin(a.apiUpdates))
	m.HandleFunc("POST /api/updates/settings", a.admin(a.apiUpdateSettings))
	m.HandleFunc("POST /api/updates/release", a.admin(a.apiUpdateRelease))
	m.HandleFunc("POST /api/updates/upload", a.admin(a.apiUpdateUpload))
	m.HandleFunc("GET /api/github", a.admin(a.apiGitHub))
	m.HandleFunc("POST /api/github/settings", a.admin(a.apiGitHubSettings))
	m.HandleFunc("POST /api/github/check", a.admin(a.apiGitHubCheck))
	m.HandleFunc("POST /api/github/update-server", a.admin(a.apiGitHubUpdateServer))
	return m
}

func (a *App) static(name, ctype string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := webFS.ReadFile(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", ctype)
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(b)
	}
}

func (a *App) withSecurityHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hd := w.Header()
		hd.Set("X-Content-Type-Options", "nosniff")
		hd.Set("Referrer-Policy", "no-referrer")
		hd.Set("X-Frame-Options", "DENY")
		hd.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if a.tls || (a.behindProxy && a.scheme(r) == "https") {
			hd.Set("Strict-Transport-Security", "max-age=31536000")
		}
		h.ServeHTTP(w, r)
	})
}

// csrf: Schreibende Anfragen müssen den Header X-NRMM tragen. Fremde Webseiten können
// diesen Header nicht setzen, zusammen mit SameSite=Strict schützt das vor CSRF.
func (a *App) csrf(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Header.Get("X-NRMM") != "1" {
			writeErr(w, 403, "Anfrage abgelehnt.")
			return
		}
		h(w, r)
	}
}

func (a *App) admin(h adminHandler) http.HandlerFunc {
	return a.csrf(func(w http.ResponseWriter, r *http.Request) {
		user := a.sessionUser(r)
		if user == "" {
			writeErr(w, 401, "Bitte anmelden.")
			return
		}
		h(w, r, user)
	})
}

func (a *App) agentAuth(h agentHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		devID, _, _ := strings.Cut(tok, ".")
		if !ok || devID == "" {
			writeErr(w, 401, "Kein gültiger Agent-Schlüssel.")
			return
		}
		hash := sha256Hex(tok)
		role := ""
		a.store.View(func(d *DBData) {
			dev := d.Devices[devID]
			if dev == nil {
				return
			}
			switch {
			case constEq(hash, dev.AgentTokenHash):
				role = "agent"
			case constEq(hash, dev.ScreenTokenHash):
				role = "screen"
			}
		})
		if role == "" {
			writeErr(w, 401, "Kein gültiger Agent-Schlüssel.")
			return
		}
		h(w, r, devID, role)
	}
}

// ---------- Hintergrundaufgaben ----------

// Nach einem Neustart sind laufende Aufträge verloren: ehrlich als "unbekannt" markieren.
func (a *App) recoverAfterRestart() {
	a.store.Update(func(d *DBData) {
		now := time.Now()
		for _, c := range d.Commands {
			if c.Status == "sent" || (c.Status == "queued" && commandHasSecrets(c.Type)) {
				c.Status = "failed"
				c.Output = "Server wurde neu gestartet, bevor eine Antwort kam. Bitte erneut ausführen."
				c.DoneAt = &now
			}
		}
	})
}

func (a *App) janitor() {
	for range time.Tick(15 * time.Second) {
		now := time.Now()
		var changed []*Command
		a.store.Update(func(d *DBData) {
			for _, c := range d.Commands {
				switch {
				case c.Status == "sent" && c.SentAt != nil && now.Sub(*c.SentAt) > time.Duration(c.Timeout+60)*time.Second:
					c.Status, c.Output = "timeout", "Keine Antwort vom Gerät innerhalb der Zeit."
				case c.Status == "queued" && now.Sub(c.CreatedAt) > 24*time.Hour:
					c.Status, c.Output = "expired", "Das Gerät war 24 Stunden nicht erreichbar."
				default:
					continue
				}
				c.DoneAt = &now
				cp := *c
				changed = append(changed, &cp)
			}
			ss := d.Sessions[:0]
			for _, s := range d.Sessions {
				if now.Before(s.Expires) {
					ss = append(ss, s)
				}
			}
			d.Sessions = ss
			ec := d.EnrollCodes[:0]
			for _, e := range d.EnrollCodes {
				if now.Sub(e.ExpiresAt) < 7*24*time.Hour {
					ec = append(ec, e)
				}
			}
			d.EnrollCodes = ec
		})
		for _, c := range changed {
			a.hub.Broadcast("command", c)
		}
		for _, devID := range a.hub.ExpireSessions() {
			a.hub.SendToAgent(devID, "screen", "screen-stop", map[string]any{})
			a.hub.Broadcast("screen", map[string]any{"deviceId": devID, "session": nil, "reason": "Sitzung beendet (keine Antwort oder kein Zuschauer)."})
		}
	}
}

func (a *App) audit(r *http.Request, user, deviceID, action, detail string) {
	e := &AuditEntry{Time: time.Now(), User: user, IP: clientIP(r), DeviceID: deviceID, Action: action, Detail: detail}
	a.store.Update(func(d *DBData) { d.Audit = append(d.Audit, e) })
}

// ---------- Hilfsfunktionen ----------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

func readJSON(w http.ResponseWriter, r *http.Request, v any, limit int64) bool {
	dec := json.NewDecoder(io.LimitReader(r.Body, limit))
	if err := dec.Decode(v); err != nil {
		writeErr(w, 400, "Ungültige Anfrage.")
		return false
	}
	return true
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}

func randHex(n int) string { return hex.EncodeToString(randBytes(n)) }

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func constEq(a, b string) bool {
	return len(a) == len(b) && a != "" && subtleEq(a, b)
}

func subtleEq(a, b string) bool {
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// newShortCode erzeugt gut abtippbare Codes wie "K7P2-X9QM".
func newShortCode() string {
	b := randBytes(8)
	out := make([]byte, 0, 9)
	for i, c := range b {
		if i == 4 {
			out = append(out, '-')
		}
		out = append(out, codeAlphabet[int(c)%len(codeAlphabet)])
	}
	return string(out)
}

func normalizeCode(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.NewReplacer(" ", "", "-", "").Replace(s)
	return s
}

func clientIP(r *http.Request) string {
	if trustProxy {
		if ip := strings.TrimSpace(r.Header.Get("X-Real-Ip")); ip != "" && net.ParseIP(ip) != nil {
			return ip
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if ip := strings.TrimSpace(parts[len(parts)-1]); net.ParseIP(ip) != nil { // vom nächsten Proxy angehängt
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n… (gekürzt)"
}

// scheme: unter welchem Protokoll der Browser bzw. Agent den Server erreicht.
func (a *App) scheme(r *http.Request) string {
	if a.tls {
		return "https"
	}
	if a.behindProxy {
		if p := r.Header.Get("X-Forwarded-Proto"); p == "http" || p == "https" {
			return p
		}
		return "https"
	}
	return "http"
}

// secureCookies: Cookies nur über HTTPS senden (auch hinter einem HTTPS-Proxy).
func (a *App) secureCookies() bool { return a.tls || a.behindProxy }
