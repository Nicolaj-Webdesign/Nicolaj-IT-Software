package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Beispiel aus RFC 8291, Abschnitt 5 / Anhang A.
func TestWebpushEncryptRFC8291(t *testing.T) {
	d := func(s string) []byte {
		b, err := b64dec(s)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	asPriv, err := ecdh.P256().NewPrivateKey(d("yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw"))
	if err != nil {
		t.Fatal(err)
	}
	if got := b64.EncodeToString(asPriv.PublicKey().Bytes()); got != "BP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A8" {
		t.Fatalf("öffentlicher Schlüssel: %s", got)
	}
	body, err := webpushEncrypt(d("V2hlbiBJIGdyb3cgdXAsIEkgd2FudCB0byBiZSBhIHdhdGVybWVsb24"),
		d("BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4"),
		d("BTBZMqHH6r4Tts7J_aSIgg"), asPriv, d("DGv6ra1nlYgDCS1FRnbzlw"))
	if err != nil {
		t.Fatal(err)
	}
	want := d("DGv6ra1nlYgDCS1FRnbzlwAAEABBBP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A_yl95bQpu6cVPTpK4Mqgkf1CXztLVBSt2Ks3oZwbuwXPXLWyouBWLVWGNWQexSgSxsj_Qulcy4a-fN")
	if !bytes.Equal(body, want) {
		t.Fatalf("Ergebnis weicht vom RFC ab:\n got %s\nwant %s", b64.EncodeToString(body), b64.EncodeToString(want))
	}
}

func TestVapidJWT(t *testing.T) {
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	e, _ := k.PublicKey.ECDH()
	keys := &vapidKeys{priv: k, pub: e.Bytes()}
	tok, err := keys.jwt("https://fcm.googleapis.com", "https://example.org")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatal("JWT hat nicht 3 Teile")
	}
	var claims map[string]any
	cb, _ := b64dec(parts[1])
	json.Unmarshal(cb, &claims)
	if claims["aud"] != "https://fcm.googleapis.com" || claims["sub"] != "https://example.org" {
		t.Fatalf("Claims: %v", claims)
	}
	sig, _ := b64dec(parts[2])
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	r, s := new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])
	if len(sig) != 64 || !ecdsa.Verify(&k.PublicKey, sum[:], r, s) {
		t.Fatal("Signatur ungültig")
	}
}

func TestPushEndpointAllowlist(t *testing.T) {
	ok := []string{"https://fcm.googleapis.com/fcm/send/abc", "https://updates.push.services.mozilla.com/wpush/v2/x",
		"https://wns2-par02p.notify.windows.com/w/?token=x", "https://web.push.apple.com/abc"}
	bad := []string{"http://fcm.googleapis.com/x", "https://evil.example/x", "https://notify.windows.com.evil.de/x",
		"https://169.254.169.254/latest", "https://user@fcm.googleapis.com/x"}
	for _, u := range ok {
		if _, v := validPushEndpoint(u); !v {
			t.Errorf("sollte erlaubt sein: %s", u)
		}
	}
	for _, u := range bad {
		if _, v := validPushEndpoint(u); v {
			t.Errorf("sollte abgelehnt werden: %s", u)
		}
	}
}

