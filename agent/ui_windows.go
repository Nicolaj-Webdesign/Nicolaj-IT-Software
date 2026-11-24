//go:build windows

// Fenster im Nicolaj-Look für Windows: Formen zeichnet ui_core.go, Text GDI (ClearType).
// Halbtransparente Fenster (UpdateLayeredWindow) für runde Ecken und weichen Schatten.

package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"
)

var (
	puUpdateLayeredWindow = user32.NewProc("UpdateLayeredWindow")
	puSetTimer            = user32.NewProc("SetTimer")
	puKillTimer           = user32.NewProc("KillTimer")
	puTrackMouseEvent     = user32.NewProc("TrackMouseEvent")
	puSetCursor           = user32.NewProc("SetCursor")
	puSetCapture          = user32.NewProc("SetCapture")
	puReleaseCapture      = user32.NewProc("ReleaseCapture")
	puGetWindowRect       = user32.NewProc("GetWindowRect")
	puSetForeground       = user32.NewProc("SetForegroundWindow")
	puFlashWindowEx       = user32.NewProc("FlashWindowEx")
	puMessageBeep         = user32.NewProc("MessageBeep")
	puOpenClipboard       = user32.NewProc("OpenClipboard")
	puEmptyClipboard      = user32.NewProc("EmptyClipboard")
	puSetClipboardData    = user32.NewProc("SetClipboardData")
	puGetClipboardData    = user32.NewProc("GetClipboardData")
	puCloseClipboard      = user32.NewProc("CloseClipboard")
	puCreateIconIndirect  = user32.NewProc("CreateIconIndirect")
	puDestroyIcon         = user32.NewProc("DestroyIcon")
	puSystemParamsInfo    = user32.NewProc("SystemParametersInfoW")
	puGetKeyState         = user32.NewProc("GetKeyState")

	puCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	puGetTextExtentPoint = gdi32.NewProc("GetTextExtentPoint32W")
	puGetTextFaceW       = gdi32.NewProc("GetTextFaceW")
	puCreateBitmap       = gdi32.NewProc("CreateBitmap")
	puGdiFlush           = gdi32.NewProc("GdiFlush")
	puGlobalAlloc        = kernel32.NewProc("GlobalAlloc")
	puGlobalLock         = kernel32.NewProc("GlobalLock")
	puGlobalUnlock       = kernel32.NewProc("GlobalUnlock")
	puGlobalFree         = kernel32.NewProc("GlobalFree")
	puGlobalSize         = kernel32.NewProc("GlobalSize")
)

const (
	wmSetCursor   = 0x0020
	wmSetIcon     = 0x0080
	wmKeyDown     = 0x0100
	wmChar        = 0x0102
	wmTimer       = 0x0113
	wmMouseMove   = 0x0200
	wmLButtonDown = 0x0201
	wmLButtonUp   = 0x0202
	wmMouseLeave  = 0x02A3
	wmAppUpdate   = 0x8001
	wmAppClose    = 0x8002

	timerAnim    = 1
	timerCount   = 2
	timerCopied  = 3
	timerCaret   = 4
	uiShadow     = 22.0 // Platz für den Schatten (logische Pixel)
	idcHand      = 32649
	idcArrow     = 32512
	idcIBeam     = 32513
	htTransparnt = ^uintptr(0)
)

type uiSize struct{ CX, CY int32 }

type trackMouse struct {
	Size  uint32
	Flags uint32
	Hwnd  uintptr
	Hover uint32
	_     uint32
}

type flashInfo struct {
	Size    uint32
	_       uint32
	Hwnd    uintptr
	Flags   uint32
	Count   uint32
	Timeout uint32
	_       uint32
}

// ---------- Schriften & Zeichnen ----------

type uiRenderer struct {
	scale  float64
	dc     uintptr
	dib    uintptr
	old    uintptr
	bits   unsafe.Pointer
	w, h   int
	fonts  map[uiFont]uintptr
	faces  [3]string
	saved  []byte
	screen uintptr
}

var (
	faceOnce  sync.Once
	faceNames [3]string
)

