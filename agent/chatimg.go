package main

// Bilder für den Chat: aus der Zwischenablage (DIB) oder aus Dateien lesen, verkleinern,
// richtig drehen (Handyfotos) und als PNG/JPEG verschicken. Plattformunabhängig und getestet.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	_ "image/gif" // Formate registrieren
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	chatImgMaxSide  = 1920     // längere Seite beim Senden
	chatImgMaxPx    = 50e6     // größere Bilder werden gar nicht erst geöffnet
	chatImgMaxBytes = 40 << 20 // Dateigröße beim Öffnen
	chatImgPNGLimit = 3 << 20  // größer als PNG? Dann als JPEG senden
)

var errNoImage = errors.New("Das ist kein Bild, das sich senden lässt (PNG, JPG, GIF oder BMP).")

func isImageFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".jfif":
		return true
	}
	return false
}

// ---------- DIB (Zwischenablage) und BMP ----------

func decodeBMPFile(b []byte) (image.Image, error) {
	if len(b) < 14+40 || b[0] != 'B' || b[1] != 'M' {
		return nil, errNoImage
	}
	off := int(binary.LittleEndian.Uint32(b[10:14]))
	return decodeDIBAt(b[14:], off-14)
}

// decodeDIB liest ein Bild im Format CF_DIB/CF_DIBV5 (Header + Pixel, ohne Dateikopf).
func decodeDIB(b []byte) (image.Image, error) { return decodeDIBAt(b, -1) }

func decodeDIBAt(b []byte, pixOff int) (image.Image, error) {
	if len(b) < 40 {
		return nil, errNoImage
	}
	le := binary.LittleEndian
	hs := int(le.Uint32(b[0:4]))
	if hs < 40 || hs > len(b) {
		return nil, errNoImage
	}
	w := int(int32(le.Uint32(b[4:8])))
	h := int(int32(le.Uint32(b[8:12])))
	bpp := int(le.Uint16(b[14:16]))
	comp := le.Uint32(b[16:20])
	clrUsed := int(le.Uint32(b[32:36]))
	topDown := h < 0
	if topDown {
		h = -h
	}
	if w < 1 || h < 1 || w > 20000 || h > 20000 || float64(w)*float64(h) > chatImgMaxPx {
		return nil, errors.New("Das Bild ist zu groß.")
	}
	var masks [4]uint32
	pos := hs
	switch comp {
	case 0: // BI_RGB
		switch bpp {
		case 16:
			masks = [4]uint32{0x7C00, 0x03E0, 0x001F, 0}
		case 32:
			masks = [4]uint32{0x00FF0000, 0x0000FF00, 0x000000FF, 0}
		}
	case 3, 6: // BI_BITFIELDS, BI_ALPHABITFIELDS
		if bpp != 16 && bpp != 32 {
			return nil, errNoImage
		}
		n := 3
		if comp == 6 {
			n = 4
		}
		if hs >= 52 {
			for i := 0; i < 3; i++ {
				masks[i] = le.Uint32(b[40+4*i:])
			}
			if hs >= 56 {
				masks[3] = le.Uint32(b[52:])
			}
		} else {
			if len(b) < hs+4*n {
				return nil, errNoImage
			}
			for i := 0; i < n; i++ {
				masks[i] = le.Uint32(b[hs+4*i:])
			}
			pos += 4 * n
		}
	default:
		return nil, errNoImage // komprimierte Bitmaps kommen in der Praxis nicht vor
	}
	var pal []color.NRGBA
	if bpp <= 8 {
		if bpp != 8 && bpp != 4 && bpp != 1 {
			return nil, errNoImage
		}
		n := clrUsed
		if n <= 0 || n > 1<<bpp {
			n = 1 << bpp
		}
		if len(b) < pos+4*n {
			return nil, errNoImage
		}
		for i := 0; i < n; i++ {
			p := b[pos+4*i:]
			pal = append(pal, color.NRGBA{p[2], p[1], p[0], 255})
		}
		pos += 4 * n
	} else if bpp != 16 && bpp != 24 && bpp != 32 {
		return nil, errNoImage
	}
	if pixOff >= pos { // BMP-Datei: Pixel stehen, wo der Dateikopf sagt
		pos = pixOff
	}
	stride := (w*bpp + 31) / 32 * 4
	if len(b) < pos+stride*h {
		return nil, errNoImage
	}
	px := b[pos:]
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	ext := func(v, m uint32) uint8 {
		if m == 0 {
			return 0
		}
		for m&1 == 0 {
			m >>= 1
			v >>= 1
		}
		v &= m
		return uint8(v * 255 / m)
	}
	anyAlpha := false
	for y := 0; y < h; y++ {
		sy := h - 1 - y
		if topDown {
			sy = y
		}
		row := px[sy*stride:]
		o := img.PixOffset(0, y)
		for x := 0; x < w; x++ {
			var c color.NRGBA
			switch bpp {
			case 1, 4, 8:
				bit := x * bpp
				idx := int(row[bit/8]>>(8-bpp-bit%8)) & (1<<bpp - 1)
				if idx < len(pal) {
					c = pal[idx]
				}
			case 24:
				c = color.NRGBA{row[x*3+2], row[x*3+1], row[x*3], 255}
			case 16:
				v := uint32(le.Uint16(row[x*2:]))
				c = color.NRGBA{ext(v, masks[0]), ext(v, masks[1]), ext(v, masks[2]), 255}
			case 32:
				v := le.Uint32(row[x*4:])
				c = color.NRGBA{ext(v, masks[0]), ext(v, masks[1]), ext(v, masks[2]), 255}
				if comp == 0 {
					c.A = uint8(v >> 24)
				} else if masks[3] != 0 {
					c.A = ext(v, masks[3])
				}
				if c.A != 0 {
					anyAlpha = true
				}
			}
			copy(img.Pix[o+x*4:], []byte{c.R, c.G, c.B, c.A})
		}
	}
	if bpp == 32 && !anyAlpha { // Alpha-Kanal ungenutzt (üblich bei Screenshots): deckend
		for i := 3; i < len(img.Pix); i += 4 {
			img.Pix[i] = 255
		}
	}
	return img, nil
}

