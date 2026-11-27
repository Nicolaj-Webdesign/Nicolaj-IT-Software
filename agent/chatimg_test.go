package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"strings"
	"testing"
)

func TestDIBRoundTrip(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 5, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 5; x++ {
			src.SetNRGBA(x, y, color.NRGBA{uint8(x * 50), uint8(y * 100), 7, 255})
		}
	}
	got, err := decodeDIB(encodeDIB(src))
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 3; y++ {
		for x := 0; x < 5; x++ {
			if r, g, b, _ := got.At(x, y).RGBA(); uint8(r>>8) != uint8(x*50) || uint8(g>>8) != uint8(y*100) || uint8(b>>8) != 7 {
				t.Fatalf("Pixel %d,%d falsch", x, y)
			}
		}
	}
}

// 24 Bit, von unten nach oben, Zeilen auf 4 Byte aufgefüllt – so liefert Windows Screenshots oft.
func TestDIB24AndBMPFile(t *testing.T) {
	w, h := 3, 2
	stride := (w*24 + 31) / 32 * 4
	dib := make([]byte, 40+stride*h)
	le := binary.LittleEndian
	le.PutUint32(dib[0:], 40)
	le.PutUint32(dib[4:], uint32(w))
	le.PutUint32(dib[8:], uint32(h))
	le.PutUint16(dib[12:], 1)
	le.PutUint16(dib[14:], 24)
	// unterste Zeile zuerst: rot; obere Zeile: blau
	for x := 0; x < w; x++ {
		copy(dib[40+x*3:], []byte{0, 0, 255})
		copy(dib[40+stride+x*3:], []byte{255, 0, 0})
	}
	img, err := decodeDIB(dib)
	if err != nil {
		t.Fatal(err)
	}
	if r, _, b, a := img.At(0, 0).RGBA(); b>>8 != 255 || r != 0 || a>>8 != 255 {
		t.Fatal("obere Zeile sollte blau sein")
	}
	if r, _, _, _ := img.At(2, 1).RGBA(); r>>8 != 255 {
		t.Fatal("untere Zeile sollte rot sein")
	}
	file := append([]byte{'B', 'M', 0, 0, 0, 0, 0, 0, 0, 0, 54, 0, 0, 0}, dib...)
	if _, err := decodeBMPFile(file); err != nil {
		t.Fatal("BMP-Datei:", err)
	}
	// 32 Bit mit ungenutztem Alpha → deckend; BITFIELDS-Masken hinter dem Header
	d32 := make([]byte, 40+12+4)
	le.PutUint32(d32[0:], 40)
	le.PutUint32(d32[4:], 1)
	le.PutUint32(d32[8:], 1)
	le.PutUint16(d32[12:], 1)
	le.PutUint16(d32[14:], 32)
	le.PutUint32(d32[16:], 3)
	le.PutUint32(d32[40:], 0x00FF0000)
	le.PutUint32(d32[44:], 0x0000FF00)
	le.PutUint32(d32[48:], 0x000000FF)
	copy(d32[52:], []byte{10, 20, 30, 0})
	img, err = decodeDIB(d32)
	if err != nil {
		t.Fatal(err)
	}
	if c := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA); c != (color.NRGBA{30, 20, 10, 255}) {
		t.Fatalf("BITFIELDS: %v", c)
	}
	// kaputt / zu groß
	for _, bad := range [][]byte{nil, dib[:30], func() []byte { b := append([]byte{}, dib...); le.PutUint32(b[4:], 1e6); return b }()} {
		if _, err := decodeDIB(bad); err == nil {
			t.Error("kaputtes DIB angenommen")
		}
	}
}

func jpegWithOrientation(t *testing.T, w, h, o int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 200
	}
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var b bytes.Buffer
	jpeg.Encode(&b, img, nil)
	// APP1-Exif mit Ausrichtung einfügen (nach SOI)
	exif := []byte("Exif\x00\x00II*\x00\x08\x00\x00\x00\x01\x00\x12\x01\x03\x00\x01\x00\x00\x00")
	exif = append(exif, byte(o), 0, 0, 0, 0, 0, 0, 0)
	seg := append([]byte{0xFF, 0xE1, byte((len(exif) + 2) >> 8), byte(len(exif) + 2)}, exif...)
	out := append([]byte{0xFF, 0xD8}, seg...)
	return append(out, b.Bytes()[2:]...)
}

