package main

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	pbkdf2Iter    = 210000
	sessionCookie = "nrmm_session"
	sessionTTL    = 12 * time.Hour
)

// ---------- Passwörter ----------

func hashPassword(pw string) (string, error) {
	salt := randBytes(16)
	key, err := pbkdf2.Key(sha256.New, pw, salt, pbkdf2Iter, 32)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", pbkdf2Iter,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func checkPassword(stored, pw string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter < 1000 {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(parts[2])
	want, err2 := base64.RawStdEncoding.DecodeString(parts[3])
	if err1 != nil || err2 != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, pw, salt, iter, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

// ---------- TOTP (Authenticator-App, RFC 6238) ----------

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

func newTOTPSecret() string { return b32.EncodeToString(randBytes(20)) }

func totpCode(secret string, step int64) string {
	key, err := b32.DecodeString(strings.ToUpper(strings.ReplaceAll(secret, " ", "")))
	if err != nil {
		return ""
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step))
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	v := (uint32(sum[off]&0x7f)<<24 | uint32(sum[off+1])<<16 | uint32(sum[off+2])<<8 | uint32(sum[off+3])) % 1000000
	return fmt.Sprintf("%06d", v)
}

// verifyTOTP prüft einen Code (±30 s) und verhindert, dass derselbe Code zweimal benutzt wird.
func verifyTOTP(secret, code string, lastStep int64) (bool, int64) {
	code = strings.TrimSpace(strings.ReplaceAll(code, " ", ""))
	if len(code) != 6 {
		return false, 0
	}
	now := time.Now().Unix() / 30
	for d := int64(-1); d <= 1; d++ {
		s := now + d
		if s <= lastStep {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(totpCode(secret, s)), []byte(code)) == 1 {
			return true, s
		}
	}
	return false, 0
}

func totpURI(user, secret string) string {
	label := url.PathEscape("Nicolaj Fernwartung:" + user)
	return fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30",
		label, secret, url.QueryEscape("Nicolaj Fernwartung"))
}

// ---------- Brute-Force-Schutz ----------

type RateLimiter struct {
	mu sync.Mutex
	m  map[string]*rlEntry
}

type rlEntry struct {
	fails int
	until time.Time
	last  time.Time
}

func NewRateLimiter() *RateLimiter { return &RateLimiter{m: map[string]*rlEntry{}} }

func (l *RateLimiter) Blocked(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e := l.m[key]; e != nil && time.Now().Before(e.until) {
		return true, time.Until(e.until).Round(time.Second)
	}
	return false, 0
}

func (l *RateLimiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.m[key]
	if e == nil || time.Since(e.last) > time.Hour {
		e = &rlEntry{}
		l.m[key] = e
	}
	e.fails++
	e.last = time.Now()
	if e.fails >= 5 {
		lock := time.Minute << min(e.fails-5, 5) // 1, 2, 4 … max. 32 Minuten
		e.until = time.Now().Add(lock)
	}
}

func (l *RateLimiter) Success(key string) {
	l.mu.Lock()
	delete(l.m, key)
	l.mu.Unlock()
}

// ---------- Sitzungen ----------

func (a *App) createSession(w http.ResponseWriter, user string) {
	tok := randHex(32)
	rec := &SessionRec{TokenHash: sha256Hex(tok), Username: user, Created: time.Now(), Expires: time.Now().Add(sessionTTL)}
	a.store.Update(func(d *DBData) { d.Sessions = append(d.Sessions, rec) })
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: tok, Path: "/", HttpOnly: true, Secure: a.secureCookies(),
		SameSite: http.SameSiteStrictMode, MaxAge: int(sessionTTL.Seconds()),
	})
}

func (a *App) sessionUser(r *http.Request) string {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return ""
	}
	h := sha256Hex(c.Value)
	user := ""
	a.store.View(func(d *DBData) {
		for _, s := range d.Sessions {
			if subtle.ConstantTimeCompare([]byte(s.TokenHash), []byte(h)) == 1 && time.Now().Before(s.Expires) {
				if d.FindAdmin(s.Username) != nil {
					user = s.Username
				}
				return
			}
		}
	})
	return user
}

func (a *App) hasAdmin() bool {
	n := 0
	a.store.View(func(d *DBData) { n = len(d.Admins) })
	return n > 0
}

