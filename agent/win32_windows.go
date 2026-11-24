//go:build windows

// Windows-Schnittstellen: Bildschirmaufnahme (GDI), Maus/Tastatur, Hinweisleiste, Dialoge.
// Nur Standardbibliothek – keine Fremdpakete.
package main

import (
	"bytes"
	"context"
	"errors"
	"image"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")

	procGetDC                         = user32.NewProc("GetDC")
	procReleaseDC                     = user32.NewProc("ReleaseDC")
	procGetSystemMetrics              = user32.NewProc("GetSystemMetrics")
	procSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procGetDpiForSystem               = user32.NewProc("GetDpiForSystem")
	procSetCursorPos                  = user32.NewProc("SetCursorPos")
	procMouseEvent                    = user32.NewProc("mouse_event")
	procKeybdEvent                    = user32.NewProc("keybd_event")
	procSendInput                     = user32.NewProc("SendInput")
	procMapVirtualKeyW                = user32.NewProc("MapVirtualKeyW")
	procMessageBoxW                   = user32.NewProc("MessageBoxW")
	procMessageBoxTimeoutW            = user32.NewProc("MessageBoxTimeoutW")
	procLockWorkStation               = user32.NewProc("LockWorkStation")
	procGetCursorInfo                 = user32.NewProc("GetCursorInfo")
	procGetIconInfo                   = user32.NewProc("GetIconInfo")
	procDrawIconEx                    = user32.NewProc("DrawIconEx")
	procRegisterClassExW              = user32.NewProc("RegisterClassExW")
	procCreateWindowExW               = user32.NewProc("CreateWindowExW")
	procDefWindowProcW                = user32.NewProc("DefWindowProcW")
	procDestroyWindow                 = user32.NewProc("DestroyWindow")
	procPostQuitMessage               = user32.NewProc("PostQuitMessage")
	procGetMessageW                   = user32.NewProc("GetMessageW")
	procTranslateMessage              = user32.NewProc("TranslateMessage")
	procDispatchMessageW              = user32.NewProc("DispatchMessageW")
	procPostMessageW                  = user32.NewProc("PostMessageW")
	procSendMessageW                  = user32.NewProc("SendMessageW")
	procShowWindow                    = user32.NewProc("ShowWindow")
	procSetWindowPos                  = user32.NewProc("SetWindowPos")
	procBeginPaint                    = user32.NewProc("BeginPaint")
	procEndPaint                      = user32.NewProc("EndPaint")
	procDrawTextW                     = user32.NewProc("DrawTextW")
	procLoadCursorW                   = user32.NewProc("LoadCursorW")
	procSetWindowDisplayAffinity      = user32.NewProc("SetWindowDisplayAffinity")

	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procGetDIBits              = gdi32.NewProc("GetDIBits")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	procCreateFontW            = gdi32.NewProc("CreateFontW")
	procSetBkMode              = gdi32.NewProc("SetBkMode")
	procSetTextColor           = gdi32.NewProc("SetTextColor")

	procCreateMutexW     = kernel32.NewProc("CreateMutexW")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")

	procIsUserAnAdmin = shell32.NewProc("IsUserAnAdmin")
	procShellExecuteW = shell32.NewProc("ShellExecuteW")
)

const (
	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCXVirtualScreen = 78
	smCYVirtualScreen = 79
	smCXScreen        = 0

	srcCopy    = 0x00CC0020
	captureBlt = 0x40000000

	mbOK            = 0x0
	mbYesNo         = 0x4
	mbIconError     = 0x10
	mbIconQuestion  = 0x20
	mbIconWarning   = 0x30
	mbIconInfo      = 0x40
	mbSystemModal   = 0x1000
	mbSetForeground = 0x10000
	mbTopmost       = 0x40000
	idYes           = 6
)

func utf16Ptr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func metric(i int) int32 {
	r, _, _ := procGetSystemMetrics.Call(uintptr(i))
	return int32(r)
}

// ---------- Prozess ----------

var mutexHandle uintptr

