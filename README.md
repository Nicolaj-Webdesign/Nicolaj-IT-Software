# nicolaj▍ Fernwartung

Eigene Fernwartungs-Software (RMM) für Nicolaj Webdesign – wie NinjaOne, Intune oder TeamViewer, nur schlank und selbst gehostet.

**Was du damit machen kannst:**

- **PC-ID für jeden PC** – 9 Ziffern wie bei TeamViewer (z. B. `482 177 351`). Steht im Dashboard (Suche danach möglich) und auf dem PC im Startmenü unter **„Nicolaj Fernwartung“** – so kann dir Oma am Telefon sagen, welcher PC es ist.

- **Bildschirm sehen und steuern** – Maus, Tastatur, Sondertasten, Text tippen lassen, Vollbild. Auch vom Handy aus.
- **Benutzer verwalten** wie im Active Directory, nur lokal: anlegen, Passwort zurücksetzen, sperren/entsperren, Admin-Rechte geben/nehmen, löschen
- **Software** installieren, aktualisieren, deinstallieren (über winget), mit Schnellauswahl für Chrome, Firefox, VLC, LibreOffice …
- **Neustart / Herunterfahren / Sperren / Nachricht** an die Person am PC
- **PowerShell** als SYSTEM, mit Vorlagen (Speicherplatz, Druckerwarteschlange leeren …)
- **Systeminfos**: Windows-Version, Hardware, Festplatten, Virenschutz, „Neustart nötig“
- **Protokoll**: Jede Aktion wird mit Zeit, Person und IP festgehalten
- **Offline-Warteschlange**: Aufträge an ausgeschaltete PCs werden ausgeführt, sobald sie online sind (bis 24 Std.)
- **Auto-Update**: Die PCs aktualisieren sich selbst – erst Testgeräte, dann alle, mit automatischem Zurücksetzen bei Problemen
- **Sicherheitspaket (optional, pro PC)**: Virenschutz überwachen, scannen, härten, Funde entfernen, Alarm bei Bedrohungen (siehe unten)
- **Website mit Download** – unter deiner Server-Adresse (z. B. `rmm.nicolaj-webdesign.de`): Infos und Installer. Wer ihn benutzt, gibt seinen Namen ein und fragt Zugang an – erst nach deinem **Annehmen** kann der PC etwas. Abgelehnte PCs entfernen das Programm selbst.
- **Gruppen** (z. B. Familie, Kunden), **Übersicht** (online, offline, braucht Aufmerksamkeit, Agent veraltet), **Mehrfachauswahl**: Nachricht, Software, Skript, Neustart an viele PCs auf einmal
- **Schnellsuche** mit `Strg + K` und **gespeicherte Skripte** (einmal schreiben, überall ausführen)
- **Fenster am PC im Nicolaj-Look**: Installer, Bildschirm-Abfrage, Nachrichten, PC-ID und Verbindungsleiste – dunkel, mit Logo, wie man es von Discord oder Steam kennt

**Adressen:** `/` ist die öffentliche Website, das Dashboard liegt unter **`/admin`**.

---

## Schnellstart (Heimnetz)

### 1. Server starten

Auf dem PC/Mini-PC, der den Server spielen soll:

| System | Datei |
|---|---|
| Windows | `nicolaj-rmm-server.exe` doppelklicken |
| Linux (z. B. Lenovo Mini-PC) | `./nicolaj-rmm-server-linux-amd64` |
| Raspberry Pi (64 Bit) | `./nicolaj-rmm-server-linux-arm64` |

Das Fenster zeigt die Adresse (z. B. `https://192.168.178.20:8443`) und einen **Einrichtungscode**.
Unter Windows fragt die Firewall beim ersten Start → **Private Netzwerke erlauben**.

Alle Daten liegen im Ordner `data/` neben dem Programm.

### 2. Dashboard öffnen

Adresse mit `/admin` am Ende im Browser öffnen (z. B. `https://192.168.178.20:8443/admin`). Die Zertifikatswarnung ist beim ersten Mal normal (eigenes Zertifikat) → „Erweitert“ → „Weiter“.
Einrichtungscode eingeben, Konto anlegen. **Danach gleich unter Einstellungen die Zwei-Faktor-Anmeldung einschalten.**

