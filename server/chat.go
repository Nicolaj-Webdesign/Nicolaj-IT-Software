package main

// Nachrichten zwischen der Person am PC und dir: Am PC schreibt man im Fenster
// „Nachricht an Nicolaj“, du antwortest im Dashboard – die Antwort erscheint am PC.

import (
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"
)

const (
	maxChatPerDevice = 300
	maxChatText      = 2000
	chatBurst        = 30 // höchstens so viele Nachrichten vom PC in 10 Minuten
)

func cleanChatText(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.ReplaceAll(s, "\r\n", "\n"))
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > maxChatText {
		s = string(r[:maxChatText])
	}
	return s
}

func chatView(m *ChatMsg) map[string]any {
	v := map[string]any{"id": m.ID, "from": m.From, "author": m.Author, "text": m.Text, "at": m.At, "read": m.Read}
	if m.Image != nil {
		v["image"] = map[string]any{"id": m.Image.ID, "w": m.Image.W, "h": m.Image.H, "type": m.Image.Type}
	} else if m.ImageGone {
		v["imageGone"] = true
	}
	return v
}

func addChat(d *DBData, devID string, m *ChatMsg) {
	if d.Chats == nil {
		d.Chats = map[string][]*ChatMsg{}
	}
	list := append(d.Chats[devID], m)
	if len(list) > maxChatPerDevice {
		list = list[len(list)-maxChatPerDevice:]
	}
	d.Chats[devID] = list
}

// ---------- PC-Seite ----------

func (a *App) agentChatGet(w http.ResponseWriter, r *http.Request, devID, role string) {
	var out []map[string]any
	marked := false
	a.store.Update(func(d *DBData) {
		list := d.Chats[devID]
		if len(list) > 100 {
			list = list[len(list)-100:]
		}
		for _, m := range list {
			if m.From == "admin" && !m.Read {
				m.Read, marked = true, true
			}
			out = append(out, chatView(m))
		}
	})
	if marked {
		a.hub.Broadcast("chat-read", map[string]any{"deviceId": devID, "by": "pc"})
	}
	if out == nil {
		out = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"messages": out})
}

func (a *App) agentChatPost(w http.ResponseWriter, r *http.Request, devID, role string) {
	var req struct{ Text, Author string }
	if !readJSON(w, r, &req, 16384) {
		return
	}
	text := cleanChatText(req.Text)
	if text == "" {
		writeErr(w, 400, "Die Nachricht ist leer.")
		return
	}
	author := cleanName(req.Author)
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
		msg = &ChatMsg{ID: randHex(6), From: "pc", Author: author, Text: text, At: time.Now()}
		addChat(d, devID, msg)
	})
	if tooMany {
		writeErr(w, 429, "Zu viele Nachrichten in kurzer Zeit. Bitte warte ein paar Minuten.")
		return
	}
	if msg == nil {
		writeErr(w, 404, "Gerät nicht gefunden.")
		return
	}
	go a.pruneChatImages(devID)
	a.hub.Broadcast("chat", map[string]any{"deviceId": devID, "deviceName": devName, "message": chatView(msg)})
	a.push("Nachricht von "+devName, text, "/admin/#/messages/"+devID, "chat-"+devID)
	writeJSON(w, 200, chatView(msg))
}

// pushUnreadChat: Verbindet sich der Bildschirm-Helfer, zeigt er ungelesene Antworten sofort an.
func (a *App) pushUnreadChat(devID string, conn *Conn) {
	var last *ChatMsg
	a.store.View(func(d *DBData) {
		for _, m := range d.Chats[devID] {
			if m.From == "admin" && !m.Read {
				cp := *m
				last = &cp
			}
		}
	})
	if last != nil {
		conn.Send(sseMsg("chat", chatView(last)))
	}
}

// ---------- Dashboard ----------

func (a *App) apiChats(w http.ResponseWriter, r *http.Request, user string) {
	type conv struct {
		DeviceID string         `json:"deviceId"`
		Name     string         `json:"name"`
		Last     map[string]any `json:"last"`
		Unread   int            `json:"unread"`
		at       time.Time
	}
	var list []conv
	a.store.View(func(d *DBData) {
		for id, msgs := range d.Chats {
			dev := d.Devices[id]
			if dev == nil || len(msgs) == 0 {
				continue
			}
			c := conv{DeviceID: id, Name: dev.Name, Last: chatView(msgs[len(msgs)-1]), at: msgs[len(msgs)-1].At}
			for _, m := range msgs {
				if m.From == "pc" && !m.Read {
					c.Unread++
				}
			}
			list = append(list, c)
		}
	})
	sort.Slice(list, func(i, j int) bool { return list[i].at.After(list[j].at) })
	if list == nil {
		list = []conv{}
	}
	writeJSON(w, 200, list)
}

func (a *App) apiDeviceChat(w http.ResponseWriter, r *http.Request, user string) {
	id := r.PathValue("id")
	var out []map[string]any
	exists, marked := false, false
	a.store.Update(func(d *DBData) {
		if d.Devices[id] == nil {
			return
		}
		exists = true
		for _, m := range d.Chats[id] {
			if m.From == "pc" && !m.Read {
				m.Read, marked = true, true
			}
			out = append(out, chatView(m))
		}
	})
	if !exists {
		writeErr(w, 404, "Gerät nicht gefunden.")
		return
	}
	if marked {
		a.hub.Broadcast("chat-read", map[string]any{"deviceId": id, "by": "admin"})
	}
	if out == nil {
		out = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"messages": out})
}

func (a *App) apiDeviceChatPost(w http.ResponseWriter, r *http.Request, user string) {
	id := r.PathValue("id")
	var req struct{ Text string }
	if !readJSON(w, r, &req, 16384) {
		return
	}
	text := cleanChatText(req.Text)
	if text == "" {
		writeErr(w, 400, "Die Nachricht ist leer.")
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
		for _, m := range d.Chats[id] { // wer antwortet, hat gelesen
			if m.From == "pc" {
				m.Read = true
			}
		}
		msg = &ChatMsg{ID: randHex(6), From: "admin", Author: user, Text: text, At: time.Now()}
		addChat(d, id, msg)
	})
	if msg == nil {
		writeErr(w, 404, "Gerät nicht gefunden.")
		return
	}
	go a.pruneChatImages(id)
	delivered := a.hub.SendToAgent(id, "screen", "chat", chatView(msg))
	a.audit(r, user, id, "Nachricht gesendet", "an „"+devName+"“")
	a.hub.Broadcast("chat", map[string]any{"deviceId": id, "deviceName": devName, "message": chatView(msg)})
	writeJSON(w, 200, map[string]any{"message": chatView(msg), "delivered": delivered})
}
