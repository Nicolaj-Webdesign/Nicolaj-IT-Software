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
	"syscall"
	"time"
	"unicode/utf16"
)

const (
	taskAgent  = `NicolajRMM\Agent`
	taskScreen = `NicolajRMM\Bildschirm`
	uninstKey  = `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\NicolajRMM`
)

// runDefault: Doppelklick auf die .exe. Mit eingebautem Code → Installation.
func runDefault() {
	exe, _ := os.Executable()
	p, base, err := readPayload(exe)
	if err != nil || p == nil {
		msgBox("Das ist der Agent von Nicolaj Fernwartung.\n\n"+
			"Zum Installieren bitte den Installer aus dem Dashboard herunterladen (Gerät hinzufügen) "+
			"oder per Kommandozeile:\n\nnicolaj-agent.exe install --server https://SERVER:8443 --code CODE --fp FINGERABDRUCK", mbIconInfo)
		return
	}
	doInstall(exe, base, p.Server, p.Code, p.Fingerprint, true, nil)
}

func runInstallCLI(args []string) {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	server := fs.String("server", "", "Serveradresse, z. B. https://192.168.178.20:8443")
	code := fs.String("code", "", "Einrichtungscode aus dem Dashboard")
	fp := fs.String("fp", "", "Fingerabdruck (SHA-256) des Server-Zertifikats")
	quiet := fs.Bool("quiet", false, "keine Rückfragen")
	if err := fs.Parse(args); err != nil || *server == "" || *code == "" {
		msgBox("Aufruf:\nnicolaj-agent.exe install --server https://SERVER:8443 --code CODE --fp FINGERABDRUCK", mbIconWarning)
		return
	}
	exe, _ := os.Executable()
	_, base, _ := readPayload(exe)
	doInstall(exe, base, *server, *code, *fp, !*quiet, append([]string{"install"}, args...))
}

func doInstall(exe string, base int64, server, code, fp string, interactive bool, args []string) {
	if !isAdmin() {
		if !relaunchElevated(exe, args) {
			msgBox("Für die Installation sind Administratorrechte nötig.", mbIconWarning)
		}
		return
	}
	if interactive {
		q := "Nicolaj Fernwartung installieren?\n\n" +
			"Damit kann Nicolaj Webdesign diesen PC aus der Ferne betreuen: Programme installieren und aktualisieren, " +
			"Benutzerkonten verwalten und bei Bedarf den Bildschirm sehen und steuern. " +
			"Während einer Bildschirm-Verbindung zeigt eine rote Leiste oben am Bildschirm, dass jemand verbunden ist.\n\n" +
			"Server: " + server + "\n\n" +
			"Du kannst das jederzeit unter Einstellungen › Apps › „Nicolaj Fernwartung“ wieder entfernen."
		if msgBox(q, mbYesNo|mbIconQuestion) != idYes {
			return
		}
	}
	name, err := install(exe, base, server, code, fp)
	if err != nil {
		msgBox("Die Installation hat nicht geklappt:\n\n"+err.Error(), mbIconError)
		return
	}
	if interactive {
		pc := ""
		if cfg, err := loadConfig(screenConfigPath()); err == nil && cfg.PCID != "" {
			pc = "\n\nDeine PC-ID: " + fmtPCID(cfg.PCID) + "\n(jederzeit im Startmenü unter „Nicolaj Fernwartung“)"
		}
		msgBox("Fertig! Dieser PC ist jetzt als „"+name+"“ mit Nicolaj Fernwartung verbunden."+pc, mbIconInfo)
	}
}