### 3. PC hinzufügen

Dashboard → **Gerät hinzufügen** → Name eingeben → **Installer herunterladen**.
Den Installer auf den PC bringen und doppelklicken → Windows fragt nach Admin-Rechten → Ja → bestätigen. Fertig, der PC taucht sofort im Dashboard auf.

Jeder Installer gilt nur für **einen** PC und läuft nach der gewählten Zeit ab.

**Oder per Link (Zugangsanfrage):** Schick einfach den Link zu deiner Website (Dashboard › Einstellungen › „Website & Zugangsanfragen“). Dort lädt man den Installer, gibt seinen Namen ein und tippt **„Zugang anfragen“**. Die Anfrage erscheint oben in der Geräteliste – **Annehmen** oder **Ablehnen**. Bis dahin kann der PC nichts. Abschaltbar in den Einstellungen.

Der `nicolaj-agent.exe` aus dem GitHub-Release funktioniert ebenfalls als Anfrage-Installer, wenn im Repo die Variable **`NRMM_SERVER_URL`** gesetzt ist (Settings › Secrets and variables › Actions › *Variables*, z. B. `https://rmm.nicolaj-webdesign.de`). Bei einem privaten Repo kann Oma dort aber nichts herunterladen – nimm dann den Link zur Website.

---

## Was auf dem PC passiert (alles sichtbar, nichts versteckt)

