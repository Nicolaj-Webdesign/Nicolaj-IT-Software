//go:build !windows

// Simulations-Modus für Entwicklung und Tests ohne Windows-PC:
//
//	nicolaj-agent sim --server https://localhost:8443 --code ABCD-EFGH --fp <fingerabdruck>
//
// Er meldet sich wie ein echter PC an, liefert Beispieldaten und einen Test-Bildschirm.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var simDir = "sim-data"

func agentConfigPath() string    { return filepath.Join(simDir, "agent.json") }
func screenConfigPath() string   { return filepath.Join(simDir, "screen.json") }
func logPath(role string) string { return "" }

func singleInstance(string) bool { return true }
func initScreenPlatform()        {}

func runDefault() {
	fmt.Println("Der Agent läuft nur unter Windows. Zum Testen: nicolaj-agent sim --server URL --code CODE --fp FINGERABDRUCK")
}
func runInstallCLI([]string) { fmt.Println("Installation ist nur unter Windows möglich.") }
func runUninstall([]string)  { fmt.Println("Deinstallation ist nur unter Windows möglich.") }

func runSim(args []string) {
	fs := flag.NewFlagSet("sim", flag.ExitOnError)
	server := fs.String("server", "https://localhost:8443", "Serveradresse")
	code := fs.String("code", "", "Einrichtungscode")
	fp := fs.String("fp", "", "Fingerabdruck des Server-Zertifikats")
	dir := fs.String("dir", "sim-data", "Ordner für die Simulations-Konfiguration")
	person := fs.String("request", "", "statt Code: Zugang anfragen mit diesem Namen")
	fs.Parse(args)
	simDir = *dir
	os.MkdirAll(simDir, 0o700)
	if _, err := loadConfig(agentConfigPath()); err != nil {
		if *code == "" && *person == "" {
			fmt.Println("Bitte --code oder --request NAME angeben.")
			os.Exit(1)
		}
		reg := func() (Config, Config, string, error) { return enroll(*server, *code, *fp) }
		if *person != "" {
			reg = func() (Config, Config, string, error) { return requestAccess(*server, *fp, *person) }
		}
		a, s, name, err := reg()
		if err != nil {
			fmt.Println("Anmeldung fehlgeschlagen:", err)
			os.Exit(1)
		}
		saveConfig(agentConfigPath(), a)
		saveConfig(screenConfigPath(), s)
		fmt.Println("Angemeldet als", name)
	}
	go runScreen()
	runService()
}

// ---------- Beispieldaten ----------

type simUser struct {
	Name     string  `json:"name"`
	FullName string  `json:"fullName"`
	Enabled  bool    `json:"enabled"`
	Admin    bool    `json:"admin"`
	Source   string  `json:"source"`
	Last     *string `json:"lastLogon"`
	Builtin  bool    `json:"builtin"`
}

var (
	simMu    sync.Mutex
	simUsers = []*simUser{
		{Name: "Administrator", Enabled: false, Admin: true, Source: "Local", Builtin: true},
		{Name: "Gast", Enabled: false, Source: "Local", Builtin: true},
		{Name: "Oma", FullName: "Helga Muster", Enabled: true, Admin: true, Source: "Local", Last: ptr("2026-11-23T09:12:00")},
		{Name: "tante@outlook.de", FullName: "Tante Ute", Enabled: true, Source: "MicrosoftAccount", Last: ptr("2026-11-20T18:40:00")},
	}
	simApps = []map[string]string{
		{"name": "Google Chrome", "version": "131.0.6778.86", "publisher": "Google LLC"},
		{"name": "Mozilla Thunderbird (x64 de)", "version": "128.4.0", "publisher": "Mozilla"},
		{"name": "VLC media player", "version": "3.0.20", "publisher": "VideoLAN"},
		{"name": "7-Zip 23.01 (x64)", "version": "23.01", "publisher": "Igor Pavlov"},
	}
)

func ptr(s string) *string { return &s }

