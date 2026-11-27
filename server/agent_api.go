package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ---------- Anmeldung eines neuen PCs ----------

func (a *App) agentEnroll(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if blocked, wait := a.limiter.Blocked("enroll:" + ip); blocked {
		writeErr(w, 429, fmt.Sprintf("Zu viele Versuche. Bitte in %s erneut versuchen.", wait))
		return
	}
	var req struct{ Code, Hostname, OS, Version string }
	if !readJSON(w, r, &req, 8192) {
		return
	}
	code := normalizeCode(req.Code)
	host := strings.TrimSpace(req.Hostname)
	if len(host) > 64 {
		host = host[:64]
	}
	devID := randHex(8)
	agentTok := devID + "." + randHex(32)
	screenTok := devID + "." + randHex(32)
	var name, errMsg, newPC string
	a.store.Update(func(d *DBData) {
		var ec *EnrollCode
		for _, e := range d.EnrollCodes {
			if normalizeCode(e.Code) == code {
				ec = e
			}
		}
		switch {
		case ec == nil:
			errMsg = "Der Einrichtungscode ist unbekannt."
			return
		case ec.UsedAt != nil:
			errMsg = "Dieser Einrichtungscode wurde schon benutzt. Bitte im Dashboard einen neuen erstellen."
			return
		case time.Now().After(ec.ExpiresAt):
			errMsg = "Der Einrichtungscode ist abgelaufen. Bitte im Dashboard einen neuen erstellen."
			return
		}
		now := time.Now()
		name = ec.Label
		if name == "" {
			name = host
		}
		if name == "" {
			name = "Neuer PC"
		}
		pcID := d.newPCID()
		d.Devices[devID] = &Device{
			ID: devID, PCID: pcID, Name: name, Hostname: host, ConsentRequired: true,
			AgentTokenHash: sha256Hex(agentTok), ScreenTokenHash: sha256Hex(screenTok),
			AgentVersion: req.Version, EnrolledAt: now, LastSeen: now, LastIP: ip,
			Data: map[string]DataSnapshot{},
		}
		ec.UsedAt, ec.DeviceID = &now, devID
		d.Audit = append(d.Audit, &AuditEntry{Time: now, User: ec.CreatedBy, IP: ip, DeviceID: devID,
			Action: "Gerät hinzugefügt", Detail: fmt.Sprintf("%s (%s), PC-ID %s, Code %s", name, host, fmtPCID(pcID), ec.Code)})
		newPC = pcID
	})
	if errMsg != "" {
		a.limiter.Fail("enroll:" + ip)
		writeErr(w, 403, errMsg)
		return
	}
	a.broadcastDevice(devID)
	writeJSON(w, 200, map[string]any{"deviceId": devID, "pcId": newPC, "agentToken": agentTok, "screenToken": screenTok, "name": name})
}

// ---------- Dauerverbindung: Server → Agent ----------

func (a *App) agentEvents(w http.ResponseWriter, r *http.Request, devID, role string) {
	fl, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, 500, "Streaming nicht möglich.")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	fmt.Fprint(w, ": verbunden\n\n")
	fl.Flush()

	conn := a.hub.RegisterAgent(devID, role)
	ip := clientIP(r)
	ver := r.Header.Get("X-Agent-Version")
	a.store.Update(func(d *DBData) {
		if dev := d.Devices[devID]; dev != nil {
			dev.LastSeen, dev.LastIP = time.Now(), ip
			if role == "agent" && ver != "" {
				dev.AgentVersion = ver
			}
		}
	})
	a.broadcastDevice(devID)
	if role == "agent" {
		a.sendConfig(devID)
		a.sendQueued(devID)
		a.pushUpdates(devID)
	}
	if role == "screen" {
		a.pushUnreadChat(devID, conn)
	}
	defer func() {
		if a.hub.UnregisterAgent(devID, role, conn) {
			a.hub.Broadcast("screen", map[string]any{"deviceId": devID, "session": nil, "reason": "Der Bildschirm-Helfer hat die Verbindung verloren."})
		}
		a.store.Update(func(d *DBData) {
			if dev := d.Devices[devID]; dev != nil {
				dev.LastSeen = time.Now()
			}
		})
		a.broadcastDevice(devID)
	}()

	ka := time.NewTicker(15 * time.Second)
	defer ka.Stop()
	for {
		select {
		case msg := <-conn.ch:
			if _, err := w.Write(msg); err != nil {
				return
			}
			fl.Flush()
		case <-ka.C:
			if _, err := io.WriteString(w, ": ka\n\n"); err != nil {
				return
			}
			fl.Flush()
		case <-conn.closed:
			return
		case <-r.Context().Done():
			return
		}
	}
}