func singleInstance(name string) bool {
	h, _, err := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(utf16Ptr(name))))
	if h == 0 {
		return true
	}
	if errno, ok := err.(syscall.Errno); ok && errno == 183 { // ERROR_ALREADY_EXISTS
		return false
	}
	mutexHandle = h
	return true
}

func initScreenPlatform() {
	// Echte Pixel statt hochskalierter Werte (wichtig bei 125 %/150 % Anzeige-Skalierung).
	if procSetProcessDpiAwarenessContext.Find() == nil {
		if r, _, _ := procSetProcessDpiAwarenessContext.Call(^uintptr(3)); r != 0 { // PER_MONITOR_AWARE_V2 (-4)
			return
		}
	}
	procSetProcessDPIAware.Call()
}

func isAdmin() bool {
	r, _, _ := procIsUserAnAdmin.Call()
	return r != 0
}

// relaunchElevated startet das Programm mit Administratorrechten neu (Windows fragt per UAC).
func relaunchElevated(exe string, args []string) bool {
	params := ""
	for i, a := range args {
		if i > 0 {
			params += " "
		}
		params += syscall.EscapeArg(a)
	}
	r, _, _ := procShellExecuteW.Call(0, uintptr(unsafe.Pointer(utf16Ptr("runas"))),
		uintptr(unsafe.Pointer(utf16Ptr(exe))), uintptr(unsafe.Pointer(utf16Ptr(params))), 0, 1)
	return r > 32
}

// ---------- Dialoge ----------

const msgCaption = "Nicolaj Fernwartung"

func msgBox(text string, flags uintptr) int {
	r, _, _ := procMessageBoxW.Call(0, uintptr(unsafe.Pointer(utf16Ptr(text))),
		uintptr(unsafe.Pointer(utf16Ptr(msgCaption))), flags|mbTopmost|mbSetForeground)
	return int(r)
}

func msgBoxTimeout(text string, flags uintptr, timeout time.Duration) int {
	if procMessageBoxTimeoutW.Find() != nil {
		return msgBox(text, flags)
	}
	r, _, _ := procMessageBoxTimeoutW.Call(0, uintptr(unsafe.Pointer(utf16Ptr(text))),
		uintptr(unsafe.Pointer(utf16Ptr(msgCaption))), flags|mbTopmost|mbSetForeground, 0, uintptr(timeout.Milliseconds()))
	return int(r)
}

func askConsentClassic(ctx context.Context, by string) bool {
	text := "Nicolaj Support (" + by + ") möchte sich mit deinem Bildschirm verbinden, um dir zu helfen.\n\n" +
		"Dabei kann dein Bildschirm gesehen und Maus und Tastatur können gesteuert werden. " +
		"Solange die Verbindung besteht, siehst du oben eine rote Leiste – dort kannst du jederzeit trennen.\n\n" +
		"Erlauben?"
	res := make(chan bool, 1)
	go func() {
		runtime.LockOSThread()
		res <- msgBoxTimeout(text, mbYesNo|mbIconQuestion|mbSystemModal, 60*time.Second) == idYes
	}()
	select {
	case ok := <-res:
		return ok && ctx.Err() == nil
	case <-ctx.Done():
		return false
	}
}

func showMessageClassic(text string) {
	go func() {
		runtime.LockOSThread()
		procMessageBoxW.Call(0, uintptr(unsafe.Pointer(utf16Ptr(text))),
			uintptr(unsafe.Pointer(utf16Ptr("Nachricht von Nicolaj Support"))), mbOK|mbIconInfo|mbSystemModal|mbTopmost|mbSetForeground)
	}()
}

func lockScreen() error {
	if r, _, err := procLockWorkStation.Call(); r == 0 {
		return err
	}
	return nil
}

// ---------- Bildschirmaufnahme ----------

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [1]uint32
}

type point struct{ X, Y int32 }

type cursorInfo struct {
	Size    uint32
	Flags   uint32
	Cursor  uintptr
	ScreenX int32
	ScreenY int32
}

type iconInfo struct {
	Icon     int32
	XHotspot uint32
	YHotspot uint32
	Mask     uintptr
	Color    uintptr
}

type gdiCapturer struct {
	mu          sync.Mutex
	vx, vy      int32
	w, h        int32
	screenDC    uintptr
	memDC       uintptr
	bmp         uintptr
	cur, prev   []byte
	rgba        *image.RGBA
	lastQuality string
}

