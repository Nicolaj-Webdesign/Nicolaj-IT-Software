package main

// Plattformunabhängiger Teil der Fenster im Nicolaj-Look: Farben, Layout und ein kleiner
// Rasterizer mit Kantenglättung. Unter Windows zeichnet ui_windows.go daraus echte Fenster
// (Text über GDI), in den Tests entstehen daraus Vorschaubilder.

import (
	"math"
	"strings"
	"unicode/utf8"
)

// ---------- Farben ----------

type rgba struct{ R, G, B, A uint8 }

func hexc(s string) rgba {
	s = strings.TrimPrefix(s, "#")
	v := func(i int) uint8 {
		var n uint8
		for _, c := range s[i : i+2] {
			n <<= 4
			switch {
			case c >= '0' && c <= '9':
				n |= uint8(c - '0')
			case c >= 'a' && c <= 'f':
				n |= uint8(c-'a') + 10
			case c >= 'A' && c <= 'F':
				n |= uint8(c-'A') + 10
			}
		}
		return n
	}
	c := rgba{v(0), v(2), v(4), 255}
	if len(s) == 8 {
		c.A = v(6)
	}
	return c
}

func (c rgba) alpha(a float64) rgba { c.A = uint8(float64(c.A)*a + 0.5); return c }

// blendOver: halbtransparente Farbe auf einem Untergrund als deckende Farbe (für Text).
func blendOver(c, bg rgba) rgba {
	a := float64(c.A) / 255
	mix := func(x, y uint8) uint8 { return uint8(float64(x)*a + float64(y)*(1-a) + 0.5) }
	return rgba{mix(c.R, bg.R), mix(c.G, bg.G), mix(c.B, bg.B), 255}
}

var pal = struct {
	card, cardTop, border, surface, surfaceBorder          rgba
	text, muted, faint                                     rgba
	accent, accentHover, accentPress                       rgba
	second, secondHover, secondPress, secondText           rgba
	danger, dangerHover, dangerPress, ok, warn, ghostHover rgba
	logoTile, logoInk                                      rgba
}{
	card: hexc("#121419"), cardTop: hexc("#171a21"), border: hexc("#2a2e38"),
	surface: hexc("#1a1d24"), surfaceBorder: hexc("#262a33"),
	text: hexc("#eceef3"), muted: hexc("#a3a9b5"), faint: hexc("#737a86"),
	accent: hexc("#5b6cff"), accentHover: hexc("#6e7dff"), accentPress: hexc("#4b5bef"),
	second: hexc("#252932"), secondHover: hexc("#2e333d"), secondPress: hexc("#20242c"), secondText: hexc("#e9ebf0"),
	danger: hexc("#e5484d"), dangerHover: hexc("#ef5b60"), dangerPress: hexc("#d33c41"),
	ok: hexc("#3ccf7a"), warn: hexc("#f0a94a"), ghostHover: hexc("#252932"),
	logoTile: hexc("#1b1e25"), logoInk: hexc("#ffffff"),
}

func toneColor(t string) rgba {
	switch t {
	case "ok":
		return pal.ok
	case "danger":
		return pal.danger
	case "warn":
		return pal.warn
	}
	return pal.accent
}

// ---------- Symbole (Segoe Fluent Icons / Segoe MDL2 Assets) ----------

const (
	gMonitor = ''
	gCheck   = ''
	gClose   = ''
	gCopy    = ''
	gTrash   = ''
	gSync    = ''
	gError   = ''
	gWarn    = ''
	gInfo    = ''
	gChat    = ''
	gShield  = ''
	gEye     = ''
)

// ---------- Szene ----------

type uiFont struct {
	Face int // 0 = Überschrift (Display), 1 = Text, 2 = Symbole
	Size float64
	Bold bool
}

