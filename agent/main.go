// Nicolaj Fernwartung – Agent für Windows-PCs
//
// Eine einzige Datei mit mehreren Rollen:
//
//	(Doppelklick)   Installer – wenn der Server Code und Adresse eingebaut hat
//	install         Installation per Kommandozeile
//	uninstall       Entfernen
//	service         Verwaltungsdienst (läuft als SYSTEM, startet mit Windows)
//	screen          Bildschirm-Helfer (läuft in der Sitzung des angemeldeten Benutzers)
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Version und Kennung werden beim Bauen gesetzt (build.sh liest die Datei VERSION bzw. den Git-Tag):
//
//	-ldflags "-X main.agentVersion=0.5.0 -X main.buildMarker=NRMM-AGENT-VERSION:0.5.0:END"
//
// buildMarker lässt den Server die Version direkt aus der .exe lesen (für Updates).
var (
	agentVersion = "dev"
	buildMarker  = "dev"
	// defaultServer: Serveradresse für den allgemeinen Installer ohne angehängte Daten
	// (z. B. direkt aus dem GitHub-Release). Per -ldflags "-X main.defaultServer=https://…".
	defaultServer = ""
)

func main() {
	mode := ""
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	var rest []string
	if len(os.Args) > 2 {
		rest = os.Args[2:]
	}
	switch mode {
	case "service":
		initLog(logPath("agent"))
		runService()
	case "screen":
		initLog(logPath("screen"))
		runScreen()
	case "install":
		runInstallCLI(rest)
	case "uninstall":
		runUninstall(rest)
	case "sim":
		runSim(rest)
	case "info":
		runInfo()
	case "chat":
		runChat()
	case "update-finish":
		runUpdateFinish(rest)
	case "version", "--version", "-v":
		fmt.Println("nicolaj-agent", agentVersion)
	default:
		runDefault()
	}
}

// ---------- Konfiguration ----------

type Config struct {
	Server      string `json:"server"`
	DeviceID    string `json:"deviceId"`
	Token       string `json:"token"`
	Fingerprint string `json:"fingerprint"`
	Name        string `json:"name,omitempty"`
	PCID        string `json:"pcId,omitempty"`
}

func fmtPCID(s string) string {
	if len(s) != 9 {
		return s
	}
	return s[:3] + " " + s[3:6] + " " + s[6:]
}

func loadConfig(path string) (Config, error) {
	var c Config
	b, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	if err == nil && (c.Server == "" || c.Token == "") {
		err = errors.New("Konfiguration unvollständig")
	}
	return c, err
}

func saveConfig(path string, c Config) error {
	b, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(path, b, 0o600)
}

// Payload: Vom Server an die .exe angehängte Daten (Serveradresse, Einmal-Code, Fingerabdruck).
type Payload struct {
	Server      string `json:"server"`
	Code        string `json:"code"`
	Fingerprint string `json:"fingerprint"`
	Label       string `json:"label"`
	Request     bool   `json:"request"` // allgemeiner Installer: Name eingeben, Zugang anfragen
}

var reFileCode = regexp.MustCompile(`(?:^|[^A-Za-z0-9])([A-Z0-9]{4}-[A-Z0-9]{4})(?:[^A-Za-z0-9]|$)`)

// codeFromFilename: Einrichtungscode aus dem Dateinamen („Nicolaj-Fernwartung-ABCD-EFGH (1).exe“).
func codeFromFilename(exe string) string {
	name := filepath.Base(exe)
	name = strings.TrimSuffix(name, filepath.Ext(name))
	if m := reFileCode.FindStringSubmatch(name); m != nil {
		return m[1]
	}
	return ""
}

// readPayload liest die angehängten Daten und gibt die Größe der eigentlichen .exe zurück.
func readPayload(exe string) (*Payload, int64, error) {
	f, err := os.Open(exe)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}
	size := st.Size()
	if size < 12 {
		return nil, size, nil
	}
	tail := make([]byte, 12)
	if _, err := f.ReadAt(tail, size-12); err != nil {
		return nil, size, err
	}
	if string(tail[4:]) != "NRMMCFG1" {
		return nil, size, nil
	}
	n := int64(binary.LittleEndian.Uint32(tail[:4]))
	if n <= 0 || n > 64<<10 || n > size-12 {
		return nil, size, nil
	}
	buf := make([]byte, n)
	if _, err := f.ReadAt(buf, size-12-n); err != nil {
		return nil, size, err
	}
	var p Payload
	if err := json.Unmarshal(buf, &p); err != nil {
		return nil, size, err
	}
	return &p, size - 12 - n, nil
}

