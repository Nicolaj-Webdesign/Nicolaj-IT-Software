package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// ---------- Geräte ----------

func deviceView(dev *Device, ls liveState, full bool) map[string]any {
	v := map[string]any{
		"id": dev.ID, "pcId": dev.PCID, "name": dev.Name, "hostname": dev.Hostname, "notes": dev.Notes,
		"protectionPausedUntil": dev.PausedUntil,
		"consentRequired":       dev.ConsentRequired, "securityEnabled": dev.SecurityEnabled, "updateTester": dev.UpdateTester, "agentVersion": dev.AgentVersion,
		"enrolledAt": dev.EnrolledAt, "lastSeen": dev.LastSeen, "lastIp": dev.LastIP,
		"online": ls.Agent, "screenOnline": ls.Screen, "session": ls.Session,
		"inventory": dev.Inventory, "inventoryAt": dev.InventoryAt,
		"pending": dev.Pending, "requestedAt": dev.RequestedAt, "group": dev.Group,
	}
	if len(dev.Inventory) == 0 {
		v["inventory"] = nil
	}
	if full {
		v["data"] = dev.Data
	}
	return v
}

func (a *App) broadcastDevice(devID string) {
	live := a.hub.Live()
	var v map[string]any
	a.store.View(func(d *DBData) {
		if dev := d.Devices[devID]; dev != nil {
			v = deviceView(dev, live[devID], false)
		}
	})
	if v != nil {
		a.hub.Broadcast("device", v)
	}
}

func (a *App) apiDevices(w http.ResponseWriter, r *http.Request, user string) {
	live := a.hub.Live()
	var out []map[string]any
	a.store.View(func(d *DBData) {
		for _, dev := range d.Devices {
			out = append(out, deviceView(dev, live[dev.ID], false))
		}
	})
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i]["name"].(string)) < strings.ToLower(out[j]["name"].(string))
	})
	if out == nil {
		out = []map[string]any{}
	}
	writeJSON(w, 200, out)
}

func (a *App) apiDevice(w http.ResponseWriter, r *http.Request, user string) {
	id := r.PathValue("id")
	live := a.hub.Live()
	var v map[string]any
	a.store.View(func(d *DBData) {
		if dev := d.Devices[id]; dev != nil {
			v = deviceView(dev, live[id], true)
		}
	})
	if v == nil {
		writeErr(w, 404, "Gerät nicht gefunden.")
		return
	}
	writeJSON(w, 200, v)
}

func (a *App) apiDeviceUpdate(w http.ResponseWriter, r *http.Request, user string) {
	id := r.PathValue("id")
	var req struct {
		Name            *string `json:"name"`
		Notes           *string `json:"notes"`
		Group           *string `json:"group"`
		ConsentRequired *bool   `json:"consentRequired"`
		SecurityEnabled *bool   `json:"securityEnabled"`
		UpdateTester    *bool   `json:"updateTester"`
	}
	if !readJSON(w, r, &req, 16384) {
		return
	}
	var changes []string
	found := false
	a.store.Update(func(d *DBData) {
		dev := d.Devices[id]
		if dev == nil {
			return
		}
		found = true
		if req.Name != nil {
			n := strings.TrimSpace(*req.Name)
			if n != "" && len([]rune(n)) <= 64 && n != dev.Name {
				changes = append(changes, fmt.Sprintf("Name: „%s“ → „%s“", dev.Name, n))
				dev.Name = n
			}
		}
		if req.Group != nil {
			g := cleanName(*req.Group)
			if len([]rune(g)) > 30 {
				g = string([]rune(g)[:30])
			}
			if g != dev.Group {
				changes = append(changes, fmt.Sprintf("Gruppe: „%s“", map[bool]string{true: "keine", false: g}[g == ""]))
				dev.Group = g
			}
		}
		if req.Notes != nil && len(*req.Notes) <= 4000 && *req.Notes != dev.Notes {
			dev.Notes = *req.Notes
			changes = append(changes, "Notizen geändert")
		}
		if req.ConsentRequired != nil && *req.ConsentRequired != dev.ConsentRequired {
			dev.ConsentRequired = *req.ConsentRequired
			if dev.ConsentRequired {
				changes = append(changes, "Zustimmung vor Fernsteuerung: an")
			} else {
				changes = append(changes, "Zustimmung vor Fernsteuerung: aus")
			}
		}
		if req.UpdateTester != nil && *req.UpdateTester != dev.UpdateTester {
			dev.UpdateTester = *req.UpdateTester
			changes = append(changes, map[bool]string{true: "Testgerät für Updates", false: "kein Testgerät mehr"}[dev.UpdateTester])
		}
		if req.SecurityEnabled != nil && *req.SecurityEnabled != dev.SecurityEnabled {
			dev.SecurityEnabled = *req.SecurityEnabled
			changes = append(changes, map[bool]string{true: "Sicherheitspaket aktiviert", false: "Sicherheitspaket deaktiviert"}[dev.SecurityEnabled])
		}
	})
	if !found {
		writeErr(w, 404, "Gerät nicht gefunden.")
		return
	}
	if len(changes) > 0 {
		a.audit(r, user, id, "Gerät bearbeitet", strings.Join(changes, "; "))
	}
	if req.Name != nil {
		a.sendConfig(id)
	}
	if req.UpdateTester != nil {
		a.pushUpdates(id)
	}
	a.broadcastDevice(id)
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *App) apiDeviceDelete(w http.ResponseWriter, r *http.Request, user string) {
	id := r.PathValue("id")
	var name string
	a.store.Update(func(d *DBData) {
		if dev := d.Devices[id]; dev != nil {
			name = dev.Name
			delete(d.Devices, id)
			delete(d.Chats, id)
		}
	})
	if name != "" {
		a.removeChatImages(id)
	}
	if name == "" {
		writeErr(w, 404, "Gerät nicht gefunden.")
		return
	}
	a.hub.DropDevice(id)
	a.audit(r, user, id, "Gerät gelöscht", "„"+name+"“ aus dem Dashboard entfernt")
	a.hub.Broadcast("device-removed", map[string]any{"id": id})
	writeJSON(w, 200, map[string]any{"ok": true})
}

