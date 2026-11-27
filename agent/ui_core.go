package main

// Plattformunabhängiger Teil der Fenster im Nicolaj-Look: Farben, Layout und ein kleiner
// Rasterizer mit Kantenglättung. Unter Windows zeichnet ui_windows.go daraus echte Fenster
// (Text über GDI), in den Tests entstehen daraus Vorschaubilder.

import (
	"image"
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
	gAttach  = '\ue723' // Büroklammer
	gPicture = '\ue8b9' // Bild
	gDown    = '\ue74b' // Pfeil nach unten
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
	opImage
	opLayer // alles danach liegt über dem Text davor (z. B. schwebender Knopf)
)

type uiRect struct{ X, Y, W, H float64 }

type uiOp struct {
	Kind       int
	X, Y, W, H float64
	R          float64 // Eckenradius
	SW         float64 // Linienstärke
	Col        rgba
	Text       string
	Font       uiFont
	Align      int      // 0 links, 1 Mitte, 2 rechts
	Clip       *uiRect  // nur innerhalb dieses Rechtecks zeichnen (Scrollbereich)
	Img        *uiImage // bei opImage
}

// uiImage: Bild im Fenster (BGRA, vormultipliert). Die auf Bildschirmgröße skalierte
// Fassung wird zwischengespeichert, weil das Fenster oft neu gezeichnet wird.
type uiImage struct {
	ID     string
	W, H   int
	Pix    []byte
	cw, ch int
	cache  []byte
}

func (im *uiImage) scaled(w, h int) []byte {
	if w == im.W && h == im.H {
		return im.Pix
	}
	if im.cache == nil || im.cw != w || im.ch != h {
		src := &image.RGBA{Pix: im.Pix, Stride: im.W * 4, Rect: image.Rect(0, 0, im.W, im.H)}
		im.cache, im.cw, im.ch = scaleImage(src, w, h).Pix, w, h // Kanalreihenfolge bleibt erhalten
	}
	return im.cache
}

type uiHit struct {
	ID         string
	X, Y, W, H float64
}

type uiScene struct {
	W, H, R float64
	Ops     []uiOp
	Hits    []uiHit
	Default string   // Enter
	Links   []string // Ziele der Links ("link:N")

	// Scrollbereich des Chats
	ScrollMax, ScrollY float64
	ScrollRatio        float64 // Scroll-Pixel pro Pixel Bewegung der Scrollleiste
	ViewTop, ViewH     float64
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
	Multiline                 bool
}

type uiChatMsg struct {
	Mine bool // von der Person am PC
	Text string
	Meta string // Uhrzeit, „gelesen“

	HasImg     bool
	Img        *uiImage // Vorschau (nil, solange sie lädt)
	ImgW, ImgH int      // Originalgröße (für den Platzhalter)
	ImgNote    string   // z. B. „Bild wird geladen …“
}

// uiAttach: Bild, das mit der nächsten Nachricht gesendet wird.
type uiAttach struct {
	Img   *uiImage
	Label string
	Busy  bool // wird noch vorbereitet
}

type uiRow struct{ Label, Value, Tone string }

type uiBullet struct {
	Glyph rune
	Text  string
}

type uiDialog struct {
	Width     float64
	Badge     rune   // Symbol im Kreis; 'L' = Logo; 0 = keins
	Tone      string // accent | ok | danger | warn
	Title     string
	Body      string
	From      string // Chat-Stil: Absender
	Message   string // Chat-Stil: Nachricht
	BigLabel  string
	Big       string
	BigCopy   bool
	Rows      []uiRow
	Bullets   []uiBullet
	Input     *uiInput
	Chat      []uiChatMsg // nicht nil: Gesprächsansicht
	ChatHint  string      // Text, solange noch nichts geschrieben wurde
	Attach    *uiAttach   // angehängtes Bild
	CanAttach bool        // Büroklammer im Eingabefeld
	Progress  float64     // < 0: keiner; 0..1
	Note      string
	Buttons   []uiBtn
	Closable  bool
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

	Scroll   float64 // Chat: Abstand vom Anfang
	Unstuck  bool    // Chat: nicht am Ende (sonst folgt die Ansicht neuen Nachrichten)
	NewBelow bool    // Chat: neue Nachricht unterhalb der Ansicht
	SelAll   bool    // Eingabefeld: alles markiert (Strg+A)
}

type builder struct {
	s    *uiScene
	m    measurer
	st   uiState
	clip *uiRect // gesetzt: alles Folgende wird darauf beschnitten
}