- Programm: `C:\Program Files\NicolajRMM\nicolaj-agent.exe`
- Daten & Logbuch: `C:\ProgramData\NicolajRMM\` (nur Admins dürfen schreiben)
- Zwei geplante Aufgaben in der Aufgabenplanung unter `NicolajRMM`:
  - **Agent** – Verwaltungsdienst, läuft als SYSTEM ab Systemstart
  - **Bildschirm** – Helfer in der Sitzung der angemeldeten Person
- Eintrag unter **Einstellungen › Apps › „Nicolaj Fernwartung“** – darüber jederzeit deinstallierbar
- Startmenü-Eintrag **„Nicolaj Fernwartung“** – zeigt PC-ID, PC-Name und ob die Verbindung steht
- Bei jeder Bildschirm-Verbindung: **Hinweisleiste** oben am Bildschirm mit „Trennen“-Knopf
- Standardmäßig fragt der PC vorher **„Erlauben?“** (pro Gerät abschaltbar, z. B. für deinen eigenen Test-PC)

Bitte nur auf PCs installieren, deren Besitzer Bescheid wissen und einverstanden sind.

---

## Sicherheitspaket (optional)

Pro PC unter **Gerät › Sicherheit** (oder Einstellungen) einschalten. Übersicht aller PCs unter **Sicherheit** in der Seitenleiste.

**Wichtig zu wissen:** Das Paket ist kein selbstgeschriebener Virenscanner. Gute Erkennung braucht riesige, ständig aktualisierte Datenbanken und Treiber. Ein Eigenbau würde viel weniger erkennen und nur falsche Sicherheit vermitteln. Stattdessen steuert und verschärft das Paket den **Microsoft Defender**, der in jedem Windows steckt. Er schneidet bei unabhängigen Testlaboren (AV-TEST, AV-Comparatives) regelmäßig in der Spitzengruppe ab, kostet nichts und darf auch gewerblich genutzt werden.

| Funktion | Was passiert |
|---|---|
| Überwachung | Status alle 5 Minuten: Echtzeitschutz, Signaturen, letzter Scan, aktive Bedrohungen, andere installierte Virenscanner |
| Alarm | Sofort-Meldung im Dashboard + Protokoll, wenn eine neue Bedrohung auftaucht oder der Echtzeitschutz ausgeht |
| Scans | Schnellscan, vollständiger Scan, **Offline-Scan** (Neustart, prüft vor dem Windows-Start – gegen hartnäckige Schädlinge) |
| Bereinigen | Aktive Bedrohungen entfernen, Fundliste mit Datei, Gefahrenstufe und Status |
| Härtung | **Schutz vor unerwünschten Programmen** (Adware, Toolbars – die Stärke von Malwarebytes), Cloud-Schutz, Netzwerkschutz gegen Phishing-Seiten, optional Ransomware-Schutz |
| Zweitmeinung | **Microsoft Safety Scanner** wird jedes Mal frisch von Microsoft geladen, die Signatur geprüft, dann „schnell, nur melden“ oder „gründlich, Funde entfernen“ |
| Für alle | „Alle: Signaturen“ und „Alle: Schnellscan“ mit einem Klick |
| **Schutz pausieren** | Für Reparaturen 15 Min. bis 2 Std. Echtzeitschutz aus. Geht **garantiert automatisch wieder an** (Agent-Wächter + geplante Windows-Aufgabe, auch nach Neustart oder Deinstallation). Keine Fehlalarme während der Pause, aber Alarm, falls der Schutz danach nicht wieder angeht. |

Hinweise:
- Ist ein anderer Virenscanner (z. B. Avast) installiert, läuft der Defender im Hintergrund („passiv“). Das Paket zeigt das an; manche Einstellungen greifen dann nicht.
- Bei aktivem **Manipulationsschutz** (in Windows 11 Standard) lassen sich einige Einstellungen – auch das Pausieren – nur direkt am PC ändern. Das ist Absicht: Sonst könnte jede Schadsoftware den Virenschutz genauso abschalten. Lösung: Bildschirm verbinden → im Sicherheit-Menü „Windows-Sicherheit am PC öffnen“ → dort kurz ausschalten. Den Manipulationsschutz danach wieder einschalten.
- Malwarebytes Free ist nicht eingebaut: Laut Lizenz nur für private Geräte, nicht für Geräte, die geschäftlich genutzt werden.

---

## Auto-Update

**Neue Version veröffentlichen** – am einfachsten per GitHub-Release (siehe nächster Abschnitt):

```sh
git tag v1.0.1
git push origin v1.0.1
```

Ohne GitHub geht es auch: Versionsnummer in der Datei `VERSION` erhöhen, `build.sh` ausführen und die Server-Datei austauschen – oder unter Einstellungen nur die neue `nicolaj-agent.exe` hochladen.

**Absicherung auf jedem PC:**

- Download nur über die geprüfte HTTPS-Verbindung zum eigenen Server, dazu SHA-256-Prüfsumme
- Probelauf der neuen Datei; ist der installierte Agent signiert, muss die neue Version **vom selben Herausgeber signiert** sein
- Tausch der Programmdatei, Neustart des Dienstes. Meldet sich die neue Version nicht innerhalb von 2,5 Minuten beim Server, wird **automatisch die alte Version wiederhergestellt** und diese Version 24 Stunden nicht erneut automatisch versucht
- Der Bildschirm-Helfer startet sich selbst neu – aber nie mitten in einer Bildschirm-Sitzung
- Nie automatisch auf eine ältere Version

**Wichtig:** Agents, die mit Version 0.1–0.3 installiert wurden, kennen das Auto-Update noch nicht. Sie müssen **einmal** neu installiert werden (neuer Installer aus dem Dashboard), danach geht alles automatisch.

Pro PC: **Gerät › Einstellungen › Testgerät** (ideal: dein eigener PC) und „Jetzt aktualisieren“.

---

## Neue Versionen per GitHub-Release

### Einmal einrichten

1. **Repository anlegen** (privat), z. B. `nicolaj-webdesign/nicolaj-it-software`, und den Inhalt von `Quellcode/` hochladen:
   ```sh
   git init && git add . && git commit -m "Nicolaj Fernwartung"
   git remote add origin https://github.com/Nicolaj-Webdesign/Nicolaj-IT-Software.git
   git push -u origin main
   ```
   Die Datei `.github/workflows/release.yml` ist schon dabei.
2. **Release-Schlüssel** (nur für Server, die sich selbst von GitHub aktualisieren – bei Coolify nicht nötig): `go run ./tools/releasekey new`
   - privaten Schlüssel in GitHub unter *Settings › Secrets and variables › Actions* als Secret `RELEASE_SIGNING_KEY` speichern (niemals ins Repo!)
   - öffentlichen Schlüssel im Dashboard unter *Einstellungen › Neue Versionen per GitHub-Release* eintragen
   - Dann übernimmt der Server **nur Releases mit gültiger Signatur**. Selbst wenn jemand an dein GitHub-Konto käme und Dateien austauscht, landet nichts Fremdes auf den PCs.
3. **Zugriffs-Token** (nur Heimnetz-Server mit privatem Repo): GitHub › Settings › Developer settings › *Fine-grained tokens* → nur dieses Repository, Berechtigung *Contents: Read-only*. Zusammen mit dem Repository-Namen im Dashboard eintragen.
4. Optional **Code-Signatur** in der Action: Repository-Variable `SIGNING=azure` plus die Zugangsdaten (Details oben in `release.yml`).

### Veröffentlichen

**Am Handy:** GitHub-App oder github.com › Repo › *Actions* › **Release** › *Run workflow* › Version eintragen (z. B. `1.0.1`) › starten.

**Am PC:**
```sh
git tag v1.0.1
git push origin v1.0.1
```

1. GitHub Actions testet, baut Agent + Server (Windows, Linux, Raspberry Pi), signiert und veröffentlicht das Release mit `SHA256SUMS.txt` (+ Signatur). Dauer: wenige Minuten.
2. Dein Server prüft stündlich (oder sofort per **„Jetzt prüfen“**), lädt die neue Agent-Version, kontrolliert Prüfsumme und Signatur.
3. Die **Testgeräte** aktualisieren sich sofort → läuft alles: **„Für alle freigeben“**.
4. Läuft der Server auf Coolify, startet Coolify das neue Image automatisch (siehe unten). Im Heimnetz-Betrieb erscheint stattdessen **„Server aktualisieren“**: Download, Prüfung, Neustart in wenigen Sekunden. Die alte Datei bleibt als `.alt` daneben liegen (Notfall: zurückbenennen).

Die Datei `VERSION` gilt nur für lokale Builds – bei Releases zählt der Tag.

---

## Grenzen (ehrlich)

- **Die Windows-Teile sind kompiliert, aber noch nicht auf einem echten PC getestet** – das gilt besonders für Update und Rückfall. Erst mit Testgerät ausprobieren. Bitte zuerst auf deinem eigenen Test-PC ausprobieren. Logbücher: `C:\ProgramData\NicolajRMM\logs\agent.log` und `%LOCALAPPDATA%\NicolajRMM\screen.log`.
- **SmartScreen / Defender**: Die .exe ist nicht signiert. Windows zeigt „Unbekannter Herausgeber“, Defender meldet bei unsignierten Fernwartungs-Programmen manchmal einen Fehlalarm. Dauerhafte Lösung: Code-Signing-Zertifikat.
- **UAC-Abfragen und Sperrbildschirm** sind im Live-Bild nicht sichtbar bzw. nicht bedienbar (gesicherter Desktop). Admin-Aufgaben lieber über die Tabs Benutzer/Software/PowerShell erledigen.
- **Strg+Alt+Entf** lässt Windows nicht fernsteuern.
- **Microsoft-Konten**: Das Passwort lässt sich nur online unter account.microsoft.com zurücksetzen, nicht lokal.
- Bildschirm erst **nach der Windows-Anmeldung** verfügbar.
- Mehrere Monitore werden zusammen als ein großes Bild übertragen.
- **Ordner `data/` sichern!** Im Heimnetz-Betrieb enthält er das Zertifikat. Wird es neu erzeugt, verbinden sich die Agents nicht mehr (sie prüfen den Fingerabdruck) und müssen neu installiert werden.
- Das Docker-Image wird erst von GitHub Actions gebaut – der Server darin ist lokal mit genau diesen Einstellungen getestet, das Image selbst noch nicht.

---

## Server auf Coolify (empfohlen für Oma & Tante)

Mit Coolify läuft der Server mit echtem HTTPS – und **aktualisiert sich selbst**:
`git tag` → GitHub baut Docker-Image + Release → Coolify startet das neue Image → die PCs ziehen nach (Testgeräte zuerst).

Das Image heißt `ghcr.io/nicolaj-webdesign/nicolaj-it-software` und entsteht beim ersten Release (siehe oben).

### Schritt 1: Coolify darf das private Image laden

GitHub › Settings › Developer settings › *Tokens (classic)* → nur `read:packages`. Dann per SSH auf dem Coolify-Server:
```sh
echo DEIN_TOKEN | docker login ghcr.io -u DEIN_GITHUB_NAME --password-stdin
```

### Schritt 2: Von außen erreichbar machen

**Ohne öffentliche IP (Server zu Hause, DS-Lite/CGNAT): Cloudflare Tunnel** – kostenlos, keine Portfreigabe.
Voraussetzung: Die Domain liegt bei Cloudflare (Nameserver umstellen; Cloudflare übernimmt die bestehenden DNS-Einträge beim Umzug).

1. Cloudflare › *Networking › Tunnels* › **Create tunnel** → Name vergeben → *Docker* wählen → nur den Token kopieren (beginnt mit `eyJ`).
2. Coolify › Projekt › *+ New* › **Cloudflared** → Token als Umgebungsvariable eintragen → Deploy. Der Tunnel zeigt in Cloudflare dann „Healthy“.
3. Im Tunnel › *Configure › Routes* › *Add route* › *Published application*:
   Subdomain `rmm`, Domain `nicolaj-webdesign.de`, Service URL `http://localhost:80`.
   (Gibt es für `rmm` schon einen DNS-Eintrag, diesen löschen – sonst legt Cloudflare den Tunnel-Eintrag nicht an.)