var (
	fTitle    = uiFont{0, 20, true}
	fBig      = uiFont{0, 30, true}
	fWord     = uiFont{0, 15, true}
	fBody     = uiFont{1, 14, false}
	fBodyBold = uiFont{1, 14, true}
	fMsg      = uiFont{1, 15, false}
	fSmall    = uiFont{1, 12, false}
	fLabel    = uiFont{1, 13, false}
	fBtn      = uiFont{1, 14, true}
	fBarText  = uiFont{1, 13, true}
)

func iconFont(size float64) uiFont { return uiFont{2, size, false} }

const (
	opFill = iota
	opStroke
	opGlow
	opLogo
	opText
)

type uiOp struct {
	Kind       int
	X, Y, W, H float64
	R          float64 // Eckenradius
	SW         float64 // Linienstärke
	Col        rgba
	Text       string
	Font       uiFont
	Align      int // 0 links, 1 Mitte, 2 rechts
}

type uiHit struct {
	ID         string
	X, Y, W, H float64
}

type uiScene struct {
	W, H, R float64
	Ops     []uiOp
	Hits    []uiHit
	Default string // Enter
}

func (s *uiScene) hitAt(x, y float64) string {
	for i := len(s.Hits) - 1; i >= 0; i-- {
		h := s.Hits[i]
		if x >= h.X && x < h.X+h.W && y >= h.Y && y < h.Y+h.H {
			return h.ID
		}
	}
	return ""
}

type measurer interface {
	textW(f uiFont, s string) float64
}

func lineH(f uiFont) float64 {
	switch {
	case f.Face == 0 && f.Size >= 26:
		return math.Round(f.Size * 1.2)
	case f.Face == 0:
		return math.Round(f.Size * 1.3)
	}
	return math.Round(f.Size * 1.5)
}

// wrapText bricht Text auf die verfügbare Breite um (Absätze per \n bleiben erhalten).
func wrapText(m measurer, f uiFont, s string, maxW float64, maxLines int) []string {
	var lines []string
	for _, para := range strings.Split(strings.ReplaceAll(s, "\r", ""), "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		cur := ""
		for _, w := range words {
			try := w
			if cur != "" {
				try = cur + " " + w
			}
			if m.textW(f, try) <= maxW {
				cur = try
				continue
			}
			if cur != "" {
				lines = append(lines, cur)
				cur = ""
			}
			for m.textW(f, w) > maxW && utf8.RuneCountInString(w) > 1 { // überlanges Wort hart trennen
				cut := 0
				for i := range w {
					if i > 0 && m.textW(f, w[:i]) > maxW {
						break
					}
					cut = i
				}
				if cut == 0 {
					_, cut = utf8.DecodeRuneInString(w)
				}
				lines = append(lines, w[:cut])
				w = w[cut:]
			}
			cur = w
		}
		lines = append(lines, cur)
	}
	if maxLines > 0 && len(lines) > maxLines {
		lines = append(lines[:maxLines-1], strings.TrimSpace(lines[maxLines-1])+" …")
	}
	return lines
}

// ---------- Dialog-Beschreibung ----------

type uiBtn struct {
	ID, Label, Kind string // Kind: primary | secondary | danger
	Disabled        bool
}

type uiInput struct {
	Label, Placeholder, Value string
	MaxLen                    int
}

type uiRow struct{ Label, Value, Tone string }

type uiBullet struct {
	Glyph rune
	Text  string
}

type uiDialog struct {
	Width    float64
	Badge    rune   // Symbol im Kreis; 'L' = Logo; 0 = keins
	Tone     string // accent | ok | danger | warn
	Title    string
	Body     string
	From     string // Chat-Stil: Absender
	Message  string // Chat-Stil: Nachricht
	BigLabel string
	Big      string
	BigCopy  bool
	Rows     []uiRow
	Bullets  []uiBullet
	Input    *uiInput
	Progress float64 // < 0: keiner; 0..1
	Note     string
	Buttons  []uiBtn
	Closable bool
}

