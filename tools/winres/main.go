// winres erzeugt eine Windows-Ressourcendatei (.syso) für den Agent: Programm-Symbol,
// Dateiinfos (Name, Firma, Version) und ein Manifest. Nur Standardbibliothek.
//
//	go run ./tools/winres -version 1.1.0 -o agent/rsrc_windows_amd64.syso
//
// Go bindet *.syso-Dateien im Paketordner automatisch mit ein.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf16"
)

const (
	rtIcon      = 3
	rtGroupIcon = 14
	rtVersion   = 16
	rtManifest  = 24
	langDE      = 0x0407
)

func main() {
	version := flag.String("version", "0.0.0", "Programmversion, z. B. 1.1.0")
	out := flag.String("o", "agent/rsrc_windows_amd64.syso", "Ausgabedatei")
	icons := flag.String("icons", "", "Ordner mit icon-<größe>.png (Standard: neben diesem Programm)")
	flag.Parse()
	dir := *icons
	if dir == "" {
		_, self, _, _ := runtime.Caller(0)
		dir = filepath.Join(filepath.Dir(self), "icons")
	}
	b, err := build(*version, dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "winres:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, b, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "winres:", err)
		os.Exit(1)
	}
	fmt.Printf("winres: %s (%d Byte, Version %s)\n", *out, len(b), *version)
}

type res struct {
	typ, id uint16
	data    []byte
}

func build(version, iconDir string) ([]byte, error) {
	var list []res
	sizes := []int{16, 20, 24, 32, 40, 48, 64, 256}
	var group bytes.Buffer
	binary.Write(&group, binary.LittleEndian, [3]uint16{0, 1, uint16(len(sizes))})
	for i, px := range sizes {
		raw, err := os.ReadFile(filepath.Join(iconDir, fmt.Sprintf("icon-%d.png", px)))
		if err != nil {
			return nil, err
		}
		data := raw // 256 px als PNG (seit Windows Vista erlaubt), kleinere als Bitmap
		if px < 256 {
			img, err := png.Decode(bytes.NewReader(raw))
			if err != nil {
				return nil, err
			}
			data = dib(img)
		}
		id := uint16(i + 1)
		list = append(list, res{rtIcon, id, data})
		w := byte(px)
		if px >= 256 {
			w = 0
		}
		group.Write([]byte{w, w, 0, 0})
		binary.Write(&group, binary.LittleEndian, uint16(1))  // Ebenen
		binary.Write(&group, binary.LittleEndian, uint16(32)) // Bit pro Pixel
		binary.Write(&group, binary.LittleEndian, uint32(len(data)))
		binary.Write(&group, binary.LittleEndian, id)
	}
	list = append(list, res{rtGroupIcon, 1, group.Bytes()})
	list = append(list, res{rtVersion, 1, versionInfo(version)})
	list = append(list, res{rtManifest, 1, []byte(manifest(version))})
	return coff(list), nil
}

// dib: Symbolbild im ICO-Format (BITMAPINFOHEADER, 32 Bit BGRA von unten nach oben, leere UND-Maske).
func dib(img image.Image) []byte {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, struct {
		Size                   uint32
		Width, Height          int32
		Planes, BitCount       uint16
		Compression, SizeImage uint32
		XPels, YPels           int32
		ClrUsed, ClrImportant  uint32
	}{40, int32(w), int32(2 * h), 1, 32, 0, 0, 0, 0, 0, 0})
	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			r, g, bl, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			if a > 0 { // RGBA() liefert vormultipliert → zurückrechnen
				r, g, bl = r*0xffff/a, g*0xffff/a, bl*0xffff/a
			}
			buf.Write([]byte{byte(bl >> 8), byte(g >> 8), byte(r >> 8), byte(a >> 8)})
		}
	}
	stride := (w + 31) / 32 * 4
	buf.Write(make([]byte, stride*h))
	return buf.Bytes()
}

// ---------- Versionsinfo (VS_VERSIONINFO) ----------

func u16z(s string) []byte {
	u := utf16.Encode([]rune(s))
	b := make([]byte, len(u)*2+2)
	for i, c := range u {
		binary.LittleEndian.PutUint16(b[i*2:], c)
	}
	return b
}

func pad4(b *bytes.Buffer) {
	for b.Len()%4 != 0 {
		b.WriteByte(0)
	}
}