func resolveFaces(dc uintptr) [3]string {
	faceOnce.Do(func() {
		cands := [3][]string{
			{"Segoe UI Variable Display", "Segoe UI"},
			{"Segoe UI Variable Text", "Segoe UI"},
			{"Segoe Fluent Icons", "Segoe MDL2 Assets"},
		}
		for i, list := range cands {
			faceNames[i] = list[len(list)-1]
			for _, name := range list {
				f, _, _ := procCreateFontW.Call(^uintptr(15), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0,
					uintptr(unsafe.Pointer(utf16Ptr(name))))
				if f == 0 {
					continue
				}
				old, _, _ := procSelectObject.Call(dc, f)
				buf := make([]uint16, 64)
				puGetTextFaceW.Call(dc, 64, uintptr(unsafe.Pointer(&buf[0])))
				procSelectObject.Call(dc, old)
				procDeleteObject.Call(f)
				if strings.EqualFold(syscall.UTF16ToString(buf), name) {
					faceNames[i] = name
					break
				}
			}
		}
	})
	return faceNames
}

func newRenderer(scale float64) *uiRenderer {
	dc, _, _ := procCreateCompatibleDC.Call(0)
	r := &uiRenderer{scale: scale, dc: dc, fonts: map[uiFont]uintptr{}}
	r.faces = resolveFaces(dc)
	r.screen, _, _ = procGetDC.Call(0)
	return r
}

func (r *uiRenderer) font(f uiFont) uintptr {
	if h, ok := r.fonts[f]; ok {
		return h
	}
	wt := uintptr(400)
	if f.Bold {
		wt = 600
	}
	px := int32(-math.Round(f.Size * r.scale))
	h, _, _ := procCreateFontW.Call(uintptr(px), 0, 0, 0, wt, 0, 0, 0, 1, 0, 0, 5, 0,
		uintptr(unsafe.Pointer(utf16Ptr(r.faces[f.Face]))))
	r.fonts[f] = h
	return h
}

func (r *uiRenderer) textW(f uiFont, s string) float64 {
	if s == "" {
		return 0
	}
	u := utf16.Encode([]rune(s))
	old, _, _ := procSelectObject.Call(r.dc, r.font(f))
	var sz uiSize
	puGetTextExtentPoint.Call(r.dc, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)), uintptr(unsafe.Pointer(&sz)))
	procSelectObject.Call(r.dc, old)
	return float64(sz.CX) / r.scale
}

