package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Jede Aktion, die das Dashboard an einen PC schicken darf, steht hier – mit Prüfung der Eingaben.
// Alles andere wird abgelehnt.

type Field struct {
	Name     string
	Kind     string // string, text, bool, int
	Required bool
	Min, Max int
	Pattern  *regexp.Regexp
	Secret   bool
	Label    string
}

type CommandSpec struct {
	Target   string // "agent" (Dienst als SYSTEM) oder "screen" (Helfer in der Benutzersitzung)
	Timeout  int    // Sekunden
	Fields   []Field
	Snapshot string // welche gespeicherte Liste das Ergebnis aktualisiert
	Summary  func(a map[string]any) string
}

var (
	reScan  = regexp.MustCompile(`^(quick|full)$`)
	reUser  = regexp.MustCompile(`^[^\x00-\x1f\\/\[\]:;|=,+*?<>"@%]{1,20}$`)
	rePkgID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.+_\-]{0,127}$`)
)

func fUser() Field {
	return Field{Name: "username", Kind: "string", Required: true, Pattern: reUser, Label: "Benutzername"}
}
func fPassword() Field {
	return Field{Name: "password", Kind: "string", Required: true, Min: 1, Max: 127, Secret: true, Label: "Passwort"}
}
func fBool(n string) Field { return Field{Name: n, Kind: "bool"} }

func q(a map[string]any, k string) string { s, _ := a[k].(string); return "„" + s + "“" }
func b(a map[string]any, k string) bool   { v, _ := a[k].(bool); return v }

var commandSpecs = map[string]CommandSpec{
	"inventory": {Target: "agent", Timeout: 120, Snapshot: "inventory",
		Summary: func(a map[string]any) string { return "Systeminfo aktualisieren" }},

	"users.list": {Target: "agent", Timeout: 120, Snapshot: "users",
		Summary: func(a map[string]any) string { return "Benutzerliste laden" }},
	"users.create": {Target: "agent", Timeout: 120, Snapshot: "users",
		Fields: []Field{fUser(), fPassword(), {Name: "fullName", Kind: "string", Max: 64, Label: "Voller Name"},
			fBool("admin"), fBool("mustChange"), fBool("neverExpires")},
		Summary: func(a map[string]any) string {
			s := "Benutzer " + q(a, "username") + " anlegen"
			if b(a, "admin") {
				s += " (Administrator)"
			}
			return s
		}},
	"users.resetPassword": {Target: "agent", Timeout: 120, Snapshot: "users",
		Fields:  []Field{fUser(), fPassword(), fBool("mustChange")},
		Summary: func(a map[string]any) string { return "Passwort von " + q(a, "username") + " zurücksetzen" }},
	"users.setEnabled": {Target: "agent", Timeout: 120, Snapshot: "users",
		Fields: []Field{fUser(), fBool("enabled")},
		Summary: func(a map[string]any) string {
			if b(a, "enabled") {
				return "Benutzer " + q(a, "username") + " aktivieren"
			}
			return "Benutzer " + q(a, "username") + " deaktivieren"
		}},
	"users.setAdmin": {Target: "agent", Timeout: 120, Snapshot: "users",
		Fields: []Field{fUser(), fBool("admin")},
		Summary: func(a map[string]any) string {
			if b(a, "admin") {
				return "Admin-Rechte für " + q(a, "username") + " geben"
			}
			return "Admin-Rechte von " + q(a, "username") + " entfernen"
		}},
	"users.delete": {Target: "agent", Timeout: 300, Snapshot: "users",
		Fields: []Field{fUser(), fBool("deleteProfile")},
		Summary: func(a map[string]any) string {
			s := "Benutzer " + q(a, "username") + " löschen"
			if b(a, "deleteProfile") {
				s += " (inkl. Dateien)"
			}
			return s
		}},

	"software.list": {Target: "agent", Timeout: 120, Snapshot: "software",
		Summary: func(a map[string]any) string { return "Installierte Programme laden" }},
	"software.updates": {Target: "agent", Timeout: 300, Snapshot: "updates",
		Summary: func(a map[string]any) string { return "Nach Updates suchen" }},
	"software.search": {Target: "agent", Timeout: 120, Snapshot: "search",
		Fields:  []Field{{Name: "query", Kind: "string", Required: true, Min: 2, Max: 80, Label: "Suchbegriff"}},
		Summary: func(a map[string]any) string { return "Software suchen: " + q(a, "query") }},
	"software.install": {Target: "agent", Timeout: 1800, Snapshot: "software",
		Fields: []Field{{Name: "id", Kind: "string", Required: true, Pattern: rePkgID, Label: "Paket-ID"},
			{Name: "name", Kind: "string", Max: 128}},
		Summary: func(a map[string]any) string { return "Installieren: " + displayName(a) }},
	"software.upgrade": {Target: "agent", Timeout: 1800, Snapshot: "software",
		Fields: []Field{{Name: "id", Kind: "string", Required: true, Pattern: rePkgID, Label: "Paket-ID"},
			{Name: "name", Kind: "string", Max: 128}},
		Summary: func(a map[string]any) string { return "Aktualisieren: " + displayName(a) }},
	"software.upgradeAll": {Target: "agent", Timeout: 3600, Snapshot: "software",
		Summary: func(a map[string]any) string { return "Alle Programme aktualisieren" }},
	"software.uninstall": {Target: "agent", Timeout: 1800, Snapshot: "software",
		Fields:  []Field{{Name: "name", Kind: "string", Required: true, Min: 1, Max: 256, Label: "Programmname"}},
		Summary: func(a map[string]any) string { return "Deinstallieren: " + q(a, "name") }},

	"power.restart": {Target: "agent", Timeout: 60,
		Fields:  []Field{{Name: "delay", Kind: "int", Min: 0, Max: 3600}, {Name: "message", Kind: "string", Max: 200}},
		Summary: func(a map[string]any) string { return fmt.Sprintf("Neustart in %v Sek.", a["delay"]) }},
	"power.shutdown": {Target: "agent", Timeout: 60,
		Fields:  []Field{{Name: "delay", Kind: "int", Min: 0, Max: 3600}, {Name: "message", Kind: "string", Max: 200}},
		Summary: func(a map[string]any) string { return fmt.Sprintf("Herunterfahren in %v Sek.", a["delay"]) }},
	"power.abort": {Target: "agent", Timeout: 60,
		Summary: func(a map[string]any) string { return "Neustart/Herunterfahren abbrechen" }},

	"script.run": {Target: "agent", Timeout: 3600,
		Fields: []Field{{Name: "script", Kind: "text", Required: true, Min: 1, Max: 100000, Label: "Skript"},
			{Name: "timeout", Kind: "int", Min: 5, Max: 3600}},
		Summary: func(a map[string]any) string {
			s, _ := a["script"].(string)
			s = strings.TrimSpace(strings.SplitN(s, "\n", 2)[0])
			if len([]rune(s)) > 60 {
				s = string([]rune(s)[:60]) + "…"
			}
			return "PowerShell: " + s
		}},

	// ---- Sicherheitspaket (nur wenn für das Gerät aktiviert) ----
	"security.status": {Target: "agent", Timeout: 180, Snapshot: "security",
		Summary: func(a map[string]any) string { return "Sicherheitsstatus laden" }},
	"security.scan": {Target: "agent", Timeout: 6 * 3600, Snapshot: "security",
		Fields: []Field{{Name: "type", Kind: "string", Required: true, Pattern: reScan, Label: "Scan-Art"}},
		Summary: func(a map[string]any) string {
			if a["type"] == "full" {
				return "Vollständiger Virenscan"
			}
			return "Schnellscan"
		}},
	"security.update": {Target: "agent", Timeout: 900, Snapshot: "security",
		Summary: func(a map[string]any) string { return "Virensignaturen aktualisieren" }},
	"security.removeThreats": {Target: "agent", Timeout: 900, Snapshot: "security",
		Summary: func(a map[string]any) string { return "Aktive Bedrohungen entfernen" }},
	"security.harden": {Target: "agent", Timeout: 180, Snapshot: "security",
		Fields:  []Field{fBool("pua"), fBool("cloud"), fBool("network"), fBool("ransomware")},
		Summary: func(a map[string]any) string { return "Schutz-Einstellungen anwenden" }},
	"security.pause": {Target: "agent", Timeout: 180, Snapshot: "security",
		Fields:  []Field{{Name: "minutes", Kind: "int", Min: 5, Max: 240, Label: "Dauer"}},
		Summary: func(a map[string]any) string { return fmt.Sprintf("Echtzeitschutz %v Min. pausieren", a["minutes"]) }},
	"security.resume": {Target: "agent", Timeout: 180, Snapshot: "security",
		Summary: func(a map[string]any) string { return "Echtzeitschutz wieder einschalten" }},
	"security.offlineScan": {Target: "agent", Timeout: 180,
		Summary: func(a map[string]any) string { return "Offline-Scan (mit Neustart)" }},
	"security.msert": {Target: "agent", Timeout: 6 * 3600,
		Fields: []Field{{Name: "type", Kind: "string", Required: true, Pattern: reScan, Label: "Scan-Art"}},
		Summary: func(a map[string]any) string {
			if a["type"] == "full" {
				return "Zweitmeinung: Microsoft Safety Scanner (gründlich, entfernt Funde)"
			}
			return "Zweitmeinung: Microsoft Safety Scanner (schnell, nur melden)"
		}},

	"agent.update": {Target: "agent", Timeout: 900,
		Summary: func(a map[string]any) string { return "Agent aktualisieren" }},
	"agent.uninstall": {Target: "agent", Timeout: 60,
		Summary: func(a map[string]any) string { return "Agent vom PC entfernen" }},

	"user.message": {Target: "screen", Timeout: 30,
		Fields:  []Field{{Name: "text", Kind: "text", Required: true, Min: 1, Max: 1000, Label: "Nachricht"}},
		Summary: func(a map[string]any) string { return "Nachricht anzeigen" }},
	"user.openSecurity": {Target: "screen", Timeout: 30,
		Summary: func(a map[string]any) string { return "Windows-Sicherheit am PC öffnen" }},
	"user.lock": {Target: "screen", Timeout: 30,
		Summary: func(a map[string]any) string { return "Bildschirm sperren" }},
}

func displayName(a map[string]any) string {
	if n, _ := a["name"].(string); n != "" {
		return "„" + n + "“"
	}
	return q(a, "id")
}

func commandHasSecrets(t string) bool {
	for _, f := range commandSpecs[t].Fields {
		if f.Secret {
			return true
		}
	}
	return false
}

// cleanArgs prüft die Eingaben und gibt (vollständig, ohne Geheimnisse) zurück.
func cleanArgs(spec CommandSpec, raw json.RawMessage) (full map[string]any, redacted map[string]any, err error) {
	in := map[string]any{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, nil, errors.New("Ungültige Parameter.")
		}
	}
	full, redacted = map[string]any{}, map[string]any{}
	for _, f := range spec.Fields {
		v, present := in[f.Name]
		label := f.Label
		if label == "" {
			label = f.Name
		}
		switch f.Kind {
		case "bool":
			bv, _ := v.(bool)
			full[f.Name] = bv
		case "int":
			n, ok := v.(float64)
			if !present || !ok {
				n = 0
			}
			if int(n) < f.Min || (f.Max > 0 && int(n) > f.Max) {
				return nil, nil, fmt.Errorf("%s muss zwischen %d und %d liegen.", label, f.Min, f.Max)
			}
			full[f.Name] = int(n)
		case "string", "text":
			s, _ := v.(string)
			if f.Kind == "string" {
				s = strings.TrimSpace(s)
			}
			if s == "" {
				if f.Required {
					return nil, nil, fmt.Errorf("%s fehlt.", label)
				}
				full[f.Name] = ""
				break
			}
			n := len([]rune(s))
			if n < f.Min || (f.Max > 0 && n > f.Max) {
				return nil, nil, fmt.Errorf("%s: %d bis %d Zeichen erlaubt.", label, f.Min, f.Max)
			}
			for _, c := range s {
				if unicode.IsControl(c) && !(f.Kind == "text" && (c == '\n' || c == '\r' || c == '\t')) {
					return nil, nil, fmt.Errorf("%s enthält unerlaubte Zeichen.", label)
				}
			}
			if f.Pattern != nil && !f.Pattern.MatchString(s) {
				return nil, nil, fmt.Errorf("%s hat ein ungültiges Format.", label)
			}
			if f.Name == "username" && strings.Trim(s, ". ") == "" {
				return nil, nil, fmt.Errorf("%s ist ungültig.", label)
			}
			full[f.Name] = s
		}
		if f.Secret {
			redacted[f.Name] = "••••••"
		} else {
			redacted[f.Name] = full[f.Name]
		}
	}
	return full, redacted, nil
}