// Simulierter Defender-Zustand: eine aktive Adware, eine alte Quarantäne.
var simSec = struct {
	pua, cloud, network, ransomware int
	active, realtime                bool
	quickAge                        int
	pausedUntil                     time.Time
}{pua: 0, cloud: 2, network: 0, ransomware: 0, active: true, realtime: true, quickAge: 9}

func pauseWatcher(c *Client) {
	for range time.Tick(3 * time.Second) {
		simMu.Lock()
		expired := !simSec.pausedUntil.IsZero() && time.Now().After(simSec.pausedUntil)
		if expired {
			simSec.realtime, simSec.pausedUntil = true, time.Time{}
		}
		simMu.Unlock()
		if expired {
			logf("(Simulation) Schutz-Pause abgelaufen – Echtzeitschutz wieder an")
			c.postRaw("/api/agent/inventory", collectInventory())
		}
	}
}

func openSecuritySettings() error { logf("(Simulation) Windows-Sicherheit geöffnet"); return nil }

func runInfo() { fmt.Println(infoText()) }

func updateDir() string                   { return filepath.Join(simDir, "update") }
func oldExePatterns() []string            { return nil }
func verifyUpdate(path, ver string) error { logf("(Simulation) Probelauf übersprungen"); return nil }
func installUpdate(path, ver string) (bool, error) {
	logf("(Simulation) Update %s würde jetzt installiert (%s)", ver, path)
	return false, nil
}
func exitForUpdate()             {}
func runUpdateFinish([]string)   { fmt.Println("Nur unter Windows.") }
func watchForUpdate(func() bool) {}

func securityJSON() json.RawMessage {
	status := 1
	if !simSec.active {
		status = 4
	}
	threats := []map[string]any{
		{"threatId": "2147735505", "name": "PUA:Win32/Presenoker", "severity": 1, "active": simSec.active, "statusId": status,
			"time": "2026-11-23T08:41:00", "resources": []string{`C:\Users\Oma\Downloads\GratisPDFKonverter_Setup.exe`}, "user": "VM\\Oma", "process": "explorer.exe"},
		{"threatId": "2147812345", "name": "Trojan:Win32/Wacatac.B!ml", "severity": 5, "active": false, "statusId": 3,
			"time": "2026-10-02T17:15:00", "resources": []string{`C:\Users\Oma\AppData\Local\Temp\rechnung.pdf.exe`}, "user": "VM\\Oma", "process": "OUTLOOK.EXE"},
	}
	act := 0
	if simSec.active {
		act = 1
	}
	b, _ := json.Marshal(map[string]any{
		"available": true, "mode": "Normal", "enabled": true, "realtime": simSec.realtime, "behavior": true, "tamper": true,
		"signatureVersion": "1.421.1180.0", "signatureUpdated": time.Now().Add(-5 * time.Hour).Format("2006-01-02T15:04:05"), "signatureAgeDays": 0,
		"engineVersion": "1.1.24090.11", "productVersion": "4.18.24090.11",
		"quickScanEnd": time.Now().AddDate(0, 0, -simSec.quickAge).Format("2006-01-02T15:04:05"), "quickScanAgeDays": simSec.quickAge,
		"fullScanEnd": nil, "fullScanAgeDays": -1, "computerState": 0,
		"pua": simSec.pua, "cloud": simSec.cloud, "network": simSec.network, "ransomware": simSec.ransomware,
		"activeThreats": act, "threats": threats,
		"products": []map[string]any{{"name": "Windows Defender", "enabled": true, "upToDate": true}},
	})
	return b
}

func usersJSON() json.RawMessage {
	b, _ := json.Marshal(simUsers)
	return b
}

func findUser(n string) *simUser {
	for _, u := range simUsers {
		if strings.EqualFold(u.Name, n) {
			return u
		}
	}
	return nil
}