// ---------- Aufträge ----------

func (a *App) apiDeviceCommands(w http.ResponseWriter, r *http.Request, user string) {
	id := r.PathValue("id")
	out := []Command{}
	a.store.View(func(d *DBData) {
		for i := len(d.Commands) - 1; i >= 0 && len(out) < 150; i-- {
			if d.Commands[i].DeviceID == id {
				out = append(out, *d.Commands[i])
			}
		}
	})
	writeJSON(w, 200, out)
}

func (a *App) apiCreateCommand(w http.ResponseWriter, r *http.Request, user string) {
	devID := r.PathValue("id")
	var req struct {
		Type string          `json:"type"`
		Args json.RawMessage `json:"args"`
	}
	if !readJSON(w, r, &req, 256<<10) {
		return
	}
	spec, ok := commandSpecs[req.Type]
	if !ok {
		writeErr(w, 400, "Unbekannte Aktion.")
		return
	}
	full, redacted, err := cleanArgs(spec, req.Args)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	exists, secOn, pending := false, false, false
	a.store.View(func(d *DBData) {
		if dev := d.Devices[devID]; dev != nil {
			exists, secOn, pending = true, dev.SecurityEnabled, dev.Pending
		}
	})
	if !exists {
		writeErr(w, 404, "Gerät nicht gefunden.")
		return
	}
	if pending {
		writeErr(w, 409, pendingMsg)
		return
	}
	if strings.HasPrefix(req.Type, "security.") && !secOn {
		writeErr(w, 409, "Das Sicherheitspaket ist für diesen PC nicht aktiviert (Gerät › Sicherheit).")
		return
	}
	agentOn, screenOn := a.hub.Online(devID)
	if spec.Target == "screen" && !screenOn {
		writeErr(w, 409, "Gerade ist niemand am PC angemeldet (Bildschirm-Helfer offline).")
		return
	}
	timeout := spec.Timeout
	if t, ok := full["timeout"].(int); ok && t > 0 {
		timeout = t
	}
	fullJSON, _ := json.Marshal(full)
	redJSON, _ := json.Marshal(redacted)
	cmd := &Command{
		ID: randHex(8), DeviceID: devID, Type: req.Type, Summary: spec.Summary(full), Args: redJSON,
		Status: "queued", CreatedBy: user, CreatedAt: time.Now(), Timeout: timeout,
	}
	a.hub.PutSecret(cmd.ID, fullJSON)
	a.store.Update(func(d *DBData) { d.Commands = append(d.Commands, cmd) })
	detail := cmd.Summary
	if !agentOn && spec.Target == "agent" {
		detail += " (wartet, PC offline)"
	}
	a.audit(r, user, devID, "Auftrag", detail)
	a.hub.Broadcast("command", *cmd)
	a.dispatch(cmd.ID)
	var cp Command
	a.store.View(func(d *DBData) { cp = *d.FindCommand(cmd.ID) })
	writeJSON(w, 200, cp)
}