func (d *uiDialog) button(id string) *uiBtn {
	for i := range d.Buttons {
		if d.Buttons[i].ID == id {
			return &d.Buttons[i]
		}
	}
	return nil
}

type uiState struct {
	Hover, Press string
	Copied       bool
	Phase        float64 // Animation 0..1 (Fortschritt)
	Focus        string  // "input", wenn das Eingabefeld aktiv ist
	CaretOn      bool
}

type builder struct {
	s  *uiScene
	m  measurer
	st uiState
}

func (b *builder) add(o uiOp) { b.s.Ops = append(b.s.Ops, o) }

func (b *builder) fill(x, y, w, h, r float64, c rgba) {
	b.add(uiOp{Kind: opFill, X: x, Y: y, W: w, H: h, R: r, Col: c})
}

func (b *builder) stroke(x, y, w, h, r, sw float64, c rgba) {
	b.add(uiOp{Kind: opStroke, X: x, Y: y, W: w, H: h, R: r, SW: sw, Col: c})
}

func (b *builder) text(x, y, w float64, f uiFont, c rgba, align int, s string) {
	b.add(uiOp{Kind: opText, X: x, Y: y, W: w, H: lineH(f), Font: f, Col: c, Align: align, Text: s})
}

func (b *builder) lines(x, y, w float64, f uiFont, c rgba, align int, ls []string) float64 {
	for _, l := range ls {
		b.text(x, y, w, f, c, align, l)
		y += lineH(f)
	}
	return y
}

func (b *builder) glyph(x, y, w, h float64, g rune, size float64, c rgba) {
	b.add(uiOp{Kind: opText, X: x, Y: y, W: w, H: h, Font: iconFont(size), Col: c, Align: 1, Text: string(g)})
}

func (b *builder) button(id, label, kind string, x, y, w, h float64, disabled bool) {
	bg, fg := pal.second, pal.secondText
	if disabled {
		if kind == "primary" || kind == "danger" {
			bg, fg = toneColor(map[bool]string{true: "danger"}[kind == "danger"]).alpha(0.32), pal.logoInk.alpha(0.55)
		} else {
			fg = pal.faint
		}
		b.fill(x, y, w, h, 10, bg)
		b.add(uiOp{Kind: opText, X: x, Y: y, W: w, H: h, Font: fBtn, Col: blendOver(fg, pal.card), Align: 1, Text: label})
		return
	}
	switch kind {
	case "primary":
		bg, fg = pal.accent, pal.logoInk
		if b.st.Press == id {
			bg = pal.accentPress
		} else if b.st.Hover == id {
			bg = pal.accentHover
		}
	case "danger":
		bg, fg = pal.danger, pal.logoInk
		if b.st.Press == id {
			bg = pal.dangerPress
		} else if b.st.Hover == id {
			bg = pal.dangerHover
		}
	default:
		if b.st.Press == id {
			bg = pal.secondPress
		} else if b.st.Hover == id {
			bg = pal.secondHover
		}
	}
	b.fill(x, y, w, h, 10, bg)
	if kind == "secondary" || kind == "" {
		b.stroke(x, y, w, h, 10, 1, hexc("#ffffff0d"))
	}
	b.add(uiOp{Kind: opText, X: x, Y: y, W: w, H: h, Font: fBtn, Col: fg, Align: 1, Text: label})
	b.s.Hits = append(b.s.Hits, uiHit{id, x, y, w, h})
}

func (b *builder) ghost(id string, x, y, w, h float64, g rune, size float64, c, hover rgba) {
	if b.st.Hover == id || b.st.Press == id {
		b.fill(x, y, w, h, 8, pal.ghostHover)
		c = hover
	}
	b.glyph(x, y, w, h, g, size, c)
	b.s.Hits = append(b.s.Hits, uiHit{id, x, y, w, h})
}

func (b *builder) logo(x, y, size float64) {
	b.add(uiOp{Kind: opLogo, X: x, Y: y, W: size, H: size})
}