**Mit öffentlicher IP:** einfach A-Eintrag `rmm.nicolaj-webdesign.de` → IP des Servers.

### Schritt 3: Anwendung anlegen

Coolify › Projekt › *+ New* › **Docker Image** → `ghcr.io/nicolaj-webdesign/nicolaj-it-software:latest`
- **Ports Exposes:** `8080`
- **Domain:** `https://rmm.nicolaj-webdesign.de`
- Beim Tunnel: unter *Domains* „Redirect HTTP to HTTPS“ **ausschalten** (sonst Weiterleitungsschleife)
- **Persistent Storage:** *Volume Mount*, Zielpfad `/data` (Datenbank, Konten, Geräte – **nicht vergessen**, sonst ist nach jedem Update alles weg)
- **Environment Variables:**
  ```
  NRMM_PUBLIC_URL=https://rmm.nicolaj-webdesign.de
  NRMM_CLOUDFLARE=1          # nur beim Cloudflare Tunnel
  ```
- **Deploy**. Den **Einrichtungscode** findest du im *Logs*-Tab → `https://rmm.nicolaj-webdesign.de/admin` öffnen → Konto anlegen → **sofort 2FA einrichten**.

### Schritt 4: Automatische Updates

**Ohne öffentliche IP** erreicht GitHub dein Coolify nicht. Deshalb schaut der Server selbst stündlich auf GitHub nach und stößt Coolify im Heimnetz an:

