package main

// Web-Push: Benachrichtigungen aufs Handy bzw. den Desktop, auch wenn das Dashboard zu ist.
// Verschlüsselung nach RFC 8291 (aes128gcm), Anmeldung beim Push-Dienst per VAPID (RFC 8292).
// Nur Standardbibliothek.

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
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type PushSub struct {
	ID       string    `json:"id"`
	Endpoint string    `json:"endpoint"`
	P256dh   string    `json:"p256dh"`
	Auth     string    `json:"auth"`
	User     string    `json:"user"`
	Label    string    `json:"label"`
	Created  time.Time `json:"created"`
	LastOK   time.Time `json:"lastOk,omitempty"`
}

var b64 = base64.RawURLEncoding

func b64dec(s string) ([]byte, error) {
	s = strings.TrimRight(strings.TrimSpace(s), "=")
	s = strings.NewReplacer("+", "-", "/", "_").Replace(s)
	return b64.DecodeString(s)
}

// ---------- Verschlüsselung (RFC 8291) ----------

func webpushEncrypt(payload, uaPublic, authSecret []byte, as *ecdh.PrivateKey, salt []byte) ([]byte, error) {
	ua, err := ecdh.P256().NewPublicKey(uaPublic)
	if err != nil {
		return nil, fmt.Errorf("ungültiger Browser-Schlüssel: %w", err)
	}
	if len(authSecret) != 16 {
		return nil, errors.New("ungültiges Browser-Geheimnis")
	}
	secret, err := as.ECDH(ua)
	if err != nil {
		return nil, err
	}
	asPub := as.PublicKey().Bytes()
	prkKey, err := hkdf.Extract(sha256.New, secret, authSecret)
	if err != nil {
		return nil, err
	}
	keyInfo := "WebPush: info\x00" + string(uaPublic) + string(asPub)
	ikm, err := hkdf.Expand(sha256.New, prkKey, keyInfo, 32)
	if err != nil {
		return nil, err
	}
	prk, err := hkdf.Extract(sha256.New, ikm, salt)
	if err != nil {
		return nil, err
	}
	cek, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, err
	}
	nonce, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plain := append(append([]byte{}, payload...), 0x02) // letzter (einziger) Datensatz
	var out bytes.Buffer
	out.Write(salt)
	binary.Write(&out, binary.BigEndian, uint32(4096))
	out.WriteByte(byte(len(asPub)))
	out.Write(asPub)
	out.Write(gcm.Seal(nil, nonce, plain, nil))
	return out.Bytes(), nil
}

// ---------- VAPID ----------

type vapidKeys struct {
	priv *ecdsa.PrivateKey
	pub  []byte // 65 Byte, unkomprimiert
}

var (
	vapidMu    sync.Mutex
	vapidCache *vapidKeys
)

// vapid lädt den Server-Schlüssel (oder erzeugt ihn beim ersten Mal).
func (a *App) vapid() (*vapidKeys, error) {
	vapidMu.Lock()
	defer vapidMu.Unlock()
	if vapidCache != nil {
		return vapidCache, nil
	}
	var stored string
	a.store.View(func(d *DBData) {
		if d.Settings != nil {
			stored = d.Settings.VapidKey
		}
	})
	var priv *ecdsa.PrivateKey
	if stored != "" {
		der, err := b64dec(stored)
		if err == nil {
			if k, err := x509.ParsePKCS8PrivateKey(der); err == nil {
				priv, _ = k.(*ecdsa.PrivateKey)
			}
		}
	}
	if priv == nil {
		k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, err
		}
		der, err := x509.MarshalPKCS8PrivateKey(k)
		if err != nil {
			return nil, err
		}
		a.store.Update(func(d *DBData) {
			if d.Settings == nil {
				d.Settings = &Settings{}
			}
			d.Settings.VapidKey = b64.EncodeToString(der)
		})
		priv = k
	}
	e, err := priv.PublicKey.ECDH()
	if err != nil {
		return nil, err
	}
	vapidCache = &vapidKeys{priv: priv, pub: e.Bytes()}
	return vapidCache, nil
}