// header: Logo + Wortmarke „nicolaj▍ Fernwartung“ (+ Schließen-Knopf)
func (b *builder) header(w float64, closable bool) {
	const pad = 22.0
	b.logo(pad, 18, 22)
	x := pad + 31
	b.text(x, 18, 200, fWord, pal.text, 0, "nicolaj")
	tw := b.m.textW(fWord, "nicolaj")
	b.fill(x+tw+2, 21.5, 2.5, 15, 1, pal.accent)
	b.text(x+tw+12, 19, 200, fLabel, pal.faint, 0, "Fernwartung")
	if closable {
		b.ghost("close", w-14-30, 14, 30, 30, gClose, 10, pal.faint, pal.text)
	}
}

// layoutDialog baut ein Dialogfenster: Kopfzeile, Symbol, Titel, Text, Inhalte, Knöpfe.
func layoutDialog(d *uiDialog, st uiState, m measurer) *uiScene {
	w := d.Width
	if w == 0 {
		w = 420
	}
	s := &uiScene{W: w, R: 14}
	b := &builder{s: s, m: m, st: st}
	const pad = 24.0
	inner := w - 2*pad
	tone := toneColor(d.Tone)

	// Hintergrund: Karte mit leichtem Verlauf oben und Lichtschein hinter dem Symbol
	b.fill(0, 0, w, 2000, s.R, pal.card)
	b.add(uiOp{Kind: opGlow, X: w / 2, Y: 40, W: 210, Col: tone.alpha(0.20)})
	if d.Message != "" {
		b.add(uiOp{Kind: opGlow, X: pad + 30, Y: 70, W: 190, Col: pal.accent.alpha(0.16)})
	}
	b.header(w, d.Closable)
	y := 66.0

	if d.Message != "" { // Chat-Stil (Nachricht)
		b.logo(pad, y+2, 38)
		x := pad + 50
		b.text(x, y, inner-50, fBodyBold, pal.text, 0, d.From)
		b.text(x+m.textW(fBodyBold, d.From)+8, y+1, 120, fSmall, pal.faint, 0, "gerade eben")
		y += lineH(fBodyBold) + 6
		ls := wrapText(m, fMsg, d.Message, inner-50-28, 18)
		bh := float64(len(ls))*lineH(fMsg) + 24
		b.fill(x, y, inner-50, bh, 12, pal.surface)
		b.stroke(x, y, inner-50, bh, 12, 1, pal.surfaceBorder)
		b.lines(x+14, y+12, inner-50-28, fMsg, pal.text, 0, ls)
		y += bh + 4
	} else {
		if d.Badge != 0 {
			bs := 60.0
			bx := (w - bs) / 2
			if d.Badge == 'L' {
				b.logo(bx, y, bs)
			} else {
				b.fill(bx, y, bs, bs, 20, tone.alpha(0.16))
				b.stroke(bx, y, bs, bs, 20, 1, tone.alpha(0.28))
				b.glyph(bx, y, bs, bs, d.Badge, 24, tone)
			}
			y += bs + 18
		}
		if d.Title != "" {
			y = b.lines(pad, y, inner, fTitle, pal.text, 1, wrapText(m, fTitle, d.Title, inner, 3))
		}
		if d.Body != "" {
			y += 6
			y = b.lines(pad, y, inner, fBody, pal.muted, 1, wrapText(m, fBody, d.Body, inner-8, 12))
		}
	}

	if d.Big != "" {
		y += 18
		if d.BigLabel != "" {
			b.text(pad, y, inner, fSmall, pal.faint, 1, strings.ToUpper(d.BigLabel))
			y += lineH(fSmall) + 4
		}
		ph := 66.0
		b.fill(pad, y, inner, ph, 14, pal.surface)
		b.stroke(pad, y, inner, ph, 14, 1, pal.surfaceBorder)
		b.add(uiOp{Kind: opText, X: pad, Y: y, W: inner, H: ph, Font: fBig, Col: pal.text, Align: 1, Text: d.Big})
		if d.BigCopy {
			g, c := gCopy, pal.faint
			if st.Copied {
				g, c = gCheck, pal.ok
			}
			b.ghost("copy", pad+inner-48, y+(ph-38)/2, 38, 38, g, 16, c, pal.text)
		}
		y += ph
	}

	if len(d.Rows) > 0 {
		y += 14
		rh := 38.0
		h := float64(len(d.Rows)) * rh
		b.fill(pad, y, inner, h, 12, pal.surface)
		b.stroke(pad, y, inner, h, 12, 1, pal.surfaceBorder)
		for i, r := range d.Rows {
			ry := y + float64(i)*rh
			if i > 0 {
				b.fill(pad+14, ry, inner-28, 1, 0, pal.surfaceBorder)
			}
			b.text(pad+14, ry+(rh-lineH(fLabel))/2, inner/2, fLabel, pal.faint, 0, r.Label)
			vx := pad + inner - 14
			vw := inner/2 + 20
			if r.Tone != "" {
				tw := m.textW(fLabel, r.Value)
				b.fill(vx-tw-14, ry+rh/2-4, 8, 8, 4, toneColor(r.Tone))
			}
			b.text(vx-vw, ry+(rh-lineH(fLabel))/2, vw, fLabel, pal.text, 2, r.Value)
		}
		y += h
	}

	if in := d.Input; in != nil {
		y += 18
		b.text(pad, y, inner, fLabel, pal.muted, 0, in.Label)
		y += lineH(fLabel) + 6
		fh := 46.0
		focused := st.Focus == "input"
		b.fill(pad, y, inner, fh, 10, pal.surface)
		if focused {
			b.stroke(pad, y, inner, fh, 10, 1.5, pal.accent)
		} else {
			b.stroke(pad, y, inner, fh, 10, 1, pal.border)
		}
		tx, tw := pad+14, inner-28
		if in.Value == "" {
			b.text(tx, y+(fh-lineH(fMsg))/2, tw, fMsg, pal.faint, 0, in.Placeholder)
			if focused && st.CaretOn {
				b.fill(tx, y+13, 1.5, fh-26, 0.5, pal.text)
			}
		} else {
			v := in.Value
			for m.textW(fMsg, v) > tw-4 && v != "" { // nur das Ende zeigen, wenn es nicht passt
				_, n := utf8.DecodeRuneInString(v)
				v = v[n:]
			}
			b.text(tx, y+(fh-lineH(fMsg))/2, tw, fMsg, pal.text, 0, v)
			if focused && st.CaretOn {
				b.fill(tx+m.textW(fMsg, v)+1, y+13, 1.5, fh-26, 0.5, pal.text)
			}
		}
		s.Hits = append(s.Hits, uiHit{"input", pad, y, inner, fh})
		y += fh
	}

	if len(d.Bullets) > 0 {
		y += 18
		rowH := 40.0
		h := float64(len(d.Bullets))*rowH + 8
		b.fill(pad, y, inner, h, 12, pal.surface)
		b.stroke(pad, y, inner, h, 12, 1, pal.surfaceBorder)
		for i, bl := range d.Bullets {
			ry := y + 4 + float64(i)*rowH
			b.fill(pad+12, ry+6, 28, 28, 8, pal.accent.alpha(0.16))
			b.glyph(pad+12, ry+6, 28, 28, bl.Glyph, 13, pal.accentHover)
			b.text(pad+52, ry+(rowH-lineH(fLabel))/2, inner-64, fLabel, pal.text, 0, bl.Text)
		}
		y += h
	}

	if d.Progress >= 0 {
		y += 22
		b.fill(pad, y, inner, 6, 3, pal.second)
		p := math.Max(0, math.Min(1, d.Progress))
		if p > 0 {
			b.fill(pad, y, math.Max(6, inner*p), 6, 3, pal.accent)
		}
		// Glanz, der über den Balken läuft
		gx := pad + (inner+120)*math.Mod(st.Phase, 1) - 60
		b.add(uiOp{Kind: opGlow, X: gx, Y: y + 3, W: 40, Col: hexc("#ffffff").alpha(0.22 * p)})
		y += 6
	}

	if d.Note != "" {
		y += 14
		y = b.lines(pad, y, inner, fSmall, pal.faint, 1, wrapText(m, fSmall, d.Note, inner, 3))
	}

	if len(d.Buttons) > 0 {
		y += 22
		bh := 42.0
		n := float64(len(d.Buttons))
		gap := 10.0
		bw := (inner - gap*(n-1)) / n
		for i, bt := range d.Buttons {
			b.button(bt.ID, bt.Label, bt.Kind, pad+float64(i)*(bw+gap), y, bw, bh, bt.Disabled)
			if bt.Kind == "primary" || bt.Kind == "danger" {
				s.Default = bt.ID
			}
		}
		if s.Default == "" {
			s.Default = d.Buttons[len(d.Buttons)-1].ID
		}
		y += bh
	}
	y += 22
	s.H = math.Ceil(y)
	s.Ops[0].H = s.H // Kartenhintergrund auf die echte Höhe
	b.stroke(0, 0, w, s.H, s.R, 1, pal.border)
	return s
}