func (b *builder) add(o uiOp) {
	if o.Clip == nil {
		o.Clip = b.clip
	}
	if c := o.Clip; c != nil && o.Kind != opGlow && o.Kind != opLayer &&
		(o.Y+o.H < c.Y || o.Y > c.Y+c.H || o.X+o.W < c.X || o.X > c.X+c.W) {
		return // ganz außerhalb des Scrollbereichs
	}
	b.s.Ops = append(b.s.Ops, o)
}

// hit: Klickfläche (bei aktivem Beschnitt nur der sichtbare Teil).
func (b *builder) hit(id string, x, y, w, h float64) {
	if c := b.clip; c != nil {
		x0, y0 := math.Max(x, c.X), math.Max(y, c.Y)
		x1, y1 := math.Min(x+w, c.X+c.W), math.Min(y+h, c.Y+c.H)
		if x1 <= x0 || y1 <= y0 {
			return
		}
		x, y, w, h = x0, y0, x1-x0, y1-y0
	}
	b.s.Hits = append(b.s.Hits, uiHit{id, x, y, w, h})
}

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

	if d.Chat != nil { // Gesprächsverlauf
		y = b.chat(d, w, y)
	} else if d.Message != "" { // Chat-Stil (Nachricht)
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

	if in := d.Input; in != nil && in.Multiline {
		y += 14
		if d.Attach != nil {
			y = b.attachChip(d.Attach, pad, y, inner) + 8
		}
		y = b.multiInput(d, pad, y, inner)
	} else if in != nil {
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

// ---------- Gesprächsansicht ----------

const chatViewH = 340.0 // Höhe des Scrollbereichs

// uiRun: Stück einer Textzeile; Link >= 0 verweist auf Scene.Links.
type uiRun struct {
	Text string
	Link int
}

// splitLink zerlegt ein Wort in Satzzeichen davor, Link und Satzzeichen danach.
func splitLink(word string) (pre, link, post string) {
	core := strings.TrimLeft(word, "(<[„\"'")
	pre = word[:len(word)-len(core)]
	lc := strings.ToLower(core)
	if !strings.HasPrefix(lc, "https://") && !strings.HasPrefix(lc, "http://") && !strings.HasPrefix(lc, "www.") {
		return word, "", ""
	}
	end := strings.TrimRight(core, ".,;:!?)]>“\"'»…")
	if strings.Count(end, "(") > strings.Count(end, ")") && len(end) < len(core) && core[len(end)] == ')' { // Wikipedia-Links mit Klammern
		end = core[:len(end)+1]
	}
	if len(end) <= len("www.") || strings.HasSuffix(strings.ToLower(end), "://") {
		return word, "", ""
	}
	return pre, end, core[len(end):]
}

// linkURL macht aus dem erkannten Text eine Adresse zum Öffnen.
func linkURL(s string) string {
	if strings.HasPrefix(strings.ToLower(s), "www.") {
		return "https://" + s
	}
	return s
}

type wordPiece struct {
	r    rune
	link int
}

func piecesText(ps []wordPiece) string {
	var sb strings.Builder
	for _, p := range ps {
		sb.WriteRune(p.r)
	}
	return sb.String()
}

func piecesRuns(ps []wordPiece) []uiRun {
	var out []uiRun
	for _, p := range ps {
		if n := len(out); n > 0 && out[n-1].Link == p.link {
			out[n-1].Text += string(p.r)
		} else {
			out = append(out, uiRun{string(p.r), p.link})
		}
	}
	return out
}

// msgLines bricht eine Nachricht um und erkennt dabei Links (auch über Zeilen hinweg).
func (b *builder) msgLines(text string, f uiFont, maxW float64) (lines [][]uiRun, widest float64) {
	m := b.m
	for _, para := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			lines = append(lines, nil)
			continue
		}
		var cur []wordPiece
		flush := func() {
			lines = append(lines, piecesRuns(cur))
			widest = math.Max(widest, m.textW(f, piecesText(cur)))
			cur = nil
		}
		for _, w := range words {
			var ps []wordPiece
			pre, link, post := splitLink(w)
			li := -1
			if link != "" {
				li = len(b.s.Links)
				b.s.Links = append(b.s.Links, linkURL(link))
			}
			for _, r := range pre {
				ps = append(ps, wordPiece{r, -1})
			}
			for _, r := range link {
				ps = append(ps, wordPiece{r, li})
			}
			for _, r := range post {
				ps = append(ps, wordPiece{r, -1})
			}
			if len(cur) > 0 {
				try := append(append(append([]wordPiece{}, cur...), wordPiece{' ', -1}), ps...)
				if m.textW(f, piecesText(try)) <= maxW {
					cur = try
					continue
				}
				flush()
			}
			for len(ps) > 1 && m.textW(f, piecesText(ps)) > maxW { // überlang (z. B. Link): hart trennen
				lo, hi := 1, len(ps)-1
				for lo < hi { // längster Anfang, der passt
					mid := (lo + hi + 1) / 2
					if m.textW(f, piecesText(ps[:mid])) <= maxW {
						lo = mid
					} else {
						hi = mid - 1
					}
				}
				cur = ps[:lo]
				flush()
				ps = ps[lo:]
			}
			cur = ps
		}
		flush()
	}
	return lines, widest
}