// Einrichtungscode: Nur wer Zugriff auf die Server-Konsole hat, darf das erste Konto anlegen.
func (a *App) ensureSetupCode() string {
	p := filepath.Join(a.dataDir, "einrichtungscode.txt")
	if b, err := os.ReadFile(p); err == nil && len(strings.TrimSpace(string(b))) >= 8 {
		return strings.TrimSpace(string(b))
	}
	code := newShortCode()
	os.WriteFile(p, []byte(code+"\n"), 0o600)
	return code
}

// ---------- Handler ----------

func (a *App) apiState(w http.ResponseWriter, r *http.Request) {
	user := a.sessionUser(r)
	resp := map[string]any{"setupNeeded": !a.hasAdmin(), "version": serverVersion, "user": nil}
	if user != "" {
		a.store.View(func(d *DBData) {
			if ad := d.FindAdmin(user); ad != nil {
				resp["user"] = map[string]any{"username": ad.Username, "totpEnabled": ad.TOTPEnabled}
			}
		})
	}
	writeJSON(w, 200, resp)
}

func (a *App) apiSetup(w http.ResponseWriter, r *http.Request) {
	var req struct{ SetupCode, Username, Password string }
	if !readJSON(w, r, &req, 4096) {
		return
	}
	ip := clientIP(r)
	if blocked, wait := a.limiter.Blocked("setup:" + ip); blocked {
		writeErr(w, 429, fmt.Sprintf("Zu viele Versuche. Bitte in %s erneut versuchen.", wait))
		return
	}
	if a.hasAdmin() {
		writeErr(w, 409, "Es gibt bereits ein Konto. Bitte anmelden.")
		return
	}
	if subtle.ConstantTimeCompare([]byte(normalizeCode(req.SetupCode)), []byte(normalizeCode(a.setupCode))) != 1 {
		a.limiter.Fail("setup:" + ip)
		writeErr(w, 403, "Der Einrichtungscode stimmt nicht. Du findest ihn im Server-Fenster oder in data/einrichtungscode.txt.")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if !validAdminName(req.Username) {
		writeErr(w, 400, "Benutzername: 3–32 Zeichen, nur Buchstaben, Zahlen, Punkt, Minus und Unterstrich.")
		return
	}
	if len(req.Password) < 10 {
		writeErr(w, 400, "Das Passwort muss mindestens 10 Zeichen haben.")
		return
	}
	h, err := hashPassword(req.Password)
	if err != nil {
		writeErr(w, 500, "Passwort konnte nicht gespeichert werden.")
		return
	}
	created := false
	a.store.Update(func(d *DBData) {
		if len(d.Admins) == 0 {
			d.Admins = append(d.Admins, &Admin{Username: req.Username, PasswordHash: h, CreatedAt: time.Now()})
			created = true
		}
	})
	if !created {
		writeErr(w, 409, "Es gibt bereits ein Konto.")
		return
	}
	os.Remove(filepath.Join(a.dataDir, "einrichtungscode.txt"))
	a.setupCode = ""
	a.audit(r, req.Username, "", "Konto angelegt", "Erstes Administrator-Konto eingerichtet")
	a.createSession(w, req.Username)
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *App) apiLogin(w http.ResponseWriter, r *http.Request) {
	var req struct{ Username, Password, Totp string }
	if !readJSON(w, r, &req, 4096) {
		return
	}
	ip := clientIP(r)
	if blocked, wait := a.limiter.Blocked("login:" + ip); blocked {
		writeErr(w, 429, fmt.Sprintf("Zu viele Fehlversuche. Bitte in %s erneut versuchen.", wait))
		return
	}
	var (
		ok, needTotp, totpOK bool
		secret               string
		lastStep             int64
	)
	a.store.View(func(d *DBData) {
		ad := d.FindAdmin(strings.TrimSpace(req.Username))
		if ad == nil {
			checkPassword("pbkdf2-sha256$210000$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", req.Password) // gleiche Rechenzeit
			return
		}
		ok = checkPassword(ad.PasswordHash, req.Password)
		needTotp = ad.TOTPEnabled
		secret, lastStep = ad.TOTPSecret, ad.TOTPLastStep
	})
	if !ok {
		a.limiter.Fail("login:" + ip)
		a.audit(r, req.Username, "", "Anmeldung fehlgeschlagen", "Falscher Benutzername oder falsches Passwort")
		writeErr(w, 401, "Benutzername oder Passwort ist falsch.")
		return
	}
	if needTotp {
		if strings.TrimSpace(req.Totp) == "" {
			writeJSON(w, 401, map[string]any{"error": "Bitte den 6-stelligen Code aus deiner Authenticator-App eingeben.", "needTotp": true})
			return
		}
		var step int64
		totpOK, step = verifyTOTP(secret, req.Totp, lastStep)
		if !totpOK {
			a.limiter.Fail("login:" + ip)
			a.audit(r, req.Username, "", "Anmeldung fehlgeschlagen", "Falscher 2FA-Code")
			writeJSON(w, 401, map[string]any{"error": "Der Code ist falsch oder abgelaufen.", "needTotp": true})
			return
		}
		a.store.Update(func(d *DBData) {
			if ad := d.FindAdmin(req.Username); ad != nil {
				ad.TOTPLastStep = step
			}
		})
	}
	a.limiter.Success("login:" + ip)
	a.audit(r, req.Username, "", "Angemeldet", "")
	a.createSession(w, strings.TrimSpace(req.Username))
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *App) apiLogout(w http.ResponseWriter, r *http.Request, user string) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		h := sha256Hex(c.Value)
		a.store.Update(func(d *DBData) {
			out := d.Sessions[:0]
			for _, s := range d.Sessions {
				if s.TokenHash != h {
					out = append(out, s)
				}
			}
			d.Sessions = out
		})
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.secureCookies(), SameSite: http.SameSiteStrictMode})
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *App) apiChangePassword(w http.ResponseWriter, r *http.Request, user string) {
	var req struct{ Current, New string }
	if !readJSON(w, r, &req, 4096) {
		return
	}
	if len(req.New) < 10 {
		writeErr(w, 400, "Das neue Passwort muss mindestens 10 Zeichen haben.")
		return
	}
	var stored string
	a.store.View(func(d *DBData) {
		if ad := d.FindAdmin(user); ad != nil {
			stored = ad.PasswordHash
		}
	})
	if !checkPassword(stored, req.Current) {
		writeErr(w, 403, "Das aktuelle Passwort stimmt nicht.")
		return
	}
	h, err := hashPassword(req.New)
	if err != nil {
		writeErr(w, 500, "Fehler beim Speichern.")
		return
	}
	a.store.Update(func(d *DBData) {
		if ad := d.FindAdmin(user); ad != nil {
			ad.PasswordHash = h
		}
	})
	a.audit(r, user, "", "Passwort geändert", "")
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *App) apiTotpBegin(w http.ResponseWriter, r *http.Request, user string) {
	secret := newTOTPSecret()
	a.store.Update(func(d *DBData) {
		if ad := d.FindAdmin(user); ad != nil {
			ad.TOTPPending = secret
		}
	})
	writeJSON(w, 200, map[string]any{"secret": secret, "uri": totpURI(user, secret)})
}

