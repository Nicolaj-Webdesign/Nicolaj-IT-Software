package main

// Inhalte aller Fenster am PC – an einer Stelle, damit Vorschau und Windows identisch sind.

import (
	"fmt"
	"net/url"
	"strings"
)

const barText = "Nicolaj ist mit deinem PC verbunden"

func serverHost(server string) string {
	if u, err := url.Parse(server); err == nil && u.Host != "" {
		return u.Hostname()
	}
	return server
}

func dlgInstallWelcome(server, problem string) *uiDialog {
	note := "Verbindet mit " + serverHost(server)
	if problem != "" {
		note = problem
	}
	return &uiDialog{Width: 440, Badge: 'L', Title: "Fernwartung einrichten",
		Body: "Damit kann dir Nicolaj Webdesign bei Problemen mit deinem PC helfen – schnell und ohne Vorbeikommen.",
		Bullets: []uiBullet{
			{gEye, "Du siehst immer, wenn jemand verbunden ist"},
			{gSync, "Programme und Updates laufen im Hintergrund"},
			{gTrash, "Jederzeit entfernbar unter Einstellungen › Apps"},
		},
		Progress: -1, Note: note, Closable: true,
		Buttons: []uiBtn{{ID: "cancel", Label: "Abbrechen", Kind: "secondary"}, {ID: "install", Label: "Installieren", Kind: "primary"}}}
}

var installSteps = []string{
	"Bitte bestätige die Windows-Abfrage …",
	"Verbinde mit dem Server …",
	"Programm wird kopiert …",
	"Hintergrunddienst wird eingerichtet …",
	"Wird gestartet …",
}

func dlgInstallProgress(step int, progress float64) *uiDialog {
	if step < 0 || step >= len(installSteps) {
		step = len(installSteps) - 1
	}
	return &uiDialog{Width: 440, Badge: 'L', Title: "Wird eingerichtet …", Body: installSteps[step],
		Progress: progress, Note: "Das dauert nur einen Moment."}
}

func dlgInstallDone(name, pcID string) *uiDialog {
	d := &uiDialog{Width: 440, Badge: gCheck, Tone: "ok", Title: "Alles erledigt!",
		Body:     "Dieser PC ist jetzt als „" + name + "“ mit Nicolaj Fernwartung verbunden.",
		Progress: -1, Closable: true,
		Note:    "Die PC-ID findest du jederzeit im Startmenü unter „Nicolaj Fernwartung“.",
		Buttons: []uiBtn{{ID: "ok", Label: "Fertig", Kind: "primary"}}}
	if id := fmtPCID(pcID); id != "" {
		d.BigLabel, d.Big, d.BigCopy = "Deine PC-ID", id, true
	}
	return d
}

func dlgInstallError(err string) *uiDialog {
	return &uiDialog{Width: 440, Badge: gError, Tone: "danger", Title: "Das hat leider nicht geklappt",
		Body: err, Progress: -1, Closable: true,
		Note:    "Probier es nochmal oder melde dich bei Nicolaj Webdesign.",
		Buttons: []uiBtn{{ID: "cancel", Label: "Schließen", Kind: "secondary"}, {ID: "retry", Label: "Nochmal versuchen", Kind: "primary"}}}
}

func dlgUninstallConfirm() *uiDialog {
	return &uiDialog{Badge: gTrash, Tone: "danger", Title: "Fernwartung entfernen?",
		Body:     "Danach kann dir Nicolaj Webdesign nicht mehr aus der Ferne helfen. Du kannst sie später jederzeit neu installieren.",
		Progress: -1, Closable: true,
		Buttons: []uiBtn{{ID: "cancel", Label: "Abbrechen", Kind: "secondary"}, {ID: "remove", Label: "Entfernen", Kind: "danger"}}}
}

func dlgUninstallDone() *uiDialog {
	return &uiDialog{Badge: gCheck, Tone: "ok", Title: "Entfernt",
		Body: "Nicolaj Fernwartung wurde von diesem PC entfernt.", Progress: -1, Closable: true,
		Buttons: []uiBtn{{ID: "ok", Label: "Schließen", Kind: "primary"}}}
}

func dlgNotice(title, body, tone string) *uiDialog {
	g := gInfo
	switch tone {
	case "danger":
		g = gError
	case "warn":
		g = gWarn
	case "ok":
		g = gCheck
	}
	return &uiDialog{Badge: g, Tone: tone, Title: title, Body: body, Progress: -1, Closable: true,
		Buttons: []uiBtn{{ID: "ok", Label: "OK", Kind: "primary"}}}
}

const consentSeconds = 60

func consentNote(left int) string {
	if left == 1 {
		return "Ohne Antwort wird in 1 Sekunde abgelehnt."
	}
	return fmt.Sprintf("Ohne Antwort wird in %d Sekunden abgelehnt.", left)
}

