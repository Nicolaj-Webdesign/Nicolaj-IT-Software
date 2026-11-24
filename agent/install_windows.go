//go:build windows

package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf16"
)

const (
	taskAgent  = `NicolajRMM\Agent`
	taskScreen = `NicolajRMM\Bildschirm`
	uninstKey  = `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\NicolajRMM`
)

// runDefault: Doppelklick auf die .exe. Mit eingebautem Code → Installation, mit „request“
// (allgemeiner Installer von der Website) → Namen eingeben und Zugang anfragen.
func runDefault() {
	exe, _ := os.Executable()
	p, base, err := readPayload(exe)
	if err == nil && p == nil && defaultServer != "" {
		p = &Payload{Server: defaultServer, Request: true}
	}
	if err != nil || p == nil {
		uiNotice("Das ist der Agent von Nicolaj Fernwartung",
			"Den Installer bekommst du auf der Website von Nicolaj Webdesign oder im Dashboard unter „Gerät hinzufügen“.", "info")
		return
	}
	// Fortsetzung nach der Windows-Abfrage (Administratorrechte): --go [--name NAME]
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	goOn := fs.Bool("go", false, "")
	person := fs.String("name", "", "")
	fs.Parse(os.Args[1:])
	if p.Request {
		requestInstallUI(exe, base, p.Server, p.Fingerprint, *person, *goOn)
		return
	}
	codeInstallUI(exe, base, p.Server, p.Code, p.Fingerprint, *goOn, []string{"--go"})
}

func runInstallCLI(args []string) {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	server := fs.String("server", "", "Serveradresse, z. B. https://192.168.178.20:8443")
	code := fs.String("code", "", "Einrichtungscode aus dem Dashboard")
	fp := fs.String("fp", "", "Fingerabdruck (SHA-256) des Server-Zertifikats")
	quiet := fs.Bool("quiet", false, "keine Rückfragen")
	goOn := fs.Bool("go", false, "")
	if err := fs.Parse(args); err != nil || *server == "" || *code == "" {
		uiNotice("Falscher Aufruf", "nicolaj-agent.exe install --server https://SERVER --code CODE [--fp FINGERABDRUCK] [--quiet]", "warn")
		return
	}
	exe, _ := os.Executable()
	_, base, _ := readPayload(exe)
	if *quiet {
		if !isAdmin() {
			relaunchElevated(exe, append([]string{"install"}, args...))
			return
		}
		installWith(exe, base, func() (Config, Config, string, error) { return enroll(*server, *code, *fp) }, nil)
		return
	}
	relaunch := []string{"install", "--server", *server, "--code", *code, "--fp", *fp, "--go"}
	codeInstallUI(exe, base, *server, *code, *fp, *goOn, relaunch)
}

// installFlow: gemeinsamer Ablauf beider Installer (Fenster, Administratorrechte, Fortschritt).
type installFlow struct {
	exe      string
	base     int64
	server   string
	welcome  func(problem string) *uiDialog
	reg      func() (Config, Config, string, error)
	done     func(name, pcID string) *uiDialog
	relaunch func() []string
	ready    func() bool // darf „Installieren“ gedrückt werden?
	onInput  func(ctl *uiCtl)
}

func (f *installFlow) run(startNow bool) {
	work := func(ctl *uiCtl) {
		ctl.Set(dlgInstallProgress(1, 0.12))
		name, pcID, err := installWith(f.exe, f.base, f.reg, func(step int) {
			ctl.Set(dlgInstallProgress(step, float64(step)/float64(len(installSteps)-1)))
		})
		if err != nil {
			ctl.Set(dlgInstallError(err.Error()))
			return
		}
		ctl.Set(dlgInstallProgress(len(installSteps)-1, 1))
		time.Sleep(450 * time.Millisecond)
		ctl.Set(f.done(name, pcID))
	}
	var busy atomic.Bool
	opts := uiOpts{Animate: true, OnClick: func(ctl *uiCtl, id string) bool {
		switch id {
		case "install", "request", "retry":
			if (f.ready != nil && !f.ready()) || !busy.CompareAndSwap(false, true) {
				return false
			}
			if isAdmin() {
				go func() { work(ctl); busy.Store(false) }()
				return false
			}
			ctl.Set(dlgInstallProgress(0, 0.04))
			go func() {
				if relaunchElevated(f.exe, f.relaunch()) {
					ctl.Close("elevated") // der Prozess mit Administratorrechten übernimmt
					return
				}
				busy.Store(false)
				ctl.Set(f.welcome("Ohne deine Bestätigung kann nicht installiert werden. Bitte nochmal versuchen und bei Windows auf „Ja“ klicken."))
			}()
			return false
		case "input-changed":
			if f.onInput != nil {
				f.onInput(ctl)
			}
			return false
		case "close", "cancel", "ok":
			return !busy.Load() || id == "ok"
		}
		return false
	}}
	first := f.welcome("")
	if startNow && isAdmin() {
		first = dlgInstallProgress(1, 0.12)
		opts.Ready = func(ctl *uiCtl) { busy.Store(true); work(ctl); busy.Store(false) }
	}
	if _, err := runUI(first, opts); err != nil {
		f.classic()
	}
}