// layoutBar: Hinweisleiste oben am Bildschirm während einer Verbindung.
func layoutBar(text string, st uiState, m measurer) *uiScene {
	h := 44.0
	tw := m.textW(fBarText, text)
	btnW := 92.0
	w := math.Ceil(18 + 10 + 10 + tw + 18 + btnW + 6)
	s := &uiScene{W: w, H: h, R: h / 2}
	b := &builder{s: s, m: m, st: st}
	b.fill(0, 0, w, h, s.R, hexc("#16181e"))
	b.stroke(0, 0, w, h, s.R, 1, hexc("#ffffff1a"))
	pulse := 0.55 + 0.45*math.Abs(math.Cos(st.Phase*math.Pi))
	b.fill(18-3, h/2-8, 16, 16, 8, pal.danger.alpha(0.25*pulse))
	b.fill(18+1, h/2-4, 8, 8, 4, pal.danger)
	b.text(18+10+10, (h-lineH(fBarText))/2, tw+4, fBarText, pal.text, 0, text)
	bx := w - btnW - 6
	bg := pal.danger.alpha(0.16)
	if st.Press == "disconnect" {
		bg = pal.danger.alpha(0.34)
	} else if st.Hover == "disconnect" {
		bg = pal.danger.alpha(0.26)
	}
	b.fill(bx, 6, btnW, h-12, (h-12)/2, bg)
	b.add(uiOp{Kind: opText, X: bx, Y: 6, W: btnW, H: h - 12, Font: fBtn, Col: hexc("#ff8a8d"), Align: 1, Text: "Trennen"})
	s.Hits = append(s.Hits, uiHit{"disconnect", bx, 6, btnW, h - 12})
	return s
}