func (a *App) apiTotpEnable(w http.ResponseWriter, r *http.Request, user string) {
	var req struct{ Code string }
	if !readJSON(w, r, &req, 1024) {
		return
	}
	var pending string
	a.store.View(func(d *DBData) {
		if ad := d.FindAdmin(user); ad != nil {
			pending = ad.TOTPPending
		}
	})
	if pending == "" {
		writeErr(w, 400, "Bitte die Einrichtung zuerst starten.")
		return
	}
	ok, step := verifyTOTP(pending, req.Code, 0)
	if !ok {
		writeErr(w, 400, "Der Code stimmt nicht. Prüfe, ob die Uhrzeit am Handy stimmt.")
		return
	}
	a.store.Update(func(d *DBData) {
		if ad := d.FindAdmin(user); ad != nil {
			ad.TOTPSecret, ad.TOTPPending, ad.TOTPEnabled, ad.TOTPLastStep = pending, "", true, step
		}
	})
	a.audit(r, user, "", "2FA aktiviert", "")
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *App) apiTotpDisable(w http.ResponseWriter, r *http.Request, user string) {
	var req struct{ Password, Code string }
	if !readJSON(w, r, &req, 4096) {
		return
	}
	var ok bool
	a.store.View(func(d *DBData) {
		if ad := d.FindAdmin(user); ad != nil && checkPassword(ad.PasswordHash, req.Password) {
			ok, _ = verifyTOTP(ad.TOTPSecret, req.Code, ad.TOTPLastStep)
		}
	})
	if !ok {
		writeErr(w, 403, "Passwort oder Code stimmt nicht.")
		return
	}
	a.store.Update(func(d *DBData) {
		if ad := d.FindAdmin(user); ad != nil {
			ad.TOTPEnabled, ad.TOTPSecret, ad.TOTPLastStep = false, "", 0
		}
	})
	a.audit(r, user, "", "2FA deaktiviert", "")
	writeJSON(w, 200, map[string]any{"ok": true})
}

func validAdminName(s string) bool {
	if len(s) < 3 || len(s) > 32 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