// classic: Rückfall auf Windows-Standardfenster, falls die eigenen Fenster nicht gehen.
func (f *installFlow) classic() {
	if !isAdmin() {
		if !relaunchElevated(f.exe, f.relaunch()) {
			msgBox("Für die Installation sind Administratorrechte nötig.", mbIconWarning)
		}
		return
	}
	name, pcID, err := installWith(f.exe, f.base, f.reg, nil)
	if err != nil {
		msgBox("Die Installation hat nicht geklappt:\n\n"+err.Error(), mbIconError)
		return
	}
	msgBox("Fertig! Dieser PC ist jetzt als „"+name+"“ mit Nicolaj Fernwartung verbunden.\n\nDeine PC-ID: "+fmtPCID(pcID), mbIconInfo)
}

func codeInstallUI(exe string, base int64, server, code, fp string, startNow bool, relaunch []string) {
	f := &installFlow{exe: exe, base: base, server: server,
		welcome:  func(problem string) *uiDialog { return dlgInstallWelcome(server, problem) },
		reg:      func() (Config, Config, string, error) { return enroll(server, code, fp) },
		done:     dlgInstallDone,
		relaunch: func() []string { return relaunch },
	}
	f.run(startNow)
}

func requestInstallUI(exe string, base int64, server, fp, person string, startNow bool) {
	var mu sync.Mutex
	current := strings.TrimSpace(person)
	name := func() string {
		mu.Lock()
		defer mu.Unlock()
		return strings.TrimSpace(current)
	}
	f := &installFlow{exe: exe, base: base, server: server,
		welcome:  func(problem string) *uiDialog { return dlgRequestWelcome(server, name(), problem) },
		reg:      func() (Config, Config, string, error) { return requestAccess(server, fp, name()) },
		done:     func(_, pcID string) *uiDialog { return dlgRequestDone(pcID) },
		relaunch: func() []string { return []string{"--go", "--name", name()} },
		ready:    func() bool { return name() != "" },
		onInput: func(ctl *uiCtl) { // Namen merken und den Knopf freischalten
			ctl.Update(func(d *uiDialog) {
				if d.Input == nil {
					return
				}
				mu.Lock()
				current = d.Input.Value
				mu.Unlock()
				if b := d.button("request"); b != nil {
					b.Disabled = strings.TrimSpace(d.Input.Value) == ""
				}
			})
		},
	}
	f.run(startNow && current != "")
}