func install(exe string, base int64, server, code, fp string) (string, error) {
	agentCfg, screenCfg, name, err := enroll(server, code, fp)
	if err != nil {
		return "", fmt.Errorf("Anmeldung am Server fehlgeschlagen: %w", err)
	}

	// Alte Version anhalten
	stopAll()

	pd, dd := programDir(), dataDir()
	if err := os.MkdirAll(pd, 0o755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(dd, "logs"), 0o755); err != nil {
		return "", err
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
			return "", fmt.Errorf("Programm konnte nicht kopiert werden: %w", lastErr)
		}
	}

	// Datenordner: nur SYSTEM und Administratoren dürfen schreiben, Benutzer nur lesen.
	icacls(dd, "/inheritance:r", "/grant:r", "*S-1-5-18:(OI)(CI)F", "*S-1-5-32-544:(OI)(CI)F", "*S-1-5-32-545:(OI)(CI)RX")
	if err := saveConfig(agentConfigPath(), agentCfg); err != nil {
		return "", err
	}
	if err := saveConfig(screenConfigPath(), screenCfg); err != nil {
		return "", err
	}
	// Den Schlüssel des Verwaltungsdienstes darf kein normaler Benutzer lesen.
	icacls(agentConfigPath(), "/inheritance:r", "/grant:r", "*S-1-5-18:F", "*S-1-5-32-544:F")

	if err := createTask(taskAgent, agentTaskXML(target)); err != nil {
		return "", fmt.Errorf("Dienst konnte nicht eingerichtet werden: %w", err)
	}
	if err := createTask(taskScreen, screenTaskXML(target)); err != nil {
		return "", fmt.Errorf("Bildschirm-Helfer konnte nicht eingerichtet werden: %w", err)
	}
	registerUninstall(target)
	createShortcut(target)

	quiet(sys32("schtasks.exe"), "/Run", "/TN", taskAgent)
	// Bildschirm-Helfer gleich für die aktuelle Sitzung starten (sonst erst nach der nächsten Anmeldung).
	c := exec.Command(target, "screen")
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000200}
	c.Start()
	return name, nil
}

func runUninstall(args []string) {
	silent := false
	for _, a := range args {
		if a == "--quiet" || a == "/quiet" {
			silent = true
		}
	}
	exe, _ := os.Executable()
	if !isAdmin() {
		if !relaunchElevated(exe, append([]string{"uninstall"}, args...)) && !silent {
			msgBox("Zum Entfernen sind Administratorrechte nötig.", mbIconWarning)
		}
		return
	}
	if !silent && msgBox("Nicolaj Fernwartung von diesem PC entfernen?\n\nDanach ist keine Fernwartung mehr möglich.", mbYesNo|mbIconQuestion) != idYes {
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
	os.RemoveAll(dataDir())

	// Programmordner löschen, sobald dieses Programm beendet ist.
	cmd := exec.Command(sys32("cmd.exe"))
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true, CreationFlags: 0x08000000 | 0x00000200,
		CmdLine: fmt.Sprintf(`cmd.exe /c ping 127.0.0.1 -n 4 >nul & rmdir /s /q "%s"`, programDir()),
	}
	cmd.Start()
	if !silent {
		msgBox("Nicolaj Fernwartung wurde entfernt.", mbIconInfo)
	}
}

// runInfo zeigt PC-Name und PC-ID – für Anrufe wie „Oma, was steht da?“.
func runInfo() {
	msgBox(infoText(), mbIconInfo)
}

func shortcutPath() string {
	pd := os.Getenv("ProgramData")
	if pd == "" {
		pd = `C:\ProgramData`
	}
	return filepath.Join(pd, `Microsoft\Windows\Start Menu\Programs\Nicolaj Fernwartung.lnk`)
}

// createShortcut legt im Startmenü (für alle Benutzer) „Nicolaj Fernwartung“ an → zeigt die PC-ID.
func createShortcut(target string) {
	script := `$s = (New-Object -ComObject WScript.Shell).CreateShortcut($env:NRMM_LNK)
$s.TargetPath = $env:NRMM_TARGET; $s.Arguments = 'info'; $s.IconLocation = $env:NRMM_TARGET + ',0'
$s.Description = 'Zeigt deine PC-ID für Nicolaj Webdesign'; $s.Save()`
	if _, _, err := runPS(context.Background(), script, map[string]string{"NRMM_LNK": shortcutPath(), "NRMM_TARGET": target}, true); err != nil {
		logf("Startmenü-Eintrag: %v", err)
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