// ---------- Rasterizer (BGRA, vormultipliertes Alpha, von oben nach unten) ----------

type canvas struct {
	W, H   int
	Pix    []byte
	S      float64   // Skalierung (DPI)
	OX, OY float64   // Versatz in Geräte-Pixeln
	clip   []float64 // optional: abgerundetes Rechteck (cx, cy, hw, hh, r), außerhalb wird nicht gezeichnet
}

func newCanvas(w, h int, scale float64) *canvas {
	return &canvas{W: w, H: h, Pix: make([]byte, w*h*4), S: scale}
}

func (c *canvas) blend(i int, col rgba, cov float64) {
	if cov <= 0 {
		return
	}
	if c.clip != nil {
		px, py := float64((i/4)%c.W)+0.5, float64((i/4)/c.W)+0.5
		cov *= math.Max(0, math.Min(1, 0.5-sdRRect(px, py, c.clip[0], c.clip[1], c.clip[2], c.clip[3], c.clip[4])))
		if cov <= 0 {
			return
		}
	}
	a := float64(col.A) / 255 * math.Min(cov, 1)
	if a <= 0 {
		return
	}
	inv := 1 - a
	p := c.Pix[i : i+4 : i+4]
	p[0] = uint8(float64(col.B)*a + float64(p[0])*inv + 0.5)
	p[1] = uint8(float64(col.G)*a + float64(p[1])*inv + 0.5)
	p[2] = uint8(float64(col.R)*a + float64(p[2])*inv + 0.5)
	p[3] = uint8(255*a + float64(p[3])*inv + 0.5)
}

