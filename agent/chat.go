package main

// Nachrichten zwischen der Person am PC und Nicolaj (plattformunabhängiger Teil).

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type chatMsg struct {
	ID     string    `json:"id"`
	From   string    `json:"from"` // "pc" oder "admin"
	Author string    `json:"author"`
	Text   string    `json:"text"`
	At     time.Time `json:"at"`
	Read   bool      `json:"read"`
}

func (c *Client) chatHistory() ([]chatMsg, error) {
	var res struct {
		Messages []chatMsg `json:"messages"`
	}
	err := c.getJSON("/api/agent/chat", &res)
	return res.Messages, err
}

func (c *Client) chatSend(text string) error {
	author := os.Getenv("USERNAME")
	return c.postJSON("/api/agent/chat", map[string]string{"text": text, "author": author}, nil)
}

func chatTime(t, now time.Time) string {
	t = t.Local()
	switch {
	case now.Sub(t) < time.Minute:
		return "gerade eben"
	case sameDay(t, now):
		return t.Format("15:04")
	case sameDay(t, now.AddDate(0, 0, -1)):
		return "gestern " + t.Format("15:04")
	}
	return t.Format("02.01. 15:04")
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func chatToUI(msgs []chatMsg) []uiChatMsg {
	now := time.Now()
	out := make([]uiChatMsg, 0, len(msgs))
	for _, m := range msgs {
		meta := chatTime(m.At, now)
		if m.From == "pc" && m.Read {
			meta += " · gelesen"
		}
		out = append(out, uiChatMsg{Mine: m.From == "pc", Text: m.Text, Meta: meta})
	}
	return out
}

// chatSignature erkennt, ob sich am Verlauf etwas geändert hat.
func chatSignature(msgs []chatMsg) string {
	var b strings.Builder
	for _, m := range msgs {
		fmt.Fprintf(&b, "%s%v;", m.ID, m.Read)
	}
	return b.String()
}