1. Coolify › *Settings* › **API aktivieren**, dann *Keys & Tokens* › *API tokens* → neues Token.
2. Anwendung › *Webhooks* › **Deploy Webhook** kopieren. Steht dort eine Adresse, die nur von außen geht, Host durch die Heimnetz-IP des Coolify-Servers ersetzen, z. B. `http://192.168.178.20:8000/api/v1/deploy?uuid=…&force=false`.
3. In der Anwendung zwei weitere Umgebungsvariablen setzen und neu deployen:
   ```
   NRMM_COOLIFY_WEBHOOK=<Deploy Webhook>
   NRMM_COOLIFY_TOKEN=<API-Token>
   ```
4. Im Dashboard › *Einstellungen › Neue Versionen per GitHub-Release*: Repository `Nicolaj-Webdesign/Nicolaj-IT-Software`, Token (Fine-grained, nur dieses Repo, *Contents: Read-only*), „automatisch übernehmen“ an → **Jetzt prüfen**.

**Mit öffentlicher IP** geht es auch direkt aus GitHub: Repo › *Settings › Secrets and variables › Actions* → `COOLIFY_WEBHOOK` und `COOLIFY_TOKEN`. Dann startet Coolify sofort nach dem Release neu.

Ab jetzt reicht für jede neue Version:
```sh
git tag v1.0.1
git push origin v1.0.1
```
Release und Image baut GitHub (ca. 10 Minuten). Spätestens eine Stunde danach läuft der neue Server – sofort per „Jetzt prüfen“ bzw. „Über Coolify aktualisieren“. Danach bekommen die Testgeräte den neuen Agent.