// Ganzer Weg: Server verschlüsselt + signiert, ein nachgebauter Push-Dienst prüft VAPID
// und entschlüsselt wie ein Browser (RFC 8291, Empfängerseite).
func TestSendPushEndToEnd(t *testing.T) {
	t.Setenv("NRMM_PUSH_ANY", "1")
	store, err := OpenStore(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatal(err)
	}
	a := &App{store: store, publicURL: "https://rmm.example.org"}
	vapidMu.Lock()
	vapidCache = nil
	vapidMu.Unlock()

	ua, _ := ecdh.P256().GenerateKey(rand.Reader)
	auth := make([]byte, 16)
	rand.Read(auth)
	got := make(chan string, 1)
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Encoding") != "aes128gcm" || r.Header.Get("TTL") == "" {
			t.Errorf("Header fehlen: %v", r.Header)
		}
		// VAPID prüfen
		h := strings.TrimPrefix(r.Header.Get("Authorization"), "vapid ")
		var tok, k string
		for _, p := range strings.Split(h, ", ") {
			if v, ok := strings.CutPrefix(p, "t="); ok {
				tok = v
			} else if v, ok := strings.CutPrefix(p, "k="); ok {
				k = v
			}
		}
		kb, _ := b64dec(k)
		x, y := elliptic.Unmarshal(elliptic.P256(), kb) //nolint:staticcheck // nur im Test
		pub := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}
		parts := strings.Split(tok, ".")
		sig, _ := b64dec(parts[2])
		sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
		if x == nil || !ecdsa.Verify(pub, sum[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
			t.Error("VAPID-Signatur ungültig")
		}
		var cl map[string]any
		cb, _ := b64dec(parts[1])
		json.Unmarshal(cb, &cl)
		if cl["aud"] != "https://"+r.Host || cl["sub"] != "https://rmm.example.org" {
			t.Errorf("Claims: %v", cl)
		}
		// Entschlüsseln
		body, _ := io.ReadAll(r.Body)
		salt, idlen := body[:16], int(body[20])
		asPub, err := ecdh.P256().NewPublicKey(body[21 : 21+idlen])
		if err != nil {
			t.Error(err)
			return
		}
		secret, _ := ua.ECDH(asPub)
		prkKey, _ := hkdf.Extract(sha256.New, secret, auth)
		ikm, _ := hkdf.Expand(sha256.New, prkKey, "WebPush: info\x00"+string(ua.PublicKey().Bytes())+string(asPub.Bytes()), 32)
		prk, _ := hkdf.Extract(sha256.New, ikm, salt)
		cek, _ := hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
		nonce, _ := hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)
		blk, _ := aes.NewCipher(cek)
		gcm, _ := cipher.NewGCM(blk)
		plain, err := gcm.Open(nil, nonce, body[21+idlen:], nil)
		if err != nil {
			t.Error("Entschlüsseln fehlgeschlagen:", err)
			return
		}
		got <- string(bytes.TrimSuffix(plain, []byte{2}))
		w.WriteHeader(201)
	}))
	defer ts.Close()
	old := pushClient
	pushClient = ts.Client()
	defer func() { pushClient = old }()

	sub := PushSub{ID: "x", Endpoint: ts.URL + "/push/abc", P256dh: b64.EncodeToString(ua.PublicKey().Bytes()), Auth: b64.EncodeToString(auth), Label: "Test"}
	if code, err := a.sendPush(sub, []byte(`{"title":"Hallo Oma"}`)); err != nil || code != 201 {
		t.Fatalf("sendPush: %d %v", code, err)
	}
	if s := <-got; s != `{"title":"Hallo Oma"}` {
		t.Fatalf("Inhalt: %q", s)
	}
	// Schlüssel bleibt nach Neustart gleich
	k1, _ := a.vapid()
	vapidMu.Lock()
	vapidCache = nil
	vapidMu.Unlock()
	k2, _ := a.vapid()
	if !bytes.Equal(k1.pub, k2.pub) {
		t.Fatal("VAPID-Schlüssel wurde nicht gespeichert")
	}
	// Abo weg (410) → wird entfernt
	gone := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(410) }))
	defer gone.Close()
	pushClient = gone.Client()
	store.Update(func(d *DBData) {
		d.PushSubs = []*PushSub{{ID: "g", Endpoint: gone.URL + "/x", P256dh: sub.P256dh, Auth: sub.Auth, Label: "Alt"}}
	})
	a.push("t", "b", "/admin/", "chat-x")
	for i := 0; i < 50; i++ {
		n := 1
		store.View(func(d *DBData) { n = len(d.PushSubs) })
		if n == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("abgelaufenes Abo wurde nicht entfernt")
}
