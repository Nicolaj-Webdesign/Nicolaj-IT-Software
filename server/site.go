package main

// Öffentliche Website (rmm.nicolaj-webdesign.de): Infos + direkter Download des allgemeinen
// Installers. Das Dashboard liegt unter /admin.

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"time"
)

var siteTmpl = template.Must(template.New("site").Parse(`<!doctype html>
<html lang="de">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Nicolaj Fernwartung – Hilfe für deinen PC</title>
<meta name="description" content="Nicolaj Webdesign hilft dir bei Problemen mit deinem Windows-PC – aus der Ferne. Installer herunterladen, Namen eingeben, fertig.">
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
<link rel="stylesheet" href="/site.css">
</head>
<body>
<header class="top">
  <div class="wrap top-row">
    <a class="brand" href="/" aria-label="Nicolaj Fernwartung – Startseite">
      <img src="/logo.svg" alt="" width="28" height="28">
      <span class="word">nicolaj<span class="cursor" aria-hidden="true"></span></span>
      <span class="product">Fernwartung</span>
    </a>
    <a class="signin" href="/admin">Anmelden</a>
  </div>
</header>

<main>
  <section class="hero wrap">
    <div class="hero-text">
      <h1>Hilfe für deinen PC – ohne dass jemand vorbeikommen muss.</h1>
      <p class="lead">Mit der Nicolaj Fernwartung kann dir Nicolaj Webdesign Programme installieren, Updates einspielen und bei Problemen helfen. Wenn er deinen Bildschirm sieht, zeigt dir das eine Leiste – mit einem Knopf zum Trennen.</p>
      <div id="download" class="cta">
        {{if and .Enabled .Available}}
        <a class="btn" href="/download/Nicolaj-Fernwartung.exe" download>Für Windows herunterladen</a>
        <p class="meta">Windows 10 und 11{{if .Size}} · {{.Size}}{{end}} · danach Namen eingeben und Zugang anfragen</p>
        {{else if not .Enabled}}
        <p class="notice">Downloads sind gerade ausgeschaltet. Melde dich bitte direkt bei Nicolaj Webdesign.</p>
        {{else}}
        <p class="notice">Der Installer ist gerade nicht verfügbar. Bitte versuch es später noch einmal.</p>
        {{end}}
      </div>
    </div>
    <figure class="hero-shot">
      <img src="/installer.png" width="492" height="605" alt="Das Installationsfenster: Feld „Wie heißt du?“ und die Knöpfe „Abbrechen“ und „Zugang anfragen“.">
    </figure>
  </section>

  <section class="band" id="so-gehts">
    <div class="wrap">
      <h2>So geht's</h2>
      <ol class="steps">
        <li>
          <h3>Herunterladen und öffnen</h3>
          <p>Lade den Installer herunter und öffne die Datei. Falls Windows warnt: auf <b>Weitere Informationen</b> und dann auf <b>Trotzdem ausführen</b> klicken.</p>
        </li>
        <li>
          <h3>Namen eingeben</h3>
          <p>Gib deinen Namen ein und klicke auf <b>Zugang anfragen</b>. Windows fragt einmal nach der Erlaubnis zum Installieren.</p>
        </li>
        <li>
          <h3>Freigabe abwarten</h3>
          <p>Nicolaj Webdesign bekommt Bescheid und gibt dich frei. Bis dahin passiert auf deinem PC nichts – du musst nichts weiter tun.</p>
        </li>
      </ol>
    </div>
  </section>

  <section class="wrap">
    <h2>Was die Fernwartung macht</h2>
    <div class="facts">
      <div class="fact">
        <h3>Programme und Updates</h3>
        <p>Programme werden im Hintergrund installiert und aktuell gehalten, ohne dass du etwas klicken musst.</p>
      </div>
      <div class="fact">
        <h3>Bildschirm nur sichtbar mit Leiste</h3>
        <p>Ist jemand mit deinem Bildschirm verbunden, siehst du oben immer eine Leiste. Ein Klick auf <b>Trennen</b> beendet die Verbindung sofort. Normalerweise wirst du vorher gefragt.</p>
      </div>
      <div class="fact">
        <h3>Deine PC-ID</h3>
        <p>Jeder PC bekommt eine Nummer. Du findest sie im Startmenü unter <b>Nicolaj Fernwartung</b> – nenn sie einfach am Telefon.</p>
      </div>
      <div class="fact">
        <h3>Jederzeit entfernbar</h3>
        <p>Unter <b>Einstellungen › Apps › Nicolaj Fernwartung</b> kannst du das Programm wie jedes andere deinstallieren.</p>
      </div>
    </div>
  </section>

  <section class="wrap faq">
    <h2>Häufige Fragen</h2>
    <details>
      <summary>Was passiert direkt nach der Installation?</summary>
      <p>Deine Anfrage landet bei Nicolaj Webdesign. Solange sie nicht angenommen ist, kann niemand etwas auf deinem PC tun. Wird sie abgelehnt, entfernt sich das Programm von selbst.</p>
    </details>
    <details>
      <summary>Ist die Verbindung sicher?</summary>
      <p>Die Verbindung ist verschlüsselt. Jede Aktion wird protokolliert, und der Zugang zum Dashboard ist mit Passwort und Zwei-Faktor-Code geschützt.</p>
    </details>
    <details>
      <summary>Warum warnt Windows beim Öffnen?</summary>
      <p>Windows kennt neue Programme anfangs noch nicht und zeigt deshalb einen Hinweis. Mit <b>Weitere Informationen</b> und <b>Trotzdem ausführen</b> geht es weiter.</p>
    </details>
    <details>
      <summary>Funktioniert das auch auf dem Mac oder Handy?</summary>
      <p>Nein, die Fernwartung gibt es nur für Windows 10 und Windows 11.</p>
    </details>
  </section>
</main>

<footer class="foot">
  <div class="wrap foot-row">
    <span>© {{.Year}} Nicolaj Webdesign</span>
    <nav aria-label="Weitere Links">
      <a href="https://nicolaj-webdesign.de">nicolaj-webdesign.de</a>
      <a href="https://nicolaj-webdesign.de/impressum">Impressum</a>
      <a href="/admin">Anmelden</a>
    </nav>
  </div>
</footer>
</body>
</html>
`))

func humanSize(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("%.0f MB", float64(n)/1e6+0.5)
}

func (a *App) publicSite(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Enabled, Available bool
		Size               string
		Year               int
	}{Enabled: !a.settings().DisableRequests, Year: time.Now().Year()}
	if pkg := a.currentAgent(); pkg != nil && pkg.data != nil {
		data.Available, data.Size = true, humanSize(len(pkg.data))
	}
	var buf bytes.Buffer
	if err := siteTmpl.Execute(&buf, data); err != nil {
		http.Error(w, "Seite nicht verfügbar", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(buf.Bytes())
}

func (a *App) publicDownloadPage(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/#download", http.StatusFound)
}