func (r *uiRenderer) ensure(w, h int) bool {
	if r.dib != 0 && r.w == w && r.h == h {
		return true
	}
	if r.dib != 0 {
		procSelectObject.Call(r.dc, r.old)
		procDeleteObject.Call(r.dib)
		r.dib = 0
	}
	bi := bitmapInfo{Header: bitmapInfoHeader{Size: uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width: int32(w), Height: -int32(h), Planes: 1, BitCount: 32}}
	var bits unsafe.Pointer
	dib, _, _ := puCreateDIBSection.Call(r.dc, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if dib == 0 || bits == nil {
		return false
	}
	r.dib, r.bits, r.w, r.h = dib, bits, w, h
	r.old, _, _ = procSelectObject.Call(r.dc, dib)
	r.saved = make([]byte, w*h)
	return true
}

// render zeichnet die Szene in die Bitmap (Formen in Go, Text mit GDI).
func (r *uiRenderer) render(s *uiScene, margin float64) bool {
	c := renderShapes(s, r.scale, margin)
	if !r.ensure(c.W, c.H) {
		return false
	}
	dst := unsafe.Slice((*byte)(r.bits), c.W*c.H*4)
	copy(dst, c.Pix)
	for i := range r.saved {
		r.saved[i] = dst[i*4+3]
	}
	procSetBkMode.Call(r.dc, 1) // TRANSPARENT
	for _, o := range s.Ops {
		if o.Kind != opText || o.Text == "" {
			continue
		}
		procSelectObject.Call(r.dc, r.font(o.Font))
		procSetTextColor.Call(r.dc, uintptr(o.Col.R)|uintptr(o.Col.G)<<8|uintptr(o.Col.B)<<16)
		rc := rect{
			Left: int32(math.Round((o.X + margin) * r.scale)), Top: int32(math.Round((o.Y + margin) * r.scale)),
			Right: int32(math.Round((o.X + o.W + margin) * r.scale)), Bottom: int32(math.Round((o.Y + o.H + margin) * r.scale)),
		}
		flags := uintptr(0x20 | 0x04 | 0x800 | 0x100) // SINGLELINE | VCENTER | NOPREFIX | NOCLIP
		switch o.Align {
		case 1:
			flags |= 0x01
		case 2:
			flags |= 0x02
		}
		procDrawTextW.Call(r.dc, uintptr(unsafe.Pointer(utf16Ptr(o.Text))), ^uintptr(0), uintptr(unsafe.Pointer(&rc)), flags)
	}
	puGdiFlush.Call()
	for i, a := range r.saved { // GDI setzt bei Text das Alpha auf 0 – Text liegt immer auf deckenden Flächen
		dst[i*4+3] = a
	}
	return true
}

func (r *uiRenderer) close() {
	if r.dib != 0 {
		procSelectObject.Call(r.dc, r.old)
		procDeleteObject.Call(r.dib)
	}
	for _, f := range r.fonts {
		procDeleteObject.Call(f)
	}
	if r.dc != 0 {
		procDeleteDC.Call(r.dc)
	}
	if r.screen != 0 {
		procReleaseDC.Call(0, r.screen)
	}
}

// ---------- Fenster ----------

type uiOpts struct {
	Title     string // Fenstertitel (Taskleiste); Standard „Nicolaj Fernwartung“
	Topmost   bool
	Beep      uintptr // MessageBeep-Typ, 0 = keiner
	Bar       bool    // Hinweisleiste: nicht aktivieren, nicht in der Taskleiste, nicht im Live-Bild
	BarText   string
	Animate   bool
	Countdown int // Sekunden bis „timeout“
	OnCount   func(d *uiDialog, left int)
	OnClick   func(ctl *uiCtl, id string) bool // true = Fenster schließen
	Ready     func(ctl *uiCtl)
	Ctx       context.Context
}

type uiWin struct {
	hwnd     uintptr
	mu       sync.Mutex
	dlg      *uiDialog
	st       uiState
	opts     uiOpts
	scene    *uiScene
	r        *uiRenderer
	result   string
	closeRes string
	tracking bool
	shown    bool
	progress float64
	left     int
}

type uiCtl struct{ w *uiWin }

// Set ersetzt den Inhalt des Fensters (aus jeder Goroutine erlaubt).
func (c *uiCtl) Set(d *uiDialog) {
	c.w.mu.Lock()
	c.w.dlg = d
	c.w.st.Hover, c.w.st.Press, c.w.st.Copied = "", "", false
	if d.Input != nil {
		c.w.st.Focus = "input"
	} else {
		c.w.st.Focus = ""
	}
	c.w.mu.Unlock()
	procPostMessageW.Call(c.w.hwnd, wmAppUpdate, 0, 0)
}

// Update ändert den aktuellen Inhalt (aus jeder Goroutine erlaubt).
func (c *uiCtl) Update(fn func(d *uiDialog)) {
	c.w.mu.Lock()
	if c.w.dlg != nil {
		fn(c.w.dlg)
	}
	c.w.mu.Unlock()
	procPostMessageW.Call(c.w.hwnd, wmAppUpdate, 0, 0)
}

func (c *uiCtl) Close(result string) {
	c.w.mu.Lock()
	c.w.closeRes = result
	c.w.mu.Unlock()
	procPostMessageW.Call(c.w.hwnd, wmAppClose, 0, 0)
}

var (
	uiMu        sync.Mutex
	uiWins      = map[uintptr]*uiWin{}
	uiClassOnce sync.Once
	uiClassOK   bool
	uiClassName = utf16Ptr("NicolajFernwartungUI")
	uiProc      = syscall.NewCallback(uiWndProc)
	uiDPIOnce   sync.Once
)

func uiScale() float64 {
	uiDPIOnce.Do(initScreenPlatform)
	if procGetDpiForSystem.Find() == nil {
		if d, _, _ := procGetDpiForSystem.Call(); d >= 96 {
			return float64(d) / 96
		}
	}
	return 1
}

func (w *uiWin) layout() *uiScene {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.opts.Bar {
		w.scene = layoutBar(w.opts.BarText, w.st, w.r)
		return w.scene
	}
	d := *w.dlg
	if d.Progress >= 0 && w.opts.Animate { // Fortschritt gleitet weich zum Ziel
		w.progress += (d.Progress - w.progress) * 0.18
		if math.Abs(d.Progress-w.progress) < 0.002 {
			w.progress = d.Progress
		}
		d.Progress = w.progress
	}
	w.scene = layoutDialog(&d, w.st, w.r)
	return w.scene
}

func (w *uiWin) redraw() {
	s := w.layout()
	if !w.r.render(s, uiShadow) {
		return
	}
	size := uiSize{int32(w.r.w), int32(w.r.h)}
	src := point{0, 0}
	blend := [4]byte{0, 0, 255, 1} // AC_SRC_OVER, Deckkraft 255, AC_SRC_ALPHA
	var pos *point
	var wr rect
	puGetWindowRect.Call(w.hwnd, uintptr(unsafe.Pointer(&wr)))
	curW, curH := wr.Right-wr.Left, wr.Bottom-wr.Top
	if !w.shown || curW != size.CX || curH != size.CY {
		var p point
		if !w.shown {
			p = w.initialPos(size)
		} else { // Größe geändert: Mitte beibehalten
			p = point{wr.Left + (curW-size.CX)/2, wr.Top + (curH-size.CY)/2}
		}
		pos = &p
	}
	puUpdateLayeredWindow.Call(w.hwnd, w.r.screen, uintptr(unsafe.Pointer(pos)), uintptr(unsafe.Pointer(&size)),
		w.r.dc, uintptr(unsafe.Pointer(&src)), 0, uintptr(unsafe.Pointer(&blend)), 2) // ULW_ALPHA
}

func (w *uiWin) initialPos(sz uiSize) point {
	var wa rect
	if r, _, _ := puSystemParamsInfo.Call(0x30, 0, uintptr(unsafe.Pointer(&wa)), 0); r == 0 { // SPI_GETWORKAREA
		wa = rect{0, 0, metric(smCXScreen), metric(1)}
	}
	x := wa.Left + (wa.Right-wa.Left-sz.CX)/2
	if w.opts.Bar {
		return point{x, wa.Top + int32(10*w.r.scale) - int32(uiShadow*w.r.scale)}
	}
	return point{x, wa.Top + (wa.Bottom-wa.Top-sz.CY)/2}
}

func (w *uiWin) local(lp uintptr, screen bool) (float64, float64) {
	x, y := int32(int16(lp&0xFFFF)), int32(int16((lp>>16)&0xFFFF))
	if screen {
		var wr rect
		puGetWindowRect.Call(w.hwnd, uintptr(unsafe.Pointer(&wr)))
		x, y = x-wr.Left, y-wr.Top
	}
	return float64(x)/w.r.scale - uiShadow, float64(y)/w.r.scale - uiShadow
}

func (w *uiWin) hit(x, y float64) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.scene == nil {
		return ""
	}
	return w.scene.hitAt(x, y)
}