// installWith meldet den PC an (Code oder Anfrage) und richtet alles ein.
// progress (optional) bekommt die Schritte 1–4 aus installSteps.
func installWith(exe string, base int64, reg func() (Config, Config, string, error), progress func(int)) (string, string, error) {
	step := func(i int) {
		if progress != nil {
			progress(i)
		}
	}
	step(1)
	agentCfg, screenCfg, name, err := reg()
	if err != nil {
		return "", "", fmt.Errorf("Anmeldung am Server fehlgeschlagen: %w", err)
	}
	step(2)

	// Alte Version anhalten
	stopAll()

	pd, dd := programDir(), dataDir()
	if err := os.MkdirAll(pd, 0o755); err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(filepath.Join(dd, "logs"), 0o755); err != nil {
		return "", "", err
	}
	target := filepath.Join(pd, "nicolaj-agent.exe")
	if !strings.EqualFold(filepath.Clean(exe), filepath.Clean(target)) {
		var lastErr error
		for i := 0; i < 10; i++ {
			if lastErr = copyPrefix(exe, target, base); lastErr == nil {
				break
			}
			time.Sleep(time.Second)
		}
		if lastErr != nil {
			return "", "", fmt.Errorf("Programm konnte nicht kopiert werden: %w", lastErr)
		}
	}

	// Datenordner: nur SYSTEM und Administratoren dürfen schreiben, Benutzer nur lesen.
	icacls(dd, "/inheritance:r", "/grant:r", "*S-1-5-18:(OI)(CI)F", "*S-1-5-32-544:(OI)(CI)F", "*S-1-5-32-545:(OI)(CI)RX")
	if err := saveConfig(agentConfigPath(), agentCfg); err != nil {
		return "", "", err
	}
	if err := saveConfig(screenConfigPath(), screenCfg); err != nil {
		return "", "", err
	}
	// Den Schlüssel des Verwaltungsdienstes darf kein normaler Benutzer lesen.
	icacls(agentConfigPath(), "/inheritance:r", "/grant:r", "*S-1-5-18:F", "*S-1-5-32-544:F")

	step(3)
	if err := createTask(taskAgent, agentTaskXML(target)); err != nil {
		return "", "", fmt.Errorf("Dienst konnte nicht eingerichtet werden: %w", err)
	}
	if err := createTask(taskScreen, screenTaskXML(target)); err != nil {
		return "", "", fmt.Errorf("Bildschirm-Helfer konnte nicht eingerichtet werden: %w", err)
	}
	registerUninstall(target)
	createShortcut(target)

	step(4)
	quiet(sys32("schtasks.exe"), "/Run", "/TN", taskAgent)
	// Bildschirm-Helfer gleich für die aktuelle Sitzung starten (sonst erst nach der nächsten Anmeldung).
	c := exec.Command(target, "screen")
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000200}
	c.Start()
	return name, agentCfg.PCID, nil
}

func runUninstall(args []string) {
	silent, confirmed := false, false
	for _, a := range args {
		switch a {
		case "--quiet", "/quiet":
			silent = true
		case "--confirmed":
			confirmed = true
		}
	}
	exe, _ := os.Executable()
	if !silent && !confirmed { // erst fragen, dann nach Administratorrechten
		res, err := runUI(dlgUninstallConfirm(), uiOpts{})
		if err != nil {
			if msgBox("Nicolaj Fernwartung von diesem PC entfernen?\n\nDanach ist keine Fernwartung mehr möglich.", mbYesNo|mbIconQuestion) != idYes {
				return
			}
		} else if res != "remove" {
			return
		}
	}
	if !isAdmin() {
		a := []string{"uninstall", "--confirmed"}
		if silent {
			a = append(a, "--quiet")
		}
		if !relaunchElevated(exe, a) && !silent {
			uiNotice("Nicht entfernt", "Zum Entfernen braucht Windows deine Bestätigung als Administrator.", "warn")
		}
		return
	}
	if cfg, err := loadConfig(agentConfigPath()); err == nil {
		newClient(cfg).postJSON("/api/agent/goodbye", map[string]any{}, nil)
	}
	if _, err := os.Stat(pauseFile()); err == nil { // nie mit pausiertem Virenschutz zurücklassen
		runPS(context.Background(), psSecurity+psSecurityScripts["security.resume"], nil, true)
	}
	stopAll()
	quiet(sys32("schtasks.exe"), "/Delete", "/TN", taskAgent, "/F")
	quiet(sys32("schtasks.exe"), "/Delete", "/TN", taskScreen, "/F")
	quiet(sys32("schtasks.exe"), "/Delete", "/TN", taskResume, "/F")
	quiet(sys32("reg.exe"), "delete", uninstKey, "/f")
	os.Remove(shortcutPath())
	os.Remove(chatShortcutPath())
	os.RemoveAll(dataDir())

	// Programmordner löschen, sobald dieses Programm beendet ist.
	cmd := exec.Command(sys32("cmd.exe"))
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true, CreationFlags: 0x08000000 | 0x00000200,
		CmdLine: fmt.Sprintf(`cmd.exe /c ping 127.0.0.1 -n 4 >nul & rmdir /s /q "%s"`, programDir()),
	}
	cmd.Start()
	if !silent {
		if _, err := runUI(dlgUninstallDone(), uiOpts{}); err != nil {
			msgBox("Nicolaj Fernwartung wurde entfernt.", mbIconInfo)
		}
	}
}