// ---------- Logbuch ----------

var logger = log.New(os.Stderr, "", log.LstdFlags)

func initLog(path string) {
	if path == "" {
		return
	}
	if st, err := os.Stat(path); err == nil && st.Size() > 5<<20 {
		os.Rename(path, path+".alt")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	logger = log.New(f, "", log.LstdFlags)
}

func logf(format string, a ...any) { logger.Printf(format, a...) }

// ---------- Verbindung zum Server ----------

var (
	errUnauthorized = errors.New("Schlüssel abgelehnt")
	errRejected     = errors.New("Zugangsanfrage abgelehnt")
	// onRejected: was passieren soll, wenn der Server die Zugangsanfrage abgelehnt hat.
	onRejected func()
)

type Client struct {
	cfg    Config
	api    *http.Client
	stream *http.Client
}

func newClient(cfg Config) *Client {
	cfg.Server = strings.TrimRight(cfg.Server, "/")
	tr := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		TLSClientConfig:     tlsConfig(cfg.Fingerprint),
		TLSHandshakeTimeout: 15 * time.Second,
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:     90 * time.Second,
	}
	return &Client{
		cfg:    cfg,
		api:    &http.Client{Transport: tr, Timeout: 60 * time.Second},
		stream: &http.Client{Transport: tr},
	}
}

// tlsConfig: Mit Fingerabdruck wird genau dieses Server-Zertifikat erwartet (auch selbstsigniert).
func tlsConfig(fp string) *tls.Config {
	if fp == "" {
		return &tls.Config{MinVersion: tls.VersionTLS12}
	}
	want, _ := hex.DecodeString(strings.ReplaceAll(strings.ToLower(fp), ":", ""))
	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true, // ersetzt durch die Prüfung des Fingerabdrucks unten
		VerifyPeerCertificate: func(raw [][]byte, _ [][]*x509.Certificate) error {
			if len(raw) == 0 {
				return errors.New("Server hat kein Zertifikat gesendet")
			}
			sum := sha256.Sum256(raw[0])
			if len(want) != 32 || subtle.ConstantTimeCompare(sum[:], want) != 1 {
				return errors.New("Zertifikat des Servers passt nicht zum gespeicherten Fingerabdruck")
			}
			return nil
		},
	}
}

func (c *Client) newReq(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.Server+path, body)
	if err != nil {
		return nil, err
	}
	if c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}
	req.Header.Set("X-Agent-Version", agentVersion)
	req.Header.Set("X-Agent-Build", buildMarker)
	req.Header.Set("User-Agent", "nicolaj-agent/"+agentVersion)
	return req, nil
}

func apiError(resp *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var e struct{ Error string }
	if json.Unmarshal(b, &e) == nil && e.Error != "" {
		return errors.New(e.Error)
	}
	return fmt.Errorf("Server antwortet mit %s", resp.Status)
}

func (c *Client) postJSON(path string, in, out any) error {
	b, _ := json.Marshal(in)
	req, err := c.newReq(context.Background(), "POST", path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.api.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return errUnauthorized
	}
	if resp.StatusCode >= 300 {
		return apiError(resp)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	io.Copy(io.Discard, resp.Body)
	return nil
}

func (c *Client) getJSON(path string, out any) error {
	req, err := c.newReq(context.Background(), "GET", path, nil)
	if err != nil {
		return err
	}
	resp, err := c.api.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return errUnauthorized
	}
	if resp.StatusCode >= 300 {
		return apiError(resp)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) postRaw(path string, body []byte) error {
	req, err := c.newReq(context.Background(), "POST", path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.api.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("Server antwortet mit %s", resp.Status)
	}
	return nil
}

func (c *Client) postFrame(session string, jpg []byte, w, h int) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := c.newReq(ctx, "POST", "/api/agent/frame?session="+session, bytes.NewReader(jpg))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "image/jpeg")
	req.Header.Set("X-Width", strconv.Itoa(w))
	req.Header.Set("X-Height", strconv.Itoa(h))
	resp, err := c.stream.Do(req)
	if err != nil {
		return 0, err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp.StatusCode, nil
}