func (w *uiWin) setState(fn func(st *uiState)) {
	w.mu.Lock()
	fn(&w.st)
	w.mu.Unlock()
	w.redraw()
}

func (w *uiWin) activate(id string) {
	if id == "" || id == "input" {
		if id == "input" {
			w.setState(func(st *uiState) { st.Focus, st.CaretOn = "input", true })
		}
		return
	}
	w.mu.Lock()
	if w.dlg != nil {
		if b := w.dlg.button(id); b != nil && b.Disabled {
			w.mu.Unlock()
			return
		}
	}
	big := ""
	if w.dlg != nil {
		big = w.dlg.Big
	}
	w.mu.Unlock()
	if id == "copy" {
		setClipboard(w.hwnd, strings.ReplaceAll(big, " ", ""))
		w.setState(func(st *uiState) { st.Copied = true })
		puSetTimer.Call(w.hwnd, timerCopied, 1800, 0)
		return
	}
	closeIt := true
	if w.opts.OnClick != nil {
		closeIt = w.opts.OnClick(&uiCtl{w}, id)
	}
	if closeIt {
		w.result = id
		procDestroyWindow.Call(w.hwnd)
	}
}

func (w *uiWin) typeChar(r rune) {
	w.mu.Lock()
	in := (*uiInput)(nil)
	if w.dlg != nil && w.st.Focus == "input" {
		in = w.dlg.Input
	}
	if in == nil {
		w.mu.Unlock()
		return
	}
	switch {
	case r == 8: // Rücktaste
		if in.Value != "" {
			_, n := utf8.DecodeLastRuneInString(in.Value)
			in.Value = in.Value[:len(in.Value)-n]
		}
	case r == 127 || r == 0x17: // Strg+Rücktaste: ganzes Wort
		v := strings.TrimRightFunc(in.Value, unicode.IsSpace)
		if i := strings.LastIndexFunc(v, unicode.IsSpace); i >= 0 {
			in.Value = v[:i+1]
		} else {
			in.Value = ""
		}
	case r == 0x16: // Strg+V
		w.mu.Unlock()
		for _, c := range getClipboard(w.hwnd) {
			if c >= 32 && c != 127 {
				w.typeChar(c)
			}
		}
		return
	case (r >= 32 || (r == '\n' && in.Multiline)) && (in.MaxLen == 0 || utf8.RuneCountInString(in.Value) < in.MaxLen):
		in.Value += string(r)
	default:
		w.mu.Unlock()
		return
	}
	w.st.CaretOn = true
	if w.opts.OnClick != nil { // Steuerung kann Knöpfe (de)aktivieren
		w.mu.Unlock()
		w.opts.OnClick(&uiCtl{w}, "input-changed")
		w.mu.Lock()
	}
	w.mu.Unlock()
	puSetTimer.Call(w.hwnd, timerCaret, 530, 0)
	w.redraw()
}

func uiWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) (ret uintptr) {
	defer func() { // ein Fehler im Fenster darf nie das ganze Programm beenden
		if e := recover(); e != nil {
			logf("Fenster-Fehler: %v", e)
			ret, _, _ = procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		}
	}()
	uiMu.Lock()
	w := uiWins[hwnd]
	uiMu.Unlock()
	if w == nil {
		r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		return r
	}
	switch msg {
	case wmNCHit:
		x, y := w.local(lParam, true)
		if w.hit(x, y) != "" {
			return htClient
		}
		w.mu.Lock()
		inside := w.scene != nil && x >= 0 && y >= 0 && x < w.scene.W && y < w.scene.H
		w.mu.Unlock()
		if inside {
			return htCaption // Fenster an der Fläche verschieben
		}
		return htClient
	case wmSetCursor:
		if lParam&0xFFFF == htClient {
			w.mu.Lock()
			hv := w.st.Hover
			w.mu.Unlock()
			cur := uintptr(idcArrow)
			if hv == "input" {
				cur = idcIBeam
			} else if hv != "" {
				cur = idcHand
			}
			h, _, _ := procLoadCursorW.Call(0, cur)
			puSetCursor.Call(h)
			return 1
		}
	case wmMouseMove:
		if !w.tracking {
			tm := trackMouse{Size: uint32(unsafe.Sizeof(trackMouse{})), Flags: 2, Hwnd: hwnd} // TME_LEAVE
			puTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tm)))
			w.tracking = true
		}
		x, y := w.local(lParam, false)
		id := w.hit(x, y)
		w.mu.Lock()
		changed := w.st.Hover != id
		w.mu.Unlock()
		if changed {
			w.setState(func(st *uiState) { st.Hover = id })
		}
		return 0
	case wmMouseLeave:
		w.tracking = false
		w.setState(func(st *uiState) {
			if st.Press == "" {
				st.Hover = ""
			}
		})
		return 0
	case wmLButtonDown:
		x, y := w.local(lParam, false)
		if id := w.hit(x, y); id != "" {
			puSetCapture.Call(hwnd)
			w.setState(func(st *uiState) {
				st.Press = id
				if id != "input" && st.Focus == "input" {
					st.Focus = "input" // Fokus bleibt beim Eingabefeld (es gibt nur eins)
				}
			})
		}
		return 0
	case wmLButtonUp:
		w.mu.Lock()
		p := w.st.Press
		w.mu.Unlock()
		if p != "" {
			puReleaseCapture.Call()
			x, y := w.local(lParam, false)
			id := w.hit(x, y)
			w.setState(func(st *uiState) { st.Press = "" })
			if id == p {
				w.activate(id)
			}
		}
		return 0
	case wmKeyDown:
		switch wParam {
		case 0x0D: // Enter (Umschalt+Enter im mehrzeiligen Feld = neue Zeile)
			if ks, _, _ := puGetKeyState.Call(0x10); int16(ks) < 0 {
				w.mu.Lock()
				multi := w.dlg != nil && w.dlg.Input != nil && w.dlg.Input.Multiline && w.st.Focus == "input"
				w.mu.Unlock()
				if multi {
					w.typeChar('\n')
					return 0
				}
			}
			w.mu.Lock()
			def := ""
			if w.scene != nil {
				def = w.scene.Default
			}
			w.mu.Unlock()
			w.activate(def)
		case 0x1B: // Esc
			if !w.opts.Bar {
				w.activate("close")
			}
		}
		return 0
	case wmChar:
		if wParam != 0x0D && wParam != 0x1B && wParam != 0x09 {
			r := rune(wParam)
			if ks, _, _ := puGetKeyState.Call(0x11); int16(ks) < 0 && wParam == 0x7F { // Strg+Rücktaste
				r = 127
			}
			w.typeChar(r)
		}
		return 0
	case wmTimer:
		switch wParam {
		case timerAnim:
			w.mu.Lock()
			w.st.Phase = math.Mod(w.st.Phase+0.033/1.6, 1)
			w.mu.Unlock()
			w.redraw()
		case timerCount:
			w.left--
			if w.left <= 0 {
				w.result = "timeout"
				procDestroyWindow.Call(hwnd)
				return 0
			}
			if w.opts.OnCount != nil {
				w.mu.Lock()
				w.opts.OnCount(w.dlg, w.left)
				w.mu.Unlock()
			}
			w.redraw()
		case timerCopied:
			puKillTimer.Call(hwnd, timerCopied)
			w.setState(func(st *uiState) { st.Copied = false })
		case timerCaret:
			w.setState(func(st *uiState) { st.CaretOn = !st.CaretOn })
		}
		return 0
	case wmAppUpdate:
		w.redraw()
		return 0
	case wmAppClose:
		w.mu.Lock()
		w.result = w.closeRes
		w.mu.Unlock()
		procDestroyWindow.Call(hwnd)
		return 0
	case wmClose:
		w.activate("close")
		return 0
	case wmDestroy:
		for _, t := range []uintptr{timerAnim, timerCount, timerCopied, timerCaret} {
			puKillTimer.Call(hwnd, t)
		}
		uiMu.Lock()
		delete(uiWins, hwnd)
		uiMu.Unlock()
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func registerUIClass() bool {
	uiClassOnce.Do(func() {
		inst, _, _ := procGetModuleHandleW.Call(0)
		cur, _, _ := procLoadCursorW.Call(0, idcArrow)
		wc := wndClassEx{Size: uint32(unsafe.Sizeof(wndClassEx{})), WndProc: uiProc, Instance: inst,
			Cursor: cur, ClassName: uiClassName, Icon: appIcon(32), IconSm: appIcon(16)}
		r, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
		uiClassOK = r != 0
	})
	return uiClassOK
}