### Gut zu wissen

- Die Daten liegen im Volume `/data`. Backup einrichten.
- Agents, die gegen einen anderen Server installiert wurden, einmal **neu installieren** (Installer aus dem Coolify-Dashboard) – die Serveradresse steckt im Agent.
- Hinter Coolify gibt es kein eigenes Zertifikat mehr: Die Agents prüfen das normale Zertifikat (Let's Encrypt bzw. Cloudflare).
- Hängen Live-Bild oder Befehle: in Coolify unter *Advanced* „Gzip Compression“ ausschalten.
- Eigener Server ohne Coolify? Gleiches Image:
  `docker run -d -p 8080:8080 -v nrmm:/data -e NRMM_PUBLIC_URL=https://rmm.example.de ghcr.io/nicolaj-webdesign/nicolaj-it-software` – davor ein Reverse-Proxy mit HTTPS.

**Den Server nie ohne 2FA ins Internet stellen.** Wer das Dashboard übernimmt, kann alle PCs steuern.

### Einstellungen (Umgebungsvariablen)

| Variable | Bedeutung | Im Image |
|---|---|---|
| `NRMM_PUBLIC_URL` | Adresse, die in den Installer kommt | **selbst setzen** |
| `NRMM_ADDR` | Port im Container | `:8080` |
| `NRMM_DATA` | Datenordner | `/data` |
| `NRMM_NO_TLS` | HTTPS macht der Proxy | `1` |
| `NRMM_BEHIND_PROXY` | Client-IP aus `X-Forwarded-For`, sichere Cookies | `1` |
| `NRMM_CONTAINER` | Updates per Image statt Selbst-Update | `1` |
| `NRMM_CLOUDFLARE` | Besucher-IP aus `CF-Connecting-IP` (nur bei Cloudflare!) | – |
| `NRMM_COOLIFY_WEBHOOK` | Deploy Webhook – Server stößt Coolify bei neuem Release an | – |
| `NRMM_COOLIFY_TOKEN` | Coolify-API-Token dazu | – |

---

## Selbst bauen & entwickeln

Voraussetzung: [Go 1.24+](https://go.dev/dl/). Keine weiteren Pakete nötig (nur Standardbibliothek).

```sh
./build.sh          # Linux/macOS (Version aus der Datei VERSION)
.\build.ps1         # Windows
go test ./agent     # Tests
```

Ohne Windows-PC testen – ein simulierter PC mit Beispieldaten und Test-Bildschirm:

```sh
go run ./server -addr :8443 -data ./data
go run ./agent sim --server https://localhost:8443 --code CODE --fp FINGERABDRUCK
# (ohne build.sh heißt die Version „dev“)
```

### Aufbau

```
server/   Server + Dashboard (Go, Dashboard in server/web, ohne Framework)
tools/    releasekey – Schlüssel für signierte GitHub-Releases
.github/  GitHub Action für Releases
agent/    Agent für Windows (Go, Win32 direkt über syscall)
          *_windows.go = echte Windows-Funktionen, platform_other.go = Simulation
```

- Agent ↔ Server: HTTPS, Server-Sent Events für Aufträge (funktioniert hinter jedem Router, keine Portfreigabe am PC). Der Agent prüft den Zertifikats-Fingerabdruck.
- Jeder PC hat zwei Schlüssel: einen für den Dienst (nur SYSTEM/Admins lesbar) und einen eingeschränkten für den Bildschirm-Helfer.
- Passwörter für Benutzer werden nur durchgereicht, nie gespeichert. Auf dem PC gelangen Eingaben nur über Umgebungsvariablen in PowerShell, nie als Befehlstext.
- Dashboard: Login mit PBKDF2-Passwort-Hash, optional TOTP (2FA), Schutz vor Brute-Force und CSRF, strenge Content-Security-Policy.

### Ideen für die nächsten Versionen

Chat-Fenster am PC (Nachricht an Nicolaj, Antwort per Telegram) · Serverwechsel per Update · Datei-Übertragung · Zwischenablage · Monitor-Auswahl · Windows-Updates · Warnungen (Platte voll, Virenschutz aus, offline seit X Tagen) · mehrere Techniker-Konten mit Rollen · Code-Signing
