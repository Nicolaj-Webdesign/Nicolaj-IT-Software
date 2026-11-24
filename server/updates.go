package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Settings: globale Einstellungen (Auto-Update).
type Settings struct {
	AutoUpdate      bool   `json:"autoUpdate"`
	TestFirst       bool   `json:"testFirst"`
	ReleasedVersion string `json:"releasedVersion"`

	GitHubRepo    string `json:"githubRepo"`              // z. B. nicolaj-webdesign/nicolaj-it-software
	GitHubToken   string `json:"githubToken,omitempty"`   // nur für private Repos (Leserecht „Contents“)
	GitHubAuto    bool   `json:"githubAuto"`              // neue Agent-Versionen automatisch übernehmen
	ReleasePubKey string `json:"releasePubKey,omitempty"` // Ed25519, verlangt signierte Releases

	DisableRequests bool `json:"disableRequests,omitempty"` // allgemeiner Installer: keine Zugangsanfragen annehmen
}

// AgentPkg ist eine Agent-Version, die der Server an die PCs verteilt.
type AgentPkg struct {
	Version    string    `json:"version"`
	Sha256     string    `json:"sha256"`
	Size       int       `json:"size"`
	Source     string    `json:"source"` // "eingebaut" oder "hochgeladen"
	UploadedAt time.Time `json:"uploadedAt,omitempty"`
	data       []byte
}

var (
	pkgMu     sync.Mutex
	pkgCached *AgentPkg
	pkgLoaded bool
)

const versionMarker = "NRMM-AGENT-VERSION:"

// agentVersionOf liest die Version aus der Agent-Datei (der Agent trägt eine Markierung in sich).
func agentVersionOf(b []byte) string {
	i := bytes.Index(b, []byte(versionMarker))
	if i < 0 {
		return ""
	}
	rest := b[i+len(versionMarker):]
	j := bytes.Index(rest, []byte(":END"))
	if j <= 0 || j > 32 {
		return ""
	}
	return string(rest[:j])
}

func parseVersion(v string) []int {
	var out []int
	for _, p := range strings.Split(strings.TrimPrefix(v, "v"), ".") {
		n, _ := strconv.Atoi(p)
		out = append(out, n)
	}
	return out
}

// versionLess: true, wenn a älter als b ist.
func versionLess(a, b string) bool {
	pa, pb := parseVersion(a), parseVersion(b)
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			return x < y
		}
	}
	return false
}

func newPkg(b []byte, source string) *AgentPkg {
	v := agentVersionOf(b)
	if v == "" || len(b) < 2 || b[0] != 'M' || b[1] != 'Z' {
		return nil
	}
	sum := sha256.Sum256(b)
	return &AgentPkg{Version: v, Sha256: hex.EncodeToString(sum[:]), Size: len(b), Source: source, data: b}
}

func (a *App) uploadPath() string {
	return filepath.Join(a.dataDir, "agent-update", "nicolaj-agent.exe")
}

// currentAgent liefert die neueste verfügbare Agent-Version (eingebaut oder hochgeladen).
func (a *App) currentAgent() *AgentPkg {
	pkgMu.Lock()
	defer pkgMu.Unlock()
	if pkgLoaded {
		return pkgCached
	}
	pkgLoaded = true
	var best *AgentPkg
	if b := agentBinary(); b != nil {
		best = newPkg(b, "eingebaut")
	}
	if b, err := os.ReadFile(a.uploadPath()); err == nil {
		src := "hochgeladen"
		if m, err := os.ReadFile(a.uploadPath() + ".quelle"); err == nil && strings.TrimSpace(string(m)) != "" {
			src = strings.TrimSpace(string(m))
		}
		if up := newPkg(b, src); up != nil {
			if st, err := os.Stat(a.uploadPath()); err == nil {
				up.UploadedAt = st.ModTime()
			}
			if best == nil || !versionLess(up.Version, best.Version) {
				best = up
			}
		}
	}
	pkgCached = best
	return best
}

func invalidatePkg() {
	pkgMu.Lock()
	pkgLoaded, pkgCached = false, nil
	pkgMu.Unlock()
}

func (a *App) settings() Settings {
	var s Settings
	a.store.View(func(d *DBData) { s = *d.Settings })
	return s
}

// eligible: Soll dieses Gerät automatisch auf pkg aktualisiert werden?
func eligible(dev *Device, s Settings, pkg *AgentPkg) bool {
	if pkg == nil || !s.AutoUpdate || dev.AgentVersion == "" || !versionLess(dev.AgentVersion, pkg.Version) {
		return false
	}
	return dev.UpdateTester || !s.TestFirst || s.ReleasedVersion == pkg.Version
}

func updateInfo(pkg *AgentPkg) map[string]any {
	return map[string]any{"version": pkg.Version, "sha256": pkg.Sha256, "size": pkg.Size}
}

// pushUpdates bietet allen berechtigten, verbundenen PCs das Update an.
func (a *App) pushUpdates(only string) {
	pkg := a.currentAgent()
	if pkg == nil {
		return
	}
	s := a.settings()
	var ids []string
	a.store.View(func(d *DBData) {
		for id, dev := range d.Devices {
			if (only == "" || only == id) && eligible(dev, s, pkg) {
				ids = append(ids, id)
			}
		}
	})
	for _, id := range ids {
		a.hub.SendToAgent(id, "agent", "update", updateInfo(pkg))
	}
}