// stream hält die Dauerverbindung (Server-Sent Events) offen und meldet jedes Ereignis.
func (c *Client) listen(ctx context.Context, onEvent func(ev string, data []byte), onOpen func()) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	req, err := c.newReq(ctx, "GET", "/api/agent/events", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := c.stream.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return errUnauthorized
	}
	if resp.StatusCode == 410 {
		return errRejected
	}
	if resp.StatusCode != 200 {
		return apiError(resp)
	}
	if onOpen != nil {
		onOpen()
	}
	watchdog := time.AfterFunc(60*time.Second, cancel)
	defer watchdog.Stop()
	rd := bufio.NewReaderSize(resp.Body, 64<<10)
	var ev string
	var data bytes.Buffer
	for {
		line, err := rd.ReadString('\n')
		if err != nil {
			return err
		}
		watchdog.Reset(60 * time.Second)
		line = strings.TrimRight(line, "\r\n")
		switch {
		case line == "":
			if data.Len() > 0 {
				name := ev
				if name == "" {
					name = "message"
				}
				onEvent(name, append([]byte(nil), data.Bytes()...))
			}
			ev = ""
			data.Reset()
		case strings.HasPrefix(line, ":"):
		case strings.HasPrefix(line, "event:"):
			ev = strings.TrimSpace(line[6:])
		case strings.HasPrefix(line, "data:"):
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimPrefix(line[5:], " "))
		}
	}
}

func runLoop(c *Client, name string, onEvent func(string, []byte), onOpen func()) {
	backoff := 2 * time.Second
	for {
		start := time.Now()
		logf("%s: verbinde mit %s", name, c.cfg.Server)
		err := c.listen(context.Background(), onEvent, onOpen)
		if errors.Is(err, errRejected) {
			logf("%s: Die Zugangsanfrage wurde abgelehnt.", name)
			if onRejected != nil {
				onRejected()
			}
			time.Sleep(time.Hour)
			continue
		}
		if errors.Is(err, errUnauthorized) {
			logf("%s: Der Server kennt diesen PC nicht mehr (im Dashboard gelöscht?). Neuer Versuch in 10 Minuten.", name)
			time.Sleep(10 * time.Minute)
			continue
		}
		if time.Since(start) > time.Minute {
			backoff = 2 * time.Second
		}
		logf("%s: Verbindung getrennt (%v) – neuer Versuch in %s", name, err, backoff)
		time.Sleep(backoff + time.Duration(rand.IntN(1000))*time.Millisecond)
		backoff = min(backoff*2, time.Minute)
	}
}

// ---------- Anmeldung beim Server ----------

func enroll(server, code, fp string) (agentCfg, screenCfg Config, name string, err error) {
	return register(server, fp, "/api/agent/enroll", map[string]string{"code": code})
}

// requestAccess: allgemeiner Installer – meldet den PC mit Namen an, Freigabe im Dashboard.
func requestAccess(server, fp, person string) (agentCfg, screenCfg Config, name string, err error) {
	return register(server, fp, "/api/agent/request", map[string]string{"name": person})
}

func register(server, fp, path string, extra map[string]string) (agentCfg, screenCfg Config, name string, err error) {
	server = strings.TrimRight(strings.TrimSpace(server), "/")
	c := newClient(Config{Server: server, Fingerprint: fp})
	host, _ := os.Hostname()
	var res struct {
		DeviceID    string `json:"deviceId"`
		PCID        string `json:"pcId"`
		AgentToken  string `json:"agentToken"`
		ScreenToken string `json:"screenToken"`
		Name        string `json:"name"`
	}
	body := map[string]string{"hostname": host, "os": runtime.GOOS, "version": agentVersion}
	for k, v := range extra {
		body[k] = v
	}
	err = c.postJSON(path, body, &res)
	if err != nil {
		return
	}
	agentCfg = Config{Server: server, DeviceID: res.DeviceID, Token: res.AgentToken, Fingerprint: fp, Name: res.Name, PCID: res.PCID}
	screenCfg = Config{Server: server, DeviceID: res.DeviceID, Token: res.ScreenToken, Fingerprint: fp, Name: res.Name, PCID: res.PCID}
	return agentCfg, screenCfg, res.Name, nil
}

// ---------- Verwaltungsdienst ----------

type Cmd struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Args    json.RawMessage `json:"args"`
	Timeout int             `json:"timeout"`
}