func (a *App) apiCancelCommand(w http.ResponseWriter, r *http.Request, user string) {
	id := r.PathValue("id")
	var cp Command
	a.store.Update(func(d *DBData) {
		if c := d.FindCommand(id); c != nil && c.Status == "queued" {
			now := time.Now()
			c.Status, c.Output, c.DoneAt = "canceled", "Abgebrochen von "+user, &now
			cp = *c
		}
	})
	if cp.ID == "" {
		writeErr(w, 409, "Nur wartende Aufträge können abgebrochen werden.")
		return
	}
	a.hub.TakeSecret(id, true)
	a.audit(r, user, cp.DeviceID, "Auftrag abgebrochen", cp.Summary)
	a.hub.Broadcast("command", cp)
	writeJSON(w, 200, cp)
}

// ---------- Bildschirm ----------

var qualities = map[string]bool{"high": true, "medium": true, "low": true}

func (a *App) apiScreenStart(w http.ResponseWriter, r *http.Request, user string) {
	devID := r.PathValue("id")
	var req struct{ Quality string }
	readJSON(w, r, &req, 1024)
	if !qualities[req.Quality] {
		req.Quality = "medium"
	}
	var consent, exists, pending bool
	a.store.View(func(d *DBData) {
		if dev := d.Devices[devID]; dev != nil {
			exists, consent, pending = true, dev.ConsentRequired, dev.Pending
		}
	})
	if !exists {
		writeErr(w, 404, "Gerät nicht gefunden.")
		return
	}
	if pending {
		writeErr(w, 409, pendingMsg)
		return
	}
	if _, screenOn := a.hub.Online(devID); !screenOn {
		writeErr(w, 409, "Gerade ist niemand am PC angemeldet. Der Bildschirm ist erst nach der Windows-Anmeldung verfügbar.")
		return
	}
	info, isNew := a.hub.StartSession(devID, req.Quality, user)
	if isNew {
		a.hub.SendToAgent(devID, "screen", "screen-start", map[string]any{
			"sessionId": info["id"], "quality": req.Quality, "consent": consent, "by": user,
		})
		a.audit(r, user, devID, "Bildschirm-Sitzung gestartet", map[bool]string{true: "mit Zustimmungsabfrage", false: "ohne Zustimmungsabfrage"}[consent])
		a.hub.Broadcast("screen", map[string]any{"deviceId": devID, "session": info})
	}
	writeJSON(w, 200, info)
}

