package main

import (
	"encoding/json"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Messwerte der Schrift „Inter“ (ähnlich breit wie Segoe UI Variable) – nur für Tests und Vorschau.
type tableMeasurer map[string]map[string]float64

func loadMeasurer(t *testing.T) tableMeasurer {
	b, err := os.ReadFile("testdata/inter-widths.json")
	if err != nil {
		t.Fatal(err)
	}
	var m tableMeasurer
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func (m tableMeasurer) textW(f uiFont, s string) float64 {
	key := "1r"
	switch {
	case f.Face == 0:
		key = "0b"
	case f.Bold:
		key = "1b"
	case f.Face == 2:
		return f.Size
	}
	w := 0.0
	for _, r := range s {
		v, ok := m[key][string(r)]
		if !ok {
			v = 0.6
		}
		w += v
	}
	return w * f.Size
}

type namedScene struct {
	name string
	sc   func(m measurer, st uiState) *uiScene
}

func allScenes() []namedScene {
	d := func(dl *uiDialog) func(measurer, uiState) *uiScene {
		return func(m measurer, st uiState) *uiScene { return layoutDialog(dl, st, m) }
	}
	return []namedScene{
		{"1-installer-willkommen", d(dlgInstallWelcome("https://rmm.nicolaj-webdesign.de", ""))},
		{"2-installer-fortschritt", d(dlgInstallProgress(2, 0.55))},
		{"3-installer-fertig", d(dlgInstallDone("Main PC", "190799880"))},
		{"4-installer-fehler", d(dlgInstallError("Anmeldung am Server fehlgeschlagen: Der Einrichtungscode ist abgelaufen."))},
		{"1b-zugang-anfragen", d(dlgRequestWelcome("https://rmm.nicolaj-webdesign.de", "Erika Muster", ""))},
		{"1c-zugang-leer", d(dlgRequestWelcome("https://rmm.nicolaj-webdesign.de", "", ""))},
		{"3b-anfrage-gesendet", d(dlgRequestDone("190799880"))},
		{"5-bildschirm-anfrage", d(dlgConsent("nicolaj", 54))},
		{"6-nachricht", d(dlgMessage("Hallo! Ich schaue mir deinen PC gleich kurz an. Du musst nichts machen – lass ihn einfach an. Viele Grüße, Nicolaj"))},
		{"7-pc-id", d(dlgInfo(pcInfo{Installed: true, PCID: "190799880", Host: "PCHAUSEN", Name: "Main PC", Online: true}))},
		{"8-entfernen", d(dlgUninstallConfirm())},
		{"9-leiste", func(m measurer, st uiState) *uiScene { return layoutBar(barText, st, m) }},
	}
}

func TestUILayoutSane(t *testing.T) {
	m := loadMeasurer(t)
	for _, ns := range allScenes() {
		s := ns.sc(m, uiState{})
		if s.W <= 0 || s.H <= 0 || s.H > 900 {
			t.Errorf("%s: Größe %vx%v", ns.name, s.W, s.H)
		}
		for i, h := range s.Hits {
			if h.X < 0 || h.Y < 0 || h.X+h.W > s.W+0.01 || h.Y+h.H > s.H+0.01 {
				t.Errorf("%s: Schaltfläche %s außerhalb", ns.name, h.ID)
			}
			for _, o := range s.Hits[i+1:] {
				if h.X < o.X+o.W && o.X < h.X+h.W && h.Y < o.Y+o.H && o.Y < h.Y+h.H {
					t.Errorf("%s: %s überlappt %s", ns.name, h.ID, o.ID)
				}
			}
		}
		for _, o := range s.Ops {
			if o.Kind == opText && o.Font.Face != 2 && m.textW(o.Font, o.Text) > o.W+1 {
				t.Errorf("%s: Text zu breit: %q", ns.name, o.Text)
			}
		}
	}
	// Klick-Erkennung
	s := layoutDialog(dlgConsent("x", 10), uiState{}, m)
	var allow uiHit
	for _, h := range s.Hits {
		if h.ID == "allow" {
			allow = h
		}
	}
	if s.hitAt(allow.X+5, allow.Y+5) != "allow" || s.Default != "allow" {
		t.Errorf("Klick/Enter-Ziel falsch: %q / %q", s.hitAt(allow.X+5, allow.Y+5), s.Default)
	}
	if got := wrapText(m, fBody, "eins zwei drei vier fünf sechs sieben", 80, 0); len(got) < 3 {
		t.Errorf("Umbruch: %q", got)
	}
}

// Vorschau: NRMM_UI_PREVIEW=/pfad go test ./agent -run TestUIPreview
func TestUIPreview(t *testing.T) {
	dir := os.Getenv("NRMM_UI_PREVIEW")
	if dir == "" {
		t.Skip("NRMM_UI_PREVIEW nicht gesetzt")
	}
	m := loadMeasurer(t)
	const scale, margin = 2.0, 26.0
	alt := map[string]string{string(gMonitor): "▭", string(gCheck): "✓", string(gClose): "✕", string(gCopy): "⧉",
		string(gTrash): "⌫", string(gSync): "⟳", string(gError): "!", string(gWarn): "⚠", string(gInfo): "i",
		string(gEye): "◉", string(gChat): "✉", string(gShield): "⛨"}
	for _, ns := range allScenes() {
		st := uiState{Phase: 0.45, Focus: "input", CaretOn: true}
		if strings.Contains(ns.name, "fertig") {
			st.Hover = "ok"
		}
		s := ns.sc(m, st)
		c := renderShapes(s, scale, margin)
		img := image.NewNRGBA(image.Rect(0, 0, c.W, c.H))
		for i := 0; i < c.W*c.H; i++ {
			b, g, r, a := c.Pix[i*4], c.Pix[i*4+1], c.Pix[i*4+2], c.Pix[i*4+3]
			if a > 0 { // vormultipliert → normal
				r, g, b = uint8(int(r)*255/int(a)), uint8(int(g)*255/int(a)), uint8(int(b)*255/int(a))
			}
			img.SetNRGBA(i%c.W, i/c.W, color.NRGBA{r, g, b, a})
		}
		f, _ := os.Create(filepath.Join(dir, ns.name+".png"))
		png.Encode(f, img)
		f.Close()

		var sb strings.Builder
		fmt.Fprintf(&sb, `<div class="win" style="width:%dpx;height:%dpx;background-image:url('%s.png')">`, c.W, c.H, ns.name)
		for _, o := range s.Ops {
			if o.Kind != opText {
				continue
			}
			fam, wt, txt := `'Inter'`, 400, html.EscapeString(o.Text)
			if o.Font.Face == 0 {
				fam, wt = `'Inter Display'`, 600
			} else if o.Font.Bold {
				wt = 600
			}
			if o.Font.Face == 2 {
				fam, txt = `'DejaVu Sans'`, alt[o.Text]
			}
			al := []string{"left", "center", "right"}[o.Align]
			fmt.Fprintf(&sb, `<div style="left:%.1fpx;top:%.1fpx;width:%.1fpx;height:%.1fpx;font:%d %.1fpx %s;line-height:%.1fpx;color:rgb(%d,%d,%d);text-align:%s">%s</div>`,
				(o.X+margin)*scale, (o.Y+margin)*scale, o.W*scale, o.H*scale, wt, o.Font.Size*scale, fam, o.H*scale,
				o.Col.R, o.Col.G, o.Col.B, al, txt)
		}
		sb.WriteString(`</div>`)
		page := `<!doctype html><meta charset="utf-8"><style>
body{margin:0;min-height:100vh;display:grid;place-items:center;padding:40px;box-sizing:border-box;
background:radial-gradient(1200px 700px at 20% 10%,#2a3a8a 0%,transparent 60%),radial-gradient(900px 600px at 90% 90%,#5a2a7a 0%,transparent 55%),#0b0d14}
.win{position:relative;background-size:100% 100%}.win>div{position:absolute;white-space:pre;overflow:visible}
</style>` + sb.String()
		os.WriteFile(filepath.Join(dir, ns.name+".html"), []byte(page), 0o644)
	}
}

// Programm-Symbole für tools/winres: NRMM_ICON_OUT=/pfad go test ./agent -run TestIconPNGs
func TestIconPNGs(t *testing.T) {
	dir := os.Getenv("NRMM_ICON_OUT")
	if dir == "" {
		t.Skip("NRMM_ICON_OUT nicht gesetzt")
	}
	for _, px := range []int{16, 20, 24, 32, 40, 48, 64, 256} {
		c := newCanvas(px, px, float64(px)/64)
		c.logo(0, 0, 64)
		img := image.NewNRGBA(image.Rect(0, 0, px, px))
		for i := 0; i < px*px; i++ {
			b, g, r, a := c.Pix[i*4], c.Pix[i*4+1], c.Pix[i*4+2], c.Pix[i*4+3]
			if a > 0 {
				r, g, b = uint8(int(r)*255/int(a)), uint8(int(g)*255/int(a)), uint8(int(b)*255/int(a))
			}
			img.SetNRGBA(i%px, i/px, color.NRGBA{r, g, b, a})
		}
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("icon-%d.png", px)))
		if err != nil {
			t.Fatal(err)
		}
		png.Encode(f, img)
		f.Close()
	}
}