// encodeDIB: Bild für die Zwischenablage (CF_DIB, 32 Bit, von unten nach oben).
func encodeDIB(img image.Image) []byte {
	bd := img.Bounds()
	w, h := bd.Dx(), bd.Dy()
	out := make([]byte, 40+w*h*4)
	le := binary.LittleEndian
	le.PutUint32(out[0:], 40)
	le.PutUint32(out[4:], uint32(w))
	le.PutUint32(out[8:], uint32(h))
	le.PutUint16(out[12:], 1)
	le.PutUint16(out[14:], 32)
	le.PutUint32(out[20:], uint32(w*h*4))
	rd := pixelReader(img)
	for y := 0; y < h; y++ {
		row := out[40+(h-1-y)*w*4:]
		for x := 0; x < w; x++ {
			r, g, b, a := rd(bd.Min.X+x, bd.Min.Y+y)
			if a != 0 && a != 255 { // vormultipliert → auf Weiß
				r, g, b = r+255-a, g+255-a, b+255-a
			}
			row[x*4], row[x*4+1], row[x*4+2], row[x*4+3] = b, g, r, 255
		}
	}
	return out
}

// ---------- Pixel lesen & skalieren ----------

// pixelReader liefert vormultiplizierte RGBA-Werte – schnell für die üblichen Bildtypen.
func pixelReader(img image.Image) func(x, y int) (r, g, b, a uint8) {
	switch m := img.(type) {
	case *image.RGBA:
		return func(x, y int) (uint8, uint8, uint8, uint8) {
			p := m.Pix[m.PixOffset(x, y):]
			return p[0], p[1], p[2], p[3]
		}
	case *image.NRGBA:
		return func(x, y int) (uint8, uint8, uint8, uint8) {
			p := m.Pix[m.PixOffset(x, y):]
			a := uint32(p[3])
			return uint8(uint32(p[0]) * a / 255), uint8(uint32(p[1]) * a / 255), uint8(uint32(p[2]) * a / 255), p[3]
		}
	case *image.YCbCr:
		return func(x, y int) (uint8, uint8, uint8, uint8) {
			yi, ci := m.YOffset(x, y), m.COffset(x, y)
			r, g, b := color.YCbCrToRGB(m.Y[yi], m.Cb[ci], m.Cr[ci])
			return r, g, b, 255
		}
	case *image.Gray:
		return func(x, y int) (uint8, uint8, uint8, uint8) {
			v := m.Pix[m.PixOffset(x, y)]
			return v, v, v, 255
		}
	}
	return func(x, y int) (uint8, uint8, uint8, uint8) {
		r, g, b, a := img.At(x, y).RGBA()
		return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)
	}
}

