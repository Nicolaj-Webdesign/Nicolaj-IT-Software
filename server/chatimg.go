package main

// Bilder im Chat (Screenshots, Fotos): liegen als Dateien unter data/chat-bilder/<Gerät>/,
// in der Datenbank steht nur, welche Nachricht welches Bild hat.

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // Formate registrieren
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	maxChatImageBytes = 10 << 20 // pro Bild
	maxChatImagePx    = 50_000_000
	maxChatImageDev   = 200 << 20 // Speicher pro Gerät, danach fliegen die ältesten Bilder raus
)

type ChatImage struct {
	ID   string `json:"id"`
	Type string `json:"type"` // png | jpeg | gif
	W    int    `json:"w"`
	H    int    `json:"h"`
	Size int64  `json:"size"`
}

var (
	reImgID = regexp.MustCompile(`^[0-9a-f]{16}$`)
	reDevID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

func (a *App) chatImageDir(devID string) string {
	return filepath.Join(a.dataDir, "chat-bilder", devID)
}

func (a *App) chatImagePath(devID string, img *ChatImage) string {
	ext := img.Type
	if ext == "jpeg" {
		ext = "jpg"
	}
	return filepath.Join(a.chatImageDir(devID), img.ID+"."+ext)
}

// readChatImage liest und prüft ein hochgeladenes Bild (nur echte PNG/JPEG/GIF in vernünftiger Größe).
func readChatImage(w http.ResponseWriter, r *http.Request) ([]byte, *ChatImage, error) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxChatImageBytes+1))
	if err != nil || len(body) > maxChatImageBytes {
		return nil, nil, errors.New("Das Bild ist zu groß (höchstens 10 MB).")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil || (format != "png" && format != "jpeg" && format != "gif") {
		return nil, nil, errors.New("Das ist kein Bild im Format PNG, JPG oder GIF.")
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 20000 || cfg.Height > 20000 || cfg.Width*cfg.Height > maxChatImagePx {
		return nil, nil, errors.New("Das Bild hat zu viele Pixel.")
	}
	return body, &ChatImage{ID: randHex(8), Type: format, W: cfg.Width, H: cfg.Height, Size: int64(len(body))}, nil
}

func (a *App) storeChatImage(devID string, data []byte, img *ChatImage) error {
	if !reDevID.MatchString(devID) {
		return errors.New("ungültiges Gerät")
	}
	dir := a.chatImageDir(devID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	p := a.chatImagePath(devID, img)
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// removeChatImages löscht alle Bilder eines Geräts (Gerät entfernt).
func (a *App) removeChatImages(devID string) {
	if reDevID.MatchString(devID) {
		os.RemoveAll(a.chatImageDir(devID))
	}
}

// pruneChatImages löscht Bilddateien, deren Nachricht es nicht mehr gibt, und hält den
// Speicher pro Gerät unter der Grenze (älteste zuerst; die Nachricht bleibt, ohne Bild).
func (a *App) pruneChatImages(devID string) {
	if !reDevID.MatchString(devID) {
		return
	}
	keep := map[string]bool{}
	var drop []*ChatImage
	a.store.Update(func(d *DBData) {
		var total int64
		list := d.Chats[devID]
		for i := len(list) - 1; i >= 0; i-- {
			m := list[i]
			if m.Image == nil {
				continue
			}
			total += m.Image.Size
			if total > maxChatImageDev {
				cp := *m.Image
				drop = append(drop, &cp)
				m.Image, m.ImageGone = nil, true
				continue
			}
			keep[m.Image.ID] = true
		}
	})
	for _, img := range drop {
		os.Remove(a.chatImagePath(devID, img))
	}
	entries, _ := os.ReadDir(a.chatImageDir(devID))
	for _, e := range entries {
		id, _, _ := strings.Cut(e.Name(), ".")
		if !keep[id] {
			if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > time.Minute { // gerade hochgeladene in Ruhe lassen
				os.Remove(filepath.Join(a.chatImageDir(devID), e.Name()))
			}
		}
	}
}

// serveChatImage liefert ein Bild aus – nur wenn es zu einer Nachricht dieses Geräts gehört.
func (a *App) serveChatImage(w http.ResponseWriter, r *http.Request, devID, imgID string) {
	if !reImgID.MatchString(imgID) || !reDevID.MatchString(devID) {
		http.NotFound(w, r)
		return
	}
	var img *ChatImage
	a.store.View(func(d *DBData) {
		for _, m := range d.Chats[devID] {
			if m.Image != nil && m.Image.ID == imgID {
				cp := *m.Image
				img = &cp
				return
			}
		}
	})
	if img == nil {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(a.chatImagePath(devID, img))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	h := w.Header()
	h.Set("Content-Type", "image/"+img.Type)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	h.Set("Cache-Control", "private, max-age=31536000, immutable")
	ext := img.Type
	if ext == "jpeg" {
		ext = "jpg"
	}
	h.Set("Content-Disposition", fmt.Sprintf(`inline; filename="Bild-%s.%s"`, img.ID[:8], ext))
	http.ServeContent(w, r, "", time.Time{}, f)
}

// ---------- PC-Seite ----------

func (a *App) agentChatImagePost(w http.ResponseWriter, r *http.Request, devID, role string) {
	data, img, err := readChatImage(w, r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	author := cleanName(r.Header.Get("X-Author"))
	if err := a.storeChatImage(devID, data, img); err != nil {
		writeErr(w, 500, "Das Bild konnte nicht gespeichert werden.")
		return
	}
	var msg *ChatMsg
	var devName string
	tooMany := false
	a.store.Update(func(d *DBData) {
		dev := d.Devices[devID]
		if dev == nil {
			return
		}
		devName = dev.Name
		n := 0
		for _, m := range d.Chats[devID] {
			if m.From == "pc" && time.Since(m.At) < 10*time.Minute {
				n++
			}
		}
		if n >= chatBurst {
			tooMany = true
			return
		}
		if author == "" {
			author = dev.Name
		}
		msg = &ChatMsg{ID: randHex(6), From: "pc", Author: author, Image: img, At: time.Now()}
		addChat(d, devID, msg)
	})
	if msg == nil {
		os.Remove(a.chatImagePath(devID, img))
		if tooMany {
			writeErr(w, 429, "Zu viele Nachrichten in kurzer Zeit. Bitte warte ein paar Minuten.")
		} else {
			writeErr(w, 404, "Gerät nicht gefunden.")
		}
		return
	}
	go a.pruneChatImages(devID)
	a.hub.Broadcast("chat", map[string]any{"deviceId": devID, "deviceName": devName, "message": chatView(msg)})
	a.push("Nachricht von "+devName, "📷 Hat dir ein Bild geschickt", "/admin/#/messages/"+devID, "chat-"+devID)
	writeJSON(w, 200, chatView(msg))
}

func (a *App) agentChatImageGet(w http.ResponseWriter, r *http.Request, devID, role string) {
	a.serveChatImage(w, r, devID, r.PathValue("img"))
}

// ---------- Dashboard ----------

func (a *App) apiDeviceChatImage(w http.ResponseWriter, r *http.Request, user string) {
	a.serveChatImage(w, r, r.PathValue("id"), r.PathValue("img"))
}

func (a *App) apiDeviceChatImagePost(w http.ResponseWriter, r *http.Request, user string) {
	id := r.PathValue("id")
	exists := false
	a.store.View(func(d *DBData) { exists = d.Devices[id] != nil })
	if !exists {
		writeErr(w, 404, "Gerät nicht gefunden.")
		return
	}
	data, img, err := readChatImage(w, r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if err := a.storeChatImage(id, data, img); err != nil {
		writeErr(w, 500, "Das Bild konnte nicht gespeichert werden.")
		return
	}
	var msg *ChatMsg
	var devName string
	a.store.Update(func(d *DBData) {
		dev := d.Devices[id]
		if dev == nil {
			return
		}
		devName = dev.Name
		for _, m := range d.Chats[id] {
			if m.From == "pc" {
				m.Read = true
			}
		}
		msg = &ChatMsg{ID: randHex(6), From: "admin", Author: user, Image: img, At: time.Now()}
		addChat(d, id, msg)
	})
	if msg == nil {
		os.Remove(a.chatImagePath(id, img))
		writeErr(w, 404, "Gerät nicht gefunden.")
		return
	}
	go a.pruneChatImages(id)
	delivered := a.hub.SendToAgent(id, "screen", "chat", chatView(msg))
	a.audit(r, user, id, "Bild gesendet", "an „"+devName+"“")
	a.hub.Broadcast("chat", map[string]any{"deviceId": id, "deviceName": devName, "message": chatView(msg)})
	writeJSON(w, 200, map[string]any{"message": chatView(msg), "delivered": delivered})
}