// runUI zeigt ein Fenster und wartet, bis es geschlossen wird. Ergebnis: ID des Knopfs,
// "close", "timeout" oder was uiCtl.Close übergeben hat.
func runUI(d *uiDialog, o uiOpts) (string, error) {
	res := make(chan struct {
		r   string
		err error
	}, 1)
	go func() {
		runtime.LockOSThread()
		r, err := runUILocked(d, o)
		res <- struct {
			r   string
			err error
		}{r, err}
	}()
	out := <-res
	return out.r, out.err
}

func runUILocked(d *uiDialog, o uiOpts) (res string, err error) {
	defer func() {
		if e := recover(); e != nil {
			logf("Fenster-Fehler: %v", e)
			res, err = "", fmt.Errorf("Fenster-Fehler: %v", e)
		}
	}()
	scale := uiScale()
	if !registerUIClass() {
		return "", errors.New("Fensterklasse nicht verfügbar")
	}
	w := &uiWin{dlg: d, opts: o, r: newRenderer(scale), left: o.Countdown}
	defer w.r.close()
	if d != nil && d.Input != nil {
		w.st.Focus, w.st.CaretOn = "input", true
	}
	if d != nil && d.Progress >= 0 {
		w.progress = d.Progress
	}

	const (
		wsPopup      = 0x80000000
		exLayered    = 0x00080000
		exTopmost    = 0x00000008
		exToolWindow = 0x00000080
		exAppWindow  = 0x00040000
		exNoActivate = 0x08000000
	)
	ex := uintptr(exLayered)
	if o.Topmost || o.Bar {
		ex |= exTopmost
	}
	if o.Bar {
		ex |= exToolWindow | exNoActivate
	} else {
		ex |= exAppWindow
	}
	inst, _, _ := procGetModuleHandleW.Call(0)
	title := o.Title
	if title == "" {
		title = "Nicolaj Fernwartung"
	}
	hwnd, _, err := procCreateWindowExW.Call(ex, uintptr(unsafe.Pointer(uiClassName)),
		uintptr(unsafe.Pointer(utf16Ptr(title))), wsPopup, 0, 0, 10, 10, 0, 0, inst, 0)
	if hwnd == 0 {
		return "", errors.New("Fenster konnte nicht erstellt werden: " + err.Error())
	}
	w.hwnd = hwnd
	uiMu.Lock()
	uiWins[hwnd] = w
	uiMu.Unlock()

	w.redraw()
	if w.r.dib == 0 {
		procDestroyWindow.Call(hwnd)
		return "", errors.New("Zeichnen nicht möglich")
	}
	w.shown = true
	if o.Bar {
		if procSetWindowDisplayAffinity.Find() == nil {
			procSetWindowDisplayAffinity.Call(hwnd, 0x11) // nicht im Live-Bild
		}
		procShowWindow.Call(hwnd, 4) // SW_SHOWNOACTIVATE
		procSetWindowPos.Call(hwnd, ^uintptr(0), 0, 0, 0, 0, 0x0001|0x0002|0x0010)
	} else {
		procSendMessageW.Call(hwnd, wmSetIcon, 1, appIcon(32))
		procSendMessageW.Call(hwnd, wmSetIcon, 0, appIcon(16))
		procShowWindow.Call(hwnd, 5) // SW_SHOW
		if r, _, _ := puSetForeground.Call(hwnd); r == 0 {
			fi := flashInfo{Size: uint32(unsafe.Sizeof(flashInfo{})), Hwnd: hwnd, Flags: 3 | 12} // ALL | TIMERNOFG
			puFlashWindowEx.Call(uintptr(unsafe.Pointer(&fi)))
		}
	}
	if o.Beep != 0 {
		puMessageBeep.Call(o.Beep)
	}
	if o.Animate || o.Bar {
		iv := uintptr(33)
		if o.Bar {
			iv = 60
		}
		puSetTimer.Call(hwnd, timerAnim, iv, 0)
	}
	if o.Countdown > 0 {
		puSetTimer.Call(hwnd, timerCount, 1000, 0)
	}
	if d != nil && d.Input != nil {
		puSetTimer.Call(hwnd, timerCaret, 530, 0)
	}
	ctl := &uiCtl{w}
	if o.Ctx != nil {
		stop := make(chan struct{})
		defer close(stop)
		go func() {
			select {
			case <-o.Ctx.Done():
				ctl.Close("cancel")
			case <-stop:
			}
		}()
	}
	if o.Ready != nil {
		go o.Ready(ctl)
	}
	var m winMsg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	if w.result == "" {
		w.result = "close"
	}
	return w.result, nil
}