type Result struct {
	ID     string          `json:"id"`
	OK     bool            `json:"ok"`
	Output string          `json:"output"`
	Data   json.RawMessage `json:"data,omitempty"`
}

func (c *Client) sendResult(r Result) {
	for i := 0; i < 5; i++ {
		if err := c.postJSON("/api/agent/result", r, nil); err == nil || errors.Is(err, errUnauthorized) {
			return
		} else {
			logf("Ergebnis senden fehlgeschlagen: %v", err)
		}
		time.Sleep(time.Duration(2<<i) * time.Second)
	}
}

func runService() {
	if !singleInstance("Global\\NicolajRMMAgent") {
		return
	}
	cfg, err := loadConfig(agentConfigPath())
	if err != nil {
		logf("Dienst: Konfiguration fehlt (%v) – ist der Agent installiert?", err)
		time.Sleep(time.Minute)
		os.Exit(1)
	}
	c := newClient(cfg)
	logf("Dienst %s gestartet (Gerät %s)", agentVersion, cfg.DeviceID)
	go ensureShortcuts()
	var rejectOnce sync.Once
	onRejected = func() { rejectOnce.Do(selfUninstall) } // abgelehnte Anfrage: Programm entfernt sich selbst

	go func() {
		time.Sleep(3 * time.Second)
		for {
			if inv := collectInventory(); len(inv) > 0 {
				if err := c.postRaw("/api/agent/inventory", inv); err != nil {
					logf("Systeminfo senden fehlgeschlagen: %v", err)
				}
			}
			time.Sleep(5 * time.Minute)
		}
	}()

	// Warteschlangen: Software (winget mag keine parallelen Läufe), Sicherheit (Scans dauern) und alles andere.
	software := make(chan Cmd, 64)
	security := make(chan Cmd, 64)
	other := make(chan Cmd, 64)
	worker := func(q chan Cmd) {
		for cmd := range q {
			logf("Auftrag %s: %s", cmd.ID, cmd.Type)
			timeout := time.Duration(max(cmd.Timeout, 30)) * time.Second
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			res := safeExec(ctx, cmd)
			cancel()
			res.ID = cmd.ID
			res.Output = truncate(res.Output, 190000)
			logf("Auftrag %s fertig: ok=%v", cmd.ID, res.OK)
			c.sendResult(res)
			if strings.HasPrefix(cmd.Type, "security.") {
				go func() { // frischer Status für Übersicht und Alarme
					if inv := collectInventory(); len(inv) > 0 {
						c.postRaw("/api/agent/inventory", inv)
					}
				}()
			}
		}
	}
	go worker(software)
	go worker(security)
	go worker(other)
	go worker(other)

	go cleanupAfterUpdate()

	runLoop(c, "Dienst", func(ev string, data []byte) {
		if ev == "config" {
			updateConfigs(data)
			return
		}
		if ev == "update" { // automatisches Update, vom Server angeboten
			go func() {
				msg, restart, err := handleUpdate(c, false)
				if err != nil {
					logf("Auto-Update: %v", err)
				} else {
					logf("Auto-Update: %s", msg)
				}
				if restart {
					exitForUpdate()
				}
			}()
			return
		}
		if ev != "cmd" {
			return
		}
		var cmd Cmd
		if json.Unmarshal(data, &cmd) != nil || cmd.ID == "" {
			return
		}
		if cmd.Type == "agent.update" { // vom Dashboard ausgelöst
			go func() {
				msg, restart, err := handleUpdate(c, true)
				res := Result{ID: cmd.ID, OK: err == nil, Output: msg}
				if err != nil {
					res.Output = err.Error()
				}
				c.sendResult(res)
				if restart {
					exitForUpdate()
				}
			}()
			return
		}
		switch {
		case strings.HasPrefix(cmd.Type, "software.") && cmd.Type != "software.list":
			software <- cmd
		case strings.HasPrefix(cmd.Type, "security.") && cmd.Type != "security.status":
			security <- cmd
		default:
			other <- cmd
		}
	}, markUpdateOK)
}

