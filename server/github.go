package main

// Neue Versionen per GitHub-Release:
// Der Server prüft stündlich das neueste Release im eingestellten Repository, lädt die
// Prüfsummen (und – falls ein Release-Schlüssel hinterlegt ist – deren Signatur) und
// übernimmt die neue Agent-Version. Die PCs aktualisieren sich dann wie gewohnt
// (Testgeräte zuerst). Den Server selbst aktualisierst du per Knopf im Dashboard.

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	reRepo   = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}/[A-Za-z0-9_.-]{1,100}$`)
	reSemver = regexp.MustCompile(`^\d{1,4}(\.\d{1,4}){1,3}$`)
	ghAPI    = "https://api.github.com" // in Tests überschreibbar (NRMM_GITHUB_API)
)

type ghAsset struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Size int    `json:"size"`
	URL  string `json:"url"`
}

type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []ghAsset `json:"assets"`
}

type ghStatus struct {
	mu           sync.Mutex
	CheckedAt    time.Time `json:"checkedAt"`
	Tag          string    `json:"tag"`
	Version      string    `json:"version"`
	URL          string    `json:"url"`
	PublishedAt  time.Time `json:"publishedAt"`
	Error        string    `json:"error"`
	Signed       bool      `json:"signed"`
	ServerUpdate string    `json:"serverUpdate"` // neuere Server-Version verfügbar
	ServerAsset  string    `json:"serverAsset"`
	Busy         bool      `json:"busy"`
}

var gh ghStatus

func init() {
	if v := os.Getenv("NRMM_GITHUB_API"); v != "" {
		ghAPI = strings.TrimRight(v, "/")
	}
}

func serverAssetName() string {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/amd64":
		return "nicolaj-rmm-server.exe"
	case "linux/amd64":
		return "nicolaj-rmm-server-linux-amd64"
	case "linux/arm64":
		return "nicolaj-rmm-server-linux-arm64"
	}
	return ""
}

var ghClient = &http.Client{Timeout: 10 * time.Minute}

func ghGet(url, accept, token string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "nicolaj-rmm-server/"+serverVersion)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token) // Go entfernt den Header bei Weiterleitung auf fremde Domains
	}
	resp, err := ghClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 200<<20))
	if err != nil {
		return nil, err
	}
	switch {
	case resp.StatusCode == 404:
		return nil, errors.New("nicht gefunden (falscher Repository-Name, noch kein Release, oder privates Repo ohne Token)")
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		return nil, fmt.Errorf("kein Zugriff (%s) – Token prüfen", resp.Status)
	case resp.StatusCode >= 300:
		return nil, fmt.Errorf("GitHub antwortet mit %s", resp.Status)
	}
	return body, nil
}

// verifiedRelease holt das neueste Release und prüft Prüfsummen (+ Signatur).
// Es liefert die Prüfsummen-Tabelle zurück, damit jede Datei einzeln kontrolliert werden kann.
func verifiedRelease(s Settings) (*ghRelease, map[string]string, bool, error) {
	body, err := ghGet(ghAPI+"/repos/"+s.GitHubRepo+"/releases/latest", "application/vnd.github+json", s.GitHubToken)
	if err != nil {
		return nil, nil, false, fmt.Errorf("Release abrufen: %w", err)
	}
	var rel ghRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, nil, false, errors.New("Antwort von GitHub nicht lesbar")
	}
	if !reSemver.MatchString(strings.TrimPrefix(rel.TagName, "v")) {
		return nil, nil, false, fmt.Errorf("Tag „%s“ ist keine Versionsnummer (erwartet z. B. v0.5.1)", rel.TagName)
	}
	sumsAsset := findAsset(&rel, "SHA256SUMS.txt")
	if sumsAsset == nil {
		return nil, nil, false, errors.New("Release enthält keine SHA256SUMS.txt")
	}
	sums, err := ghGet(sumsAsset.URL, "application/octet-stream", s.GitHubToken)
	if err != nil {
		return nil, nil, false, fmt.Errorf("Prüfsummen laden: %w", err)
	}
	signed := false
	if s.ReleasePubKey != "" {
		pub, err := base64.StdEncoding.DecodeString(s.ReleasePubKey)
		if err != nil || len(pub) != ed25519.PublicKeySize {
			return nil, nil, false, errors.New("hinterlegter Release-Schlüssel ist ungültig")
		}
		sigAsset := findAsset(&rel, "SHA256SUMS.txt.sig")
		if sigAsset == nil {
			return nil, nil, false, errors.New("Release ist nicht signiert (SHA256SUMS.txt.sig fehlt) – wird aus Sicherheitsgründen nicht übernommen")
		}
		sigB64, err := ghGet(sigAsset.URL, "application/octet-stream", s.GitHubToken)
		if err != nil {
			return nil, nil, false, fmt.Errorf("Signatur laden: %w", err)
		}
		sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigB64)))
		if err != nil || !ed25519.Verify(pub, sums, sig) {
			return nil, nil, false, errors.New("SIGNATUR UNGÜLTIG – Release wird nicht übernommen. Wurde das Release manipuliert?")
		}
		signed = true
	}
	table := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 && len(f[0]) == 64 {
			table[strings.TrimPrefix(f[1], "*")] = strings.ToLower(f[0])
		}
	}
	return &rel, table, signed, nil
}

func findAsset(rel *ghRelease, name string) *ghAsset {
	for i := range rel.Assets {
		if rel.Assets[i].Name == name {
			return &rel.Assets[i]
		}
	}
	return nil
}

func downloadVerified(rel *ghRelease, sums map[string]string, name, token string) ([]byte, error) {
	as := findAsset(rel, name)
	want := sums[name]
	if as == nil || want == "" {
		return nil, fmt.Errorf("%s fehlt im Release oder in den Prüfsummen", name)
	}
	b, err := ghGet(as.URL, "application/octet-stream", token)
	if err != nil {
		return nil, fmt.Errorf("%s laden: %w", name, err)
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != want {
		return nil, fmt.Errorf("Prüfsumme von %s stimmt nicht – Download verworfen", name)
	}
	return b, nil
}

// checkGitHub prüft auf ein neues Release und übernimmt ggf. die Agent-Version.
func (a *App) checkGitHub(r *http.Request) error {
	s := a.settings()
	if s.GitHubRepo == "" {
		return nil
	}
	gh.mu.Lock()
	if gh.Busy {
		gh.mu.Unlock()
		return errors.New("Prüfung läuft bereits")
	}
	gh.Busy = true
	gh.mu.Unlock()
	defer func() { gh.mu.Lock(); gh.Busy = false; gh.mu.Unlock() }()

	rel, sums, signed, err := verifiedRelease(s)
	gh.mu.Lock()
	gh.CheckedAt = time.Now()
	if err != nil {
		gh.Error = err.Error()
		gh.mu.Unlock()
		return err
	}
	version := strings.TrimPrefix(rel.TagName, "v")
	gh.Tag, gh.Version, gh.URL, gh.PublishedAt, gh.Signed, gh.Error = rel.TagName, version, rel.HTMLURL, rel.PublishedAt, signed, ""
	gh.ServerAsset, gh.ServerUpdate = serverAssetName(), ""
	if !a.container && gh.ServerAsset != "" && findAsset(rel, gh.ServerAsset) != nil && versionLess(serverVersion, version) {
		gh.ServerUpdate = version
	}
	gh.mu.Unlock()

	cur := a.currentAgent()
	if !s.GitHubAuto || (cur != nil && !versionLess(cur.Version, version)) {
		return nil
	}
	bin, err := downloadVerified(rel, sums, "nicolaj-agent.exe", s.GitHubToken)
	if err == nil {
		if pkg := newPkg(bin, "github"); pkg == nil || pkg.Version != version {
			err = fmt.Errorf("nicolaj-agent.exe im Release hat nicht die Version %s", version)
		}
	}
	if err == nil {
		err = a.storeAgent(bin, "GitHub "+rel.TagName)
	}
	if err != nil {
		gh.mu.Lock()
		gh.Error = err.Error()
		gh.mu.Unlock()
		return err
	}
	detail := fmt.Sprintf("Agent %s aus %s übernommen (%s)", version, s.GitHubRepo, map[bool]string{true: "Signatur geprüft", false: "ohne Release-Signatur"}[signed])
	e := &AuditEntry{Time: time.Now(), User: "GitHub", Action: "Neue Version von GitHub", Detail: detail}
	if r != nil {
		e.IP = clientIP(r)
	}
	a.store.Update(func(d *DBData) { d.Audit = append(d.Audit, e) })
	a.hub.Broadcast("alert", map[string]any{"level": "info", "title": "Neue Version " + version, "detail": "Von GitHub übernommen – die PCs aktualisieren sich (Testgeräte zuerst)."})
	a.pushUpdates("")
	return nil
}

func (a *App) githubLoop() {
	time.Sleep(90 * time.Second)
	for {
		if err := a.checkGitHub(nil); err != nil {
			fmt.Fprintln(os.Stderr, "GitHub-Prüfung:", err)
		}
		time.Sleep(time.Hour)
	}
}

// ---------- Dashboard ----------

func (a *App) githubView() map[string]any {
	s := a.settings()
	gh.mu.Lock()
	status := map[string]any{
		"checkedAt": gh.CheckedAt, "tag": gh.Tag, "version": gh.Version, "url": gh.URL, "publishedAt": gh.PublishedAt,
		"error": gh.Error, "signed": gh.Signed, "serverUpdate": gh.ServerUpdate, "serverAsset": gh.ServerAsset,
		"busy": gh.Busy, "serverVersion": serverVersion, "container": a.container,
	}
	gh.mu.Unlock()
	return map[string]any{"repo": s.GitHubRepo, "hasToken": s.GitHubToken != "", "auto": s.GitHubAuto, "pubKey": s.ReleasePubKey, "status": status}
}

func (a *App) apiGitHub(w http.ResponseWriter, r *http.Request, user string) {
	writeJSON(w, 200, a.githubView())
}

func (a *App) apiGitHubSettings(w http.ResponseWriter, r *http.Request, user string) {
	var req struct {
		Repo       *string `json:"repo"`
		Token      *string `json:"token"`
		ClearToken bool    `json:"clearToken"`
		Auto       *bool   `json:"auto"`
		PubKey     *string `json:"pubKey"`
	}
	if !readJSON(w, r, &req, 16<<10) {
		return
	}
	if req.Repo != nil {
		v := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(*req.Repo, "https://github.com/"), "github.com/"))
		v = strings.TrimSuffix(v, ".git")
		if v != "" && !reRepo.MatchString(v) {
			writeErr(w, 400, "Repository bitte als „besitzer/name“ angeben, z. B. nicolaj-webdesign/nicolaj-it-software.")
			return
		}
		req.Repo = &v
	}
	if req.PubKey != nil {
		v := strings.TrimSpace(*req.PubKey)
		if v != "" {
			if b, err := base64.StdEncoding.DecodeString(v); err != nil || len(b) != ed25519.PublicKeySize {
				writeErr(w, 400, "Der öffentliche Release-Schlüssel ist ungültig (go run ./tools/releasekey new).")
				return
			}
		}
		req.PubKey = &v
	}
	var changes []string
	a.store.Update(func(d *DBData) {
		st := d.Settings
		if req.Repo != nil && *req.Repo != st.GitHubRepo {
			st.GitHubRepo = *req.Repo
			changes = append(changes, "Repository: "+st.GitHubRepo)
		}
		if req.Token != nil && strings.TrimSpace(*req.Token) != "" {
			st.GitHubToken = strings.TrimSpace(*req.Token)
			changes = append(changes, "Token gespeichert")
		}
		if req.ClearToken && st.GitHubToken != "" {
			st.GitHubToken = ""
			changes = append(changes, "Token gelöscht")
		}
		if req.Auto != nil && *req.Auto != st.GitHubAuto {
			st.GitHubAuto = *req.Auto
			changes = append(changes, map[bool]string{true: "Agent-Versionen automatisch übernehmen", false: "Agent-Versionen nicht automatisch übernehmen"}[st.GitHubAuto])
		}
		if req.PubKey != nil && *req.PubKey != st.ReleasePubKey {
			st.ReleasePubKey = *req.PubKey
			changes = append(changes, map[bool]string{true: "Release-Schlüssel hinterlegt", false: "Release-Schlüssel entfernt"}[st.ReleasePubKey != ""])
		}
	})
	if len(changes) > 0 {
		a.audit(r, user, "", "GitHub-Einstellungen", strings.Join(changes, "; "))
	}
	writeJSON(w, 200, a.githubView())
}

func (a *App) apiGitHubCheck(w http.ResponseWriter, r *http.Request, user string) {
	if a.settings().GitHubRepo == "" {
		writeErr(w, 400, "Bitte zuerst ein Repository eintragen.")
		return
	}
	a.checkGitHub(r) // Fehler stehen im Status
	writeJSON(w, 200, a.githubView())
}

// apiGitHubUpdateServer lädt die neue Server-Version, prüft sie und startet neu.
func (a *App) apiGitHubUpdateServer(w http.ResponseWriter, r *http.Request, user string) {
	if a.container {
		writeErr(w, 409, "Der Server läuft in Docker/Coolify – Server-Updates kommen automatisch über ein neues Image.")
		return
	}
	s := a.settings()
	name := serverAssetName()
	if s.GitHubRepo == "" || name == "" {
		writeErr(w, 400, "Server-Update ist für dieses System nicht verfügbar.")
		return
	}
	rel, sums, signed, err := verifiedRelease(s)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	version := strings.TrimPrefix(rel.TagName, "v")
	if !versionLess(serverVersion, version) {
		writeErr(w, 409, "Der Server ist bereits auf dem neuesten Stand ("+serverVersion+").")
		return
	}
	bin, err := downloadVerified(rel, sums, name, s.GitHubToken)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		writeErr(w, 500, "Programmpfad unbekannt.")
		return
	}
	if err := replaceExecutable(exe, bin); err != nil {
		writeErr(w, 500, "Server-Datei konnte nicht ersetzt werden: "+err.Error())
		return
	}
	a.audit(r, user, "", "Server-Update", fmt.Sprintf("%s → %s (%s)", serverVersion, version, map[bool]string{true: "Signatur geprüft", false: "ohne Release-Signatur"}[signed]))
	a.store.Flush()
	writeJSON(w, 200, map[string]any{"ok": true, "version": version})
	go func() {
		time.Sleep(800 * time.Millisecond)
		a.store.Flush()
		restartSelf(exe, a)
	}()
}