// ---------- Zwischenablage ----------

// sysPtr: Zeiger aus einem Windows-Aufruf (nicht vom Go-Speicher verwaltet).
func sysPtr(u uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&u)) }

func setClipboard(hwnd uintptr, s string) {
	if r, _, _ := puOpenClipboard.Call(hwnd); r == 0 {
		return
	}
	defer puCloseClipboard.Call()
	puEmptyClipboard.Call()
	u := utf16.Encode([]rune(s + "\x00"))
	h, _, _ := puGlobalAlloc.Call(0x0002, uintptr(len(u)*2)) // GMEM_MOVEABLE
	if h == 0 {
		return
	}
	p, _, _ := puGlobalLock.Call(h)
	if p == 0 {
		puGlobalFree.Call(h)
		return
	}
	copy(unsafe.Slice((*uint16)(sysPtr(p)), len(u)), u)
	puGlobalUnlock.Call(h)
	if r, _, _ := puSetClipboardData.Call(13, h); r == 0 { // CF_UNICODETEXT
		puGlobalFree.Call(h)
	}
}

func getClipboard(hwnd uintptr) string {
	if r, _, _ := puOpenClipboard.Call(hwnd); r == 0 {
		return ""
	}
	defer puCloseClipboard.Call()
	h, _, _ := puGetClipboardData.Call(13)
	if h == 0 {
		return ""
	}
	p, _, _ := puGlobalLock.Call(h)
	if p == 0 {
		return ""
	}
	defer puGlobalUnlock.Call(h)
	sz, _, _ := puGlobalSize.Call(h)
	n := int(sz / 2)
	if n > 4096 {
		n = 4096
	}
	if n == 0 {
		return ""
	}
	var u []uint16
	src := unsafe.Slice((*uint16)(sysPtr(p)), n)
	for _, c := range src {
		if c == 0 {
			break
		}
		u = append(u, c)
	}
	s := string(utf16.Decode(u))
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	return s
}

// ---------- Programm-Symbol (aus dem Logo gerendert) ----------

var (
	iconMu    sync.Mutex
	iconCache = map[int]uintptr{}
)

