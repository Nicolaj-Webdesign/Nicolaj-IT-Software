//go:build windows

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	createNoWindow         = 0x08000000
	createNewProcessGroup  = 0x00000200
	createBreakawayFromJob = 0x01000000
)

func updateDir() string { return filepath.Join(dataDir(), "update") }

func installedExe() string { return filepath.Join(programDir(), "nicolaj-agent.exe") }

func oldExePatterns() []string {
	return []string{filepath.Join(programDir(), "nicolaj-agent.old-*.exe"), filepath.Join(programDir(), "nicolaj-agent.exe.fehlerhaft")}
}

// verifyUpdate: Probelauf und – falls der installierte Agent signiert ist – gleiche Signatur verlangen.
func verifyUpdate(path, ver string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, code, err := run(ctx, path, "version")
	if err != nil || code != 0 || !strings.Contains(out, ver) {
		return fmt.Errorf("Probelauf der neuen Version fehlgeschlagen (%v, Code %d, Ausgabe %q)", err, code, strings.TrimSpace(out))
	}
	script := `$cur = Get-AuthenticodeSignature -FilePath $env:NRMM_CUR
$new = Get-AuthenticodeSignature -FilePath $env:NRMM_NEW
if ($cur.Status -eq 'Valid') {
  if ($new.Status -ne 'Valid') { throw "Neue Version ist nicht gültig signiert ($($new.Status))." }
  if ($new.SignerCertificate.Subject -ne $cur.SignerCertificate.Subject) { throw "Neue Version ist von jemand anderem signiert: $($new.SignerCertificate.Subject)" }
  'Signatur geprüft: ' + $new.SignerCertificate.Subject
} else { 'Installierte Version ist unsigniert – Signaturprüfung übersprungen.' }`
	out2, _, err := runPS(ctx, script, map[string]string{"NRMM_CUR": installedExe(), "NRMM_NEW": path}, true)
	logf("Update-Prüfung: %s", strings.TrimSpace(out2))
	if err != nil {
		return errors.New(cleanPSError(out2))
	}
	return nil
}

// installUpdate tauscht die Programmdatei und startet den Abschluss-Prozess.
func installUpdate(path, ver string) (bool, error) {
	target := installedExe()
	old := filepath.Join(programDir(), fmt.Sprintf("nicolaj-agent.old-%d.exe", time.Now().Unix()))
	if err := os.Rename(target, old); err != nil { // laufende Programme lassen sich umbenennen, nicht überschreiben
		return false, fmt.Errorf("Programmdatei konnte nicht umbenannt werden: %w", err)
	}
	st, err := os.Stat(path)
	if err == nil {
		err = copyPrefix(path, target, st.Size())
	}
	if err != nil {
		os.Remove(target)
		os.Rename(old, target)
		return false, fmt.Errorf("Neue Version konnte nicht kopiert werden: %w", err)
	}
	os.WriteFile(filepath.Join(updateDir(), "pending"), []byte(ver), 0o600)

	// Der Abschluss-Prozess läuft aus dem Update-Ordner, damit er bei Bedarf zurücktauschen kann.
	args := []string{"update-finish", "--pid", strconv.Itoa(os.Getpid()), "--version", ver, "--old", old}
	start := func(flags uint32) error {
		cmd := exec.Command(path, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: flags}
		return cmd.Start()
	}
	if err := start(createNoWindow | createNewProcessGroup | createBreakawayFromJob); err != nil {
		if err2 := start(createNoWindow | createNewProcessGroup); err2 != nil {
			os.Remove(target)
			os.Rename(old, target)
			os.Remove(filepath.Join(updateDir(), "pending"))
			return false, fmt.Errorf("Abschluss-Prozess startet nicht: %w", err2)
		}
	}
	return true, nil
}

func exitForUpdate() {
	logf("Beende alten Dienst für das Update")
	time.Sleep(time.Second)
	os.Exit(0)
}

// runUpdateFinish: startet den neuen Dienst und prüft, ob er sich beim Server meldet. Sonst Rückfall.
func runUpdateFinish(args []string) {
	initLog(logPath("agent"))
	fs := flag.NewFlagSet("update-finish", flag.ContinueOnError)
	pid := fs.Int("pid", 0, "")
	ver := fs.String("version", "", "")
	old := fs.String("old", "", "")
	if fs.Parse(args) != nil || *ver == "" || *old == "" {
		return
	}
	logf("Update-Abschluss %s: warte auf Ende des alten Dienstes (PID %d)", *ver, *pid)
	if h, err := syscall.OpenProcess(0x00100000 /* SYNCHRONIZE */, false, uint32(*pid)); err == nil {
		syscall.WaitForSingleObject(h, 60000)
		syscall.CloseHandle(h)
	}
	time.Sleep(time.Second)
	quiet(sys32("schtasks.exe"), "/Run", "/TN", taskAgent)

	ok := filepath.Join(updateDir(), "ok-"+*ver)
	for i := 0; i < 75; i++ { // bis zu 2,5 Minuten
		if _, err := os.Stat(ok); err == nil {
			logf("Update auf %s erfolgreich", *ver)
			os.Remove(*old) // klappt, sobald auch die Bildschirm-Helfer neu gestartet sind
			return
		}
		time.Sleep(2 * time.Second)
	}

	logf("Update auf %s wurde nicht bestätigt – stelle die alte Version wieder her", *ver)
	quiet(sys32("schtasks.exe"), "/End", "/TN", taskAgent)
	quiet(sys32("taskkill.exe"), "/F", "/FI", "IMAGENAME eq nicolaj-agent.exe", "/FI", "USERNAME eq SYSTEM", "/FI", "PID ne "+strconv.Itoa(os.Getpid()))
	time.Sleep(2 * time.Second)
	target := installedExe()
	os.Remove(target + ".fehlerhaft")
	if err := os.Rename(target, target+".fehlerhaft"); err != nil {
		logf("Rückfall: neue Datei lässt sich nicht verschieben: %v", err)
	}
	if err := os.Rename(*old, target); err != nil {
		logf("Rückfall: alte Datei lässt sich nicht zurückholen: %v", err)
	}
	markFailed(*ver)
	os.Remove(filepath.Join(updateDir(), "pending"))
	quiet(sys32("schtasks.exe"), "/Run", "/TN", taskAgent)
	logf("Alte Version wieder gestartet")
}

// watchForUpdate: Der Bildschirm-Helfer startet sich neu, wenn die Programmdatei ausgetauscht wurde
// (aber nie mitten in einer Bildschirm-Sitzung).
func watchForUpdate(busy func() bool) {
	path := installedExe()
	st0, err := os.Stat(path)
	if err != nil {
		return
	}
	for range time.Tick(30 * time.Second) {
		st, err := os.Stat(path)
		if err != nil || (st.Size() == st0.Size() && st.ModTime().Equal(st0.ModTime())) || busy() {
			continue
		}
		time.Sleep(10 * time.Second) // warten, bis die Datei fertig geschrieben ist
		st2, err := os.Stat(path)
		if err != nil || st2.Size() != st.Size() || !st2.ModTime().Equal(st.ModTime()) {
			continue
		}
		logf("Neue Programmversion erkannt – Bildschirm-Helfer startet neu")
		if mutexHandle != 0 {
			syscall.CloseHandle(syscall.Handle(mutexHandle))
			mutexHandle = 0
		}
		cmd := exec.Command(path, "screen")
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup}
		if err := cmd.Start(); err != nil {
			logf("Neustart fehlgeschlagen: %v", err)
			singleInstance("Local\\NicolajRMMScreen")
			continue
		}
		os.Exit(0)
	}
}
