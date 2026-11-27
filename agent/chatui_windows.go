//go:build windows

// Windows-Helfer fürs Nachrichtenfenster: Links und Bilder öffnen, Bilder aus der
// Zwischenablage, Dateien hineinziehen, Öffnen-/Speichern-Dialoge.

package main

import (
	"image"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	ole32    = syscall.NewLazyDLL("ole32.dll")
	comdlg32 = syscall.NewLazyDLL("comdlg32.dll")

	puCoInitializeEx      = ole32.NewProc("CoInitializeEx")
	puCoUninitialize      = ole32.NewProc("CoUninitialize")
	puDragAcceptFiles     = shell32.NewProc("DragAcceptFiles")
	puDragQueryFileW      = shell32.NewProc("DragQueryFileW")
	puDragFinish          = shell32.NewProc("DragFinish")
	puIsClipboardFormat   = user32.NewProc("IsClipboardFormatAvailable")
	puGetOpenFileNameW    = comdlg32.NewProc("GetOpenFileNameW")
	puGetSaveFileNameW    = comdlg32.NewProc("GetSaveFileNameW")
	puAllowSetForeground  = user32.NewProc("AllowSetForegroundWindow")
	puChangeWindowMsgFilt = user32.NewProc("ChangeWindowMessageFilterEx")
)

const (
	cfUnicodeText = 13
	cfDIB         = 8
	cfDIBV5       = 17
	cfHDrop       = 15
)

// shellOpen öffnet eine Datei oder Webadresse mit dem passenden Programm.
func shellOpen(target string) {
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		puCoInitializeEx.Call(0, 0x2|0x4) // APARTMENTTHREADED | DISABLE_OLE1DDE
		defer puCoUninitialize.Call()
		puAllowSetForeground.Call(^uintptr(0)) // ASFW_ANY: Browser darf nach vorne
		procShellExecuteW.Call(0, uintptr(unsafe.Pointer(utf16Ptr("open"))), uintptr(unsafe.Pointer(utf16Ptr(target))), 0, 0, 1)
	}()
}

// openURL öffnet nur echte Webadressen (http/https) im Standardbrowser.
func openURL(raw string) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || strings.ContainsAny(raw, " \t\r\n\"") {
		return
	}
	shellOpen(u.String())
}