// sendConfig teilt dem PC seine PC-ID und seinen Namen mit (für das Info-Fenster im Startmenü).
func (a *App) sendConfig(devID string) {
	var pc, name string
	a.store.View(func(d *DBData) {
		if dev := d.Devices[devID]; dev != nil {
			pc, name = dev.PCID, dev.Name
		}
	})
	if pc != "" {
		a.hub.SendToAgent(devID, "agent", "config", map[string]any{"pcId": pc, "name": name})
	}
}

func fmtPCID(s string) string {
	if len(s) != 9 {
		return s
	}
	return s[:3] + " " + s[3:6] + " " + s[6:]
}

func (a *App) sendQueued(devID string) {
	var ids []string
	a.store.View(func(d *DBData) {
		for _, c := range d.Commands {
			if c.DeviceID == devID && c.Status == "queued" && commandSpecs[c.Type].Target == "agent" {
				ids = append(ids, c.ID)
			}
		}
	})
	for _, id := range ids {
		a.dispatch(id)
	}
}

// dispatch schickt einen Auftrag an den PC, falls er online ist.
func (a *App) dispatch(cmdID string) {
	var c Command
	found := false
	a.store.View(func(d *DBData) {
		if p := d.FindCommand(cmdID); p != nil && p.Status == "queued" {
			c, found = *p, true
		}
	})
	if !found {
		return
	}
	spec := commandSpecs[c.Type]
	args := a.hub.TakeSecret(c.ID, false)
	if args == nil {
		if commandHasSecrets(c.Type) {
			return
		}
		args = c.Args
	}
	payload := map[string]any{"id": c.ID, "type": c.Type, "args": args, "timeout": c.Timeout}
	if !a.hub.SendToAgent(c.DeviceID, spec.Target, "cmd", payload) {
		return
	}
	var cp Command
	a.store.Update(func(d *DBData) {
		if p := d.FindCommand(cmdID); p != nil && p.Status == "queued" {
			now := time.Now()
			p.Status, p.SentAt = "sent", &now
			cp = *p
		}
	})
	if cp.ID != "" {
		a.hub.Broadcast("command", cp)
	}
}

// ---------- Ergebnisse ----------

func (a *App) agentResult(w http.ResponseWriter, r *http.Request, devID, role string) {
	var req struct {
		ID     string          `json:"id"`
		OK     bool            `json:"ok"`
		Output string          `json:"output"`
		Data   json.RawMessage `json:"data"`
	}
	if !readJSON(w, r, &req, 8<<20) {
		return
	}
	var cp Command
	found := false
	a.store.Update(func(d *DBData) {
		c := d.FindCommand(req.ID)
		if c == nil || c.DeviceID != devID || c.Final() || commandSpecs[c.Type].Target != role {
			return
		}
		now := time.Now()
		c.Status = "failed"
		if req.OK {
			c.Status = "done"
		}
		c.Output, c.DoneAt = truncate(req.Output, 200000), &now
		cp, found = *c, true
		dev := d.Devices[devID]
		if dev != nil && req.OK {
			switch c.Type {
			case "security.pause":
				var args struct{ Minutes int }
				json.Unmarshal(c.Args, &args)
				until := now.Add(time.Duration(args.Minutes) * time.Minute)
				dev.PausedUntil, dev.PausedAt = &until, &now
			case "security.resume":
				dev.PausedUntil, dev.PausedAt = nil, nil
			}
		}
		snap := commandSpecs[c.Type].Snapshot
		if dev == nil || snap == "" || len(req.Data) == 0 || string(req.Data) == "null" || !json.Valid(req.Data) {
			return
		}
		if snap == "inventory" {
			dev.Inventory, dev.InventoryAt = req.Data, now
			updateHostname(dev)
			return
		}
		if dev.Data == nil {
			dev.Data = map[string]DataSnapshot{}
		}
		dev.Data[snap] = DataSnapshot{At: now, Value: req.Data}
	})
	a.hub.TakeSecret(req.ID, true)
	if found {
		a.hub.Broadcast("command", cp)
		a.broadcastDevice(devID)
	}
	writeJSON(w, 200, map[string]any{"ok": found})
}