var screenGeom struct {
	sync.Mutex
	vx, vy, w, h int32
}

func newCapturer() (Capturer, error) {
	c := &gdiCapturer{}
	if err := c.setup(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *gdiCapturer) setup() error {
	c.vx, c.vy = metric(smXVirtualScreen), metric(smYVirtualScreen)
	c.w, c.h = metric(smCXVirtualScreen), metric(smCYVirtualScreen)
	if c.w <= 0 || c.h <= 0 {
		return errors.New("Bildschirmgröße nicht ermittelbar")
	}
	screenGeom.Lock()
	screenGeom.vx, screenGeom.vy, screenGeom.w, screenGeom.h = c.vx, c.vy, c.w, c.h
	screenGeom.Unlock()
	c.screenDC, _, _ = procGetDC.Call(0)
	if c.screenDC == 0 {
		return errors.New("Kein Zugriff auf den Bildschirm (GetDC)")
	}
	c.memDC, _, _ = procCreateCompatibleDC.Call(c.screenDC)
	c.bmp, _, _ = procCreateCompatibleBitmap.Call(c.screenDC, uintptr(c.w), uintptr(c.h))
	if c.memDC == 0 || c.bmp == 0 {
		c.release()
		return errors.New("Speicher für Bildschirmaufnahme nicht verfügbar")
	}
	n := int(c.w) * int(c.h) * 4
	c.cur, c.prev = make([]byte, n), make([]byte, n)
	c.rgba = image.NewRGBA(image.Rect(0, 0, int(c.w), int(c.h)))
	c.lastQuality = ""
	return nil
}

func (c *gdiCapturer) release() {
	if c.bmp != 0 {
		procDeleteObject.Call(c.bmp)
		c.bmp = 0
	}
	if c.memDC != 0 {
		procDeleteDC.Call(c.memDC)
		c.memDC = 0
	}
	if c.screenDC != 0 {
		procReleaseDC.Call(0, c.screenDC)
		c.screenDC = 0
	}
}

func (c *gdiCapturer) Size() (int, int) { return int(c.w), int(c.h) }

func (c *gdiCapturer) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.release()
}

func (c *gdiCapturer) Capture(quality string, force bool) ([]byte, int, int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Auflösung geändert (Monitor angesteckt, Skalierung geändert)?
	if metric(smCXVirtualScreen) != c.w || metric(smCYVirtualScreen) != c.h ||
		metric(smXVirtualScreen) != c.vx || metric(smYVirtualScreen) != c.vy {
		c.release()
		if err := c.setup(); err != nil {
			return nil, 0, 0, err
		}
		force = true
	}
	old, _, _ := procSelectObject.Call(c.memDC, c.bmp)
	r, _, err := procBitBlt.Call(c.memDC, 0, 0, uintptr(c.w), uintptr(c.h), c.screenDC,
		uintptr(c.vx), uintptr(c.vy), srcCopy|captureBlt)
	if r == 0 {
		procSelectObject.Call(c.memDC, old)
		return nil, 0, 0, errors.New("Aufnahme nicht möglich (gesperrter Bildschirm oder UAC-Abfrage?): " + err.Error())
	}
	c.drawCursor()
	procSelectObject.Call(c.memDC, old) // Bitmap für GetDIBits aus dem DC lösen

	bi := bitmapInfo{Header: bitmapInfoHeader{
		Size: uint32(unsafe.Sizeof(bitmapInfoHeader{})), Width: c.w, Height: -c.h, // negativ = von oben nach unten
		Planes: 1, BitCount: 32, Compression: 0,
	}}
	lines, _, _ := procGetDIBits.Call(c.memDC, c.bmp, 0, uintptr(c.h),
		uintptr(unsafe.Pointer(&c.cur[0])), uintptr(unsafe.Pointer(&bi)), 0)
	if lines == 0 {
		return nil, 0, 0, errors.New("Bilddaten konnten nicht gelesen werden")
	}
	changed := !bytes.Equal(c.cur, c.prev)
	if !changed && !force && quality == c.lastQuality {
		return nil, int(c.w), int(c.h), nil
	}
	c.cur, c.prev = c.prev, c.cur
	c.lastQuality = quality
	bgraToRGBA(c.prev, c.rgba)
	return encodeFrame(c.rgba, quality), int(c.w), int(c.h), nil
}

