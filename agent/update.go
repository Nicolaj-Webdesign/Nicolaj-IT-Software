package main

// Selbst-Update des Agents – gemeinsamer Teil.
//
// Ablauf: herunterladen (über die geprüfte HTTPS-Verbindung) → SHA-256 prüfen →
// Probelauf + Signaturprüfung → Datei tauschen → Abschluss-Prozess startet den Dienst neu
// und stellt bei Problemen automatisch die alte Version wieder her.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

var (
	updating  atomic.Bool
	reVersion = regexp.MustCompile(`^\d{1,4}(\.\d{1,4}){1,3}$`)
)

func newerVersion(a, b string) bool { // a neuer als b?
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			fmt.Sscan(pa[i], &x)
		}
		if i < len(pb) {
			fmt.Sscan(pb[i], &y)
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func (c *Client) downloadUpdate(dir string) (path, version string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	req, err := c.newReq(ctx, "GET", "/api/agent/update", nil)
	if err != nil {
		return "", "", err
	}
	resp, err := c.stream.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", apiError(resp)
	}
	version = resp.Header.Get("X-Version")
	want := strings.ToLower(resp.Header.Get("X-Sha256"))
	if !reVersion.MatchString(version) || len(want) != 64 {
		return "", "", errors.New("Server lieferte keine gültigen Update-Daten")
	}
	os.MkdirAll(dir, 0o700)
	path = filepath.Join(dir, "nicolaj-agent-"+version+".exe")
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return "", "", err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, 100<<20))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return "", "", err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		os.Remove(tmp)
		return "", "", fmt.Errorf("Prüfsumme stimmt nicht (erwartet %s…, erhalten %s…)", want[:12], got[:12])
	}
	os.Remove(path)
	if err := os.Rename(tmp, path); err != nil {
		return "", "", err
	}
	return path, version, nil
}

func failedRecently(version string) bool {
	st, err := os.Stat(filepath.Join(updateDir(), "failed-"+version))
	return err == nil && time.Since(st.ModTime()) < 24*time.Hour
}

func markFailed(version string) {
	os.WriteFile(filepath.Join(updateDir(), "failed-"+version), []byte(time.Now().Format(time.RFC3339)), 0o600)
}

// handleUpdate lädt und installiert die Version vom Server. force = vom Dashboard ausgelöst.
func handleUpdate(c *Client, force bool) (msg string, restart bool, err error) {
	if !updating.CompareAndSwap(false, true) {
		return "Ein Update läuft bereits.", false, nil
	}
	defer func() {
		if !restart {
			updating.Store(false)
		}
	}()
	path, ver, err := c.downloadUpdate(updateDir())
	if err != nil {
		return "", false, fmt.Errorf("Download fehlgeschlagen: %w", err)
	}
	switch {
	case ver == agentVersion:
		os.Remove(path)
		return "Der Agent ist bereits auf Version " + ver + ".", false, nil
	case !force && !newerVersion(ver, agentVersion):
		os.Remove(path)
		return "Keine neuere Version verfügbar.", false, nil
	case !force && failedRecently(ver):
		os.Remove(path)
		return "Version " + ver + " ist hier zuletzt fehlgeschlagen – automatischer Versuch erst morgen wieder.", false, nil
	}
	logf("Update %s → %s: heruntergeladen und Prüfsumme ok", agentVersion, ver)
	if err := verifyUpdate(path, ver); err != nil {
		markFailed(ver)
		os.Remove(path)
		return "", false, fmt.Errorf("Prüfung fehlgeschlagen, Update abgebrochen: %w", err)
	}
	restart, err = installUpdate(path, ver)
	if err != nil {
		markFailed(ver)
		return "", false, fmt.Errorf("Installation fehlgeschlagen, alte Version bleibt: %w", err)
	}
	if !restart {
		return "(Simulation) Version " + ver + " heruntergeladen und geprüft. Installieren geht nur unter Windows.", false, nil
	}
	return fmt.Sprintf("Update von %s auf %s wird installiert. Der Agent startet in wenigen Sekunden neu; klappt das nicht, stellt er die alte Version automatisch wieder her.", agentVersion, ver), true, nil
}

// markUpdateOK bestätigt dem Abschluss-Prozess, dass die neue Version läuft und den Server erreicht.
func markUpdateOK() {
	p := filepath.Join(updateDir(), "pending")
	b, err := os.ReadFile(p)
	if err != nil || strings.TrimSpace(string(b)) != agentVersion {
		return
	}
	os.WriteFile(filepath.Join(updateDir(), "ok-"+agentVersion), []byte(time.Now().Format(time.RFC3339)), 0o600)
	os.Remove(p)
	logf("Update auf %s bestätigt", agentVersion)
}

// cleanupAfterUpdate räumt alte Programmdateien weg, sobald sie nicht mehr benutzt werden.
func cleanupAfterUpdate() {
	for i := 0; i < 24; i++ {
		time.Sleep(time.Duration(min(i+1, 10)) * time.Minute)
		for _, pat := range oldExePatterns() {
			matches, _ := filepath.Glob(pat)
			for _, m := range matches {
				os.Remove(m) // klappt erst, wenn kein Prozess die Datei mehr nutzt
			}
		}
		entries, _ := os.ReadDir(updateDir())
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				continue
			}
			if strings.HasPrefix(e.Name(), "ok-") && time.Since(info.ModTime()) > 24*time.Hour ||
				strings.HasSuffix(e.Name(), ".exe") && time.Since(info.ModTime()) > 10*time.Minute {
				os.Remove(filepath.Join(updateDir(), e.Name()))
			}
		}
	}
}