// dropPaths liest die Dateien aus WM_DROPFILES.
func dropPaths(hDrop uintptr) []string {
	defer puDragFinish.Call(hDrop)
	n, _, _ := puDragQueryFileW.Call(hDrop, 0xFFFFFFFF, 0, 0)
	var out []string
	for i := uintptr(0); i < n && i < 20; i++ {
		buf := make([]uint16, 1024)
		l, _, _ := puDragQueryFileW.Call(hDrop, i, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if l > 0 && int(l) < len(buf) {
			out = append(out, syscall.UTF16ToString(buf[:l]))
		}
	}
	return out
}

// allowDrop erlaubt Ziehen aus dem Explorer, auch wenn Fenster und Explorer unterschiedlich laufen.
func allowDrop(hwnd uintptr) {
	if puChangeWindowMsgFilt.Find() == nil {
		for _, m := range []uintptr{wmDropFiles, 0x004A /* WM_COPYDATA */, 0x0049 /* WM_COPYGLOBALDATA */} {
			puChangeWindowMsgFilt.Call(hwnd, m, 1, 0) // MSGFLT_ALLOW
		}
	}
}

// clipboardBytes liest ein Format aus der Zwischenablage (höchstens max Bytes).
func clipboardBytes(format uintptr, max int) []byte {
	h, _, _ := puGetClipboardData.Call(format)
	if h == 0 {
		return nil
	}
	p, _, _ := puGlobalLock.Call(h)
	if p == 0 {
		return nil
	}
	defer puGlobalUnlock.Call(h)
	sz, _, _ := puGlobalSize.Call(h)
	if sz == 0 || int(sz) > max {
		return nil
	}
	out := make([]byte, sz)
	copy(out, unsafe.Slice((*byte)(sysPtr(p)), int(sz)))
	return out
}

// clipboardContent: Was liegt in der Zwischenablage? Text hat Vorrang (z. B. aus Word),
// sonst ein Bild (Screenshot) oder kopierte Bilddateien aus dem Explorer.
func clipboardContent(hwnd uintptr) (img image.Image, files []string, hasText bool) {
	if r, _, _ := puIsClipboardFormat.Call(cfUnicodeText); r != 0 {
		return nil, nil, true
	}
	if r, _, _ := puOpenClipboard.Call(hwnd); r == 0 {
		return nil, nil, false
	}
	defer puCloseClipboard.Call()
	for _, f := range []uintptr{cfDIBV5, cfDIB} {
		if r, _, _ := puIsClipboardFormat.Call(f); r == 0 {
			continue
		}
		if b := clipboardBytes(f, 300<<20); b != nil {
			if m, err := decodeDIB(b); err == nil {
				return m, nil, false
			}
		}
	}
	if r, _, _ := puIsClipboardFormat.Call(cfHDrop); r != 0 {
		if h, _, _ := puGetClipboardData.Call(cfHDrop); h != 0 {
			n, _, _ := puDragQueryFileW.Call(h, 0xFFFFFFFF, 0, 0)
			for i := uintptr(0); i < n && i < 20; i++ {
				buf := make([]uint16, 1024)
				l, _, _ := puDragQueryFileW.Call(h, i, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
				if l > 0 && int(l) < len(buf) {
					files = append(files, syscall.UTF16ToString(buf[:l]))
				}
			}
		}
	}
	return nil, files, false
}

// getClipboardFull: Text aus der Zwischenablage mit Zeilenumbrüchen (höchstens 8000 Zeichen).
func getClipboardFull(hwnd uintptr) string {
	if r, _, _ := puOpenClipboard.Call(hwnd); r == 0 {
		return ""
	}
	defer puCloseClipboard.Call()
	b := clipboardBytes(cfUnicodeText, 1<<20)
	if len(b) < 2 {
		return ""
	}
	u := unsafe.Slice((*uint16)(unsafe.Pointer(&b[0])), len(b)/2)
	if i := indexZero(u); i >= 0 {
		u = u[:i]
	}
	if len(u) > 8000 {
		u = u[:8000]
	}
	return string(utf16.Decode(u))
}

func indexZero(u []uint16) int {
	for i, c := range u {
		if c == 0 {
			return i
		}
	}
	return -1
}

// setClipboardImage legt ein Bild in die Zwischenablage (zum Einfügen in Word, Paint, …).
func setClipboardImage(hwnd uintptr, img image.Image) bool {
	dib := encodeDIB(img)
	if r, _, _ := puOpenClipboard.Call(hwnd); r == 0 {
		return false
	}
	defer puCloseClipboard.Call()
	puEmptyClipboard.Call()
	h, _, _ := puGlobalAlloc.Call(0x0002, uintptr(len(dib))) // GMEM_MOVEABLE
	if h == 0 {
		return false
	}
	p, _, _ := puGlobalLock.Call(h)
	if p == 0 {
		puGlobalFree.Call(h)
		return false
	}
	copy(unsafe.Slice((*byte)(sysPtr(p)), len(dib)), dib)
	puGlobalUnlock.Call(h)
	if r, _, _ := puSetClipboardData.Call(cfDIB, h); r == 0 {
		puGlobalFree.Call(h)
		return false
	}
	return true
}

// ---------- Öffnen / Speichern ----------

type openFileName struct {
	StructSize    uint32
	Owner         uintptr
	Instance      uintptr
	Filter        *uint16
	CustomFilter  *uint16
	MaxCustFilter uint32
	FilterIndex   uint32
	File          *uint16
	MaxFile       uint32
	FileTitle     *uint16
	MaxFileTitle  uint32
	InitialDir    *uint16
	Title         *uint16
	Flags         uint32
	FileOffset    uint16
	FileExtension uint16
	DefExt        *uint16
	CustData      uintptr
	FnHook        uintptr
	TemplateName  *uint16
	PvReserved    uintptr
	DwReserved    uint32
	FlagsEx       uint32
}

// filterList baut die doppelt nullterminierte Filterliste („Name\0Muster\0…\0\0“).
func filterList(pairs ...string) *uint16 {
	u := utf16.Encode([]rune(strings.Join(pairs, "\x00") + "\x00\x00"))
	return &u[0]
}

func pictureDir() string {
	if p := os.Getenv("USERPROFILE"); p != "" {
		for _, d := range []string{"Pictures", "Bilder", "Desktop"} {
			if st, err := os.Stat(filepath.Join(p, d)); err == nil && st.IsDir() {
				return filepath.Join(p, d)
			}
		}
	}
	return ""
}

// pickImageFile: „Bild auswählen“-Dialog.
func pickImageFile(owner uintptr) (string, bool) {
	buf := make([]uint16, 1024)
	ofn := openFileName{Owner: owner, File: &buf[0], MaxFile: uint32(len(buf)),
		Filter: filterList("Bilder (PNG, JPG, GIF, BMP)", "*.png;*.jpg;*.jpeg;*.jfif;*.gif;*.bmp", "Alle Dateien", "*.*"),
		Title:  utf16Ptr("Bild an Nicolaj senden"),
		Flags:  0x1000 | 0x800 | 0x8 | 0x80000 | 0x4, // FILEMUSTEXIST | PATHMUSTEXIST | NOCHANGEDIR | EXPLORER | HIDEREADONLY
	}
	if d := pictureDir(); d != "" {
		ofn.InitialDir = utf16Ptr(d)
	}
	ofn.StructSize = uint32(unsafe.Sizeof(ofn))
	if r, _, _ := puGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn))); r == 0 {
		return "", false
	}
	return syscall.UTF16ToString(buf), true
}

// pickSavePath: „Speichern unter“-Dialog für ein Bild.
func pickSavePath(owner uintptr, name, ext string) (string, bool) {
	buf := make([]uint16, 1024)
	copy(buf, utf16.Encode([]rune(name)))
	label := map[string]string{"png": "PNG-Bild", "jpg": "JPEG-Bild", "gif": "GIF-Bild"}[ext]
	ofn := openFileName{Owner: owner, File: &buf[0], MaxFile: uint32(len(buf)),
		Filter: filterList(label+" (*."+ext+")", "*."+ext),
		DefExt: utf16Ptr(ext),
		Title:  utf16Ptr("Bild speichern"),
		Flags:  0x2 | 0x800 | 0x8 | 0x80000 | 0x4, // OVERWRITEPROMPT | PATHMUSTEXIST | NOCHANGEDIR | EXPLORER | HIDEREADONLY
	}
	if d := pictureDir(); d != "" {
		ofn.InitialDir = utf16Ptr(d)
	}
	ofn.StructSize = uint32(unsafe.Sizeof(ofn))
	if r, _, _ := puGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&ofn))); r == 0 {
		return "", false
	}
	return syscall.UTF16ToString(buf), true
}
