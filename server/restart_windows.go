//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
)

// replaceExecutable: Unter Windows lässt sich ein laufendes Programm umbenennen, aber nicht überschreiben.
func replaceExecutable(exe string, bin []byte) error {
	os.Remove(exe + ".alt")
	if err := os.Rename(exe, exe+".alt"); err != nil {
		return err
	}
	if err := os.WriteFile(exe, bin, 0o755); err != nil {
		os.Rename(exe+".alt", exe)
		return err
	}
	return nil
}

// restartSelf beendet den Server und startet die neue Datei im selben Fenster.
func restartSelf(exe string, a *App) {
	fmt.Println("Starte neu mit neuer Version …")
	if a.srv != nil {
		a.srv.Close()
	}
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "Neustart fehlgeschlagen – bitte Server manuell starten:", err)
		return
	}
	os.Exit(0)
}