// scaleImage verkleinert per Flächenmittel (vergrößert: nächster Nachbar). Ergebnis vormultipliert.
func scaleImage(img image.Image, dw, dh int) *image.RGBA {
	bd := img.Bounds()
	sw, sh := bd.Dx(), bd.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	rd := pixelReader(img)
	for dy := 0; dy < dh; dy++ {
		y0 := dy * sh / dh
		y1 := (dy + 1) * sh / dh
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for dx := 0; dx < dw; dx++ {
			x0 := dx * sw / dw
			x1 := (dx + 1) * sw / dw
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var r, g, b, a, n uint32
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					pr, pg, pb, pa := rd(bd.Min.X+x, bd.Min.Y+y)
					r += uint32(pr)
					g += uint32(pg)
					b += uint32(pb)
					a += uint32(pa)
					n++
				}
			}
			p := dst.Pix[dst.PixOffset(dx, dy):]
			p[0], p[1], p[2], p[3] = uint8(r/n), uint8(g/n), uint8(b/n), uint8(a/n)
		}
	}
	return dst
}

// fitSize: Größe innerhalb von maxW×maxH mit gleichem Seitenverhältnis (nie größer als das Original).
func fitSize(w, h, maxW, maxH int) (int, int) {
	if w <= maxW && h <= maxH {
		return w, h
	}
	k := min(float64(maxW)/float64(w), float64(maxH)/float64(h))
	return max(1, int(float64(w)*k+0.5)), max(1, int(float64(h)*k+0.5))
}

// ---------- Handyfotos richtig drehen (EXIF) ----------

func jpegOrientation(b []byte) int {
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(b) && b[i] == 0xFF {
		marker := b[i+1]
		n := int(binary.BigEndian.Uint16(b[i+2:]))
		if marker == 0xDA || n < 2 || i+2+n > len(b) {
			break
		}
		seg := b[i+4 : i+2+n]
		if marker == 0xE1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			t := seg[6:]
			var bo binary.ByteOrder
			switch string(t[:2]) {
			case "II":
				bo = binary.LittleEndian
			case "MM":
				bo = binary.BigEndian
			default:
				return 1
			}
			ifd := int(bo.Uint32(t[4:8]))
			if ifd+2 > len(t) {
				return 1
			}
			cnt := int(bo.Uint16(t[ifd:]))
			for k := 0; k < cnt; k++ {
				e := ifd + 2 + 12*k
				if e+12 > len(t) {
					break
				}
				if bo.Uint16(t[e:]) == 0x0112 {
					if v := int(bo.Uint16(t[e+8:])); v >= 1 && v <= 8 {
						return v
					}
				}
			}
			return 1
		}
		i += 2 + n
	}
	return 1
}

// orient dreht/spiegelt nach EXIF-Ausrichtung (1 = nichts tun).
func orient(src *image.RGBA, o int) *image.RGBA {
	if o <= 1 || o > 8 {
		return src
	}
	w, h := src.Rect.Dx(), src.Rect.Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var nx, ny int
			switch o {
			case 2:
				nx, ny = w-1-x, y
			case 3:
				nx, ny = w-1-x, h-1-y
			case 4:
				nx, ny = x, h-1-y
			case 5:
				nx, ny = y, x
			case 6:
				nx, ny = h-1-y, x
			case 7:
				nx, ny = h-1-y, w-1-x
			case 8:
				nx, ny = y, w-1-x
			}
			copy(dst.Pix[dst.PixOffset(nx, ny):dst.PixOffset(nx, ny)+4], src.Pix[src.PixOffset(x, y):])
		}
	}
	return dst
}

