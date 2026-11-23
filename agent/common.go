package main

import (
	"bytes"
	"image"
	"image/jpeg"
	"regexp"
	"strings"
)

// Capturer nimmt den Bildschirm auf. Capture liefert nil, wenn sich nichts geändert hat
// (außer force ist gesetzt) – so wird bei ruhigem Bildschirm kaum Leistung verbraucht.
type Capturer interface {
	Capture(quality string, force bool) (jpg []byte, w, h int, err error)
	Size() (int, int)
	Close()
}

type closer interface{ Close() }

// ---------- Bildkodierung ----------

var qualityPresets = map[string]struct{ maxW, q int }{
	"high":   {2560, 80},
	"medium": {1920, 62},
	"low":    {1280, 45},
}

func encodeFrame(img *image.RGBA, quality string) []byte {
	p, ok := qualityPresets[quality]
	if !ok {
		p = qualityPresets["medium"]
	}
	b := img.Bounds()
	if b.Dx() > p.maxW {
		nh := b.Dy() * p.maxW / b.Dx()
		img = scaleBox(img, p.maxW, nh)
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: p.q})
	return buf.Bytes()
}

// scaleBox verkleinert ein Bild (Mittelwert über die Quellpixel – gut lesbarer Text).
func scaleBox(src *image.RGBA, dw, dh int) *image.RGBA {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for dy := 0; dy < dh; dy++ {
		y0, y1 := dy*sh/dh, (dy+1)*sh/dh
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for dx := 0; dx < dw; dx++ {
			x0, x1 := dx*sw/dw, (dx+1)*sw/dw
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var r, g, bl, n uint32
			for y := y0; y < y1; y++ {
				row := src.Pix[y*src.Stride:]
				for x := x0; x < x1; x++ {
					i := x * 4
					r += uint32(row[i])
					g += uint32(row[i+1])
					bl += uint32(row[i+2])
					n++
				}
			}
			o := dy*dst.Stride + dx*4
			dst.Pix[o], dst.Pix[o+1], dst.Pix[o+2], dst.Pix[o+3] = uint8(r/n), uint8(g/n), uint8(bl/n), 255
		}
	}
	return dst
}

// bgraToRGBA wandelt Windows-Pixel (BGRA) in ein Go-Bild um.
func bgraToRGBA(src []byte, dst *image.RGBA) {
	p := dst.Pix
	for i := 0; i+3 < len(src) && i+3 < len(p); i += 4 {
		p[i], p[i+1], p[i+2], p[i+3] = src[i+2], src[i+1], src[i], 255
	}
}

// ---------- winget-Ausgabe ----------

var reANSI = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)
var reSpinner = regexp.MustCompile(`^[\s\-\\|/█▒░]*$`)

// cleanOutput entfernt Fortschrittsbalken und Steuerzeichen aus Konsolenausgaben.
func cleanOutput(s string) string {
	s = reANSI.ReplaceAllString(s, "")
	var out []string
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		if i := strings.LastIndex(line, "\r"); i >= 0 {
			line = line[i+1:]
		}
		line = strings.TrimRight(line, " \t")
		if t := strings.TrimSpace(line); t != "" && len(t) < 6 && reSpinner.MatchString(line) {
			continue // Dreh-Animation "- \ | /" (nicht die Trennlinie "-----" der Tabellen)
		}
		if strings.ContainsAny(line, "█▒") && strings.Contains(line, "/") && len([]rune(line)) < 80 {
			continue // Fortschrittsbalken "██████▒▒▒  1.2 MB / 3.4 MB"
		}
		out = append(out, line)
	}
	return strings.TrimSpace(collapseBlank(out))
}

func collapseBlank(lines []string) string {
	var b strings.Builder
	blank := 0
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			blank++
			if blank > 1 {
				continue
			}
		} else {
			blank = 0
		}
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return b.String()
}

// parseWingetTable liest die erste Tabelle aus "winget search" / "winget upgrade".
// Die Spalten werden anhand der Kopfzeile erkannt (funktioniert auch mit deutscher Ausgabe).
func parseWingetTable(out, kind string) []map[string]string {
	lines := strings.Split(out, "\n")
	sep := -1
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if len(t) >= 10 && strings.Trim(t, "-") == "" {
			sep = i
			break
		}
	}
	if sep < 1 {
		return []map[string]string{}
	}
	header := []rune(lines[sep-1])
	var starts []int
	for i, r := range header {
		if r != ' ' && (i == 0 || header[i-1] == ' ') {
			starts = append(starts, i)
		}
	}
	if len(starts) < 3 {
		return []map[string]string{}
	}
	keys := []string{"name", "id", "version"}
	switch {
	case kind == "updates" && len(starts) >= 4:
		keys = append(keys, "available")
	case len(starts) == 5:
		keys = append(keys, "match")
	}
	if len(starts) > len(keys) {
		keys = append(keys, "source")
	}
	rows := []map[string]string{}
	for _, l := range lines[sep+1:] {
		r := []rune(strings.TrimRight(l, " "))
		if len(strings.TrimSpace(string(r))) == 0 {
			break
		}
		if len(r) <= starts[2] { // z. B. Fußzeile "2 Aktualisierungen verfügbar."
			continue
		}
		row := map[string]string{}
		for k := 0; k < len(starts) && k < len(keys); k++ {
			s := starts[k]
			e := len(r)
			if k+1 < len(starts) {
				e = min(starts[k+1], len(r))
			}
			if s >= len(r) {
				break
			}
			row[keys[k]] = strings.TrimSpace(string(r[s:e]))
		}
		if row["id"] == "" || strings.Contains(row["id"], " ") || row["version"] == "" {
			continue
		}
		if kind == "updates" && row["available"] == "" {
			continue
		}
		if strings.HasSuffix(row["id"], "…") {
			row["truncated"] = "1"
		}
		rows = append(rows, row)
	}
	return rows
}