// runLines zeichnet umgebrochene Zeilen mit Links (unterstrichen, anklickbar).
func (b *builder) runLines(x, y float64, f uiFont, col, linkCol rgba, lines [][]uiRun) {
	lh := lineH(f)
	for _, ln := range lines {
		cx := x
		for _, r := range ln {
			tw := b.m.textW(f, r.Text)
			c := col
			if r.Link >= 0 {
				id := "link:" + itoa(r.Link)
				c = linkCol
				if b.st.Hover == id {
					c = blendOver(linkCol.alpha(0.75), pal.logoInk)
				}
				b.fill(cx, y+lh-4, tw, 1, 0, c)
				b.hit(id, cx, y, tw, lh)
			}
			b.text(cx, y, tw+1, f, c, 0, r.Text)
			cx += tw
		}
		y += lh
	}
}

func itoa(n int) string {
	if n <= 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}

// chat: Titel und Gesprächsverlauf in einem Scrollbereich.
func (b *builder) chat(d *uiDialog, w, y float64) float64 {
	const pad = 24.0
	inner := w - 2*pad
	m := b.m
	if d.Title != "" {
		b.text(pad, y, inner, fTitle, pal.text, 0, d.Title)
		y += lineH(fTitle) + 2
	}
	if d.Body != "" {
		b.text(pad, y, inner, fSmall, pal.faint, 0, d.Body)
		y += lineH(fSmall)
	}
	y += 14
	if len(d.Chat) == 0 {
		h := 132.0
		b.fill(pad, y, inner, h, 14, pal.surface)
		b.stroke(pad, y, inner, h, 14, 1, pal.surfaceBorder)
		b.logo((w-40)/2, y+22, 40)
		ls := wrapText(m, fBody, d.ChatHint, inner-48, 3)
		b.lines(pad+24, y+74, inner-48, fBody, pal.muted, 1, ls)
		return y + h
	}

	top, viewH := y, chatViewH
	type item struct {
		msg    uiChatMsg
		lines  [][]uiRun
		bw, bh float64 // Textblase
		iw, ih float64 // Bild
		h      float64
	}
	maxBW := inner * 0.8
	items := make([]item, len(d.Chat))
	content := 14.0
	for i, cm := range d.Chat {
		it := item{msg: cm}
		avail := maxBW
		if !cm.Mine {
			avail = math.Min(maxBW, inner-34)
		}
		if cm.HasImg {
			nw, nh := float64(cm.ImgW), float64(cm.ImgH)
			if cm.Img != nil && (nw <= 0 || nh <= 0) {
				nw, nh = float64(cm.Img.W), float64(cm.Img.H)
			}
			if nw <= 0 || nh <= 0 {
				nw, nh = 4, 3
			}
			k := math.Min(1, math.Min(math.Min(avail, 300)/nw, 240/nh))
			it.iw, it.ih = math.Max(64, math.Round(nw*k)), math.Max(48, math.Round(nh*k))
			it.h += it.ih
		}
		if strings.TrimSpace(cm.Text) != "" {
			ls, tw := b.msgLines(cm.Text, fMsg, avail-28)
			it.lines = ls
			it.bw = math.Min(avail, tw+28)
			it.bh = float64(len(ls))*lineH(fMsg) + 20
			if cm.HasImg {
				it.h += 6
			}
			it.h += it.bh
		}
		it.h += 3 + lineH(fSmall) + 10
		items[i] = it
		content += it.h
	}
	content += 2
	maxScroll := math.Max(0, content-viewH)
	sc := b.st.Scroll
	if !b.st.Unstuck || sc > maxScroll {
		sc = maxScroll
	}
	sc = math.Max(0, sc)
	b.s.ScrollMax, b.s.ScrollY, b.s.ViewTop, b.s.ViewH = maxScroll, sc, top, viewH

	b.fill(0, top, w, 1, 0, pal.surfaceBorder)
	b.fill(0, top+viewH-1, w, 1, 0, pal.surfaceBorder)
	b.clip = &uiRect{1, top + 1, w - 2, viewH - 2}
	cy := top + 14 - sc
	for i, it := range items {
		if cy+it.h < top || cy > top+viewH {
			cy += it.h
			continue
		}
		cm := it.msg
		blockW := math.Max(it.iw, it.bw)
		x := pad + 34
		if cm.Mine {
			x = pad + inner - blockW
		}
		id := itoa(i)
		b.hit("msg:"+id, x, cy, blockW, it.h-13-lineH(fSmall))
		yy := cy
		if cm.HasImg {
			ix := x
			if cm.Mine {
				ix = pad + inner - it.iw
			}
			if cm.Img != nil {
				b.add(uiOp{Kind: opImage, X: ix, Y: yy, W: it.iw, H: it.ih, R: 12, Img: cm.Img})
				b.stroke(ix, yy, it.iw, it.ih, 12, 1, hexc("#ffffff1a"))
				if b.st.Hover == "img:"+id {
					b.fill(ix, yy, it.iw, it.ih, 12, hexc("#ffffff14"))
				}
				b.hit("img:"+id, ix, yy, it.iw, it.ih)
			} else {
				b.fill(ix, yy, it.iw, it.ih, 12, pal.surface)
				b.stroke(ix, yy, it.iw, it.ih, 12, 1, pal.surfaceBorder)
				gy := yy + it.ih/2 - 12
				if cm.ImgNote != "" && it.iw > 100 && it.ih > 70 {
					gy = yy + it.ih/2 - 22
					b.text(ix+8, gy+28, it.iw-16, fSmall, pal.faint, 1, cm.ImgNote)
				}
				b.glyph(ix, gy, it.iw, 24, gPicture, 18, pal.faint)
			}
			yy += it.ih
			if it.lines != nil {
				yy += 6
			}
		}
		if it.lines != nil {
			if cm.Mine {
				bx := pad + inner - it.bw
				b.fill(bx, yy, it.bw, it.bh, 14, pal.accent)
				b.runLines(bx+14, yy+10, fMsg, pal.logoInk, pal.logoInk, it.lines)
			} else {
				b.fill(x, yy, it.bw, it.bh, 14, pal.surface)
				b.stroke(x, yy, it.bw, it.bh, 14, 1, pal.surfaceBorder)
				b.runLines(x+14, yy+10, fMsg, pal.text, hexc("#a8b2ff"), it.lines)
			}
			yy += it.bh
		}
		if cm.Mine {
			b.text(pad, yy+3, inner, fSmall, pal.faint, 2, cm.Meta)
		} else {
			b.logo(pad, yy-26, 26)
			b.text(x, yy+3, inner-34, fSmall, pal.faint, 0, "Nicolaj · "+cm.Meta)
		}
		cy += it.h
	}
	b.clip = nil

	if maxScroll > 0 { // Scrollleiste
		trackT, trackH := top+6, viewH-12
		th := math.Max(30, trackH*viewH/content)
		ty := trackT + (trackH-th)*sc/maxScroll
		b.s.ScrollRatio = maxScroll / math.Max(1, trackH-th)
		bw, col := 4.0, pal.faint.alpha(0.45)
		if b.st.Hover == "scroll" || b.st.Press == "scroll" {
			bw, col = 6, pal.muted.alpha(0.75)
		}
		b.fill(w-7-bw, ty, bw, th, bw/2, col)
		b.hit("scroll", w-19, ty, 16, th)
	}
	if b.st.NewBelow && sc < maxScroll-1 { // Hinweis auf neue Nachricht (über dem Text)
		b.add(uiOp{Kind: opLayer})
		label := "Neue Nachricht"
		pw := m.textW(fBarText, label) + 46
		px, py := (w-pw)/2, top+viewH-14-32
		bg := pal.accent
		if b.st.Hover == "newbelow" {
			bg = pal.accentHover
		}
		b.fill(px, py, pw, 32, 16, bg)
		b.glyph(px+12, py, 14, 32, gDown, 10, pal.logoInk)
		b.text(px+32, py+(32-lineH(fBarText))/2, pw-40, fBarText, pal.logoInk, 0, label)
		b.hit("newbelow", px, py, pw, 32)
	}
	return top + viewH
}