func execCommand(ctx context.Context, cmd Cmd) Result {
	a := map[string]any{}
	json.Unmarshal(cmd.Args, &a)
	simMu.Lock()
	defer simMu.Unlock()
	time.Sleep(300 * time.Millisecond)
	user := argString(a, "username")
	switch cmd.Type {
	case "inventory":
		return Result{OK: true, Output: "Systeminfo aktualisiert.", Data: collectInventory()}
	case "users.list":
		return Result{OK: true, Output: fmt.Sprintf("%d Benutzer gefunden.", len(simUsers)), Data: usersJSON()}
	case "users.create":
		if findUser(user) != nil {
			return Result{OK: false, Output: "Der Benutzer " + user + " ist bereits vorhanden."}
		}
		simUsers = append(simUsers, &simUser{Name: user, FullName: argString(a, "fullName"), Enabled: true, Admin: argBool(a, "admin"), Source: "Local"})
		return Result{OK: true, Output: "Benutzer '" + user + "' wurde angelegt.", Data: usersJSON()}
	case "users.resetPassword":
		u := findUser(user)
		if u == nil {
			return Result{OK: false, Output: "Benutzer nicht gefunden."}
		}
		if u.Source == "MicrosoftAccount" {
			return Result{OK: false, Output: "'" + user + "' ist ein Microsoft-Konto. Das Passwort kann nur online unter account.microsoft.com zurückgesetzt werden."}
		}
		return Result{OK: true, Output: "Passwort für '" + user + "' wurde zurückgesetzt.", Data: usersJSON()}
	case "users.setEnabled":
		if u := findUser(user); u != nil {
			u.Enabled = argBool(a, "enabled")
		}
		return Result{OK: true, Output: "Erledigt.", Data: usersJSON()}
	case "users.setAdmin":
		if u := findUser(user); u != nil {
			u.Admin = argBool(a, "admin")
		}
		return Result{OK: true, Output: "Erledigt.", Data: usersJSON()}
	case "users.delete":
		out := simUsers[:0]
		for _, u := range simUsers {
			if !strings.EqualFold(u.Name, user) {
				out = append(out, u)
			}
		}
		simUsers = out
		return Result{OK: true, Output: "Benutzer '" + user + "' wurde gelöscht.", Data: usersJSON()}
	case "software.list":
		b, _ := json.Marshal(simApps)
		return Result{OK: true, Output: fmt.Sprintf("%d Programme.", len(simApps)), Data: b}
	case "software.updates":
		out := "Name               ID                 Version  Verfügbar Quelle\n" +
			"----------------------------------------------------------------\n" +
			"VLC media player   VideoLAN.VLC       3.0.20   3.0.21    winget\n" +
			"7-Zip 23.01 (x64)  7zip.7zip          23.01    24.08     winget\n" +
			"2 Aktualisierungen verfügbar."
		b, _ := json.Marshal(parseWingetTable(out, "updates"))
		return Result{OK: true, Output: out, Data: b}
	case "software.search":
		q := argString(a, "query")
		out := "Name             ID                       Version     Quelle\n" +
			"----------------------------------------------------------------\n" +
			"Mozilla Firefox  Mozilla.Firefox          133.0       winget\n" +
			"Firefox ESR      Mozilla.Firefox.ESR      128.5.0     winget\n" +
			"LibreWolf        LibreWolf.LibreWolf      133.0-1     winget"
		rows := parseWingetTable(out, "search")
		b, _ := json.Marshal(map[string]any{"query": q, "rows": rows})
		return Result{OK: true, Output: fmt.Sprintf("%d Treffer.", len(rows)), Data: b}
	case "software.install", "software.upgrade":
		time.Sleep(time.Second)
		name := argString(a, "name")
		if name == "" {
			name = argString(a, "id")
		}
		if cmd.Type == "software.install" {
			simApps = append(simApps, map[string]string{"name": name, "version": "1.0", "publisher": "winget"})
		}
		b, _ := json.Marshal(simApps)
		return Result{OK: true, Output: "Gefunden " + name + " [" + argString(a, "id") + "]\nErfolgreich installiert", Data: b}
	case "software.upgradeAll":
		b, _ := json.Marshal(simApps)
		return Result{OK: true, Output: "2 Pakete aktualisiert.", Data: b}
	case "software.uninstall":
		name := argString(a, "name")
		out := simApps[:0]
		for _, ap := range simApps {
			if ap["name"] != name {
				out = append(out, ap)
			}
		}
		simApps = out
		b, _ := json.Marshal(simApps)
		return Result{OK: true, Output: "Erfolgreich deinstalliert", Data: b}
	case "power.restart", "power.shutdown", "power.abort":
		return Result{OK: true, Output: "(Simulation) " + cmd.Type}
	case "script.run":
		return Result{OK: true, Output: "(Simulation – kein echtes PowerShell)\n" + argString(a, "script")}
	case "agent.uninstall":
		return Result{OK: true, Output: "(Simulation) Agent würde entfernt."}
	case "security.status":
		return Result{OK: true, Output: "Status geladen.", Data: securityJSON()}
	case "security.scan":
		time.Sleep(2 * time.Second)
		simSec.quickAge = 0
		msg := "Schnellscan abgeschlossen."
		if argString(a, "type") == "full" {
			msg = "Vollständiger Scan abgeschlossen."
		}
		if simSec.active {
			msg += "\nAchtung: 1 aktive Bedrohung(en) gefunden."
		}
		return Result{OK: true, Output: msg, Data: securityJSON()}
	case "security.update":
		return Result{OK: true, Output: "Virensignaturen sind aktuell (Version 1.421.1180.0).", Data: securityJSON()}
	case "security.removeThreats":
		was := simSec.active
		simSec.active = false
		if !was {
			return Result{OK: true, Output: "Es gab keine aktiven Bedrohungen.", Data: securityJSON()}
		}
		return Result{OK: true, Output: "1 Bedrohung(en) entfernt.", Data: securityJSON()}
	case "security.harden":
		on := func(k string, v int) int {
			if argBool(a, k) {
				return v
			}
			return 0
		}
		simSec.pua, simSec.cloud, simSec.network, simSec.ransomware = on("pua", 1), on("cloud", 2), on("network", 1), on("ransomware", 1)
		return Result{OK: true, Output: "✓ Schutz-Einstellungen übernommen (Simulation).", Data: securityJSON()}
	case "security.pause":
		d := time.Duration(argInt(a, "minutes")) * time.Minute
		if sec := os.Getenv("SIM_PAUSE_SECONDS"); sec != "" { // zum Testen: Minuten als Sekunden
			n, _ := strconv.Atoi(sec)
			d = time.Duration(n) * time.Second
		}
		simSec.realtime, simSec.pausedUntil = false, time.Now().Add(d)
		return Result{OK: true, Output: fmt.Sprintf("Echtzeitschutz für %d Minuten pausiert.\nWieder an um %s Uhr.", argInt(a, "minutes"), simSec.pausedUntil.Format("15:04")), Data: securityJSON()}
	case "security.resume":
		simSec.realtime, simSec.pausedUntil = true, time.Time{}
		return Result{OK: true, Output: "Der Echtzeitschutz ist wieder an.", Data: securityJSON()}
	case "security.offlineScan":
		return Result{OK: true, Output: "(Simulation) PC würde für den Offline-Scan neu starten."}
	case "security.msert":
		time.Sleep(2 * time.Second)
		return Result{OK: true, Output: "Lade Microsoft Safety Scanner …\nSignatur von Microsoft geprüft.\nFertig nach 2s (Code 0).\n\nResults Summary:\n----------------\nNo infection found.\n(Simulation)"}
	}
	return Result{OK: false, Output: "Unbekannte Aktion: " + cmd.Type}
}

