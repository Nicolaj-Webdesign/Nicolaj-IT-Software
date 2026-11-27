package main

import (
	"encoding/json"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/png"
	"math"
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
	st   *uiState // eigener Zustand für die Vorschau
}

// testShot: erfundener Screenshot (Fenster mit Titelleiste und Text-Balken) für Vorschauen.
func testShot(w, h int, accent color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{uint8(40 + 60*x/w), uint8(70 + 50*y/h), 140, 255}
			bx, by := x-w/8, y-h/6
			if bx >= 0 && by >= 0 && bx < w*3/4 && by < h*2/3 { // Fenster
				c = color.RGBA{245, 246, 248, 255}
				if by < h/12 {
					c = color.RGBA{225, 228, 234, 255}
				} else if by%(h/10) < h/30 && bx > w/20 && bx < w/2 {
					c = color.RGBA{180, 186, 196, 255}
				} else if bx > w/2 && bx < w*2/3 && by > h/4 && by < h/2 {
					c = accent
				}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func allScenes() []namedScene {
	d := func(dl *uiDialog) func(measurer, uiState) *uiScene {
		return func(m measurer, st uiState) *uiScene { return layoutDialog(dl, st, m) }
	}
	return []namedScene{
		{"1-installer-willkommen", d(dlgInstallWelcome("https://rmm.nicolaj-webdesign.de", "")), nil},
		{"2-installer-fortschritt", d(dlgInstallProgress(2, 0.55)), nil},
		{"3-installer-fertig", d(dlgInstallDone("Main PC", "190799880")), nil},
		{"4-installer-fehler", d(dlgInstallError("Anmeldung am Server fehlgeschlagen: Der Einrichtungscode ist abgelaufen.")), nil},
		{"1b-zugang-anfragen", d(dlgRequestWelcome("https://rmm.nicolaj-webdesign.de", "Erika Muster", "")), nil},
		{"1c-zugang-leer", d(dlgRequestWelcome("https://rmm.nicolaj-webdesign.de", "", "")), nil},
		{"3b-anfrage-gesendet", d(dlgRequestDone("190799880")), nil},
		{"5-bildschirm-anfrage", d(dlgConsent("nicolaj", 54)), nil},
		{"6-nachricht", d(dlgMessage("Hallo! Ich schaue mir deinen PC gleich kurz an. Du musst nichts machen – lass ihn einfach an. Viele Grüße, Nicolaj")), nil},
		{"7-pc-id", d(dlgInfo(pcInfo{Installed: true, PCID: "190799880", Host: "PCHAUSEN", Name: "Main PC", Online: true})), nil},
		{"8-entfernen", d(dlgUninstallConfirm()), nil},
		{"9-leiste", func(m measurer, st uiState) *uiScene { return layoutBar(barText, st, m) }, nil},
		{"10-chat-leer", d(dlgChat(nil, "", "")), nil},
		{"11-chat", d(dlgChat([]uiChatMsg{
			{Mine: true, Text: "Hallo Nicolaj, mein Drucker druckt nicht mehr. Er zeigt nur noch „Fehler“ an.", Meta: "gestern 18:02 · gelesen"},
			{Mine: false, Text: "Hallo Helga! Ich schau mir das gleich an – lass den PC einfach an.", Meta: "gestern 18:10"},
			{Mine: false, Text: "So, fertig. Die Druckerwarteschlange hing. Probier es mal aus!", Meta: "gestern 18:24"},
			{Mine: true, Text: "Super, geht wieder! Danke dir 😊", Meta: "gerade eben"},
		}, "Ich hätte noch eine Frage zu meinen E-Mails", "Gesendet ✓ – Nicolaj bekommt deine Nachricht im Dashboard.")), nil},
		{"12-chat-bilder-links", d(longChat(false)), nil},
		{"13-chat-hochgescrollt", d(longChat(false)), &uiState{Scroll: 120, Unstuck: true, NewBelow: true, Hover: "scroll", Focus: "input"}},
		{"14-chat-anhang", d(longChat(true)), &uiState{Focus: "input", SelAll: true}},
	}
}

func longChat(attach bool) *uiDialog {
	shot := newUIImage("a", testShot(1280, 800, color.RGBA{229, 72, 77, 255}), 600, 480)
	phone := newUIImage("b", testShot(600, 900, color.RGBA{60, 200, 120, 255}), 600, 480)
	msgs := []uiChatMsg{
		{Mine: true, Text: "Hallo Nicolaj, bei mir kommt beim Drucken immer diese Meldung:", Meta: "gestern 17:58 · gelesen"},
		{Mine: true, HasImg: true, Img: shot, ImgW: 1280, ImgH: 800, Meta: "gestern 17:58 · gelesen"},
		{Mine: false, Text: "Danke für den Screenshot! Lad dir bitte den neuen Treiber hier runter: https://support.hp.com/de-de/drivers/selfservice/hp-deskjet-2700-series/12345678", Meta: "gestern 18:10"},
		{Mine: false, HasImg: true, Img: phone, ImgW: 600, ImgH: 900, Meta: "gestern 18:11"},
		{Mine: false, HasImg: true, ImgW: 1600, ImgH: 900, ImgNote: "Bild wird geladen …", Meta: "gestern 18:12"},
		{Mine: true, Text: "Hat geklappt, danke! Kannst du mir noch www.web.de als Startseite einstellen?", Meta: "gerade eben"},
	}
	d := dlgChat(msgs, "Und noch eine Frage zu den E-Mails", "")
	if attach {
		d.Attach = &uiAttach{Img: shot, Label: "Screenshot · 1280 × 800"}
	}
	return d
}

func TestUILayoutSane(t *testing.T) {
	m := loadMeasurer(t)
	for _, ns := range allScenes() {
		st := uiState{}
		if ns.st != nil {
			st = *ns.st
		}
		s := ns.sc(m, st)
		if s.W <= 0 || s.H <= 0 || s.H > 900 {
			t.Errorf("%s: Größe %vx%v", ns.name, s.W, s.H)
		}
		for i, h := range s.Hits {
			if h.X < 0 || h.Y < 0 || h.X+h.W > s.W+0.01 || h.Y+h.H > s.H+0.01 {
				t.Errorf("%s: Schaltfläche %s außerhalb", ns.name, h.ID)
			}
			for _, o := range s.Hits[i+1:] {
				nested := strings.HasPrefix(h.ID, "msg:") || strings.HasPrefix(o.ID, "msg:") || h.ID == "newbelow" || o.ID == "newbelow" ||
					(h.ID == "input" && o.ID == "attach")
				if !nested && h.X < o.X+o.W && o.X < h.X+h.W && h.Y < o.Y+o.H && o.Y < h.Y+h.H {
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
		string(gEye): "◉", string(gChat): "✉", string(gShield): "⛨", string(gAttach): "⌕", string(gPicture): "▣", string(gDown): "↓"}
	for _, ns := range allScenes() {
		st := uiState{Phase: 0.45, Focus: "input", CaretOn: true}
		if strings.Contains(ns.name, "fertig") {
			st.Hover = "ok"
		}
		if ns.st != nil {
			st = *ns.st
			st.Phase, st.CaretOn = 0.45, true
		}
		s := ns.sc(m, st)
		base := sceneCanvas(s, scale, margin)
		var sb strings.Builder
		fmt.Fprintf(&sb, `<div class="win" style="width:%dpx;height:%dpx;background-image:url('%s.png')">`, base.W, base.H, ns.name)
		for li, ops := range s.layers() {
			c := base
			file := ns.name + ".png"
			if li > 0 { // weitere Ebene: eigenes Bild über dem Text davor
				c = newCanvas(base.W, base.H, scale)
				c.OX, c.OY = base.OX, base.OY
				file = fmt.Sprintf("%s-%d.png", ns.name, li)
				fmt.Fprintf(&sb, `<img src="%s" style="left:0;top:0;width:100%%;height:100%%">`, file)
			}
			c.paintOps(s, ops)
			img := image.NewNRGBA(image.Rect(0, 0, c.W, c.H))
			for i := 0; i < c.W*c.H; i++ {
				b, g, r, a := c.Pix[i*4], c.Pix[i*4+1], c.Pix[i*4+2], c.Pix[i*4+3]
				if a > 0 { // vormultipliert → normal
					r, g, b = uint8(int(r)*255/int(a)), uint8(int(g)*255/int(a)), uint8(int(b)*255/int(a))
				}
				img.SetNRGBA(i%c.W, i/c.W, color.NRGBA{r, g, b, a})
			}
			f, _ := os.Create(filepath.Join(dir, file))
			png.Encode(f, img)
			f.Close()
			for _, o := range ops {
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
				clip := ""
				if c := o.Clip; c != nil {
					clip = fmt.Sprintf(";clip-path:inset(%.1fpx %.1fpx %.1fpx %.1fpx)", math.Max(0, (c.Y-o.Y)*scale), math.Max(0, (o.X+o.W-c.X-c.W)*scale),
						math.Max(0, (o.Y+o.H-c.Y-c.H)*scale), math.Max(0, (c.X-o.X)*scale))
				}
				fmt.Fprintf(&sb, `<div style="left:%.1fpx;top:%.1fpx;width:%.1fpx;height:%.1fpx;font:%d %.1fpx %s;line-height:%.1fpx;color:rgb(%d,%d,%d);text-align:%s%s">%s</div>`,
					(o.X+margin)*scale, (o.Y+margin)*scale, o.W*scale, o.H*scale, wt, o.Font.Size*scale, fam, o.H*scale,
					o.Col.R, o.Col.G, o.Col.B, al, clip, txt)
			}
		}
		sb.WriteString(`</div>`)
		page := `<!doctype html><meta charset="utf-8"><style>
body{margin:0;min-height:100vh;display:grid;place-items:center;padding:40px;box-sizing:border-box;
background:radial-gradient(1200px 700px at 20% 10%,#2a3a8a 0%,transparent 60%),radial-gradient(900px 600px at 90% 90%,#5a2a7a 0%,transparent 55%),#0b0d14}
.win{position:relative;background-size:100% 100%}.win>div,.win>img{position:absolute;white-space:pre;overflow:visible}
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

// App-Symbole fürs Dashboard (PWA): NRMM_WEB_ICONS=/pfad go test ./agent -run TestWebIcons
func TestWebIcons(t *testing.T) {
	dir := os.Getenv("NRMM_WEB_ICONS")
	if dir == "" {
		t.Skip("NRMM_WEB_ICONS nicht gesetzt")
	}
	save := func(name string, c *canvas) {
		img := image.NewNRGBA(image.Rect(0, 0, c.W, c.H))
		for i := 0; i < c.W*c.H; i++ {
			b, g, r, a := c.Pix[i*4], c.Pix[i*4+1], c.Pix[i*4+2], c.Pix[i*4+3]
			if a > 0 {
				r, g, b = uint8(int(r)*255/int(a)), uint8(int(g)*255/int(a)), uint8(int(b)*255/int(a))
			}
			img.SetNRGBA(i%c.W, i/c.W, color.NRGBA{r, g, b, a})
		}
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		png.Encode(f, img)
		f.Close()
	}
	for _, px := range []int{192, 512} { // normales Symbol (abgerundete Kachel)
		c := newCanvas(px, px, float64(px)/64)
		c.logo(0, 0, 64)
		save(fmt.Sprintf("icon-%d.png", px), c)
	}
	for name, px := range map[string]int{"maskable-512.png": 512, "apple-touch-icon.png": 180} { // randlos, Logo im sicheren Bereich
		c := newCanvas(px, px, float64(px)/64)
		c.rr(0, 0, 64, 64, 0, 0, pal.logoTile, false)
		c.logoMark(8, 8, 48)
		save(name, c)
	}
	// Android-Benachrichtigungssymbol: nur weiße Form auf transparent
	c := newCanvas(96, 96, 96.0/64)
	c.logo(0, 0, 64)
	for i := 0; i < 96*96; i++ {
		lum := int(c.Pix[i*4]) + int(c.Pix[i*4+1]) + int(c.Pix[i*4+2])
		a := uint8(0)
		if lum > 3*150 || (c.Pix[i*4] > 180 && c.Pix[i*4+2] < 140) { // „n“ (weiß) und Strich (blau)
			a = 255
		}
		c.Pix[i*4], c.Pix[i*4+1], c.Pix[i*4+2], c.Pix[i*4+3] = a, a, a, a
	}
	save("badge-96.png", c)
}