// attachChip: angehängtes Bild über dem Eingabefeld.
func (b *builder) attachChip(at *uiAttach, x, y, w float64) float64 {
	h := 60.0
	b.fill(x, y, w, h, 12, pal.surface)
	b.stroke(x, y, w, h, 12, 1, pal.surfaceBorder)
	ts := 44.0
	if at.Img != nil && !at.Busy {
		k := math.Min(ts/float64(at.Img.W), ts/float64(at.Img.H))
		iw, ih := math.Max(8, float64(at.Img.W)*k), math.Max(8, float64(at.Img.H)*k)
		b.add(uiOp{Kind: opImage, X: x + 8 + (ts-iw)/2, Y: y + 8 + (ts-ih)/2, W: iw, H: ih, R: 6, Img: at.Img})
	} else {
		b.fill(x+8, y+8, ts, ts, 8, pal.second)
		b.glyph(x+8, y+8, ts, ts, gPicture, 16, pal.faint)
	}
	label := at.Label
	if at.Busy {
		label = "wird vorbereitet …"
	}
	tx := x + 8 + ts + 12
	tw := w - (tx - x) - 46
	b.text(tx, y+10, tw, fBodyBold, pal.text, 0, "Bild angehängt")
	b.text(tx, y+10+lineH(fBodyBold), tw, fSmall, pal.faint, 0, label)
	b.ghost("unattach", x+w-40, y+(h-30)/2, 30, 30, gClose, 10, pal.faint, pal.text)
	return y + h
}