// sdRRect: vorzeichenbehafteter Abstand eines Punkts zum abgerundeten Rechteck (negativ = innen).
func sdRRect(px, py, cx, cy, hw, hh, r float64) float64 {
	r = math.Min(r, math.Min(hw, hh))
	qx := math.Abs(px-cx) - hw + r
	qy := math.Abs(py-cy) - hh + r
	out := math.Hypot(math.Max(qx, 0), math.Max(qy, 0))
	return out + math.Min(math.Max(qx, qy), 0) - r
}

func (c *canvas) box(x, y, w, h, grow float64) (int, int, int, int) {
	x0 := int(math.Floor(x*c.S + c.OX - grow))
	y0 := int(math.Floor(y*c.S + c.OY - grow))
	x1 := int(math.Ceil((x+w)*c.S + c.OX + grow))
	y1 := int(math.Ceil((y+h)*c.S + c.OY + grow))
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > c.W {
		x1 = c.W
	}
	if y1 > c.H {
		y1 = c.H
	}
	return x0, y0, x1, y1
}

func (c *canvas) rr(x, y, w, h, r, sw float64, col rgba, stroke bool) {
	cx, cy := (x+w/2)*c.S+c.OX, (y+h/2)*c.S+c.OY
	hw, hh, rr := w/2*c.S, h/2*c.S, r*c.S
	ssw := math.Max(1, sw*c.S)
	x0, y0, x1, y1 := c.box(x, y, w, h, 2)
	for py := y0; py < y1; py++ {
		for px := x0; px < x1; px++ {
			d := sdRRect(float64(px)+0.5, float64(py)+0.5, cx, cy, hw, hh, rr)
			var cov float64
			if stroke {
				d += ssw / 2 // Linie liegt innen an der Kante
				cov = math.Max(0, math.Min(1, ssw/2+0.5-math.Abs(d)))
			} else {
				cov = math.Max(0, math.Min(1, 0.5-d))
			}
			c.blend((py*c.W+px)*4, col, cov)
		}
	}
}

func smoothstep(e0, e1, x float64) float64 {
	t := math.Max(0, math.Min(1, (x-e0)/(e1-e0)))
	return t * t * (3 - 2*t)
}

// shadow: weicher Schatten unter einer Karte.
func (c *canvas) shadow(x, y, w, h, r, blur float64, col rgba) {
	cx, cy := (x+w/2)*c.S+c.OX, (y+h/2)*c.S+c.OY
	hw, hh, rr, bl := w/2*c.S, h/2*c.S, r*c.S, blur*c.S
	x0, y0, x1, y1 := c.box(x, y, w, h, bl+2)
	for py := y0; py < y1; py++ {
		for px := x0; px < x1; px++ {
			d := sdRRect(float64(px)+0.5, float64(py)+0.5, cx, cy, hw, hh, rr)
			c.blend((py*c.W+px)*4, col, 1-smoothstep(-bl*0.35, bl, d))
		}
	}
}