func block(key string, typ uint16, value []byte, valueLen uint16, children ...[]byte) []byte {
	var b bytes.Buffer
	b.Write([]byte{0, 0})
	binary.Write(&b, binary.LittleEndian, valueLen)
	binary.Write(&b, binary.LittleEndian, typ)
	b.Write(u16z(key))
	pad4(&b)
	b.Write(value)
	for _, c := range children {
		pad4(&b)
		b.Write(c)
	}
	out := b.Bytes()
	binary.LittleEndian.PutUint16(out, uint16(len(out)))
	return out
}

func str(key, val string) []byte {
	v := u16z(val)
	return block(key, 1, v, uint16(len(v)/2))
}

func versionNums(v string) [4]uint16 {
	var n [4]uint16
	for i, p := range strings.SplitN(strings.TrimPrefix(v, "v"), ".", 4) {
		x, _ := strconv.Atoi(strings.TrimFunc(p, func(r rune) bool { return r < '0' || r > '9' }))
		n[i] = uint16(x)
	}
	return n
}

func versionInfo(version string) []byte {
	n := versionNums(version)
	ms := uint32(n[0])<<16 | uint32(n[1])
	ls := uint32(n[2])<<16 | uint32(n[3])
	var fixed bytes.Buffer
	binary.Write(&fixed, binary.LittleEndian, [13]uint32{
		0xFEEF04BD, 0x00010000, ms, ls, ms, ls, 0x3F, 0, 0x00040004, 1, 0, 0, 0,
	})
	strs := block("040704B0", 1, nil, 0,
		str("CompanyName", "Nicolaj Webdesign"),
		str("FileDescription", "Nicolaj Fernwartung"),
		str("FileVersion", version),
		str("InternalName", "nicolaj-agent"),
		str("LegalCopyright", "© Nicolaj Webdesign"),
		str("OriginalFilename", "nicolaj-agent.exe"),
		str("ProductName", "Nicolaj Fernwartung"),
		str("ProductVersion", version),
	)
	sfi := block("StringFileInfo", 1, nil, 0, strs)
	tr := make([]byte, 4)
	binary.LittleEndian.PutUint16(tr, langDE)
	binary.LittleEndian.PutUint16(tr[2:], 1200) // Unicode
	vfi := block("VarFileInfo", 1, nil, 0, block("Translation", 0, tr, 4))
	return block("VS_VERSION_INFO", 0, fixed.Bytes(), uint16(fixed.Len()), sfi, vfi)
}

