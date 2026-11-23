//go:build !windows

package main

import (
	"fmt"
	"os"
	"syscall"
)

// replaceExecutable ersetzt die laufende Programmdatei (die alte bleibt als .alt erhalten).
func replaceExecutable(exe string, bin []byte) error {
	tmp := exe + ".neu"
	if err := os.WriteFile(tmp, bin, 0o755); err != nil {
		return err
	}
	os.Remove(exe + ".alt")
	if err := os.Link(exe, exe+".alt"); err != nil {
		os.Rename(exe, exe+".alt")
	}
	return os.Rename(tmp, exe)
}

// restartSelf startet den Server mit der neuen Datei neu (gleiche Prozess-ID, passt zu systemd).
func restartSelf(exe string, a *App) {
	fmt.Println("Starte neu mit neuer Version …")
	if err := syscall.Exec(exe, os.Args, os.Environ()); err != nil {
		fmt.Fprintln(os.Stderr, "Neustart fehlgeschlagen – bitte Server manuell neu starten:", err)
	}
}
