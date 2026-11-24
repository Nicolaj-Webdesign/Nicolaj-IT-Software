package main

// Allgemeiner Installer: Jeder kann sich den Installer von der Website holen, seinen Namen
// eingeben und Zugang anfragen. Bis du im Dashboard freigibst, kann der PC nichts.

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	pendingMsg  = "Dieses Gerät wartet noch auf deine Freigabe (Annehmen oder Ablehnen)."
	maxPending  = 25
	maxNameLen  = 40
	rejectedTTL = 180 * 24 * time.Hour
)

func cleanName(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '<' || r == '>' {
			return -1
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	for utf8.RuneCountInString(s) > maxNameLen {
		_, n := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-n]
	}
	return strings.TrimSpace(s)
}

func (a *App) agentRequest(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if blocked, wait := a.limiter.Blocked("request:" + ip); blocked {
		writeErr(w, 429, fmt.Sprintf("Zu viele Anfragen von diesem Anschluss. Bitte in %s erneut versuchen.", wait))
		return
	}
	if a.settings().DisableRequests {
		writeErr(w, 403, "Zugangsanfragen sind gerade ausgeschaltet. Bitte melde dich direkt bei Nicolaj Webdesign.")
		return
	}
	var req struct{ Name, Hostname, OS, Version string }
	if !readJSON(w, r, &req, 8192) {
		return
	}
	name := cleanName(req.Name)
	if name == "" {
		writeErr(w, 400, "Bitte gib deinen Namen ein.")
		return
	}
	host := cleanName(req.Hostname)
	devID := randHex(8)
	agentTok := devID + "." + randHex(32)
	screenTok := devID + "." + randHex(32)
	var pcID string
	full := false
	a.store.Update(func(d *DBData) {
		n := 0
		for _, dev := range d.Devices {
			if dev.Pending {
				n++
			}
		}
		if n >= maxPending {
			full = true
			return
		}
		now := time.Now()
		pcID = d.newPCID()
		d.Devices[devID] = &Device{
			ID: devID, PCID: pcID, Name: name, Hostname: host, ConsentRequired: true,
			Pending: true, RequestedAt: &now,
			AgentTokenHash: sha256Hex(agentTok), ScreenTokenHash: sha256Hex(screenTok),
			AgentVersion: req.Version, EnrolledAt: now, LastSeen: now, LastIP: ip,
			Data: map[string]DataSnapshot{},
		}
		d.Audit = append(d.Audit, &AuditEntry{Time: now, User: "PC", IP: ip, DeviceID: devID,
			Action: "Zugang angefragt", Detail: fmt.Sprintf("%s (%s), PC-ID %s", name, host, fmtPCID(pcID))})
	})
	if full {
		writeErr(w, 429, "Gerade warten schon sehr viele Anfragen. Bitte melde dich direkt bei Nicolaj Webdesign.")
		return
	}
	a.limiter.Fail("request:" + ip) // zählt Anfragen pro Anschluss (ab 5 pro Stunde kurze Sperre)
	a.broadcastDevice(devID)
	a.hub.Broadcast("alert", map[string]any{"level": "info", "title": "Neue Zugangsanfrage",
		"detail": fmt.Sprintf("%s (%s) möchte freigeschaltet werden.", name, host), "deviceId": devID})
	writeJSON(w, 200, map[string]any{"deviceId": devID, "pcId": pcID, "agentToken": agentTok,
		"screenToken": screenTok, "name": name, "pending": true})
}

func (a *App) apiDeviceApprove(w http.ResponseWriter, r *http.Request, user string) {
	id := r.PathValue("id")
	var name string
	a.store.Update(func(d *DBData) {
		if dev := d.Devices[id]; dev != nil && dev.Pending {
			name = dev.Name
			dev.Pending = false
		}
	})
	if name == "" {
		writeErr(w, 404, "Keine offene Anfrage für dieses Gerät.")
		return
	}
	a.audit(r, user, id, "Zugang freigegeben", "„"+name+"“ angenommen")
	a.broadcastDevice(id)
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *App) apiDeviceReject(w http.ResponseWriter, r *http.Request, user string) {
	id := r.PathValue("id")
	var name string
	a.store.Update(func(d *DBData) {
		if dev := d.Devices[id]; dev != nil && dev.Pending {
			name = dev.Name
			delete(d.Devices, id)
			if d.Rejected == nil {
				d.Rejected = map[string]time.Time{}
			}
			now := time.Now()
			d.Rejected[id] = now
			for k, t := range d.Rejected {
				if now.Sub(t) > rejectedTTL {
					delete(d.Rejected, k)
				}
			}
		}
	})
	if name == "" {
		writeErr(w, 404, "Keine offene Anfrage für dieses Gerät.")
		return
	}
	a.hub.DropDevice(id) // Agent verbindet sich neu, bekommt „abgelehnt“ und entfernt sich selbst
	a.audit(r, user, id, "Zugang abgelehnt", "„"+name+"“ abgelehnt – der Agent entfernt sich selbst")
	a.hub.Broadcast("device-removed", map[string]any{"id": id})
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *App) publicBase(r *http.Request) string {
	if a.publicURL != "" {
		return a.publicURL
	}
	return a.scheme(r) + "://" + r.Host
}

func (a *App) apiRequestSettings(w http.ResponseWriter, r *http.Request, user string) {
	if r.Method == "POST" {
		var req struct {
			Enabled *bool `json:"enabled"`
		}
		if !readJSON(w, r, &req, 1024) {
			return
		}
		if req.Enabled != nil {
			a.store.Update(func(d *DBData) {
				if d.Settings == nil {
					d.Settings = &Settings{}
				}
				d.Settings.DisableRequests = !*req.Enabled
			})
			a.audit(r, user, "", "Zugangsanfragen", map[bool]string{true: "erlaubt", false: "ausgeschaltet"}[*req.Enabled])
		}
	}
	writeJSON(w, 200, map[string]any{"enabled": !a.settings().DisableRequests,
		"link": a.publicBase(r), "agentAvailable": a.currentAgent() != nil})
}

// publicInstaller: der allgemeine Installer (Agent + Serveradresse, ohne Code).
func (a *App) publicInstaller(r *http.Request) []byte {
	pkg := a.currentAgent()
	if pkg == nil || pkg.data == nil {
		return nil
	}
	payload, _ := json.Marshal(map[string]any{"server": a.publicBase(r), "fingerprint": a.fingerprint, "request": true})
	var buf bytes.Buffer
	buf.Grow(len(pkg.data) + len(payload) + 12)
	buf.Write(pkg.data)
	buf.Write(payload)
	binary.Write(&buf, binary.LittleEndian, uint32(len(payload)))
	buf.WriteString("NRMMCFG1")
	return buf.Bytes()
}

func (a *App) publicDownload(w http.ResponseWriter, r *http.Request) {
	if a.settings().DisableRequests {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	ip := clientIP(r)
	if blocked, _ := a.limiter.Blocked("download:" + ip); blocked {
		http.Error(w, "Zu viele Downloads – bitte später erneut versuchen.", 429)
		return
	}
	bin := a.publicInstaller(r)
	if bin == nil {
		http.Error(w, "Der Installer ist gerade nicht verfügbar.", 503)
		return
	}
	a.limiter.Fail("download:" + ip) // ab 5 Downloads pro Stunde kurze Pause
	w.Header().Set("Content-Type", "application/vnd.microsoft.portable-executable")
	w.Header().Set("Content-Disposition", `attachment; filename="Nicolaj-Fernwartung.exe"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Write(bin)
}