func manifest(version string) string {
	n := versionNums(version)
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <assemblyIdentity type="win32" name="NicolajWebdesign.Fernwartung" version="%d.%d.%d.%d" processorArchitecture="amd64"/>
  <description>Nicolaj Fernwartung</description>
  <trustInfo xmlns="urn:schemas-microsoft-com:asm.v3">
    <security><requestedPrivileges><requestedExecutionLevel level="asInvoker" uiAccess="false"/></requestedPrivileges></security>
  </trustInfo>
  <compatibility xmlns="urn:schemas-microsoft-com:compatibility.v1">
    <application><supportedOS Id="{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"/></application>
  </compatibility>
  <application xmlns="urn:schemas-microsoft-com:asm.v3">
    <windowsSettings>
      <dpiAware xmlns="http://schemas.microsoft.com/SMI/2005/WindowsSettings">true/pm</dpiAware>
      <dpiAwareness xmlns="http://schemas.microsoft.com/SMI/2016/WindowsSettings">PerMonitorV2</dpiAwareness>
    </windowsSettings>
  </application>
  <dependency>
    <dependentAssembly>
      <assemblyIdentity type="win32" name="Microsoft.Windows.Common-Controls" version="6.0.0.0" processorArchitecture="*" publicKeyToken="6595b64144ccf1df" language="*"/>
    </dependentAssembly>
  </dependency>
</assembly>
`, n[0], n[1], n[2], n[3])
}

// ---------- COFF-Objekt mit .rsrc-Abschnitt ----------

func coff(list []res) []byte {
	// Gruppieren: Typ → IDs (aufsteigend; die Liste ist bereits sortiert)
	type typGroup struct {
		typ   uint16
		items []res
	}
	var groups []typGroup
	for _, r := range list {
		if len(groups) == 0 || groups[len(groups)-1].typ != r.typ {
			groups = append(groups, typGroup{typ: r.typ})
		}
		groups[len(groups)-1].items = append(groups[len(groups)-1].items, r)
	}
	dirSize := func(n int) int { return 16 + 8*n }

	// Aufbau: Wurzel | Typ-Verzeichnisse | ID-Verzeichnisse (je 1 Sprache) | Datensätze | Daten
	off := dirSize(len(groups))
	typOff := make([]int, len(groups))
	for i, g := range groups {
		typOff[i] = off
		off += dirSize(len(g.items))
	}
	idOff := map[[2]uint16]int{}
	for _, g := range groups {
		for _, r := range g.items {
			idOff[[2]uint16{r.typ, r.id}] = off
			off += dirSize(1)
		}
	}
	entryOff := map[[2]uint16]int{}
	for _, g := range groups {
		for _, r := range g.items {
			entryOff[[2]uint16{r.typ, r.id}] = off
			off += 16
		}
	}
	dataOff := map[[2]uint16]int{}
	for _, g := range groups {
		for _, r := range g.items {
			off = (off + 7) &^ 7
			dataOff[[2]uint16{r.typ, r.id}] = off
			off += len(r.data)
		}
	}
	sec := make([]byte, (off+7)&^7)
	putDir := func(at int, entries [][2]uint32) {
		binary.LittleEndian.PutUint16(sec[at+14:], uint16(len(entries))) // nur ID-Einträge
		for i, e := range entries {
			binary.LittleEndian.PutUint32(sec[at+16+i*8:], e[0])
			binary.LittleEndian.PutUint32(sec[at+16+i*8+4:], e[1])
		}
	}
	var root [][2]uint32
	for i, g := range groups {
		root = append(root, [2]uint32{uint32(g.typ), 0x80000000 | uint32(typOff[i])})
	}
	putDir(0, root)
	var relocs []uint32
	for i, g := range groups {
		var ents [][2]uint32
		for _, r := range g.items {
			k := [2]uint16{r.typ, r.id}
			ents = append(ents, [2]uint32{uint32(r.id), 0x80000000 | uint32(idOff[k])})
			putDir(idOff[k], [][2]uint32{{langDE, uint32(entryOff[k])}})
			e := entryOff[k]
			binary.LittleEndian.PutUint32(sec[e:], uint32(dataOff[k])) // wird per Relocation zur RVA
			binary.LittleEndian.PutUint32(sec[e+4:], uint32(len(r.data)))
			relocs = append(relocs, uint32(e))
			copy(sec[dataOff[k]:], r.data)
		}
		putDir(typOff[i], ents)
	}

	var f bytes.Buffer
	const hdr, shdr = 20, 40
	rawPtr := hdr + shdr
	relocPtr := rawPtr + len(sec)
	symPtr := relocPtr + 10*len(relocs)
	binary.Write(&f, binary.LittleEndian, struct {
		Machine          uint16
		Sections         uint16
		Time             uint32
		SymPtr           uint32
		Syms             uint32
		OptSize, Charact uint16
	}{0x8664, 1, 0, uint32(symPtr), 1, 0, 0})
	var name [8]byte
	copy(name[:], ".rsrc")
	binary.Write(&f, binary.LittleEndian, struct {
		Name                      [8]byte
		VSize, VAddr, RawSize     uint32
		RawPtr, RelocPtr, LinePtr uint32
		Relocs, Lines             uint16
		Charact                   uint32
	}{name, 0, 0, uint32(len(sec)), uint32(rawPtr), uint32(relocPtr), 0, uint16(len(relocs)), 0, 0x40000040})
	f.Write(sec)
	for _, r := range relocs {
		binary.Write(&f, binary.LittleEndian, r)
		binary.Write(&f, binary.LittleEndian, uint32(0)) // Symbol 0 = .rsrc
		binary.Write(&f, binary.LittleEndian, uint16(3)) // IMAGE_REL_AMD64_ADDR32NB
	}
	f.Write(name[:])                                 // Symbolname
	binary.Write(&f, binary.LittleEndian, uint32(0)) // Wert
	binary.Write(&f, binary.LittleEndian, int16(1))  // Abschnitt 1
	binary.Write(&f, binary.LittleEndian, uint16(0)) // Typ
	f.Write([]byte{3, 0})                            // STATIC, keine Zusatzsymbole
	binary.Write(&f, binary.LittleEndian, uint32(4)) // leere Stringtabelle
	return f.Bytes()
}