func (c *canvas) glow(gx, gy, rad float64, col rgba) {
	cx, cy, rr := gx*c.S+c.OX, gy*c.S+c.OY, rad*c.S
	x0, y0, x1, y1 := c.box(gx-rad, gy-rad, 2*rad, 2*rad, 1)
	for py := y0; py < y1; py++ {
		for px := x0; px < x1; px++ {
			d := math.Hypot(float64(px)+0.5-cx, float64(py)+0.5-cy) / rr
			if d < 1 {
				t := 1 - d
				c.blend((py*c.W+px)*4, col, t*t)
			}
		}
	}
}

// logo: Kachel mit „n“ und Cursor-Strich (wie logo.svg), geglättet durch Mehrfachabtastung.
func (c *canvas) logo(x, y, size float64) {
	c.rr(x, y, size, size, size*14/64, 0, pal.logoTile, false)
	c.rr(x, y, size, size, size*14/64, 1, hexc("#ffffff14"), true)
	k := size / 64
	inN := func(u, v float64) bool { // u, v in Logo-Einheiten (0..64)
		if u >= 15 && u <= 21 && v >= 24 && v <= 46 { // linker Strich
			return true
		}
		if u >= 30.3 && u <= 36.5 && v >= 33 && v <= 46 { // rechter Strich
			return true
		}
		if v <= 34 && u >= 15 && u <= 36.5 { // Bogen
			ox, oy := (u-28.25)/8.25, (v-33.0)/9.6
			ix, iy := (u-25.65)/4.65, (v-34.6)/5.9
			if ox*ox+oy*oy <= 1 && ix*ix+iy*iy > 1 && u >= 15 {
				return u >= 20 || v >= 26
			}
		}
		return false
	}
	ink := pal.logoInk
	x0, y0, x1, y1 := c.box(x, y, size, size, 0)
	const n = 4
	for py := y0; py < y1; py++ {
		for px := x0; px < x1; px++ {
			hits := 0
			for sy := 0; sy < n; sy++ {
				for sx := 0; sx < n; sx++ {
					u := ((float64(px)+(float64(sx)+0.5)/n-c.OX)/c.S - x) / k
					v := ((float64(py)+(float64(sy)+0.5)/n-c.OY)/c.S - y) / k
					if inN(u, v) {
						hits++
					}
				}
			}
			if hits > 0 {
				c.blend((py*c.W+px)*4, ink, float64(hits)/(n*n))
			}
		}
	}
	c.rr(x+42*k, y+16*k, 6*k, 34*k, 1.5*k, 0, pal.accent, false)
}

// paint zeichnet alle Formen einer Szene (Text übernimmt das jeweilige System).
func (c *canvas) paint(s *uiScene) {
	c.clip = []float64{s.W/2*c.S + c.OX, s.H/2*c.S + c.OY, s.W / 2 * c.S, s.H / 2 * c.S, s.R * c.S}
	defer func() { c.clip = nil }()
	for _, o := range s.Ops {
		switch o.Kind {
		case opFill:
			c.rr(o.X, o.Y, o.W, o.H, o.R, 0, o.Col, false)
		case opStroke:
			c.rr(o.X, o.Y, o.W, o.H, o.R, o.SW, o.Col, true)
		case opGlow:
			c.glow(o.X, o.Y, o.W, o.Col)
		case opLogo:
			c.logo(o.X, o.Y, o.W)
		}
	}
}

// renderShapes: Szene mit Schattenrand auf eine neue Leinwand (Geräte-Pixel).
func renderShapes(s *uiScene, scale, margin float64) *canvas {
	w := int(math.Ceil((s.W + 2*margin) * scale))
	h := int(math.Ceil((s.H + 2*margin) * scale))
	c := newCanvas(w, h, scale)
	c.OX, c.OY = margin*scale, margin*scale
	if margin > 0 {
		c.shadow(0, 6, s.W, s.H, s.R, margin*0.9, hexc("#000000").alpha(0.55))
	}
	c.paint(s)
	return c
}
