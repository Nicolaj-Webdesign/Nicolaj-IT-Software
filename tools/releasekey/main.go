// Release-Schlüssel: signiert die Prüfsummen jedes GitHub-Releases. Der Server übernimmt
// nur Versionen mit gültiger Signatur – selbst wenn jemand an dein GitHub-Konto käme und
// Dateien austauscht, landet nichts Fremdes auf den PCs.
//
//	go run ./tools/releasekey new
//	    → erzeugt ein Schlüsselpaar. Den PRIVATEN Schlüssel als GitHub-Secret
//	      RELEASE_SIGNING_KEY speichern, den ÖFFENTLICHEN im Dashboard eintragen.
//
//	RELEASE_SIGNING_KEY=… go run ./tools/releasekey sign dist/SHA256SUMS.txt
//	    → schreibt dist/SHA256SUMS.txt.sig (macht die GitHub-Action automatisch)
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "new":
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			fail(err)
		}
		fmt.Println("Öffentlicher Schlüssel (ins Dashboard › Einstellungen › GitHub-Releases):")
		fmt.Println("  " + base64.StdEncoding.EncodeToString(pub))
		fmt.Println()
		fmt.Println("Privater Schlüssel (als GitHub-Secret RELEASE_SIGNING_KEY – NIEMALS weitergeben oder ins Repo legen):")
		fmt.Println("  " + base64.StdEncoding.EncodeToString(priv.Seed()))
	case "sign":
		if len(os.Args) < 3 {
			usage()
		}
		seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Getenv("RELEASE_SIGNING_KEY")))
		if err != nil || len(seed) != ed25519.SeedSize {
			fail(fmt.Errorf("RELEASE_SIGNING_KEY fehlt oder ist ungültig"))
		}
		data, err := os.ReadFile(os.Args[2])
		if err != nil {
			fail(err)
		}
		sig := ed25519.Sign(ed25519.NewKeyFromSeed(seed), data)
		out := os.Args[2] + ".sig"
		if err := os.WriteFile(out, []byte(base64.StdEncoding.EncodeToString(sig)+"\n"), 0o644); err != nil {
			fail(err)
		}
		fmt.Println("Signiert:", out)
	default:
		usage()
	}
}

func usage() {
	fmt.Println("Aufruf: releasekey new | releasekey sign <datei>")
	os.Exit(2)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "Fehler:", err)
	os.Exit(1)
}
