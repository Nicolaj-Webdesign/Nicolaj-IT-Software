package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = byte(i * 7)
	}
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

func TestChatImages(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenStore(filepath.Join(dir, "db.json"))
	if err != nil {
		t.Fatal(err)
	}
	a := &App{store: store, dataDir: dir}
	store.Update(func(d *DBData) {
		d.Devices = map[string]*Device{"dev1": {ID: "dev1", Name: "Oma"}, "dev2": {ID: "dev2", Name: "Tante"}}
	})

	// kein Bild / kaputtes Bild wird abgelehnt
	for _, body := range [][]byte{[]byte("<svg onload=alert(1)>"), []byte("GIF89a....")} {
		r := httptest.NewRequest("POST", "/", bytes.NewReader(body))
		if _, _, err := readChatImage(httptest.NewRecorder(), r); err == nil {
			t.Errorf("sollte abgelehnt werden: %q", body)
		}
	}
	data := testPNG(64, 48)
	_, img, err := readChatImage(httptest.NewRecorder(), httptest.NewRequest("POST", "/", bytes.NewReader(data)))
	if err != nil || img.W != 64 || img.H != 48 || img.Type != "png" {
		t.Fatalf("readChatImage: %v %+v", err, img)
	}
	if err := a.storeChatImage("dev1", data, img); err != nil {
		t.Fatal(err)
	}
	store.Update(func(d *DBData) { addChat(d, "dev1", &ChatMsg{ID: "m1", From: "pc", Image: img, At: time.Now()}) })

	get := func(dev, id string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		a.serveChatImage(w, httptest.NewRequest("GET", "/", nil), dev, id)
		return w
	}
	if w := get("dev1", img.ID); w.Code != 200 || !bytes.Equal(w.Body.Bytes(), data) || w.Header().Get("Content-Type") != "image/png" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("ausliefern: %d %v", w.Code, w.Header())
	}
	if w := get("dev2", img.ID); w.Code != 404 { // fremdes Gerät
		t.Fatalf("fremdes Gerät bekommt Bild: %d", w.Code)
	}
	if w := get("dev1", "../../db.json"); w.Code != 404 {
		t.Fatalf("Pfad-Trick: %d", w.Code)
	}
	if v := chatView(&ChatMsg{Image: img}); v["image"] == nil {
		t.Fatal("chatView ohne Bild")
	}

	// verwaiste Datei (Nachricht weg) wird aufgeräumt
	orphan := filepath.Join(a.chatImageDir("dev1"), "0123456789abcdef.png")
	os.WriteFile(orphan, data, 0o600)
	old := time.Now().Add(-time.Hour)
	os.Chtimes(orphan, old, old)
	a.pruneChatImages("dev1")
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatal("verwaiste Datei nicht gelöscht")
	}
	if _, err := os.Stat(a.chatImagePath("dev1", img)); err != nil {
		t.Fatal("benutztes Bild gelöscht")
	}
	a.removeChatImages("dev1")
	if _, err := os.Stat(a.chatImageDir("dev1")); !os.IsNotExist(err) {
		t.Fatal("Ordner nicht entfernt")
	}
	sentinel := filepath.Join(dir, "wichtig.txt")
	os.WriteFile(sentinel, []byte("x"), 0o600)
	a.removeChatImages("..") // darf nichts tun
	a.removeChatImages("../..")
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatal("Datei außerhalb gelöscht!")
	}
}