func updateHostname(dev *Device) {
	var inv struct{ Hostname string }
	if json.Unmarshal(dev.Inventory, &inv) == nil && inv.Hostname != "" {
		dev.Hostname = inv.Hostname
	}
}

func (a *App) agentInventory(w http.ResponseWriter, r *http.Request, devID, role string) {
	if role != "agent" {
		writeErr(w, 403, "Nicht erlaubt.")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil || !json.Valid(body) {
		writeErr(w, 400, "Ungültige Daten.")
		return
	}
	var old json.RawMessage
	var secOn bool
	var name string
	var paused, pausedAt *time.Time
	a.store.Update(func(d *DBData) {
		if dev := d.Devices[devID]; dev != nil {
			old, secOn, name, paused, pausedAt = dev.Inventory, dev.SecurityEnabled, dev.Name, dev.PausedUntil, dev.PausedAt
			dev.Inventory, dev.InventoryAt, dev.LastSeen = body, time.Now(), time.Now()
			updateHostname(dev)
		}
	})
	if secOn {
		a.checkSecurityAlerts(r, devID, name, old, body, paused, pausedAt)
	}
	a.broadcastDevice(devID)
	writeJSON(w, 200, map[string]any{"ok": true})
}

type defSummary struct {
	Enabled       bool `json:"enabled"`
	Realtime      bool `json:"realtime"`
	ActiveThreats int  `json:"activeThreats"`
}

func parseDefender(inv json.RawMessage) (defSummary, bool) {
	var x struct {
		Defender *defSummary `json:"defender"`
	}
	if len(inv) == 0 || json.Unmarshal(inv, &x) != nil || x.Defender == nil {
		return defSummary{}, false
	}
	return *x.Defender, true
}

// checkSecurityAlerts meldet neue Bedrohungen und abgeschalteten Virenschutz sofort im Dashboard.
func (a *App) checkSecurityAlerts(r *http.Request, devID, name string, oldInv, newInv json.RawMessage, paused, pausedAt *time.Time) {
	nw, ok := parseDefender(newInv)
	if !ok {
		return
	}
	clearPause := func() {
		a.store.Update(func(d *DBData) {
			if dev := d.Devices[devID]; dev != nil {
				dev.PausedUntil, dev.PausedAt = nil, nil
			}
		})
	}
	// Schutz ist vorzeitig wieder an (z. B. von Windows selbst oder am PC eingeschaltet) → Pause beenden.
	if paused != nil && nw.Realtime && (pausedAt == nil || time.Since(*pausedAt) > 90*time.Second) {
		clearPause()
		paused = nil
	}
	pausing := paused != nil && time.Now().Before(paused.Add(3*time.Minute))
	if paused != nil && !pausing {
		switch {
		case nw.Realtime:
			clearPause()
		default:
			clearPause()
			a.store.Update(func(d *DBData) {
				d.Audit = append(d.Audit, &AuditEntry{Time: time.Now(), User: "Sicherheit", IP: clientIP(r), DeviceID: devID,
					Action: "Schutz nach Pause noch aus", Detail: name + ": Der Echtzeitschutz ist nach Ende der Pause nicht wieder angegangen."})
			})
			a.hub.Broadcast("alert", map[string]any{"deviceId": devID, "name": name, "level": "danger", "title": "Schutz nach Pause noch aus",
				"detail": name + ": Der Echtzeitschutz ist nach Ende der Pause nicht wieder angegangen. Bitte prüfen."})
			pausing = true // kein zweiter Alarm für dasselbe Problem
		}
	}
	od, hadOld := parseDefender(oldInv)
	alert := func(title, detail string) {
		a.store.Update(func(d *DBData) {
			d.Audit = append(d.Audit, &AuditEntry{Time: time.Now(), User: "Sicherheit", IP: clientIP(r), DeviceID: devID, Action: title, Detail: detail})
		})
		a.hub.Broadcast("alert", map[string]any{"deviceId": devID, "name": name, "level": "danger", "title": title, "detail": detail, "tag": "sec-" + devID})
		a.push(title, name+": "+detail, "/admin/#/d/"+devID+"/security", "sec-"+devID)
	}
	if nw.ActiveThreats > 0 && (!hadOld || nw.ActiveThreats > od.ActiveThreats) {
		alert("Bedrohung erkannt", fmt.Sprintf("%s: %d aktive Bedrohung(en). Bitte unter Sicherheit prüfen.", name, nw.ActiveThreats))
	}
	if hadOld && od.Realtime && !nw.Realtime && !pausing { // während einer Pause ist das Abschalten gewollt
		alert("Echtzeitschutz ausgeschaltet", name+": Der Virenschutz überwacht den PC nicht mehr in Echtzeit.")
	}
}

// ---------- Bildschirm ----------

func (a *App) agentScreenStatus(w http.ResponseWriter, r *http.Request, devID, role string) {
	var req struct {
		SessionID string `json:"sessionId"`
		State     string `json:"state"`
		Message   string `json:"message"`
		W         int    `json:"w"`
		H         int    `json:"h"`
	}
	if !readJSON(w, r, &req, 8192) {
		return
	}
	if role != "screen" {
		writeErr(w, 403, "Nicht erlaubt.")
		return
	}
	switch req.State {
	case "active":
		ok, info := a.hub.UpdateSession(devID, req.SessionID, "active", req.Message, req.W, req.H)
		if !ok {
			writeErr(w, 410, "Sitzung beendet.")
			return
		}
		a.hub.Broadcast("screen", map[string]any{"deviceId": devID, "session": info})
	case "denied", "error", "ended":
		if a.hub.EndSession(devID, req.SessionID) {
			reason := map[string]string{
				"denied": "Die Person am PC hat den Zugriff abgelehnt.",
				"ended":  "Die Person am PC hat die Verbindung getrennt.",
			}[req.State]
			if reason == "" {
				reason = "Fehler am PC: " + req.Message
			}
			a.store.Update(func(d *DBData) {
				d.Audit = append(d.Audit, &AuditEntry{Time: time.Now(), User: "PC", IP: clientIP(r), DeviceID: devID, Action: "Bildschirm-Sitzung beendet", Detail: reason})
			})
			a.hub.Broadcast("screen", map[string]any{"deviceId": devID, "session": nil, "reason": reason})
		}
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *App) agentFrame(w http.ResponseWriter, r *http.Request, devID, role string) {
	if role != "screen" {
		writeErr(w, 403, "Nicht erlaubt.")
		return
	}
	frame, err := io.ReadAll(io.LimitReader(r.Body, 12<<20))
	if err != nil || len(frame) < 4 {
		writeErr(w, 400, "Kein Bild.")
		return
	}
	wd, _ := strconv.Atoi(r.Header.Get("X-Width"))
	ht, _ := strconv.Atoi(r.Header.Get("X-Height"))
	ok, became := a.hub.PushFrame(devID, r.URL.Query().Get("session"), frame, wd, ht)
	if !ok {
		writeErr(w, 410, "Sitzung beendet.")
		return
	}
	if became {
		a.hub.Broadcast("screen", map[string]any{"deviceId": devID, "session": a.hub.Session(devID)})
	}
	w.WriteHeader(204)
}

func (a *App) agentGoodbye(w http.ResponseWriter, r *http.Request, devID, role string) {
	if role != "agent" {
		writeErr(w, 403, "Nicht erlaubt.")
		return
	}
	var name string
	a.store.Update(func(d *DBData) {
		if dev := d.Devices[devID]; dev != nil {
			name = dev.Name
			delete(d.Devices, devID)
			delete(d.Chats, devID)
			defer a.removeChatImages(devID)
			d.Audit = append(d.Audit, &AuditEntry{Time: time.Now(), User: "PC", IP: clientIP(r), DeviceID: devID,
				Action: "Agent deinstalliert", Detail: "Der Agent wurde auf „" + name + "“ entfernt."})
		}
	})
	writeJSON(w, 200, map[string]any{"ok": true})
	go func() {
		time.Sleep(500 * time.Millisecond)
		a.hub.DropDevice(devID)
		a.hub.Broadcast("device-removed", map[string]any{"id": devID})
	}()
}