func (a *App) apiScreenStop(w http.ResponseWriter, r *http.Request, user string) {
	devID := r.PathValue("id")
	if a.hub.EndSession(devID, "") {
		a.hub.SendToAgent(devID, "screen", "screen-stop", map[string]any{})
		a.audit(r, user, devID, "Bildschirm-Sitzung beendet", "")
		a.hub.Broadcast("screen", map[string]any{"deviceId": devID, "session": nil, "reason": "Verbindung getrennt."})
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *App) apiScreenConfig(w http.ResponseWriter, r *http.Request, user string) {
	devID := r.PathValue("id")
	var req struct{ SessionID, Quality string }
	if !readJSON(w, r, &req, 1024) {
		return
	}
	if !qualities[req.Quality] || !a.hub.SetQuality(devID, req.SessionID, req.Quality) {
		writeErr(w, 409, "Keine laufende Sitzung.")
		return
	}
	a.hub.SendToAgent(devID, "screen", "screen-config", map[string]any{"sessionId": req.SessionID, "quality": req.Quality})
	writeJSON(w, 200, map[string]any{"ok": true})
}

type inputEvent struct {
	T  string  `json:"t"`
	X  float64 `json:"x,omitempty"`
	Y  float64 `json:"y,omitempty"`
	B  int     `json:"b,omitempty"`
	D  bool    `json:"d,omitempty"`
	Dy float64 `json:"dy,omitempty"`
	C  string  `json:"c,omitempty"`
	S  string  `json:"s,omitempty"`
}

func (a *App) apiScreenInput(w http.ResponseWriter, r *http.Request, user string) {
	devID := r.PathValue("id")
	var req struct {
		SessionID string       `json:"sessionId"`
		Events    []inputEvent `json:"events"`
	}
	if !readJSON(w, r, &req, 64<<10) {
		return
	}
	if req.SessionID == "" || a.hub.ActiveSessionID(devID) != req.SessionID {
		writeErr(w, 409, "Keine aktive Sitzung.")
		return
	}
	clean := make([]inputEvent, 0, len(req.Events))
	for i, e := range req.Events {
		if i >= 300 {
			break
		}
		switch e.T {
		case "m", "b", "w":
			if e.X < 0 || e.X > 1 || e.Y < 0 || e.Y > 1 || e.B < 0 || e.B > 2 || e.Dy < -10000 || e.Dy > 10000 {
				continue
			}
		case "k":
			if len(e.C) == 0 || len(e.C) > 32 {
				continue
			}
		case "text":
			if len([]rune(e.S)) > 2000 {
				continue
			}
		default:
			continue
		}
		clean = append(clean, e)
	}
	if len(clean) > 0 {
		a.hub.SendToAgent(devID, "screen", "input", map[string]any{"sessionId": req.SessionID, "events": clean})
	}
	w.WriteHeader(204)
}

// apiScreenStream liefert das Live-Bild als MJPEG – jeder Browser zeigt das direkt in einem <img> an.
func (a *App) apiScreenStream(w http.ResponseWriter, r *http.Request, user string) {
	devID := r.PathValue("id")
	ch, first, ok := a.hub.AddViewer(devID, r.URL.Query().Get("session"))
	if !ok {
		writeErr(w, 404, "Keine Sitzung.")
		return
	}
	defer a.hub.RemoveViewer(devID, ch)
	fl, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=nrmmframe")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(200)
	send := func(f []byte) bool {
		if _, err := fmt.Fprintf(w, "--nrmmframe\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", len(f)); err != nil {
			return false
		}
		if _, err := w.Write(f); err != nil {
			return false
		}
		if _, err := io.WriteString(w, "\r\n"); err != nil {
			return false
		}
		if fl != nil {
			fl.Flush()
		}
		return true
	}
	if len(first) > 0 && !send(first) {
		return
	}
	for {
		select {
		case f, ok := <-ch:
			if !ok || !send(f) {
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}

// ---------- Einrichtungscodes & Installer ----------

func (a *App) apiEnrollCodes(w http.ResponseWriter, r *http.Request, user string) {
	out := []EnrollCode{}
	a.store.View(func(d *DBData) {
		for i := len(d.EnrollCodes) - 1; i >= 0; i-- {
			out = append(out, *d.EnrollCodes[i])
		}
	})
	writeJSON(w, 200, out)
}

func (a *App) apiCreateEnrollCode(w http.ResponseWriter, r *http.Request, user string) {
	var req struct {
		Label string `json:"label"`
		Hours int    `json:"hours"`
	}
	if !readJSON(w, r, &req, 4096) {
		return
	}
	req.Label = strings.TrimSpace(req.Label)
	if len([]rune(req.Label)) > 64 {
		writeErr(w, 400, "Name: höchstens 64 Zeichen.")
		return
	}
	if req.Hours < 1 || req.Hours > 24*14 {
		req.Hours = 24
	}
	ec := &EnrollCode{Code: newShortCode(), Label: req.Label, CreatedBy: user, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Duration(req.Hours) * time.Hour)}
	a.store.Update(func(d *DBData) { d.EnrollCodes = append(d.EnrollCodes, ec) })
	a.audit(r, user, "", "Einrichtungscode erstellt", fmt.Sprintf("%s für „%s“, %d Std. gültig", ec.Code, ec.Label, req.Hours))
	writeJSON(w, 200, ec)
}

func (a *App) apiDeleteEnrollCode(w http.ResponseWriter, r *http.Request, user string) {
	code := r.PathValue("code")
	a.store.Update(func(d *DBData) {
		out := d.EnrollCodes[:0]
		for _, e := range d.EnrollCodes {
			if e.Code != code {
				out = append(out, e)
			}
		}
		d.EnrollCodes = out
	})
	a.audit(r, user, "", "Einrichtungscode gelöscht", code)
	writeJSON(w, 200, map[string]any{"ok": true})
}

func agentBinary() []byte {
	b, err := assetsFS.ReadFile("assets/nicolaj-agent.exe")
	if err != nil {
		return nil
	}
	return b
}

// apiInstaller liefert den Agent mit eingebautem Code + Serveradresse: Doppelklick genügt.
func (a *App) apiInstaller(w http.ResponseWriter, r *http.Request, user string) {
	code := r.PathValue("code")
	var ec *EnrollCode
	a.store.View(func(d *DBData) {
		for _, e := range d.EnrollCodes {
			if e.Code == code {
				cp := *e
				ec = &cp
			}
		}
	})
	if ec == nil || ec.UsedAt != nil || time.Now().After(ec.ExpiresAt) {
		writeErr(w, 404, "Code ungültig, benutzt oder abgelaufen.")
		return
	}
	server := r.URL.Query().Get("server")
	u, err := url.Parse(server)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.Path != "" && u.Path != "/" {
		writeErr(w, 400, "Ungültige Serveradresse.")
		return
	}
	var bin []byte
	if pkg := a.currentAgent(); pkg != nil {
		bin = pkg.data
	}
	if bin == nil {
		writeErr(w, 404, "Im Server ist noch kein Agent enthalten. Bitte mit build.sh bzw. build.ps1 bauen.")
		return
	}
	payload, _ := json.Marshal(map[string]string{
		"server": u.Scheme + "://" + u.Host, "code": ec.Code, "fingerprint": a.fingerprint, "label": ec.Label,
	})
	var buf bytes.Buffer
	buf.Grow(len(bin) + len(payload) + 12)
	buf.Write(bin)
	buf.Write(payload)
	binary.Write(&buf, binary.LittleEndian, uint32(len(payload)))
	buf.WriteString("NRMMCFG1")
	w.Header().Set("Content-Type", "application/vnd.microsoft.portable-executable")
	w.Header().Set("Content-Disposition", `attachment; filename="Nicolaj-Fernwartung-Setup.exe"`)
	w.Write(buf.Bytes())
	a.audit(r, user, "", "Installer heruntergeladen", "Code "+ec.Code)
}

func (a *App) apiDownloadAgent(w http.ResponseWriter, r *http.Request, user string) {
	var bin []byte
	if pkg := a.currentAgent(); pkg != nil {
		bin = pkg.data
	}
	if bin == nil {
		writeErr(w, 404, "Im Server ist noch kein Agent enthalten. Bitte mit build.sh bzw. build.ps1 bauen.")
		return
	}
	w.Header().Set("Content-Type", "application/vnd.microsoft.portable-executable")
	w.Header().Set("Content-Disposition", `attachment; filename="nicolaj-agent.exe"`)
	w.Write(bin)
}

// ---------- Sonstiges ----------

func (a *App) apiServerInfo(w http.ResponseWriter, r *http.Request, user string) {
	scheme := a.scheme(r)
	writeJSON(w, 200, map[string]any{
		"version": serverVersion, "tls": a.tls, "behindProxy": a.behindProxy, "container": a.container,
		"fingerprint": a.fingerprint, "fingerprintPretty": fmtFingerprint(a.fingerprint),
		"addresses": a.serverURLs(scheme), "agentAvailable": a.currentAgent() != nil,
		"current": scheme + "://" + r.Host, "agentLatest": latestVersion(a.currentAgent()),
	})
}

func (a *App) apiAudit(w http.ResponseWriter, r *http.Request, user string) {
	out := []AuditEntry{}
	names := map[string]string{}
	a.store.View(func(d *DBData) {
		for i := len(d.Audit) - 1; i >= 0 && len(out) < 1000; i-- {
			out = append(out, *d.Audit[i])
		}
		for id, dev := range d.Devices {
			names[id] = dev.Name
		}
	})
	writeJSON(w, 200, map[string]any{"entries": out, "deviceNames": names})
}

func (a *App) apiEvents(w http.ResponseWriter, r *http.Request, user string) {
	fl, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, 500, "Streaming nicht möglich.")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	c := a.hub.AddDash()
	defer a.hub.RemoveDash(c)
	io.WriteString(w, "event: hello\ndata: {}\n\n")
	fl.Flush()
	ka := time.NewTicker(20 * time.Second)
	defer ka.Stop()
	for {
		select {
		case m := <-c.ch:
			if _, err := w.Write(m); err != nil {
				return
			}
			fl.Flush()
		case <-ka.C:
			if _, err := io.WriteString(w, ": ka\n\n"); err != nil {
				return
			}
			fl.Flush()
		case <-c.closed:
			return
		case <-r.Context().Done():
			return
		}
	}
}

func latestVersion(p *AgentPkg) string {
	if p == nil {
		return ""
	}
	return p.Version
}