func (k *vapidKeys) jwt(aud, sub string) (string, error) {
	h := b64.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`))
	c, _ := json.Marshal(map[string]any{"aud": aud, "exp": time.Now().Add(12 * time.Hour).Unix(), "sub": sub})
	msg := h + "." + b64.EncodeToString(c)
	sum := sha256.Sum256([]byte(msg))
	r, s, err := ecdsa.Sign(rand.Reader, k.priv, sum[:])
	if err != nil {
		return "", err
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return msg + "." + b64.EncodeToString(sig), nil
}

// Nur echte Push-Dienste anschreiben (kein Weiterleiten an beliebige Adressen).
var pushHosts = []string{"fcm.googleapis.com", "updates.push.services.mozilla.com", "push.services.mozilla.com",
	".notify.windows.com", "web.push.apple.com", ".push.apple.com"}

func validPushEndpoint(raw string) (*url.URL, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return nil, false
	}
	if os.Getenv("NRMM_PUSH_ANY") == "1" { // nur für Tests
		return u, true
	}
	h := strings.ToLower(u.Hostname())
	for _, p := range pushHosts {
		if h == strings.TrimPrefix(p, ".") || (strings.HasPrefix(p, ".") && strings.HasSuffix(h, p)) {
			return u, true
		}
	}
	return nil, false
}

var pushClient = &http.Client{Timeout: 15 * time.Second}

func (a *App) pushContact() string {
	if strings.HasPrefix(a.publicURL, "https://") {
		return a.publicURL
	}
	return "https://nicolaj-webdesign.de"
}

func (a *App) sendPush(sub PushSub, payload []byte) (int, error) {
	u, ok := validPushEndpoint(sub.Endpoint)
	if !ok {
		return 0, errors.New("unbekannter Push-Dienst")
	}
	keys, err := a.vapid()
	if err != nil {
		return 0, err
	}
	ua, err := b64dec(sub.P256dh)
	if err != nil {
		return 0, err
	}
	auth, err := b64dec(sub.Auth)
	if err != nil {
		return 0, err
	}
	eph, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return 0, err
	}
	salt := make([]byte, 16)
	rand.Read(salt)
	body, err := webpushEncrypt(payload, ua, auth, eph, salt)
	if err != nil {
		return 0, err
	}
	tok, err := keys.jwt(u.Scheme+"://"+u.Host, a.pushContact())
	if err != nil {
		return 0, err
	}
	req, _ := http.NewRequest("POST", sub.Endpoint, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("TTL", "86400")
	req.Header.Set("Urgency", "high")
	req.Header.Set("Authorization", "vapid t="+tok+", k="+b64.EncodeToString(keys.pub))
	resp, err := pushClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	return resp.StatusCode, nil
}

// push schickt eine Benachrichtigung an alle angemeldeten Geräte (im Hintergrund).
func (a *App) push(title, body, href, tag string) {
	var subs []PushSub
	a.store.View(func(d *DBData) {
		for _, s := range d.PushSubs {
			subs = append(subs, *s)
		}
	})
	if len(subs) == 0 {
		return
	}
	if r := []rune(body); len(r) > 240 {
		body = string(r[:240]) + " …"
	}
	payload, _ := json.Marshal(map[string]string{"title": title, "body": body, "href": href, "tag": tag})
	for _, s := range subs {
		go func(s PushSub) {
			code, err := a.sendPush(s, payload)
			a.store.Update(func(d *DBData) {
				for i, x := range d.PushSubs {
					if x.ID != s.ID {
						continue
					}
					if code == 404 || code == 410 { // Abo im Browser beendet
						d.PushSubs = append(d.PushSubs[:i], d.PushSubs[i+1:]...)
					} else if err == nil {
						x.LastOK = time.Now()
					}
					return
				}
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Push an %s: %v\n", s.Label, err)
			}
		}(s)
	}
}

// ---------- Dashboard ----------

func (a *App) apiPush(w http.ResponseWriter, r *http.Request, user string) {
	keys, err := a.vapid()
	if err != nil {
		writeErr(w, 500, "Push-Schlüssel nicht verfügbar.")
		return
	}
	var list []map[string]any
	a.store.View(func(d *DBData) {
		for _, s := range d.PushSubs {
			host := ""
			if u, err := url.Parse(s.Endpoint); err == nil {
				host = u.Hostname()
			}
			list = append(list, map[string]any{"id": s.ID, "label": s.Label, "user": s.User, "created": s.Created,
				"lastOk": s.LastOK, "service": host, "endpoint": s.Endpoint})
		}
	})
	if list == nil {
		list = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"publicKey": b64.EncodeToString(keys.pub), "subscriptions": list})
}

func (a *App) apiPushSubscribe(w http.ResponseWriter, r *http.Request, user string) {
	var req struct {
		Endpoint string `json:"endpoint"`
		Keys     struct {
			P256dh string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
		Label string `json:"label"`
	}
	if !readJSON(w, r, &req, 8192) {
		return
	}
	if _, ok := validPushEndpoint(req.Endpoint); !ok {
		writeErr(w, 400, "Dieser Browser nutzt einen unbekannten Push-Dienst.")
		return
	}
	if p, err := b64dec(req.Keys.P256dh); err != nil || len(p) != 65 {
		writeErr(w, 400, "Ungültiger Schlüssel vom Browser.")
		return
	}
	if au, err := b64dec(req.Keys.Auth); err != nil || len(au) != 16 {
		writeErr(w, 400, "Ungültiges Geheimnis vom Browser.")
		return
	}
	label := cleanName(req.Label)
	if label == "" {
		label = "Browser"
	}
	var sub *PushSub
	full, created := false, false
	a.store.Update(func(d *DBData) {
		for _, s := range d.PushSubs {
			if s.Endpoint == req.Endpoint {
				s.P256dh, s.Auth, s.Label, s.User = req.Keys.P256dh, req.Keys.Auth, label, user
				sub = s
				return
			}
		}
		if len(d.PushSubs) >= 20 {
			full = true
			return
		}
		sub = &PushSub{ID: randHex(6), Endpoint: req.Endpoint, P256dh: req.Keys.P256dh, Auth: req.Keys.Auth,
			User: user, Label: label, Created: time.Now()}
		d.PushSubs = append(d.PushSubs, sub)
		created = true
	})
	if full {
		writeErr(w, 409, "Es sind schon 20 Geräte für Benachrichtigungen angemeldet.")
		return
	}
	if created {
		a.audit(r, user, "", "Push-Benachrichtigungen", "eingeschaltet für „"+label+"“")
	}
	writeJSON(w, 200, map[string]any{"id": sub.ID})
}

func (a *App) apiPushDelete(w http.ResponseWriter, r *http.Request, user string) {
	id := r.PathValue("id")
	ok := false
	a.store.Update(func(d *DBData) {
		for i, s := range d.PushSubs {
			if s.ID == id {
				d.PushSubs = append(d.PushSubs[:i], d.PushSubs[i+1:]...)
				ok = true
				return
			}
		}
	})
	if !ok {
		writeErr(w, 404, "Nicht gefunden.")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *App) apiPushTest(w http.ResponseWriter, r *http.Request, user string) {
	var subs []PushSub
	a.store.View(func(d *DBData) {
		for _, s := range d.PushSubs {
			subs = append(subs, *s)
		}
	})
	if len(subs) == 0 {
		writeErr(w, 409, "Noch kein Gerät für Benachrichtigungen angemeldet.")
		return
	}
	payload, _ := json.Marshal(map[string]string{"title": "Test von Nicolaj Fernwartung",
		"body": "So sieht es aus, wenn dir jemand schreibt.", "href": "/admin/#/messages", "tag": "test"})
	type res struct {
		Label string `json:"label"`
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	out := make([]res, len(subs))
	var wg sync.WaitGroup
	for i, s := range subs {
		wg.Add(1)
		go func(i int, s PushSub) {
			defer wg.Done()
			_, err := a.sendPush(s, payload)
			out[i] = res{Label: s.Label, OK: err == nil}
			if err != nil {
				out[i].Error = err.Error()
			}
		}(i, s)
	}
	wg.Wait()
	writeJSON(w, 200, out)
}