// updateConfigs speichert PC-ID und Namen, die der Server mitteilt (z. B. nach Umbenennen im Dashboard).
func updateConfigs(data []byte) {
	var m struct {
		PCID string `json:"pcId"`
		Name string `json:"name"`
	}
	if json.Unmarshal(data, &m) != nil || m.PCID == "" {
		return
	}
	for _, p := range []string{agentConfigPath(), screenConfigPath()} {
		cfg, err := loadConfig(p)
		if err != nil || (cfg.PCID == m.PCID && cfg.Name == m.Name) {
			continue
		}
		cfg.PCID, cfg.Name = m.PCID, m.Name
		if err := saveConfig(p, cfg); err != nil {
			logf("Konfiguration %s nicht aktualisiert: %v", p, err)
		}
	}
}

// serverReachable prüft für das Info-Fenster, ob der Server antwortet.
func serverReachable(cfg Config) bool {
	c := newClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	req, err := c.newReq(ctx, "GET", "/api/state", nil)
	if err != nil {
		return false
	}
	resp, err := c.api.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func infoText() string {
	cfg, err := loadConfig(screenConfigPath())
	if err != nil {
		cfg, err = loadConfig(agentConfigPath())
	}
	host, _ := os.Hostname()
	if err != nil {
		return "Nicolaj Fernwartung ist auf diesem PC nicht (vollständig) installiert.\n\nPC-Name: " + host
	}
	id := fmtPCID(cfg.PCID)
	if id == "" {
		id = "wird beim nächsten Verbinden zugeteilt"
	}
	status := "✓ verbunden"
	if !serverReachable(cfg) {
		status = "✗ Server gerade nicht erreichbar"
	}
	return "Deine PC-ID:   " + id + "\n\nPC-Name:   " + host + "\nIm Dashboard als:   " + cfg.Name + "\nVerbindung:   " + status +
		"\n\nNenne diese PC-ID, wenn du Nicolaj Webdesign um Hilfe bittest."
}

func safeExec(ctx context.Context, cmd Cmd) (res Result) {
	defer func() {
		if r := recover(); r != nil {
			res = Result{OK: false, Output: fmt.Sprintf("Interner Fehler im Agent: %v", r)}
		}
	}()
	return execCommand(ctx, cmd)
}

// ---------- Bildschirm-Helfer ----------

type InputEvent struct {
	T  string  `json:"t"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
	B  int     `json:"b"`
	D  bool    `json:"d"`
	Dy float64 `json:"dy"`
	C  string  `json:"c"`
	S  string  `json:"s"`
}

type screenSess struct {
	id      string
	quality atomic.Value
	active  atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
}

func runScreen() {
	if !singleInstance("Local\\NicolajRMMScreen") {
		return
	}
	initScreenPlatform()
	cfg, err := loadConfig(screenConfigPath())
	if err != nil {
		logf("Bildschirm-Helfer: Konfiguration fehlt (%v)", err)
		time.Sleep(time.Minute)
		os.Exit(1)
	}
	c := newClient(cfg)
	logf("Bildschirm-Helfer %s gestartet", agentVersion)
	startTray(c)

	var mu sync.Mutex
	var cur *screenSess
	go watchForUpdate(func() bool {
		mu.Lock()
		defer mu.Unlock()
		return cur != nil
	})
	stop := func() {
		if cur != nil {
			cur.cancel()
			cur = nil
		}
	}

	runLoop(c, "Bildschirm", func(ev string, data []byte) {
		switch ev {
		case "screen-start":
			var m struct {
				SessionID string `json:"sessionId"`
				Quality   string `json:"quality"`
				Consent   bool   `json:"consent"`
				By        string `json:"by"`
			}
			if json.Unmarshal(data, &m) != nil || m.SessionID == "" {
				return
			}
			mu.Lock()
			stop()
			ctx, cancel := context.WithCancel(context.Background())
			s := &screenSess{id: m.SessionID, ctx: ctx, cancel: cancel}
			s.quality.Store(m.Quality)
			cur = s
			mu.Unlock()
			go func() {
				runSession(c, s, m.Consent, m.By)
				mu.Lock()
				if cur == s {
					cur = nil
				}
				mu.Unlock()
			}()
		case "screen-config":
			var m struct{ SessionID, Quality string }
			json.Unmarshal(data, &m)
			mu.Lock()
			if cur != nil && cur.id == m.SessionID {
				cur.quality.Store(m.Quality)
			}
			mu.Unlock()
		case "screen-stop":
			mu.Lock()
			stop()
			mu.Unlock()
		case "chat": // Antwort von Nicolaj: Nachrichten-Fenster öffnen
			openChatWindow(c, true)
		case "input":
			var m struct {
				SessionID string       `json:"sessionId"`
				Events    []InputEvent `json:"events"`
			}
			if json.Unmarshal(data, &m) != nil {
				return
			}
			mu.Lock()
			ok := cur != nil && cur.id == m.SessionID && cur.active.Load()
			mu.Unlock()
			if ok {
				for _, e := range m.Events {
					applyInput(e)
				}
			}
		case "cmd":
			var cmd Cmd
			if json.Unmarshal(data, &cmd) != nil {
				return
			}
			go func() {
				res := Result{ID: cmd.ID, OK: true}
				var a struct{ Text string }
				json.Unmarshal(cmd.Args, &a)
				switch cmd.Type {
				case "user.message":
					showMessage(a.Text)
					res.Output = "Nachricht wird angezeigt."
				case "user.openSecurity":
					if err := openSecuritySettings(); err != nil {
						res.OK, res.Output = false, err.Error()
					} else {
						res.Output = "Windows-Sicherheit wurde am PC geöffnet (Viren- & Bedrohungsschutz › Einstellungen)."
					}
				case "user.lock":
					if err := lockScreen(); err != nil {
						res.OK, res.Output = false, err.Error()
					} else {
						res.Output = "Bildschirm gesperrt."
					}
				default:
					res.OK, res.Output = false, "Unbekannte Aktion: "+cmd.Type
				}
				c.sendResult(res)
			}()
		}
	}, nil)
}

func runSession(c *Client, s *screenSess, consent bool, by string) {
	status := func(state, msg string, w, h int) {
		c.postJSON("/api/agent/screen-status", map[string]any{"sessionId": s.id, "state": state, "message": msg, "w": w, "h": h}, nil)
	}
	if consent {
		logf("Frage Zustimmung für Bildschirm-Sitzung von %s", by)
		if !askConsent(s.ctx, by) {
			if s.ctx.Err() == nil {
				status("denied", "", 0, 0)
			}
			return
		}
	}
	if s.ctx.Err() != nil {
		return
	}
	ov := showOverlay(by, func() {
		logf("Person am PC hat die Sitzung getrennt")
		status("ended", "", 0, 0)
		s.cancel()
	})
	defer ov.Close()

	capt, err := newCapturer()
	if err != nil {
		status("error", err.Error(), 0, 0)
		return
	}
	defer capt.Close()
	w, h := capt.Size()
	status("active", "", w, h)
	s.active.Store(true)
	logf("Bildschirm-Sitzung %s aktiv (%dx%d)", s.id, w, h)

	var lastSent time.Time
	fails := 0
	for s.ctx.Err() == nil {
		t0 := time.Now()
		q, _ := s.quality.Load().(string)
		jpg, fw, fh, err := capt.Capture(q, time.Since(lastSent) > 2*time.Second)
		if err != nil {
			fails++
			if fails == 1 || fails%50 == 0 {
				logf("Aufnahme fehlgeschlagen: %v", err)
			}
			if fails > 600 {
				status("error", "Bildschirm kann nicht aufgenommen werden: "+err.Error(), 0, 0)
				return
			}
		} else {
			fails = 0
		}
		if jpg != nil {
			code, err := c.postFrame(s.id, jpg, fw, fh)
			if err == nil && (code == 410 || code == 404 || code == 401) {
				logf("Bildschirm-Sitzung %s vom Server beendet", s.id)
				return
			}
			lastSent = time.Now()
		}
		interval := map[string]time.Duration{"high": 70 * time.Millisecond, "low": 160 * time.Millisecond}[q]
		if interval == 0 {
			interval = 100 * time.Millisecond
		}
		select {
		case <-s.ctx.Done():
		case <-time.After(interval - time.Since(t0)):
		}
	}
}

// ---------- Hilfen ----------

var (
	reUser = regexp.MustCompile(`^[^\x00-\x1f\\/\[\]:;|=,+*?<>"@%]{1,20}$`)
	rePkg  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.+_\-]{0,127}$`)
)

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n… (gekürzt)"
}

func argString(a map[string]any, k string) string { s, _ := a[k].(string); return s }
func argBool(a map[string]any, k string) bool     { b, _ := a[k].(bool); return b }
func argInt(a map[string]any, k string) int       { f, _ := a[k].(float64); return int(f) }
func boolEnv(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