func TestPrepareImage(t *testing.T) {
	// Handyfoto hochkant gespeichert (Ausrichtung 6): wird gedreht
	b := jpegWithOrientation(t, 40, 20, 6)
	if o := jpegOrientation(b); o != 6 {
		t.Fatalf("Ausrichtung: %d", o)
	}
	p, err := prepareImageBytes(b)
	if err != nil {
		t.Fatal(err)
	}
	if p.W != 20 || p.H != 40 || p.Mime != "image/jpeg" {
		t.Fatalf("gedreht: %dx%d %s", p.W, p.H, p.Mime)
	}
	// großer Screenshot: auf 1920 verkleinert, PNG
	big := image.NewRGBA(image.Rect(0, 0, 3840, 2160))
	for i := 3; i < len(big.Pix); i += 4 {
		big.Pix[i] = 255
	}
	p, err = prepareChatImage(big, false, 1)
	if err != nil || p.W != 1920 || p.H != 1080 || p.Mime != "image/png" {
		t.Fatalf("Screenshot: %v %dx%d %s", err, p.W, p.H, p.Mime)
	}
	if _, err := prepareImageBytes([]byte("kein Bild")); err == nil {
		t.Fatal("Unsinn angenommen")
	}
	if !isImageFile(`C:\Users\Oma\Bilder\Foto.JPG`) || isImageFile("brief.docx") {
		t.Fatal("isImageFile")
	}
	th := newUIImage("x", big, 300, 240)
	if th.W != 300 || th.H != 169 || len(th.Pix) != th.W*th.H*4 {
		t.Fatalf("Vorschau %dx%d", th.W, th.H)
	}
	if s := th.scaled(150, 85); len(s) != 150*85*4 {
		t.Fatal("skalieren")
	}
}

func TestLinksAndWrap(t *testing.T) {
	cases := map[string][3]string{
		"(https://example.org/a).": {"(", "https://example.org/a", ")."},
		"www.web.de,":              {"", "www.web.de", ","},
		"https://de.wikipedia.org/wiki/Test_(Begriff)": {"", "https://de.wikipedia.org/wiki/Test_(Begriff)", ""},
		"Hallo":    {"Hallo", "", ""},
		"https://": {"https://", "", ""},
	}
	for in, want := range cases {
		pre, link, post := splitLink(in)
		if [3]string{pre, link, post} != want {
			t.Errorf("%q → %q %q %q", in, pre, link, post)
		}
	}
	m := loadMeasurer(t)
	b := &builder{s: &uiScene{}, m: m}
	lines, _ := b.msgLines("Lad dir bitte den Treiber hier runter: https://support.hp.com/de-de/drivers/selfservice/hp-deskjet-2700-series/12345678 danke", fMsg, 260)
	if len(b.s.Links) != 1 || b.s.Links[0] != "https://support.hp.com/de-de/drivers/selfservice/hp-deskjet-2700-series/12345678" {
		t.Fatalf("Links: %q", b.s.Links)
	}
	var joined strings.Builder
	linkParts := 0
	for _, ln := range lines {
		w := 0.0
		for _, r := range ln {
			w += m.textW(fMsg, r.Text)
			if r.Link == 0 {
				joined.WriteString(r.Text)
				linkParts++
			}
		}
		if w > 261 {
			t.Errorf("Zeile zu breit: %.0f", w)
		}
	}
	if joined.String() != b.s.Links[0] || linkParts < 2 {
		t.Fatalf("Link über Zeilen: %q (%d Teile)", joined.String(), linkParts)
	}
	b2 := &builder{s: &uiScene{}, m: m}
	b2.msgLines("schau auf www.web.de", fMsg, 300)
	if len(b2.s.Links) != 1 || linkURL("www.web.de") != "https://www.web.de" || b2.s.Links[0] != "https://www.web.de" {
		t.Fatalf("www-Link: %q", b2.s.Links)
	}
}

func TestChatScroll(t *testing.T) {
	m := loadMeasurer(t)
	d := longChat(false)
	s := layoutDialog(d, uiState{}, m)
	if s.ScrollMax <= 0 || s.ScrollY != s.ScrollMax {
		t.Fatalf("sollte unten stehen: %v / %v", s.ScrollY, s.ScrollMax)
	}
	s2 := layoutDialog(d, uiState{Scroll: 50, Unstuck: true}, m)
	if s2.ScrollY != 50 {
		t.Fatalf("Scroll-Position: %v", s2.ScrollY)
	}
	for _, o := range s2.Ops { // nichts aus dem Verlauf ragt aus dem Bereich
		if o.Clip != nil && (o.Kind == opText || o.Kind == opImage) && (o.Y+o.H < o.Clip.Y || o.Y > o.Clip.Y+o.Clip.H) {
			t.Errorf("unsichtbares Element gezeichnet: %+v", o.Text)
		}
	}
	for _, h := range s2.Hits {
		if strings.HasPrefix(h.ID, "msg:") || strings.HasPrefix(h.ID, "img:") || strings.HasPrefix(h.ID, "link:") {
			if h.Y < s2.ViewTop || h.Y+h.H > s2.ViewTop+s2.ViewH {
				t.Errorf("%s ragt aus dem Scrollbereich", h.ID)
			}
		}
	}
	// ganz oben: erste Nachricht sichtbar und anklickbar
	s3 := layoutDialog(d, uiState{Scroll: 0, Unstuck: true}, m)
	if s3.hitAt(460-30, s3.ViewTop+30) != "msg:0" {
		t.Errorf("erste Nachricht nicht anklickbar: %q", s3.hitAt(460-30, s3.ViewTop+30))
	}
}
