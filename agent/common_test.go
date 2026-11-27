package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"image"
	"os"
	"path/filepath"
	"testing"
)

func TestParseWingetUpgradeGerman(t *testing.T) {
	out := cleanOutput("   - \r   \\ \r" + `Name                               ID                           Version       Verfügbar     Quelle
------------------------------------------------------------------------------------------------------
Mozilla Firefox (x64 de)           Mozilla.Firefox.de           132.0.1       133.0         winget
Microsoft Visual C++ 2015-2022 Re… Microsoft.VCRedist.2015+.x64 14.38.33135.0 14.42.34433.0 winget
3 Aktualisierungen verfügbar.
`)
	rows := parseWingetTable(out, "updates")
	if len(rows) != 2 {
		t.Fatalf("2 Zeilen erwartet, bekommen %d: %v", len(rows), rows)
	}
	if rows[0]["id"] != "Mozilla.Firefox.de" || rows[0]["available"] != "133.0" || rows[0]["version"] != "132.0.1" {
		t.Errorf("Zeile 1 falsch: %v", rows[0])
	}
	if rows[1]["id"] != "Microsoft.VCRedist.2015+.x64" || rows[1]["available"] != "14.42.34433.0" {
		t.Errorf("Zeile 2 falsch: %v", rows[1])
	}
}

func TestParseWingetSearchEnglishWithMatch(t *testing.T) {
	out := `Name                         Id                          Version  Match        Source
-------------------------------------------------------------------------------------
Mozilla Firefox              Mozilla.Firefox             133.0    Tag: firefox winget
Mozilla Firefox Developer E… Mozilla.Firefox.DeveloperE… 134.0b5               winget
`
	rows := parseWingetTable(out, "search")
	if len(rows) != 2 {
		t.Fatalf("2 Zeilen erwartet: %v", rows)
	}
	if rows[0]["id"] != "Mozilla.Firefox" || rows[0]["source"] != "winget" || rows[0]["truncated"] != "" {
		t.Errorf("Zeile 1 falsch: %v", rows[0])
	}
	if rows[1]["truncated"] != "1" {
		t.Errorf("gekürzte ID nicht erkannt: %v", rows[1])
	}
}

func TestParseWingetNoTable(t *testing.T) {
	if rows := parseWingetTable("Es wurde kein Paket gefunden, das den Eingabekriterien entspricht.", "search"); len(rows) != 0 {
		t.Errorf("keine Zeilen erwartet: %v", rows)
	}
}

func TestCleanOutputRemovesProgress(t *testing.T) {
	in := "Gefunden VLC [VideoLAN.VLC]\r\n  ██████████████▒▒▒▒▒▒  20.0 MB / 40.0 MB\r  ████████████████████  40.0 MB / 40.0 MB\nErfolgreich installiert\n"
	got := cleanOutput(in)
	want := "Gefunden VLC [VideoLAN.VLC]\nErfolgreich installiert"
	if got != want {
		t.Errorf("cleanOutput:\n%q\nerwartet\n%q", got, want)
	}
}

func TestPayloadRoundtrip(t *testing.T) {
	exe := []byte("MZ-fake-exe-content-0123456789")
	p, _ := json.Marshal(Payload{Server: "https://192.168.1.5:8443", Code: "ABCD-EFGH", Fingerprint: "ab12"})
	var buf bytes.Buffer
	buf.Write(exe)
	buf.Write(p)
	binary.Write(&buf, binary.LittleEndian, uint32(len(p)))
	buf.WriteString("NRMMCFG1")
	path := filepath.Join(t.TempDir(), "setup.exe")
	os.WriteFile(path, buf.Bytes(), 0o644)
	got, base, err := readPayload(path)
	if err != nil || got == nil {
		t.Fatalf("Payload nicht gelesen: %v", err)
	}
	if got.Code != "ABCD-EFGH" || got.Server != "https://192.168.1.5:8443" || base != int64(len(exe)) {
		t.Errorf("falsch: %+v base=%d", got, base)
	}
	plain := filepath.Join(t.TempDir(), "plain.exe")
	os.WriteFile(plain, exe, 0o644)
	if got, _, _ := readPayload(plain); got != nil {
		t.Errorf("ohne Payload sollte nil kommen")
	}
}

func TestScaleAndEncode(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3000, 1500))
	for i := range img.Pix {
		img.Pix[i] = byte(i)
	}
	jpg := encodeFrame(img, "low")
	cfg, _, err := image.DecodeConfig(bytes.NewReader(jpg))
	if err != nil || cfg.Width != 1280 || cfg.Height != 640 {
		t.Errorf("Größe falsch: %v %v", cfg, err)
	}
}

func TestKeyMapComplete(t *testing.T) {
	for _, code := range []string{"KeyA", "KeyZ", "Digit0", "Enter", "Backspace", "ArrowLeft", "MetaLeft", "AltRight", "IntlBackslash", "F12", "Delete"} {
		if _, ok := keyMap[code]; !ok {
			t.Errorf("Taste fehlt: %s", code)
		}
	}
}

func TestCodeFromFilename(t *testing.T) {
	cases := map[string]string{
		"/tmp/Nicolaj-Fernwartung-K7QM-3XPA.exe":     "K7QM-3XPA",
		"/tmp/Nicolaj-Fernwartung-K7QM-3XPA (1).exe": "K7QM-3XPA",
		"/tmp/Nicolaj-Fernwartung-K7QM-3XPA(2).exe":  "K7QM-3XPA",
		"/tmp/Nicolaj-Fernwartung.exe":               "",
		"/tmp/nicolaj-agent.exe":                     "",
		"/tmp/Nicolaj-Fernwartung-k7qm-3xpa.exe":     "",
	}
	for in, want := range cases {
		if got := codeFromFilename(in); got != want {
			t.Errorf("%s → %q, erwartet %q", in, got, want)
		}
	}
}
