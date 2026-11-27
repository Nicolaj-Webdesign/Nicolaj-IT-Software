package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionStaysLoggedIn(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatal(err)
	}
	a := &App{store: store}
	store.Update(func(d *DBData) { d.Admins = append(d.Admins, &Admin{Username: "nicolaj"}) })

	req := func(c *http.Cookie) *http.Request {
		r := httptest.NewRequest("GET", "/api/state", nil)
		r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0 Safari/537.36 Edg/131.0")
		if c != nil {
			r.AddCookie(c)
		}
		return r
	}
	w := httptest.NewRecorder()
	a.createSession(w, req(nil), "nicolaj")
	ck := w.Result().Cookies()[0]
	if ck.MaxAge != int((120 * 24 * time.Hour).Seconds()) {
		t.Fatalf("Cookie hält nicht 120 Tage: %d", ck.MaxAge)
	}
	// alte Sitzung (wie vor dem Update: 12 Stunden) wird bei Nutzung verlängert
	store.Update(func(d *DBData) { d.Sessions[0].Expires = time.Now().Add(2 * time.Hour) })
	w = httptest.NewRecorder()
	if u, _ := a.session(w, req(ck)); u != "nicolaj" {
		t.Fatal("nicht angemeldet")
	}
	if len(w.Result().Cookies()) != 1 || w.Result().Cookies()[0].Value != ck.Value {
		t.Fatal("Cookie wurde nicht erneuert")
	}
	var exp time.Time
	var agent string
	store.View(func(d *DBData) { exp, agent = d.Sessions[0].Expires, d.Sessions[0].Agent })
	if time.Until(exp) < 119*24*time.Hour {
		t.Fatalf("Ablauf nicht verlängert: %v", exp)
	}
	if agent != "Windows · Edge" {
		t.Fatalf("Gerät: %q", agent)
	}
	// gleich danach: kein neues Cookie (nur einmal am Tag)
	w = httptest.NewRecorder()
	a.session(w, req(ck))
	if len(w.Result().Cookies()) != 0 {
		t.Fatal("Cookie wird bei jeder Anfrage neu gesetzt")
	}
	// abgelaufen -> abgemeldet
	store.Update(func(d *DBData) { d.Sessions[0].Expires = time.Now().Add(-time.Minute) })
	if u, _ := a.session(nil, req(ck)); u != "" {
		t.Fatal("abgelaufene Sitzung gilt noch")
	}
	// andere Geräte abmelden behält die eigene
	store.Update(func(d *DBData) {
		d.Sessions = []*SessionRec{{TokenHash: sha256Hex("a"), Username: "nicolaj", Expires: time.Now().Add(time.Hour)},
			{TokenHash: sha256Hex("b"), Username: "nicolaj", Expires: time.Now().Add(time.Hour)},
			{TokenHash: sha256Hex("c"), Username: "jemand", Expires: time.Now().Add(time.Hour)}}
		if n := dropSessions(d, "nicolaj", d.Sessions[0].SID()); n != 1 || len(d.Sessions) != 2 {
			t.Fatalf("dropSessions: %d, übrig %d", n, len(d.Sessions))
		}
	})
}