// multiInput: mehrzeiliges Eingabefeld (wächst bis 5 Zeilen, zeigt das Ende).
func (b *builder) multiInput(d *uiDialog, x, y, w float64) float64 {
	in := d.Input
	m, st := b.m, b.st
	focused := st.Focus == "input"
	tw := w - 28
	if d.CanAttach {
		tw -= 34
	}
	var ls []string
	if in.Value != "" {
		ls = wrapText(m, fMsg, in.Value, tw-4, 0)
		if strings.HasSuffix(in.Value, "\n") {
			ls = append(ls, "")
		}
	}
	n := len(ls)
	if n < 2 {
		n = 2
	}
	if n > 5 {
		n = 5
	}
	if len(ls) > 5 {
		ls = ls[len(ls)-5:]
	}
	fh := float64(n)*lineH(fMsg) + 22
	b.fill(x, y, w, fh, 12, pal.surface)
	if focused {
		b.stroke(x, y, w, fh, 12, 1.5, pal.accent)
	} else {
		b.stroke(x, y, w, fh, 12, 1, pal.border)
	}
	if len(ls) == 0 {
		b.text(x+14, y+11, tw, fMsg, pal.faint, 0, in.Placeholder)
		if focused && st.CaretOn {
			b.fill(x+14, y+14, 1.5, lineH(fMsg)-6, 0.5, pal.text)
		}
	} else {
		if st.SelAll { // alles markiert (Strg+A)
			for i, l := range ls {
				if lw := m.textW(fMsg, l); lw > 0 {
					b.fill(x+13, y+11+float64(i)*lineH(fMsg)+2, lw+3, lineH(fMsg)-3, 3, pal.accent.alpha(0.45))
				}
			}
		}
		b.lines(x+14, y+11, tw, fMsg, pal.text, 0, ls)
		if focused && st.CaretOn && !st.SelAll {
			last := ls[len(ls)-1]
			cy := y + 11 + float64(len(ls)-1)*lineH(fMsg)
			b.fill(x+14+m.textW(fMsg, last)+1, cy+3, 1.5, lineH(fMsg)-6, 0.5, pal.text)
		}
	}
	b.s.Hits = append(b.s.Hits, uiHit{"input", x, y, w, fh})
	if d.CanAttach {
		b.ghost("attach", x+w-40, y+fh-38, 32, 32, gAttach, 15, pal.faint, pal.text)
	}
	return y + fh
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
	rect   []int     // optional: Rechteck (x0, y0, x1, y1) in Geräte-Pixeln, z. B. Scrollbereich
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
	if r := c.rect; r != nil {
		x0, y0, x1, y1 = max(x0, r[0]), max(y0, r[1]), min(x1, r[2]), min(y1, r[3])
	}
	return x0, y0, x1, y1
}