func (c *gdiCapturer) drawCursor() {
	ci := cursorInfo{Size: uint32(unsafe.Sizeof(cursorInfo{}))}
	if r, _, _ := procGetCursorInfo.Call(uintptr(unsafe.Pointer(&ci))); r == 0 || ci.Flags&1 == 0 || ci.Cursor == 0 {
		return
	}
	var ii iconInfo
	hx, hy := int32(0), int32(0)
	if r, _, _ := procGetIconInfo.Call(ci.Cursor, uintptr(unsafe.Pointer(&ii))); r != 0 {
		hx, hy = int32(ii.XHotspot), int32(ii.YHotspot)
		if ii.Mask != 0 {
			procDeleteObject.Call(ii.Mask)
		}
		if ii.Color != 0 {
			procDeleteObject.Call(ii.Color)
		}
	}
	x := ci.ScreenX - c.vx - hx
	y := ci.ScreenY - c.vy - hy
	procDrawIconEx.Call(c.memDC, uintptr(x), uintptr(y), ci.Cursor, 0, 0, 0, 0, 3) // DI_NORMAL
}

// ---------- Maus & Tastatur ----------

const (
	mouseLeftDown   = 0x0002
	mouseLeftUp     = 0x0004
	mouseRightDown  = 0x0008
	mouseRightUp    = 0x0010
	mouseMiddleDown = 0x0020
	mouseMiddleUp   = 0x0040
	mouseWheel      = 0x0800
	keyExtended     = 0x0001
	keyUp           = 0x0002
	keyUnicode      = 0x0004
)

func moveTo(nx, ny float64) {
	screenGeom.Lock()
	vx, vy, w, h := screenGeom.vx, screenGeom.vy, screenGeom.w, screenGeom.h
	screenGeom.Unlock()
	if w <= 0 || h <= 0 {
		vx, vy, w, h = metric(smXVirtualScreen), metric(smYVirtualScreen), metric(smCXVirtualScreen), metric(smCYVirtualScreen)
	}
	x := vx + int32(nx*float64(w-1)+0.5)
	y := vy + int32(ny*float64(h-1)+0.5)
	procSetCursorPos.Call(uintptr(x), uintptr(y))
}

func applyInput(e InputEvent) {
	switch e.T {
	case "m":
		moveTo(e.X, e.Y)
	case "b":
		moveTo(e.X, e.Y)
		flags := map[[2]int]uintptr{
			{0, 1}: mouseLeftDown, {0, 0}: mouseLeftUp,
			{1, 1}: mouseMiddleDown, {1, 0}: mouseMiddleUp,
			{2, 1}: mouseRightDown, {2, 0}: mouseRightUp,
		}[[2]int{e.B, map[bool]int{true: 1, false: 0}[e.D]}]
		if flags != 0 {
			procMouseEvent.Call(flags, 0, 0, 0, 0)
		}
	case "w":
		moveTo(e.X, e.Y)
		d := int32(e.Dy)
		procMouseEvent.Call(mouseWheel, 0, 0, uintptr(uint32(d)), 0)
	case "k":
		pressKey(e.C, e.D)
	case "text":
		typeText(e.S)
	}
}

func pressKey(code string, down bool) {
	k, ok := keyMap[code]
	if !ok {
		return
	}
	vk := uintptr(k.vk)
	if vk == 0 { // Zeichen-Taste: passend zum Tastaturlayout des PCs
		vk, _, _ = procMapVirtualKeyW.Call(uintptr(k.scan), 1) // MAPVK_VSC_TO_VK
		if vk == 0 {
			return
		}
	}
	var flags uintptr
	if k.ext {
		flags |= keyExtended
	}
	if !down {
		flags |= keyUp
	}
	procKeybdEvent.Call(vk, uintptr(k.scan&0xFF), flags, 0)
}

