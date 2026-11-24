//go:build windows

// Nachrichten-Fenster und Symbol im Infobereich (unten rechts neben der Uhr).

package main

import (
	"errors"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var (
	puFindWindowW      = user32.NewProc("FindWindowW")
	puShowWindow       = user32.NewProc("ShowWindow")
	puCreatePopupMenu  = user32.NewProc("CreatePopupMenu")
	puAppendMenuW      = user32.NewProc("AppendMenuW")
	puTrackPopupMenu   = user32.NewProc("TrackPopupMenu")
	puDestroyMenu      = user32.NewProc("DestroyMenu")
	puGetCursorPos     = user32.NewProc("GetCursorPos")
	puRegisterWindowMs = user32.NewProc("RegisterWindowMessageW")
	puShellNotifyIcon  = shell32.NewProc("Shell_NotifyIconW")
)

// ---------- Nachrichten-Fenster ----------

var (
	chatMu      sync.Mutex
	chatOpen    bool
	chatRefresh = make(chan struct{}, 1)
)

// runChat: Startmenü „Nachricht an Nicolaj“ (läuft als angemeldete Person).
func runChat() {
	cfg, err := loadConfig(screenConfigPath())
	if err != nil {
		uiNotice("Nicht eingerichtet", "Nicolaj Fernwartung ist auf diesem PC nicht (vollständig) installiert.", "warn")
		return
	}
	showChat(newClient(cfg), false)
}

// openChatWindow: aus dem Bildschirm-Helfer (Antwort von Nicolaj oder Klick aufs Symbol).
func openChatWindow(c *Client, incoming bool) {
	chatMu.Lock()
	open := chatOpen
	chatMu.Unlock()
	if open {
		select {
		case chatRefresh <- struct{}{}:
		default:
		}
		focusChat()
		return
	}
	go showChat(c, incoming)
}

func focusChat() bool {
	h, _, _ := puFindWindowW.Call(uintptr(unsafe.Pointer(uiClassName)), uintptr(unsafe.Pointer(utf16Ptr(chatTitle))))
	if h == 0 {
		return false
	}
	puShowWindow.Call(h, 5)
	puSetForeground.Call(h)
	return true
}

// showChat zeigt das Fenster (nur einmal pro Benutzer, auch über Prozesse hinweg).
func showChat(c *Client, incoming bool) {
	name := utf16Ptr("Local\\NicolajRMMChat")
	h, _, err := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h != 0 {
		defer syscall.CloseHandle(syscall.Handle(h))
	}
	if errno, ok := err.(syscall.Errno); ok && errno == 183 { // schon offen (anderer Prozess)
		focusChat()
		return
	}
	chatMu.Lock()
	chatOpen = true
	chatMu.Unlock()
	defer func() {
		chatMu.Lock()
		chatOpen = false
		chatMu.Unlock()
	}()

	msgs, herr := c.chatHistory()
	note := ""
	if herr != nil {
		note = "Gerade keine Verbindung – deine Nachricht wird gesendet, sobald es wieder geht."
		if errors.Is(herr, errUnauthorized) {
			note = "Dieser PC ist nicht mehr mit Nicolaj verbunden."
		}
	}
	var (
		mu      sync.Mutex
		sig     = chatSignature(msgs)
		closed  atomic.Bool
		sending atomic.Bool
	)
	draft := func(d *uiDialog) string {
		if d.Input == nil {
			return ""
		}
		return d.Input.Value
	}
	refresh := func(ctl *uiCtl) {
		list, err := c.chatHistory()
		if err != nil {
			return
		}
		mu.Lock()
		s := chatSignature(list)
		changed := s != sig
		sig = s
		mu.Unlock()
		if changed {
			ctl.Update(func(d *uiDialog) { d.Chat = chatToUI(list) })
		}
	}
	send := func(ctl *uiCtl, text string) {
		defer sending.Store(false)
		var err error
		for i := 0; i < 3; i++ { // kurze Aussetzer überbrücken
			if err = c.chatSend(text); err == nil {
				break
			}
			time.Sleep(2 * time.Second)
		}
		if err != nil {
			ctl.Update(func(d *uiDialog) {
				if d.Input != nil && strings.TrimSpace(d.Input.Value) == "" {
					d.Input.Value = text
				}
				d.Note = "Nicht gesendet: " + err.Error()
				if b := d.button("send"); b != nil {
					b.Disabled = false
				}
			})
			return
		}
		ctl.Update(func(d *uiDialog) { d.Note = "Gesendet ✓ – Nicolaj bekommt deine Nachricht im Dashboard." })
		refresh(ctl)
	}
	opts := uiOpts{Title: chatTitle, Topmost: incoming,
		OnClick: func(ctl *uiCtl, id string) bool {
			switch id {
			case "input-changed":
				ctl.Update(func(d *uiDialog) {
					if b := d.button("send"); b != nil {
						b.Disabled = strings.TrimSpace(draft(d)) == "" || sending.Load()
					}
				})
				return false
			case "send":
				if !sending.CompareAndSwap(false, true) {
					return false
				}
				var text string
				ctl.Update(func(d *uiDialog) {
					text = strings.TrimSpace(draft(d))
					if text == "" {
						return
					}
					d.Input.Value = ""
					d.Note = "Wird gesendet …"
					if b := d.button("send"); b != nil {
						b.Disabled = true
					}
				})
				if text == "" {
					sending.Store(false)
					return false
				}
				go send(ctl, text)
				return false
			case "close":
				return true
			}
			return false
		},
		Ready: func(ctl *uiCtl) {
			t := time.NewTicker(3 * time.Second)
			defer t.Stop()
			for !closed.Load() {
				select {
				case <-t.C:
				case <-chatRefresh:
				}
				if !closed.Load() {
					refresh(ctl)
				}
			}
		},
	}
	if incoming {
		opts.Beep = 0x40
	}
	_, uerr := runUI(dlgChat(chatToUI(msgs), "", note), opts)
	closed.Store(true)
	select {
	case chatRefresh <- struct{}{}: // Abfrage-Schleife beenden
	default:
	}
	if uerr != nil && incoming { // Rückfall: wenigstens die letzte Antwort zeigen
		if len(msgs) > 0 {
			showMessageClassic(msgs[len(msgs)-1].Text)
		}
	}
}

// ---------- Symbol im Infobereich ----------

type notifyIconData struct {
	Size        uint32
	_           uint32
	Hwnd        uintptr
	ID          uint32
	Flags       uint32
	Callback    uint32
	_           uint32
	Icon        uintptr
	Tip         [128]uint16
	State       uint32
	StateMask   uint32
	Info        [256]uint16
	Version     uint32
	InfoTitle   [64]uint16
	InfoFlags   uint32
	GuidItem    [16]byte
	BalloonIcon uintptr
}

const (
	trayMsg   = 0x8005 // WM_APP + 5
	trayChat  = 1
	trayInfo  = 2
	nimAdd    = 0
	nimDelete = 2
)

var (
	trayC         *Client
	trayData      notifyIconData
	trayRecreated uintptr
	trayProc      = syscall.NewCallback(trayWndProc)
	trayClass     = utf16Ptr("NicolajFernwartungTray")
)

func trayAdd() {
	puShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&trayData)))
}

func trayWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) (ret uintptr) {
	defer func() {
		if e := recover(); e != nil {
			logf("Infobereich-Fehler: %v", e)
			ret = 0
		}
	}()
	switch {
	case msg == trayMsg:
		switch lParam & 0xFFFF {
		case wmLButtonUp:
			openChatWindow(trayC, false)
		case 0x0205: // WM_RBUTTONUP
			trayMenu(hwnd)
		}
		return 0
	case trayRecreated != 0 && uintptr(msg) == trayRecreated: // Explorer neu gestartet
		trayAdd()
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func trayMenu(hwnd uintptr) {
	m, _, _ := puCreatePopupMenu.Call()
	if m == 0 {
		return
	}
	defer puDestroyMenu.Call(m)
	puAppendMenuW.Call(m, 0, trayChat, uintptr(unsafe.Pointer(utf16Ptr("Nachricht an Nicolaj …"))))
	puAppendMenuW.Call(m, 0, trayInfo, uintptr(unsafe.Pointer(utf16Ptr("Meine PC-ID anzeigen"))))
	var p point
	puGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	puSetForeground.Call(hwnd)
	cmd, _, _ := puTrackPopupMenu.Call(m, 0x0100|0x0002, uintptr(p.X), uintptr(p.Y), 0, hwnd, 0) // RETURNCMD | RIGHTBUTTON
	procPostMessageW.Call(hwnd, 0, 0, 0)
	switch cmd {
	case trayChat:
		openChatWindow(trayC, false)
	case trayInfo:
		go showInfo()
	}
}

// startTray zeigt das „n“-Symbol unten rechts: Klick = Nachricht, Rechtsklick = Menü.
func startTray(c *Client) {
	trayC = c
	go func() {
		runtime.LockOSThread()
		defer func() {
			if e := recover(); e != nil {
				logf("Infobereich-Fehler: %v", e)
			}
		}()
		uiScale()
		inst, _, _ := procGetModuleHandleW.Call(0)
		wc := wndClassEx{Size: uint32(unsafe.Sizeof(wndClassEx{})), WndProc: trayProc, Instance: inst, ClassName: trayClass}
		if r, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
			logf("Infobereich: Fensterklasse nicht verfügbar")
			return
		}
		hwnd, _, _ := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(trayClass)), uintptr(unsafe.Pointer(utf16Ptr("Nicolaj Fernwartung"))),
			0, 0, 0, 0, 0, 0, 0, inst, 0)
		if hwnd == 0 {
			logf("Infobereich: Fenster konnte nicht erstellt werden")
			return
		}
		trayRecreated, _, _ = puRegisterWindowMs.Call(uintptr(unsafe.Pointer(utf16Ptr("TaskbarCreated"))))
		trayData = notifyIconData{Hwnd: hwnd, ID: 1, Flags: 1 | 2 | 4, Callback: trayMsg, Icon: appIcon(16)} // MESSAGE | ICON | TIP
		trayData.Size = uint32(unsafe.Sizeof(trayData))
		copy(trayData.Tip[:], syscall.StringToUTF16("Nicolaj Fernwartung – klicken für eine Nachricht an Nicolaj"))
		for i := 0; i < 30; i++ { // Kurz nach der Anmeldung ist die Taskleiste evtl. noch nicht da
			if r, _, _ := puShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&trayData))); r != 0 {
				break
			}
			time.Sleep(2 * time.Second)
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
		puShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&trayData)))
	}()
}