// runInfo zeigt PC-ID, PC-Name und Verbindung – für Anrufe wie „Oma, was steht da?“.
func runInfo() {
	if showInfo() == "chat" {
		runChat()
	}
}

// showInfo: Fenster mit PC-ID. Ergebnis "chat", wenn „Nachricht an Nicolaj“ gedrückt wurde.
func showInfo() string {
	cfg, err := loadConfig(screenConfigPath())
	if err != nil {
		cfg, err = loadConfig(agentConfigPath())
	}
	host, _ := os.Hostname()
	info := pcInfo{Installed: err == nil, PCID: cfg.PCID, Host: host, Name: cfg.Name, Checking: true}
	opts := uiOpts{}
	if err == nil {
		opts.Ready = func(ctl *uiCtl) {
			ok := serverReachable(cfg)
			ctl.Update(func(d *uiDialog) {
				i := info
				i.Checking, i.Online = false, ok
				*d = *dlgInfo(i)
			})
		}
	}
	res, uerr := runUI(dlgInfo(info), opts)
	if uerr != nil {
		msgBox(infoText(), mbIconInfo)
		return ""
	}
	if res == "chat" && trayC != nil { // im Bildschirm-Helfer: Fenster dort öffnen
		openChatWindow(trayC, false)
		return ""
	}
	return res
}

func shortcutPath() string {
	pd := os.Getenv("ProgramData")
	if pd == "" {
		pd = `C:\ProgramData`
	}
	return filepath.Join(pd, `Microsoft\Windows\Start Menu\Programs\Nicolaj Fernwartung.lnk`)
}

// createShortcut legt im Startmenü (für alle Benutzer) „Nicolaj Fernwartung“ an → zeigt die PC-ID.
func chatShortcutPath() string {
	return filepath.Join(filepath.Dir(shortcutPath()), "Nachricht an Nicolaj.lnk")
}

func createShortcut(target string) {
	script := `$sh = New-Object -ComObject WScript.Shell
$s = $sh.CreateShortcut($env:NRMM_LNK)
$s.TargetPath = $env:NRMM_TARGET; $s.Arguments = 'info'; $s.IconLocation = $env:NRMM_TARGET + ',0'
$s.Description = 'Zeigt deine PC-ID für Nicolaj Webdesign'; $s.Save()
$c = $sh.CreateShortcut($env:NRMM_CHAT)
$c.TargetPath = $env:NRMM_TARGET; $c.Arguments = 'chat'; $c.IconLocation = $env:NRMM_TARGET + ',0'
$c.Description = 'Schreib Nicolaj Webdesign eine Nachricht'; $c.Save()`
	if _, _, err := runPS(context.Background(), script, map[string]string{"NRMM_LNK": shortcutPath(), "NRMM_CHAT": chatShortcutPath(), "NRMM_TARGET": target}, true); err != nil {
		logf("Startmenü-Einträge: %v", err)
	}
}

// ensureShortcuts legt fehlende Startmenü-Einträge an (z. B. nach einem Update).
func ensureShortcuts() {
	if _, err := os.Stat(chatShortcutPath()); err == nil {
		return
	}
	if exe, err := os.Executable(); err == nil {
		createShortcut(exe)
	}
}

// stopAll beendet laufende Agent-Prozesse (außer diesem).
func stopAll() {
	quiet(sys32("schtasks.exe"), "/End", "/TN", taskAgent)
	quiet(sys32("schtasks.exe"), "/End", "/TN", taskScreen)
	quiet(sys32("taskkill.exe"), "/F", "/FI", "IMAGENAME eq nicolaj-agent.exe", "/FI", "PID ne "+strconv.Itoa(os.Getpid()))
	time.Sleep(time.Second)
}

func quiet(exe string, args ...string) (string, error) {
	c := exec.Command(exe, args...)
	c.SysProcAttr = hidden()
	out, err := c.CombinedOutput()
	return decodeConsole(out), err
}

