package main

// Nachrichten zwischen der Person am PC und Nicolaj (plattformunabhängiger Teil).

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"
)

type chatMsg struct {
	ID        string     `json:"id"`
	From      string     `json:"from"` // "pc" oder "admin"
	Author    string     `json:"author"`
	Text      string     `json:"text"`
	At        time.Time  `json:"at"`
	Read      bool       `json:"read"`
	Image     *chatImage `json:"image,omitempty"`
	ImageGone bool       `json:"imageGone,omitempty"`
}

type chatImage struct {
	ID   string `json:"id"`
	W    int    `json:"w"`
	H    int    `json:"h"`
	Type string `json:"type"`
}

func (ci *chatImage) ext() string {
	if ci.Type == "jpeg" {
		return "jpg"
	}
	if ci.Type == "gif" {
		return "gif"
	}
	return "png"
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

func (c *Client) chatSendImage(p *preparedImage) error {
	req, err := c.newReq(context.Background(), "POST", "/api/agent/chat/image", bytes.NewReader(p.Data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", p.Mime)
	req.Header.Set("X-Author", os.Getenv("USERNAME"))
	resp, err := c.api.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return errUnauthorized
	}
	if resp.StatusCode >= 300 {
		return apiError(resp)
	}
	io.Copy(io.Discard, resp.Body)
	return nil
}

// chatImageData lädt ein Bild aus dem Chat (höchstens 12 MB).
func (c *Client) chatImageData(id string) ([]byte, error) {
	req, err := c.newReq(context.Background(), "GET", "/api/agent/chat/image/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.api.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return nil, errUnauthorized
	}
	if resp.StatusCode >= 300 {
		return nil, apiError(resp)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 12<<20+1))
	if err == nil && len(b) > 12<<20 {
		err = errors.New("Bild zu groß")
	}
	return b, err
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

// chatToUI baut die Anzeige; thumb liefert geladene Vorschaubilder (nil = lädt noch, failed = Fehler).
func chatToUI(msgs []chatMsg, thumb func(id string) (img *uiImage, failed bool)) []uiChatMsg {
	now := time.Now()
	out := make([]uiChatMsg, 0, len(msgs))
	for _, m := range msgs {
		meta := chatTime(m.At, now)
		if m.From == "pc" && m.Read {
			meta += " · gelesen"
		}
		u := uiChatMsg{Mine: m.From == "pc", Text: m.Text, Meta: meta}
		switch {
		case m.Image != nil:
			u.HasImg, u.ImgW, u.ImgH = true, m.Image.W, m.Image.H
			if thumb != nil {
				img, failed := thumb(m.Image.ID)
				u.Img = img
				if failed {
					u.ImgNote = "Bild konnte nicht geladen werden"
				} else if img == nil {
					u.ImgNote = "Bild wird geladen …"
				}
			}
		case m.ImageGone:
			u.HasImg, u.ImgW, u.ImgH, u.ImgNote = true, 4, 3, "Bild nicht mehr verfügbar"
		}
		out = append(out, u)
	}
	return out
}

// chatVisible: nur Nachrichten, die etwas zeigen (Text oder Bild).
func chatVisible(msgs []chatMsg) []chatMsg {
	out := make([]chatMsg, 0, len(msgs))
	for _, m := range msgs {
		if m.Image != nil || m.ImageGone || strings.TrimSpace(m.Text) != "" {
			out = append(out, m)
		}
	}
	return out
}

// chatSignature erkennt, ob sich am Verlauf etwas geändert hat.
func chatSignature(msgs []chatMsg) string {
	var b strings.Builder
	for _, m := range msgs {
		fmt.Fprintf(&b, "%s%v%v;", m.ID, m.Read, m.Image != nil)
	}
	return b.String()
}
