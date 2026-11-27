//go:build windows

// Nachrichten-Fenster und Symbol im Infobereich (unten rechts neben der Uhr).

package main

import (
	"errors"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
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
	cleanChatTemp()

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
		list    = chatVisible(msgs)
		sig     = chatSignature(msgs)
		closed  atomic.Bool
		sending atomic.Bool
		thumbs  = map[string]*uiImage{}
		failed  = map[string]bool{}
		loading = map[string]bool{}
		files   = map[string]string{} // Bild-ID → Datei im Temp-Ordner
		att     *preparedImage        // angehängtes Bild
		attBusy bool
		sem     = make(chan struct{}, 2) // höchstens zwei Bilder gleichzeitig laden
	)
	thumbOf := func(id string) (*uiImage, bool) {
		mu.Lock()
		defer mu.Unlock()
		return thumbs[id], failed[id]
	}
	rebuild := func(ctl *uiCtl) {
		mu.Lock()
		l := list
		mu.Unlock()
		ctl.Update(func(d *uiDialog) { d.Chat = chatToUI(l, thumbOf) })
	}
	msgAt := func(id string) (chatMsg, bool) { // "msg:N" / "img:N" → Nachricht
		_, n, ok := strings.Cut(id, ":")
		if !ok {
			return chatMsg{}, false
		}
		i := 0
		for _, ch := range n {
			i = i*10 + int(ch-'0')
		}
		mu.Lock()
		defer mu.Unlock()
		if i < 0 || i >= len(list) {
			return chatMsg{}, false
		}
		return list[i], true
	}
	// Bild herunterladen und im Temp-Ordner ablegen (zum Öffnen, Speichern, Kopieren)
	imageFile := func(ci *chatImage) (string, error) {
		mu.Lock()
		p, ok := files[ci.ID]
		mu.Unlock()
		if ok {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
		data, err := c.chatImageData(ci.ID)
		if err != nil {
			return "", err
		}
		p = filepath.Join(chatTempDir(), "Bild-"+ci.ID+"."+ci.ext())
		if err := os.WriteFile(p, data, 0o600); err != nil {
			return "", err
		}
		mu.Lock()
		files[ci.ID] = p
		mu.Unlock()
		return p, nil
	}
	loadThumbs := func(ctl *uiCtl) {
		mu.Lock()
		var todo []*chatImage
		for _, m := range list {
			if ci := m.Image; ci != nil && thumbs[ci.ID] == nil && !failed[ci.ID] && !loading[ci.ID] {
				loading[ci.ID] = true
				todo = append(todo, ci)
			}
		}
		mu.Unlock()
		for _, ci := range todo {
			go func(ci *chatImage) {
				sem <- struct{}{}
				defer func() { <-sem }()
				var th *uiImage
				p, err := imageFile(ci)
				if err == nil {
					var b []byte
					if b, err = os.ReadFile(p); err == nil {
						var img image.Image
						if img, _, err = decodeImageLimited(b); err == nil {
							k := math.Min(2, uiScale()) // so groß wie angezeigt (höchstens 300 × 240), nicht größer
							th = newUIImage(ci.ID, img, int(300*k), int(240*k))
						}
					}
				}
				mu.Lock()
				delete(loading, ci.ID)
				if th != nil {
					thumbs[ci.ID] = th
				} else {
					failed[ci.ID] = true
				}
				mu.Unlock()
				if !closed.Load() {
					rebuild(ctl)
				}
			}(ci)
		}
	}
	refresh := func(ctl *uiCtl) {
		l, err := c.chatHistory()
		if err != nil {
			return
		}
		mu.Lock()
		s := chatSignature(l)
		changed := s != sig
		sig = s
		if changed {
			list = chatVisible(l)
		}
		mu.Unlock()
		if changed {
			rebuild(ctl)
			loadThumbs(ctl)
		}
	}
	draft := func(d *uiDialog) string {
		if d.Input == nil {
			return ""
		}
		return d.Input.Value
	}
	updateSend := func(ctl *uiCtl) {
		mu.Lock()
		hasAtt := att != nil
		mu.Unlock()
		ctl.Update(func(d *uiDialog) {
			if b := d.button("send"); b != nil {
				b.Disabled = (strings.TrimSpace(draft(d)) == "" && !hasAtt) || sending.Load()
			}
		})
	}
	// Bild anhängen: vorbereiten (verkleinern) läuft im Hintergrund
	attach := func(ctl *uiCtl, label string, prep func() (*preparedImage, error)) {
		mu.Lock()
		if attBusy {
			mu.Unlock()
			return
		}
		attBusy = true
		mu.Unlock()
		ctl.Update(func(d *uiDialog) { d.Attach, d.Note = &uiAttach{Busy: true, Label: label}, "" })
		go func() {
			p, err := prep()
			var th *uiImage
			if err == nil {
				th = newUIImage("anhang", p.Img, 120, 120)
			}
			mu.Lock()
			attBusy = false
			if err == nil {
				att = p
			}
			mu.Unlock()
			ctl.Update(func(d *uiDialog) {
				if err != nil {
					d.Attach, d.Note = nil, err.Error()
					return
				}
				d.Attach = &uiAttach{Img: th, Label: fmt.Sprintf("%s · %d × %d · %s", label, p.W, p.H, sizeText(len(p.Data)))}
			})
			updateSend(ctl)
		}()
	}
	attachFile := func(ctl *uiCtl, path string) {
		if !isImageFile(path) {
			ctl.Update(func(d *uiDialog) { d.Note = errNoImage.Error() })
			return
		}
		attach(ctl, filepath.Base(path), func() (*preparedImage, error) { return loadImageFile(path) })
	}
	send := func(ctl *uiCtl, text string, img *preparedImage) {
		defer sending.Store(false)
		try := func(f func() error) (err error) {
			for i := 0; i < 3; i++ { // kurze Aussetzer überbrücken
				if err = f(); err == nil || errors.Is(err, errUnauthorized) {
					return err
				}
				time.Sleep(2 * time.Second)
			}
			return err
		}
		var err error
		if img != nil {
			err = try(func() error { return c.chatSendImage(img) })
		}
		if err == nil && text != "" {
			err = try(func() error { return c.chatSend(text) })
			if err != nil {
				img = nil // Bild ist schon angekommen
			}
		}
		if err != nil {
			mu.Lock()
			if img != nil && att == nil {
				att = img
			}
			restore := att
			mu.Unlock()
			ctl.Update(func(d *uiDialog) {
				if d.Input != nil && strings.TrimSpace(d.Input.Value) == "" {
					d.Input.Value = text
				}
				if restore != nil && d.Attach == nil {
					d.Attach = &uiAttach{Img: newUIImage("anhang", restore.Img, 120, 120), Label: "Noch nicht gesendet"}
				}
				d.Note = "Nicht gesendet: " + err.Error()
			})
			updateSend(ctl)
			return
		}
		ctl.Update(func(d *uiDialog) { d.Note = "Gesendet ✓ – Nicolaj bekommt deine Nachricht im Dashboard." })
		ctl.State(func(st *uiState) { st.Unstuck = false })
		refresh(ctl)
		updateSend(ctl)
	}
	openImage := func(ctl *uiCtl, m chatMsg) {
		if m.Image == nil {
			return
		}
		go func() {
			p, err := imageFile(m.Image)
			if err != nil {
				ctl.Update(func(d *uiDialog) { d.Note = "Bild konnte nicht geöffnet werden: " + err.Error() })
				return
			}
			shellOpen(p)
		}()
	}

	opts := uiOpts{Title: chatTitle, Topmost: incoming,
		OnClick: func(ctl *uiCtl, id string) bool {
			switch {
			case id == "input-changed":
				updateSend(ctl)
			case id == "send":
				if !sending.CompareAndSwap(false, true) {
					return false
				}
				mu.Lock()
				img := att
				busy := attBusy
				mu.Unlock()
				if busy {
					sending.Store(false)
					return false
				}
				var text string
				ctl.Update(func(d *uiDialog) {
					text = strings.TrimSpace(draft(d))
					if text == "" && img == nil {
						return
					}
					d.Input.Value = ""
					d.Attach = nil
					d.Note = "Wird gesendet …"
					if b := d.button("send"); b != nil {
						b.Disabled = true
					}
				})
				if text == "" && img == nil {
					sending.Store(false)
					return false
				}
				mu.Lock()
				att = nil
				mu.Unlock()
				go send(ctl, text, img)
			case id == "attach":
				if p, ok := pickImageFile(ctl.Hwnd()); ok {
					attachFile(ctl, p)
				}
			case id == "unattach":
				mu.Lock()
				att = nil
				mu.Unlock()
				ctl.Update(func(d *uiDialog) { d.Attach = nil })
				updateSend(ctl)
			case strings.HasPrefix(id, "img:"):
				if m, ok := msgAt(id); ok {
					openImage(ctl, m)
				}
			case id == "close":
				return true
			}
			return false
		},
		OnMenu: func(ctl *uiCtl, id string) []uiMenuItem {
			if !strings.HasPrefix(id, "msg:") && !strings.HasPrefix(id, "img:") {
				return nil
			}
			m, ok := msgAt(id)
			if !ok {
				return nil
			}
			var items []uiMenuItem
			if t := strings.TrimSpace(m.Text); t != "" {
				items = append(items, uiMenuItem{Label: "Text kopieren", Do: func() {
					setClipboard(ctl.Hwnd(), strings.ReplaceAll(t, "\n", "\r\n"))
					ctl.Update(func(d *uiDialog) { d.Note = "Text kopiert – mit Strg+V einfügen." })
				}})
			}
			if ci := m.Image; ci != nil {
				items = append(items,
					uiMenuItem{Label: "Bild öffnen", Do: func() { openImage(ctl, m) }},
					uiMenuItem{Label: "Bild kopieren", Do: func() {
						go func() {
							msg := "Bild kopiert – mit Strg+V einfügen."
							p, err := imageFile(ci)
							var b []byte
							var img image.Image
							if err == nil {
								if b, err = os.ReadFile(p); err == nil {
									img, _, err = decodeImageLimited(b)
								}
							}
							if err != nil || !setClipboardImage(ctl.Hwnd(), img) {
								msg = "Bild konnte nicht kopiert werden."
							}
							ctl.Update(func(d *uiDialog) { d.Note = msg })
						}()
					}},
					uiMenuItem{Label: "Bild speichern unter …", Do: func() {
						dst, ok := pickSavePath(ctl.Hwnd(), "Bild von Nicolaj "+m.At.Local().Format("2006-01-02 15-04")+"."+ci.ext(), ci.ext())
						if !ok {
							return
						}
						go func() {
							msg := "Gespeichert: " + filepath.Base(dst)
							p, err := imageFile(ci)
							var b []byte
							if err == nil {
								if b, err = os.ReadFile(p); err == nil {
									err = os.WriteFile(dst, b, 0o644)
								}
							}
							if err != nil {
								msg = "Nicht gespeichert: " + err.Error()
							}
							ctl.Update(func(d *uiDialog) { d.Note = msg })
						}()
					}})
			}
			return items
		},
		OnPaste: func(ctl *uiCtl) bool {
			img, paths, hasText := clipboardContent(ctl.Hwnd())
			if hasText {
				return false
			}
			if img != nil {
				attach(ctl, "Screenshot", func() (*preparedImage, error) { return prepareChatImage(img, false, 1) })
				return true
			}
			for _, p := range paths {
				if isImageFile(p) {
					attachFile(ctl, p)
					return true
				}
			}
			return false
		},
		OnDrop: func(ctl *uiCtl, paths []string) {
			for _, p := range paths {
				if isImageFile(p) {
					attachFile(ctl, p)
					return
				}
			}
			ctl.Update(func(d *uiDialog) { d.Note = "Hier lassen sich nur Bilder senden (PNG, JPG, GIF oder BMP)." })
		},
		Ready: func(ctl *uiCtl) {
			loadThumbs(ctl)
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
	_, uerr := runUI(dlgChat(chatToUI(list, thumbOf), "", note), opts)
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

func sizeText(n int) string {
	if n >= 1<<20 {
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	}
	return fmt.Sprintf("%d KB", max(1, n>>10))
}

// chatTempDir: Ordner für heruntergeladene Bilder (gehört der angemeldeten Person).
func chatTempDir() string {
	d := filepath.Join(os.TempDir(), "NicolajFernwartung")
	os.MkdirAll(d, 0o700)
	return d
}

// cleanChatTemp löscht Bilder, die älter als eine Woche sind.
func cleanChatTemp() {
	d := chatTempDir()
	entries, _ := os.ReadDir(d)
	for _, e := range entries {
		if info, err := e.Info(); err == nil && !e.IsDir() && time.Since(info.ModTime()) > 7*24*time.Hour {
			os.Remove(filepath.Join(d, e.Name()))
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