// keyboardInput entspricht KEYBDINPUT in einer INPUT-Struktur (64-Bit-Layout, 40 Byte).
type keyboardInput struct {
	Type      uint32
	_         uint32
	Vk        uint16
	Scan      uint16
	Flags     uint32
	Time      uint32
	_         uint32
	ExtraInfo uintptr
	_         [8]byte
}

// typeText tippt beliebigen Text (auch Umlaute, Emojis) unabhängig vom Tastaturlayout.
func typeText(s string) {
	for _, r := range s {
		if r == '\n' {
			pressKey("Enter", true)
			pressKey("Enter", false)
			continue
		}
		if r == '\r' {
			continue
		}
		units := utf16.Encode([]rune{r})
		in := make([]keyboardInput, 0, len(units)*2)
		for _, u := range units {
			in = append(in, keyboardInput{Type: 1, Scan: u, Flags: keyUnicode})
		}
		for _, u := range units {
			in = append(in, keyboardInput{Type: 1, Scan: u, Flags: keyUnicode | keyUp})
		}
		procSendInput.Call(uintptr(len(in)), uintptr(unsafe.Pointer(&in[0])), unsafe.Sizeof(keyboardInput{}))
		time.Sleep(2 * time.Millisecond)
	}
}

// ---------- Rote Hinweisleiste während einer Sitzung ----------

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type winMsg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
	Private uint32
}

type rect struct{ Left, Top, Right, Bottom int32 }

type paintStruct struct {
	Hdc         uintptr
	Erase       int32
	Paint       rect
	Restore     int32
	IncUpdate   int32
	RGBReserved [32]byte
}

const (
	wmDestroy  = 0x0002
	wmClose    = 0x0010
	wmPaint    = 0x000F
	wmCommand  = 0x0111
	wmSetFont  = 0x0030
	wmNCHit    = 0x0084
	htCaption  = 2
	htClient   = 1
	overlayBtn = 1001
)

type overlayState struct {
	text         string
	font         uintptr
	onDisconnect func()
	fired        bool
	pad          int32
	textRight    int32
	height       int32
}

var (
	overlayMu      sync.Mutex
	overlays       = map[uintptr]*overlayState{}
	overlayClass   sync.Once
	overlayClassOK bool
	overlayProc    = syscall.NewCallback(overlayWndProc)
	overlayName    = utf16Ptr("NicolajRMMOverlay")
)

func overlayWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmCommand:
		if wParam&0xFFFF == overlayBtn {
			overlayMu.Lock()
			st := overlays[hwnd]
			var cb func()
			if st != nil && !st.fired {
				st.fired, cb = true, st.onDisconnect
			}
			overlayMu.Unlock()
			if cb != nil {
				go cb()
			}
			procPostMessageW.Call(hwnd, wmClose, 0, 0)
			return 0
		}
	case wmNCHit:
		r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		if r == htClient {
			return htCaption // Leiste lässt sich mit der Maus verschieben
		}
		return r
	case wmPaint:
		var ps paintStruct
		hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		overlayMu.Lock()
		st := overlays[hwnd]
		overlayMu.Unlock()
		if st != nil && hdc != 0 {
			procSetBkMode.Call(hdc, 1)
			procSetTextColor.Call(hdc, 0x00FFFFFF)
			if st.font != 0 {
				procSelectObject.Call(hdc, st.font)
			}
			area := rect{Left: st.pad, Top: 0, Right: st.textRight, Bottom: st.height}
			procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(utf16Ptr(st.text))), ^uintptr(0),
				uintptr(unsafe.Pointer(&area)), 0x20|0x04|0x8000) // SINGLELINE|VCENTER|END_ELLIPSIS
		}
		procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmClose:
		procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		overlayMu.Lock()
		if st := overlays[hwnd]; st != nil && st.font != 0 {
			procDeleteObject.Call(st.font)
		}
		delete(overlays, hwnd)
		overlayMu.Unlock()
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

type overlayWin struct {
	hwnd uintptr
	done chan struct{}
}

func (o *overlayWin) Close() {
	if o.hwnd == 0 {
		return
	}
	procPostMessageW.Call(o.hwnd, wmClose, 0, 0)
	select {
	case <-o.done:
	case <-time.After(2 * time.Second):
	}
}