// ---------- Schnittstellen ----------

// Agent lädt die aktuelle Version (über die geprüfte HTTPS-Verbindung).
func (a *App) agentDownloadUpdate(w http.ResponseWriter, r *http.Request, devID, role string) {
	if role != "agent" {
		writeErr(w, 403, "Nicht erlaubt.")
		return
	}
	pkg := a.currentAgent()
	if pkg == nil {
		writeErr(w, 404, "Kein Update verfügbar.")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Version", pkg.Version)
	w.Header().Set("X-Sha256", pkg.Sha256)
	w.Header().Set("Content-Length", strconv.Itoa(pkg.Size))
	w.Write(pkg.data)
}

func (a *App) apiUpdates(w http.ResponseWriter, r *http.Request, user string) {
	pkg := a.currentAgent()
	s := a.settings()
	live := a.hub.Live()
	versions := map[string]int{}
	outdated, waiting, testers := 0, 0, 0
	a.store.View(func(d *DBData) {
		for id, dev := range d.Devices {
			v := dev.AgentVersion
			if v == "" {
				v = "?"
			}
			versions[v]++
			if dev.UpdateTester {
				testers++
			}
			if pkg != nil && dev.AgentVersion != "" && versionLess(dev.AgentVersion, pkg.Version) {
				outdated++
				if !eligible(dev, s, pkg) || !live[id].Agent {
					waiting++
				}
			}
		}
	})
	resp := map[string]any{"settings": s, "available": pkg, "versions": versions, "outdated": outdated, "waiting": waiting, "testers": testers,
		"released": pkg != nil && (!s.TestFirst || s.ReleasedVersion == pkg.Version)}
	writeJSON(w, 200, resp)
}

func (a *App) apiUpdateSettings(w http.ResponseWriter, r *http.Request, user string) {
	var req struct {
		AutoUpdate *bool `json:"autoUpdate"`
		TestFirst  *bool `json:"testFirst"`
	}
	if !readJSON(w, r, &req, 1024) {
		return
	}
	var changes []string
	a.store.Update(func(d *DBData) {
		if req.AutoUpdate != nil && *req.AutoUpdate != d.Settings.AutoUpdate {
			d.Settings.AutoUpdate = *req.AutoUpdate
			changes = append(changes, map[bool]string{true: "Auto-Update an", false: "Auto-Update aus"}[*req.AutoUpdate])
		}
		if req.TestFirst != nil && *req.TestFirst != d.Settings.TestFirst {
			d.Settings.TestFirst = *req.TestFirst
			changes = append(changes, map[bool]string{true: "Neue Versionen zuerst an Testgeräte", false: "Neue Versionen sofort an alle"}[*req.TestFirst])
		}
	})
	if len(changes) > 0 {
		a.audit(r, user, "", "Update-Einstellungen", strings.Join(changes, "; "))
	}
	a.pushUpdates("")
	a.apiUpdates(w, r, user)
}

func (a *App) apiUpdateRelease(w http.ResponseWriter, r *http.Request, user string) {
	pkg := a.currentAgent()
	if pkg == nil {
		writeErr(w, 404, "Keine Agent-Version vorhanden.")
		return
	}
	a.store.Update(func(d *DBData) { d.Settings.ReleasedVersion = pkg.Version })
	a.audit(r, user, "", "Update freigegeben", "Agent "+pkg.Version+" für alle PCs freigegeben")
	a.pushUpdates("")
	a.apiUpdates(w, r, user)
}

// apiUpdateUpload nimmt eine neue nicolaj-agent.exe entgegen (ohne den Server neu zu bauen).
func (a *App) apiUpdateUpload(w http.ResponseWriter, r *http.Request, user string) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 80<<20))
	if err != nil {
		writeErr(w, 400, "Upload fehlgeschlagen.")
		return
	}
	pkg := newPkg(body, "hochgeladen")
	if pkg == nil {
		writeErr(w, 400, "Das ist keine gültige nicolaj-agent.exe (Versionskennung fehlt).")
		return
	}
	if cur := a.currentAgent(); cur != nil && versionLess(pkg.Version, cur.Version) {
		writeErr(w, 409, fmt.Sprintf("Version %s ist älter als die aktuelle (%s).", pkg.Version, cur.Version))
		return
	}
	if err := a.storeAgent(body, "hochgeladen"); err != nil {
		writeErr(w, 500, "Speichern fehlgeschlagen.")
		return
	}
	a.audit(r, user, "", "Agent-Version hochgeladen", fmt.Sprintf("Version %s (SHA-256 %s…)", pkg.Version, pkg.Sha256[:12]))
	a.pushUpdates("")
	a.apiUpdates(w, r, user)
}

// storeAgent speichert eine neue Agent-Version als „hochgeladen“.
func (a *App) storeAgent(body []byte, source string) error {
	os.MkdirAll(filepath.Dir(a.uploadPath()), 0o700)
	tmp := a.uploadPath() + ".tmp"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, a.uploadPath()); err != nil {
		return err
	}
	os.WriteFile(a.uploadPath()+".quelle", []byte(source), 0o600)
	invalidatePkg()
	return nil
}