// image zeichnet ein Bild mit abgerundeten Ecken.
func (c *canvas) image(o uiOp) {
	if o.Img == nil || o.Img.W < 1 || o.Img.H < 1 {
		return
	}
	dx, dy := math.Round(o.X*c.S+c.OX), math.Round(o.Y*c.S+c.OY)
	dw, dh := int(math.Round(o.W*c.S)), int(math.Round(o.H*c.S))
	if dw < 1 || dh < 1 || dw > 8000 || dh > 8000 {
		return
	}
	pix := o.Img.scaled(dw, dh)
	cx, cy := dx+float64(dw)/2, dy+float64(dh)/2
	hw, hh, rr := float64(dw)/2, float64(dh)/2, o.R*c.S
	x0, y0, x1, y1 := c.box(o.X, o.Y, o.W, o.H, 1)
	for py := y0; py < y1; py++ {
		sy := py - int(dy)
		if sy < 0 || sy >= dh {
			continue
		}
		for px := x0; px < x1; px++ {
			sx := px - int(dx)
			if sx < 0 || sx >= dw {
				continue
			}
			fx, fy := float64(px)+0.5, float64(py)+0.5
			cov := math.Max(0, math.Min(1, 0.5-sdRRect(fx, fy, cx, cy, hw, hh, rr)))
			if c.clip != nil {
				cov *= math.Max(0, math.Min(1, 0.5-sdRRect(fx, fy, c.clip[0], c.clip[1], c.clip[2], c.clip[3], c.clip[4])))
			}
			if cov <= 0 {
				continue
			}
			s := pix[(sy*dw+sx)*4:]
			a := float64(s[3]) / 255 * cov
			inv := 1 - a
			p := c.Pix[(py*c.W+px)*4 : (py*c.W+px)*4+4 : (py*c.W+px)*4+4]
			p[0] = uint8(math.Min(255, float64(s[0])*cov+float64(p[0])*inv+0.5))
			p[1] = uint8(math.Min(255, float64(s[1])*cov+float64(p[1])*inv+0.5))
			p[2] = uint8(math.Min(255, float64(s[2])*cov+float64(p[2])*inv+0.5))
			p[3] = uint8(math.Min(255, 255*a+float64(p[3])*inv+0.5))
		}
	}
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
	c.logoMark(x, y, size)
}

// logoMark: nur „n“ und Cursor-Strich (ohne Kachel), Koordinaten wie bei logo.
func (c *canvas) logoMark(x, y, size float64) {
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

// layers teilt die Zeichenbefehle an opLayer auf (Text einer Ebene liegt unter der nächsten).
func (s *uiScene) layers() [][]uiOp {
	var out [][]uiOp
	start := 0
	for i, o := range s.Ops {
		if o.Kind == opLayer {
			out = append(out, s.Ops[start:i])
			start = i + 1
		}
	}
	return append(out, s.Ops[start:])
}

// paint zeichnet alle Formen einer Szene (Text übernimmt das jeweilige System).
func (c *canvas) paint(s *uiScene) { c.paintOps(s, s.Ops) }

func (c *canvas) paintOps(s *uiScene, ops []uiOp) {
	c.clip = []float64{s.W/2*c.S + c.OX, s.H/2*c.S + c.OY, s.W / 2 * c.S, s.H / 2 * c.S, s.R * c.S}
	defer func() { c.clip = nil }()
	defer func() { c.rect = nil }()
	for _, o := range ops {
		c.rect = nil
		if r := o.Clip; r != nil {
			c.rect = []int{int(math.Floor(r.X*c.S + c.OX)), int(math.Floor(r.Y*c.S + c.OY)),
				int(math.Ceil((r.X+r.W)*c.S + c.OX)), int(math.Ceil((r.Y+r.H)*c.S + c.OY))}
		}
		switch o.Kind {
		case opImage:
			c.image(o)
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

// sceneCanvas: leere Leinwand mit Schattenrand (Geräte-Pixel).
func sceneCanvas(s *uiScene, scale, margin float64) *canvas {
	w := int(math.Ceil((s.W + 2*margin) * scale))
	h := int(math.Ceil((s.H + 2*margin) * scale))
	c := newCanvas(w, h, scale)
	c.OX, c.OY = margin*scale, margin*scale
	if margin > 0 {
		c.shadow(0, 6, s.W, s.H, s.R, margin*0.9, hexc("#000000").alpha(0.55))
	}
	return c
}

// renderShapes: alle Formen einer Szene mit Schattenrand.
func renderShapes(s *uiScene, scale, margin float64) *canvas {
	c := sceneCanvas(s, scale, margin)
	c.paint(s)
	return c
}