func showOverlayClassic(by string, onDisconnect func()) closer {
	o := &overlayWin{done: make(chan struct{})}
	ready := make(chan uintptr, 1)
	go func() {
		runtime.LockOSThread()
		defer close(o.done)
		hwnd := createOverlay("●  Nicolaj Support ist mit diesem PC verbunden", onDisconnect)
		ready <- hwnd
		if hwnd == 0 {
			return
		}
		var m winMsg
		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 {
				return
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
	}()
	select {
	case o.hwnd = <-ready:
	case <-time.After(3 * time.Second):
	}
	return o
}

func createOverlay(text string, onDisconnect func()) uintptr {
	inst, _, _ := procGetModuleHandleW.Call(0)
	overlayClass.Do(func() {
		cur, _, _ := procLoadCursorW.Call(0, 32512)          // IDC_ARROW
		brush, _, _ := procCreateSolidBrush.Call(0x002626DC) // Rot (#DC2626) als COLORREF 0x00BBGGRR
		wc := wndClassEx{
			Size: uint32(unsafe.Sizeof(wndClassEx{})), WndProc: overlayProc, Instance: inst,
			Cursor: cur, Background: brush, ClassName: overlayName,
		}
		r, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
		overlayClassOK = r != 0
	})
	if !overlayClassOK {
		return 0
	}
	dpi := uintptr(96)
	if procGetDpiForSystem.Find() == nil {
		if d, _, _ := procGetDpiForSystem.Call(); d > 0 {
			dpi = d
		}
	}
	sc := func(v int) int32 { return int32(uintptr(v) * dpi / 96) }
	w, h := sc(470), sc(44)
	bw, bh := sc(96), sc(30)
	x := (metric(smCXScreen) - w) / 2
	y := sc(8)

	const (
		wsPopup        = 0x80000000
		wsChild        = 0x40000000
		wsVisible      = 0x10000000
		wsExTopmost    = 0x00000008
		wsExToolWindow = 0x00000080
		wsExNoActivate = 0x08000000
	)
	hwnd, _, _ := procCreateWindowExW.Call(wsExTopmost|wsExToolWindow|wsExNoActivate,
		uintptr(unsafe.Pointer(overlayName)), uintptr(unsafe.Pointer(utf16Ptr("Nicolaj Fernwartung"))),
		wsPopup, uintptr(x), uintptr(y), uintptr(w), uintptr(h), 0, 0, inst, 0)
	if hwnd == 0 {
		return 0
	}
	font, _, _ := procCreateFontW.Call(uintptr(int32(-sc(15))), 0, 0, 0, 600, 0, 0, 0, 1, 0, 0, 5, 0,
		uintptr(unsafe.Pointer(utf16Ptr("Segoe UI"))))
	overlayMu.Lock()
	overlays[hwnd] = &overlayState{text: text, font: font, onDisconnect: onDisconnect,
		pad: sc(14), textRight: w - bw - sc(14), height: h}
	overlayMu.Unlock()

	btn, _, _ := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(utf16Ptr("BUTTON"))),
		uintptr(unsafe.Pointer(utf16Ptr("Trennen"))), wsChild|wsVisible,
		uintptr(w-bw-sc(7)), uintptr((h-bh)/2), uintptr(bw), uintptr(bh), hwnd, overlayBtn, inst, 0)
	if btn != 0 && font != 0 {
		procSendMessageW.Call(btn, wmSetFont, font, 1)
	}
	if procSetWindowDisplayAffinity.Find() == nil {
		procSetWindowDisplayAffinity.Call(hwnd, 0x11) // WDA_EXCLUDEFROMCAPTURE: Leiste verdeckt nicht das Live-Bild
	}
	procShowWindow.Call(hwnd, 4)                                               // SW_SHOWNOACTIVATE
	procSetWindowPos.Call(hwnd, ^uintptr(0), 0, 0, 0, 0, 0x0001|0x0002|0x0010) // HWND_TOPMOST, NOSIZE|NOMOVE|NOACTIVATE
	return hwnd
}