func collectInventory() json.RawMessage {
	host, _ := os.Hostname()
	b, _ := json.Marshal(map[string]any{
		"hostname": strings.ToUpper(host), "os": "Microsoft Windows 11 Home (Simulation)", "osVersion": "10.0.26100", "build": "26100",
		"manufacturer": "LENOVO", "model": "IdeaPad 5 14ALC05", "serial": "PF2SIM01", "cpu": "AMD Ryzen 5 5500U with Radeon Graphics",
		"cores": 12, "ramGB": 15.4, "ramFreeGB": 7.9, "uptimeHours": 26.4, "loggedOnUser": host + "\\Oma",
		"disks":  []map[string]any{{"name": "C:", "sizeGB": 476.3, "freeGB": 201.7}},
		"ips":    []string{"192.168.178.34"},
		"domain": "WORKGROUP", "partOfDomain": false,
		"defender": map[string]any{"enabled": true, "realtime": simSec.realtime, "signatureAgeDays": 0, "quickScanAgeDays": simSec.quickAge,
			"activeThreats": map[bool]int{true: 1, false: 0}[simSec.active], "tamper": true, "mode": "Normal", "pua": simSec.pua},
		"antivirus":     []map[string]any{{"name": "Windows Defender", "enabled": true}},
		"rebootPending": false, "lastHotfix": "KB5046617 (2026-11-12)", "agentVersion": agentVersion,
	})
	return b
}