// ---------- Tastatur: Browser-Tastencode → Windows-Scancode ----------

type keyDef struct {
	scan uint16
	vk   uint16 // 0 = aus Scancode und aktuellem Tastaturlayout ermitteln (Buchstaben, Zeichen)
	ext  bool
}

var keyMap = map[string]keyDef{
	"Escape": {0x01, 0x1B, false}, "Digit1": {0x02, 0, false}, "Digit2": {0x03, 0, false}, "Digit3": {0x04, 0, false},
	"Digit4": {0x05, 0, false}, "Digit5": {0x06, 0, false}, "Digit6": {0x07, 0, false}, "Digit7": {0x08, 0, false},
	"Digit8": {0x09, 0, false}, "Digit9": {0x0A, 0, false}, "Digit0": {0x0B, 0, false}, "Minus": {0x0C, 0, false},
	"Equal": {0x0D, 0, false}, "Backspace": {0x0E, 0x08, false}, "Tab": {0x0F, 0x09, false},
	"KeyQ": {0x10, 0, false}, "KeyW": {0x11, 0, false}, "KeyE": {0x12, 0, false}, "KeyR": {0x13, 0, false},
	"KeyT": {0x14, 0, false}, "KeyY": {0x15, 0, false}, "KeyU": {0x16, 0, false}, "KeyI": {0x17, 0, false},
	"KeyO": {0x18, 0, false}, "KeyP": {0x19, 0, false}, "BracketLeft": {0x1A, 0, false}, "BracketRight": {0x1B, 0, false},
	"Enter": {0x1C, 0x0D, false}, "ControlLeft": {0x1D, 0xA2, false},
	"KeyA": {0x1E, 0, false}, "KeyS": {0x1F, 0, false}, "KeyD": {0x20, 0, false}, "KeyF": {0x21, 0, false},
	"KeyG": {0x22, 0, false}, "KeyH": {0x23, 0, false}, "KeyJ": {0x24, 0, false}, "KeyK": {0x25, 0, false},
	"KeyL": {0x26, 0, false}, "Semicolon": {0x27, 0, false}, "Quote": {0x28, 0, false}, "Backquote": {0x29, 0, false},
	"ShiftLeft": {0x2A, 0xA0, false}, "Backslash": {0x2B, 0, false},
	"KeyZ": {0x2C, 0, false}, "KeyX": {0x2D, 0, false}, "KeyC": {0x2E, 0, false}, "KeyV": {0x2F, 0, false},
	"KeyB": {0x30, 0, false}, "KeyN": {0x31, 0, false}, "KeyM": {0x32, 0, false}, "Comma": {0x33, 0, false},
	"Period": {0x34, 0, false}, "Slash": {0x35, 0, false}, "ShiftRight": {0x36, 0xA1, false},
	"NumpadMultiply": {0x37, 0x6A, false}, "AltLeft": {0x38, 0xA4, false}, "Space": {0x39, 0x20, false},
	"CapsLock": {0x3A, 0x14, false},
	"F1":       {0x3B, 0x70, false}, "F2": {0x3C, 0x71, false}, "F3": {0x3D, 0x72, false}, "F4": {0x3E, 0x73, false},
	"F5": {0x3F, 0x74, false}, "F6": {0x40, 0x75, false}, "F7": {0x41, 0x76, false}, "F8": {0x42, 0x77, false},
	"F9": {0x43, 0x78, false}, "F10": {0x44, 0x79, false}, "F11": {0x57, 0x7A, false}, "F12": {0x58, 0x7B, false},
	"NumLock": {0x45, 0x90, true}, "ScrollLock": {0x46, 0x91, false},
	"Numpad7": {0x47, 0x67, false}, "Numpad8": {0x48, 0x68, false}, "Numpad9": {0x49, 0x69, false},
	"NumpadSubtract": {0x4A, 0x6D, false}, "Numpad4": {0x4B, 0x64, false}, "Numpad5": {0x4C, 0x65, false},
	"Numpad6": {0x4D, 0x66, false}, "NumpadAdd": {0x4E, 0x6B, false}, "Numpad1": {0x4F, 0x61, false},
	"Numpad2": {0x50, 0x62, false}, "Numpad3": {0x51, 0x63, false}, "Numpad0": {0x52, 0x60, false},
	"NumpadDecimal": {0x53, 0x6E, false}, "IntlBackslash": {0x56, 0, false},
	"NumpadEnter": {0x1C, 0x0D, true}, "ControlRight": {0x1D, 0xA3, true}, "NumpadDivide": {0x35, 0x6F, true},
	"PrintScreen": {0x37, 0x2C, true}, "AltRight": {0x38, 0xA5, true}, "Pause": {0x45, 0x13, false},
	"Home": {0x47, 0x24, true}, "ArrowUp": {0x48, 0x26, true}, "PageUp": {0x49, 0x21, true},
	"ArrowLeft": {0x4B, 0x25, true}, "ArrowRight": {0x4D, 0x27, true}, "End": {0x4F, 0x23, true},
	"ArrowDown": {0x50, 0x28, true}, "PageDown": {0x51, 0x22, true}, "Insert": {0x52, 0x2D, true},
	"Delete": {0x53, 0x2E, true}, "MetaLeft": {0x5B, 0x5B, true}, "MetaRight": {0x5C, 0x5C, true},
	"OSLeft": {0x5B, 0x5B, true}, "OSRight": {0x5C, 0x5C, true}, "ContextMenu": {0x5D, 0x5D, true},
}