func icacls(path string, args ...string) {
	if out, err := quiet(sys32("icacls.exe"), append([]string{path}, args...)...); err != nil {
		logf("icacls %s: %v %s", path, err, out)
	}
}

func copyPrefix(src, dst string, n int64) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".neu"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, io.LimitReader(in, n)); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	os.Remove(dst)
	return os.Rename(tmp, dst)
}

func registerUninstall(target string) {
	reg := sys32("reg.exe")
	set := func(name, typ, val string) { quiet(reg, "add", uninstKey, "/v", name, "/t", typ, "/d", val, "/f") }
	set("DisplayName", "REG_SZ", "Nicolaj Fernwartung")
	set("DisplayVersion", "REG_SZ", agentVersion)
	set("Publisher", "REG_SZ", "Nicolaj Webdesign")
	set("URLInfoAbout", "REG_SZ", "https://nicolaj-webdesign.de")
	set("DisplayIcon", "REG_SZ", target)
	set("InstallLocation", "REG_SZ", filepath.Dir(target))
	set("UninstallString", "REG_SZ", `"`+target+`" uninstall`)
	set("QuietUninstallString", "REG_SZ", `"`+target+`" uninstall --quiet`)
	set("NoModify", "REG_DWORD", "1")
	set("NoRepair", "REG_DWORD", "1")
	set("EstimatedSize", "REG_DWORD", "12000")
}

// ---------- Geplante Aufgaben ----------

func createTask(name, xml string) error {
	f, err := os.CreateTemp("", "nrmm-task-*.xml")
	if err != nil {
		return err
	}
	path := f.Name()
	defer os.Remove(path)
	// schtasks erwartet UTF-16 mit BOM
	u := utf16.Encode([]rune(xml))
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xFE})
	binary.Write(&buf, binary.LittleEndian, u)
	f.Write(buf.Bytes())
	f.Close()
	out, err := quiet(sys32("schtasks.exe"), "/Create", "/TN", name, "/XML", path, "/F")
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(out))
	}
	return nil
}

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}

const taskSettings = `<Settings>
    <MultipleInstancesPolicy>%s</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>true</StartWhenAvailable>
    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>
    <IdleSettings><StopOnIdleEnd>false</StopOnIdleEnd><RestartOnIdle>false</RestartOnIdle></IdleSettings>
    <AllowStartOnDemand>true</AllowStartOnDemand>
    <Enabled>true</Enabled>
    <Hidden>false</Hidden>
    <RunOnlyIfIdle>false</RunOnlyIfIdle>
    <WakeToRun>false</WakeToRun>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Priority>7</Priority>
    <RestartOnFailure><Interval>PT1M</Interval><Count>999</Count></RestartOnFailure>
  </Settings>`

func agentTaskXML(exe string) string {
	return `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo><Author>Nicolaj Webdesign</Author><Description>Nicolaj Fernwartung – Verwaltungsdienst (Software, Benutzer, Neustart)</Description></RegistrationInfo>
  <Triggers><BootTrigger><Enabled>true</Enabled></BootTrigger></Triggers>
  <Principals><Principal id="Author"><UserId>S-1-5-18</UserId><RunLevel>HighestAvailable</RunLevel></Principal></Principals>
  ` + fmt.Sprintf(taskSettings, "IgnoreNew") + `
  <Actions Context="Author"><Exec><Command>"` + xmlEscape(exe) + `"</Command><Arguments>service</Arguments></Exec></Actions>
</Task>`
}

func screenTaskXML(exe string) string {
	return `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo><Author>Nicolaj Webdesign</Author><Description>Nicolaj Fernwartung – Bildschirm-Helfer (läuft in der Sitzung des angemeldeten Benutzers)</Description></RegistrationInfo>
  <Triggers><LogonTrigger><Enabled>true</Enabled></LogonTrigger></Triggers>
  <Principals><Principal id="Author"><GroupId>S-1-5-32-545</GroupId><RunLevel>HighestAvailable</RunLevel></Principal></Principals>
  ` + fmt.Sprintf(taskSettings, "Parallel") + `
  <Actions Context="Author"><Exec><Command>"` + xmlEscape(exe) + `"</Command><Arguments>screen</Arguments></Exec></Actions>
</Task>`
}