// ---------- Senden ----------

type preparedImage struct {
	Data []byte
	Mime string
	W, H int
	Img  *image.RGBA // verkleinerte Fassung (für die Vorschau)
}

// prepareChatImage verkleinert auf höchstens 1920 Pixel und kodiert als PNG
// (Screenshots, scharfer Text) oder JPEG (Fotos und sehr große PNGs). Entfernt nebenbei
// alle Metadaten (GPS usw.).
func prepareChatImage(img image.Image, photo bool, orientation int) (*preparedImage, error) {
	bd := img.Bounds()
	if bd.Dx() < 1 || bd.Dy() < 1 {
		return nil, errNoImage
	}
	sw, sh := bd.Dx(), bd.Dy()
	if orientation >= 5 {
		sw, sh = sh, sw
	}
	dw, dh := fitSize(sw, sh, chatImgMaxSide, chatImgMaxSide)
	if orientation >= 5 {
		dw, dh = dh, dw
	}
	small := orient(scaleImage(img, dw, dh), orientation)
	opaque := small.Opaque()
	var buf bytes.Buffer
	if !photo || !opaque {
		enc := png.Encoder{CompressionLevel: png.BestCompression}
		if err := enc.Encode(&buf, small); err != nil {
			return nil, err
		}
		if buf.Len() <= chatImgPNGLimit || !opaque {
			return &preparedImage{buf.Bytes(), "image/png", small.Rect.Dx(), small.Rect.Dy(), small}, nil
		}
		buf.Reset()
	}
	if err := jpeg.Encode(&buf, small, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return &preparedImage{buf.Bytes(), "image/jpeg", small.Rect.Dx(), small.Rect.Dy(), small}, nil
}

// loadImageFile öffnet ein Bild von der Festplatte und bereitet es zum Senden vor.
func loadImageFile(path string) (*preparedImage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, chatImgMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > chatImgMaxBytes {
		return nil, errors.New("Die Datei ist zu groß (höchstens 40 MB).")
	}
	return prepareImageBytes(b)
}

func prepareImageBytes(b []byte) (*preparedImage, error) {
	if len(b) > 2 && b[0] == 'B' && b[1] == 'M' {
		img, err := decodeBMPFile(b)
		if err != nil {
			return nil, err
		}
		return prepareChatImage(img, false, 1)
	}
	img, format, err := decodeImageLimited(b)
	if err != nil {
		return nil, err
	}
	o := 1
	if format == "jpeg" {
		o = jpegOrientation(b)
	}
	return prepareChatImage(img, format == "jpeg", o)
}

// decodeImageLimited prüft erst die Größe, dann wird dekodiert (Schutz vor Riesenbildern).
func decodeImageLimited(b []byte) (image.Image, string, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return nil, "", errNoImage
	}
	if cfg.Width < 1 || cfg.Height < 1 || float64(cfg.Width)*float64(cfg.Height) > chatImgMaxPx {
		return nil, "", errors.New("Das Bild ist zu groß.")
	}
	img, format, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, "", errNoImage
	}
	return img, format, nil
}

// ---------- Anzeige ----------

// newUIImage macht aus einem Bild eine Vorschau fürs Fenster (höchstens maxW×maxH Pixel).
func newUIImage(id string, img image.Image, maxW, maxH int) *uiImage {
	bd := img.Bounds()
	w, h := fitSize(bd.Dx(), bd.Dy(), maxW, maxH)
	small := scaleImage(img, w, h)
	return uiImageFromRGBA(id, small)
}

func uiImageFromRGBA(id string, m *image.RGBA) *uiImage {
	w, h := m.Rect.Dx(), m.Rect.Dy()
	pix := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		s := m.Pix[m.PixOffset(m.Rect.Min.X, m.Rect.Min.Y+y):]
		d := pix[y*w*4:]
		for x := 0; x < w; x++ {
			d[x*4], d[x*4+1], d[x*4+2], d[x*4+3] = s[x*4+2], s[x*4+1], s[x*4], s[x*4+3] // BGRA, vormultipliert
		}
	}
	return &uiImage{ID: id, W: w, H: h, Pix: pix}
}