func appIcon(size int) uintptr {
	iconMu.Lock()
	defer iconMu.Unlock()
	if h, ok := iconCache[size]; ok {
		return h
	}
	px := int(math.Round(float64(size) * uiScale()))
	c := newCanvas(px, px, float64(px)/64)
	c.logo(0, 0, 64)
	bi := bitmapInfo{Header: bitmapInfoHeader{Size: uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width: int32(px), Height: -int32(px), Planes: 1, BitCount: 32}}
	var bits unsafe.Pointer
	color, _, _ := puCreateDIBSection.Call(0, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if color == 0 || bits == nil {
		iconCache[size] = 0
		return 0
	}
	dst := unsafe.Slice((*byte)(bits), px*px*4)
	for i := 0; i < px*px; i++ { // Symbole erwarten nicht vormultipliertes Alpha
		a := c.Pix[i*4+3]
		for k := 0; k < 3; k++ {
			v := c.Pix[i*4+k]
			if a > 0 {
				v = uint8(min(255, int(v)*255/int(a)))
			}
			dst[i*4+k] = v
		}
		dst[i*4+3] = a
	}
	maskBits := make([]byte, ((px+15)/16*2)*px) // alles 0 = Farbe gilt (Alpha regelt die Transparenz)
	mask, _, _ := puCreateBitmap.Call(uintptr(px), uintptr(px), 1, 1, uintptr(unsafe.Pointer(&maskBits[0])))
	ii := iconInfo{Icon: 1, Mask: mask, Color: color}
	h, _, _ := puCreateIconIndirect.Call(uintptr(unsafe.Pointer(&ii)))
	procDeleteObject.Call(color)
	procDeleteObject.Call(mask)
	iconCache[size] = h
	return h
}

// ---------- Bequeme Aufrufe mit Rückfall auf Windows-Standardfenster ----------

func uiNotice(title, body, tone string) {
	if _, err := runUI(dlgNotice(title, body, tone), uiOpts{}); err != nil {
		icon := uintptr(mbIconInfo)
		switch tone {
		case "danger":
			icon = mbIconError
		case "warn":
			icon = mbIconWarning
		}
		msgBox(title+"\n\n"+body, icon)
	}
}

// ---------- Fenster während der Fernwartung ----------

// askConsent fragt die Person am PC, ob sich Nicolaj mit dem Bildschirm verbinden darf.
func askConsent(ctx context.Context, by string) bool {
	res, err := runUI(dlgConsent(by, consentSeconds), uiOpts{Topmost: true, Beep: 0x20, Countdown: consentSeconds, Ctx: ctx,
		OnCount: func(d *uiDialog, left int) { d.Note = consentNote(left) },
		OnClick: func(ctl *uiCtl, id string) bool { return id != "input-changed" }})
	if err != nil {
		return askConsentClassic(ctx, by)
	}
	return res == "allow" && ctx.Err() == nil
}

func showMessage(text string) {
	go func() {
		if _, err := runUI(dlgMessage(text), uiOpts{Topmost: true, Beep: 0x40}); err != nil {
			showMessageClassic(text)
		}
	}()
}

type uiBar struct {
	mu       sync.Mutex
	ctl      *uiCtl
	done     chan struct{}
	fallback closer
}

func (b *uiBar) Close() {
	b.mu.Lock()
	ctl, fb := b.ctl, b.fallback
	b.mu.Unlock()
	if fb != nil {
		fb.Close()
		return
	}
	if ctl != nil {
		ctl.Close("cancel")
	}
	select {
	case <-b.done:
	case <-time.After(2 * time.Second):
	}
}

// showOverlay: Hinweisleiste oben am Bildschirm mit „Trennen“.
func showOverlay(by string, onDisconnect func()) closer {
	b := &uiBar{done: make(chan struct{})}
	ready := make(chan bool, 1)
	var once sync.Once
	go func() {
		defer close(b.done)
		_, err := runUI(nil, uiOpts{Bar: true, BarText: barText,
			Ready: func(ctl *uiCtl) {
				b.mu.Lock()
				b.ctl = ctl
				b.mu.Unlock()
				once.Do(func() { ready <- true })
			},
			OnClick: func(ctl *uiCtl, id string) bool {
				if id == "disconnect" {
					go onDisconnect()
					return true
				}
				return false
			}})
		if err != nil {
			once.Do(func() { ready <- false })
		}
	}()
	select {
	case ok := <-ready:
		if !ok {
			b.mu.Lock()
			b.fallback = showOverlayClassic(by, onDisconnect)
			b.mu.Unlock()
		}
	case <-time.After(3 * time.Second):
	}
	return b
}