func dlgConsent(by string, secondsLeft int) *uiDialog {
	who := "Nicolaj Support"
	if by != "" && !strings.EqualFold(by, "nicolaj") {
		who += " (" + by + ")"
	}
	return &uiDialog{Badge: gMonitor, Title: "Darf Nicolaj deinen Bildschirm sehen?",
		Body: who + " möchte sich verbinden, um dir zu helfen. Dabei sieht er deinen Bildschirm und kann Maus und Tastatur bedienen. " +
			"Oben erscheint eine Leiste – dort kannst du jederzeit trennen.",
		Progress: -1, Note: consentNote(secondsLeft),
		Buttons: []uiBtn{{ID: "deny", Label: "Ablehnen", Kind: "secondary"}, {ID: "allow", Label: "Erlauben", Kind: "primary"}}}
}

func dlgMessage(text string) *uiDialog {
	return &uiDialog{Width: 440, From: "Nicolaj", Message: text, Progress: -1, Closable: true,
		Buttons: []uiBtn{{ID: "ok", Label: "Alles klar", Kind: "primary"}}}
}

type pcInfo struct {
	Installed bool
	PCID      string
	Host      string
	Name      string
	Online    bool
	Checking  bool
}

func dlgInfo(i pcInfo) *uiDialog {
	if !i.Installed {
		return &uiDialog{Badge: gWarn, Tone: "warn", Title: "Nicht eingerichtet",
			Body:     "Nicolaj Fernwartung ist auf diesem PC nicht (vollständig) installiert.",
			Rows:     []uiRow{{"PC-Name", i.Host, ""}},
			Progress: -1, Closable: true, Buttons: []uiBtn{{ID: "ok", Label: "Schließen", Kind: "primary"}}}
	}
	id := fmtPCID(i.PCID)
	if id == "" {
		id = "– – –"
	}
	status, tone := "verbunden", "ok"
	switch {
	case i.Checking:
		status, tone = "wird geprüft …", "warn"
	case !i.Online:
		status, tone = "Server nicht erreichbar", "danger"
	}
	return &uiDialog{Badge: 0, Title: "Deine PC-ID", Body: "Nenne diese Nummer, wenn du Nicolaj Webdesign um Hilfe bittest.",
		Big: id, BigCopy: i.PCID != "",
		Rows:     []uiRow{{"PC-Name", i.Host, ""}, {"Im Dashboard als", i.Name, ""}, {"Verbindung", status, tone}},
		Progress: -1, Closable: true, Buttons: []uiBtn{{ID: "chat", Label: "Nachricht an Nicolaj", Kind: "secondary"}, {ID: "ok", Label: "Schließen", Kind: "primary"}}}
}

// ---------- Allgemeiner Installer mit Zugangsanfrage ----------

func dlgRequestWelcome(server, name, problem string) *uiDialog {
	note := "Verbindet mit " + serverHost(server)
	if problem != "" {
		note = problem
	}
	return &uiDialog{Width: 440, Badge: 'L', Title: "Fernwartung einrichten",
		Body:  "Gib deinen Namen ein und frag den Zugang an. Nicolaj Webdesign gibt dich einmal frei – danach kann er dir bei Problemen mit dem PC helfen.",
		Input: &uiInput{Label: "Wie heißt du?", Placeholder: "z. B. Erika Muster", Value: name, MaxLen: 40},
		Bullets: []uiBullet{
			{gEye, "Du siehst immer, wenn jemand verbunden ist"},
			{gTrash, "Jederzeit entfernbar unter Einstellungen › Apps"},
		},
		Progress: -1, Note: note, Closable: true,
		Buttons: []uiBtn{{ID: "cancel", Label: "Abbrechen", Kind: "secondary"},
			{ID: "request", Label: "Zugang anfragen", Kind: "primary", Disabled: strings.TrimSpace(name) == ""}}}
}

func dlgRequestDone(pcID string) *uiDialog {
	d := &uiDialog{Width: 440, Badge: gCheck, Tone: "ok", Title: "Anfrage gesendet!",
		Body:     "Nicolaj Webdesign bekommt jetzt Bescheid. Sobald du freigegeben bist, kann er dir helfen – du musst nichts weiter tun.",
		Progress: -1, Closable: true,
		Note:    "Die PC-ID findest du jederzeit im Startmenü unter „Nicolaj Fernwartung“.",
		Buttons: []uiBtn{{ID: "ok", Label: "Fertig", Kind: "primary"}}}
	if id := fmtPCID(pcID); id != "" {
		d.BigLabel, d.Big, d.BigCopy = "Deine PC-ID", id, true
	}
	return d
}

// ---------- Nachrichten ----------

const chatTitle = "Nachrichten – Nicolaj Fernwartung"

func dlgChat(msgs []uiChatMsg, draft, note string) *uiDialog {
	if msgs == nil {
		msgs = []uiChatMsg{}
	}
	return &uiDialog{Width: 460, Title: "Nachricht an Nicolaj", Body: "Nicolaj antwortet dir hier – meist noch am selben Tag.",
		Chat: msgs, ChatHint: "Schreib einfach, wobei du Hilfe brauchst. Die Antwort erscheint hier und das Fenster geht von selbst auf.",
		Input:    &uiInput{Placeholder: "Deine Nachricht …", Value: draft, MaxLen: 2000, Multiline: true},
		Progress: -1, Note: note, Closable: true,
		Buttons: []uiBtn{{ID: "close", Label: "Schließen", Kind: "secondary"},
			{ID: "send", Label: "Senden", Kind: "primary", Disabled: strings.TrimSpace(draft) == ""}}}
}