// ---------- Test-Bildschirm ----------

var simCursor struct {
	sync.Mutex
	x, y   float64
	clicks int
	keys   int
	text   string
}

type simCapturer struct{ start time.Time }

func newCapturer() (Capturer, error)    { return &simCapturer{start: time.Now()}, nil }
func (s *simCapturer) Size() (int, int) { return 1280, 720 }
func (s *simCapturer) Close()           {}
func (s *simCapturer) Capture(q string, force bool) ([]byte, int, int, error) {
	const w, h = 1280, 720
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	t := time.Since(s.start).Seconds()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			o := y*img.Stride + x*4
			img.Pix[o] = uint8(30 + x*40/w)
			img.Pix[o+1] = uint8(60 + y*60/h)
			img.Pix[o+2] = 140
			img.Pix[o+3] = 255
		}
	}
	// "Taskleiste"
	fill(img, 0, h-48, w, h, color.RGBA{20, 22, 30, 255})
	// wandernder Balken, damit man Bewegung sieht
	bx := int((math.Sin(t)+1)/2*float64(w-200)) + 0
	fill(img, bx, 80, bx+200, 110, color.RGBA{255, 255, 255, 200})
	simCursor.Lock()
	cx, cy, clicks, keys := int(simCursor.x*w), int(simCursor.y*h), simCursor.clicks, simCursor.keys
	simCursor.Unlock()
	// Klicks und Tasten als Kästchen links unten
	for i := 0; i < clicks%40; i++ {
		fill(img, 10+i*14, h-38, 20+i*14, h-28, color.RGBA{80, 220, 120, 255})
	}
	for i := 0; i < keys%40; i++ {
		fill(img, 10+i*14, h-20, 20+i*14, h-10, color.RGBA{250, 200, 60, 255})
	}
	// Mauszeiger
	fill(img, cx-2, cy-12, cx+2, cy+12, color.RGBA{255, 60, 60, 255})
	fill(img, cx-12, cy-2, cx+12, cy+2, color.RGBA{255, 60, 60, 255})
	return encodeFrame(img, q), w, h, nil
}

func fill(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	r := image.Rect(x0, y0, x1, y1).Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func applyInput(e InputEvent) {
	simCursor.Lock()
	defer simCursor.Unlock()
	switch e.T {
	case "m", "w":
		simCursor.x, simCursor.y = e.X, e.Y
	case "b":
		simCursor.x, simCursor.y = e.X, e.Y
		if e.D {
			simCursor.clicks++
		}
	case "k":
		if e.D {
			simCursor.keys++
		}
	case "text":
		simCursor.text += e.S
		simCursor.keys += len([]rune(e.S))
	}
}

func askConsent(ctx context.Context, by string) bool {
	logf("(Simulation) Zustimmung für %s – automatisch erlaubt", by)
	time.Sleep(time.Second)
	return os.Getenv("SIM_DENY") == ""
}

type noopCloser struct{}

func (noopCloser) Close() {}

func showOverlay(by string, onDisconnect func()) closer {
	logf("(Simulation) Leiste „Nicolaj Support ist verbunden“ angezeigt")
	return noopCloser{}
}
func showMessage(text string) { logf("(Simulation) Nachricht: %s", text) }
func lockScreen() error       { logf("(Simulation) Bildschirm gesperrt"); return nil }

func selfUninstall() { logf("Simulation: Agent würde sich jetzt selbst entfernen."); os.Exit(0) }
