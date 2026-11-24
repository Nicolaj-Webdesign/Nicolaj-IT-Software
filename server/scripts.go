package main

// Gespeicherte PowerShell-Skripte: einmal schreiben, auf jedem PC (oder vielen auf einmal) ausführen.

import (
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	maxScripts    = 100
	maxScriptBody = 100000
)

func (a *App) apiScripts(w http.ResponseWriter, r *http.Request, user string) {
	var list []Script
	a.store.View(func(d *DBData) {
		for _, s := range d.Scripts {
			list = append(list, *s)
		}
	})
	sort.Slice(list, func(i, j int) bool { return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name) })
	if list == nil {
		list = []Script{}
	}
	writeJSON(w, 200, list)
}

func (a *App) apiSaveScript(w http.ResponseWriter, r *http.Request, user string) {
	var req struct{ Name, Script string }
	if !readJSON(w, r, &req, maxScriptBody+4096) {
		return
	}
	name := cleanName(req.Name)
	if name == "" || strings.TrimSpace(req.Script) == "" {
		writeErr(w, 400, "Bitte Name und Skript angeben.")
		return
	}
	if len(req.Script) > maxScriptBody {
		writeErr(w, 400, "Das Skript ist zu lang.")
		return
	}
	var saved *Script
	full := false
	a.store.Update(func(d *DBData) {
		for _, s := range d.Scripts { // gleicher Name = überschreiben
			if strings.EqualFold(s.Name, name) {
				s.Body, s.CreatedBy, s.CreatedAt = req.Script, user, time.Now()
				saved = s
				return
			}
		}
		if len(d.Scripts) >= maxScripts {
			full = true
			return
		}
		saved = &Script{ID: randHex(6), Name: name, Body: req.Script, CreatedBy: user, CreatedAt: time.Now()}
		d.Scripts = append(d.Scripts, saved)
	})
	if full {
		writeErr(w, 409, "Es sind schon 100 Skripte gespeichert. Bitte erst eins löschen.")
		return
	}
	a.audit(r, user, "", "Skript gespeichert", "„"+name+"“")
	writeJSON(w, 200, saved)
}

func (a *App) apiDeleteScript(w http.ResponseWriter, r *http.Request, user string) {
	id := r.PathValue("id")
	var name string
	a.store.Update(func(d *DBData) {
		for i, s := range d.Scripts {
			if s.ID == id {
				name = s.Name
				d.Scripts = append(d.Scripts[:i], d.Scripts[i+1:]...)
				return
			}
		}
	})
	if name == "" {
		writeErr(w, 404, "Skript nicht gefunden.")
		return
	}
	a.audit(r, user, "", "Skript gelöscht", "„"+name+"“")
	writeJSON(w, 200, map[string]any{"ok": true})
}
