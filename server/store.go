package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Admin ist ein Benutzer des Dashboards (Techniker).
type Admin struct {
	Username     string    `json:"username"`
	PasswordHash string    `json:"passwordHash"`
	TOTPSecret   string    `json:"totpSecret,omitempty"`
	TOTPEnabled  bool      `json:"totpEnabled"`
	TOTPLastStep int64     `json:"totpLastStep,omitempty"`
	TOTPPending  string    `json:"totpPending,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

// DataSnapshot ist der zuletzt vom Agent gemeldete Stand einer Liste (Benutzer, Software …).
type DataSnapshot struct {
	At    time.Time       `json:"at"`
	Value json.RawMessage `json:"value"`
}

// Device ist ein verwalteter PC.
type Device struct {
	ID              string                  `json:"id"`
	PCID            string                  `json:"pcId"` // 9-stellige, gut vorlesbare PC-ID
	PausedUntil     *time.Time              `json:"protectionPausedUntil,omitempty"`
	PausedAt        *time.Time              `json:"protectionPausedAt,omitempty"`
	Name            string                  `json:"name"`
	Hostname        string                  `json:"hostname"`
	Notes           string                  `json:"notes"`
	ConsentRequired bool                    `json:"consentRequired"`
	SecurityEnabled bool                    `json:"securityEnabled"`
	UpdateTester    bool                    `json:"updateTester"`
	Group           string                  `json:"group,omitempty"`   // z. B. „Familie“, „Kunden“
	Pending         bool                    `json:"pending,omitempty"` // Zugangsanfrage: wartet auf Freigabe
	RequestedAt     *time.Time              `json:"requestedAt,omitempty"`
	AgentTokenHash  string                  `json:"agentTokenHash"`
	ScreenTokenHash string                  `json:"screenTokenHash"`
	AgentVersion    string                  `json:"agentVersion"`
	EnrolledAt      time.Time               `json:"enrolledAt"`
	LastSeen        time.Time               `json:"lastSeen"`
	LastIP          string                  `json:"lastIp"`
	Inventory       json.RawMessage         `json:"inventory,omitempty"`
	InventoryAt     time.Time               `json:"inventoryAt"`
	Data            map[string]DataSnapshot `json:"data,omitempty"`
}

// EnrollCode ist ein Einmal-Code, mit dem sich ein neuer PC anmeldet.
type EnrollCode struct {
	Code      string     `json:"code"`
	Label     string     `json:"label"`
	CreatedBy string     `json:"createdBy"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt time.Time  `json:"expiresAt"`
	UsedAt    *time.Time `json:"usedAt,omitempty"`
	DeviceID  string     `json:"deviceId,omitempty"`
}

// Command ist ein Auftrag an einen PC. Geheime Felder (Passwörter) werden hier nie gespeichert.
type Command struct {
	ID        string          `json:"id"`
	DeviceID  string          `json:"deviceId"`
	Type      string          `json:"type"`
	Summary   string          `json:"summary"`
	Args      json.RawMessage `json:"args"`
	Status    string          `json:"status"` // queued, sent, done, failed, timeout, canceled, expired
	Output    string          `json:"output"`
	CreatedBy string          `json:"createdBy"`
	CreatedAt time.Time       `json:"createdAt"`
	SentAt    *time.Time      `json:"sentAt,omitempty"`
	DoneAt    *time.Time      `json:"doneAt,omitempty"`
	Timeout   int             `json:"timeout"`
}

func (c *Command) Final() bool {
	switch c.Status {
	case "queued", "sent":
		return false
	}
	return true
}

// AuditEntry protokolliert jede Aktion im Dashboard.
type AuditEntry struct {
	Time     time.Time `json:"time"`
	User     string    `json:"user"`
	IP       string    `json:"ip"`
	DeviceID string    `json:"deviceId,omitempty"`
	Action   string    `json:"action"`
	Detail   string    `json:"detail"`
}

type SessionRec struct {
	TokenHash string    `json:"tokenHash"`
	Username  string    `json:"username"`
	Created   time.Time `json:"created"`
	Expires   time.Time `json:"expires"`
}

type DBData struct {
	Admins      []*Admin              `json:"admins"`
	Devices     map[string]*Device    `json:"devices"`
	EnrollCodes []*EnrollCode         `json:"enrollCodes"`
	Commands    []*Command            `json:"commands"`
	Audit       []*AuditEntry         `json:"audit"`
	Sessions    []*SessionRec         `json:"sessions"`
	Settings    *Settings             `json:"settings"`
	Rejected    map[string]time.Time  `json:"rejected,omitempty"` // abgelehnte Anfragen: Agent entfernt sich beim nächsten Kontakt
	Scripts     []*Script             `json:"scripts,omitempty"`  // gespeicherte PowerShell-Skripte
	Chats       map[string][]*ChatMsg `json:"chats,omitempty"`    // Nachrichten pro Gerät
}

type ChatMsg struct {
	ID     string    `json:"id"`
	From   string    `json:"from"` // "pc" oder "admin"
	Author string    `json:"author"`
	Text   string    `json:"text"`
	At     time.Time `json:"at"`
	Read   bool      `json:"read"` // von der Gegenseite gelesen
}

type Script struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Body      string    `json:"script"`
	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
}

const (
	maxCommands = 3000
	maxAudit    = 10000
)

// Store hält alle Daten im Speicher und schreibt sie verzögert als JSON auf die Platte.
type Store struct {
	mu    sync.Mutex
	path  string
	d     *DBData
	dirty chan struct{}
}

func OpenStore(path string) (*Store, error) {
	s := &Store{path: path, d: &DBData{}, dirty: make(chan struct{}, 1)}
	b, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(b, s.d); err != nil {
			return nil, fmt.Errorf("%s ist beschädigt: %w", path, err)
		}
	case !os.IsNotExist(err):
		return nil, err
	}
	if s.d.Devices == nil {
		s.d.Devices = map[string]*Device{}
	}
	if s.d.Settings == nil {
		s.d.Settings = &Settings{AutoUpdate: true, TestFirst: true, GitHubAuto: true}
	}
	go s.writer()
	return s, nil
}

func (s *Store) View(fn func(d *DBData)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(s.d)
}

func (s *Store) Update(fn func(d *DBData)) {
	s.mu.Lock()
	fn(s.d)
	if n := len(s.d.Commands); n > maxCommands {
		s.d.Commands = append([]*Command(nil), s.d.Commands[n-maxCommands:]...)
	}
	if n := len(s.d.Audit); n > maxAudit {
		s.d.Audit = append([]*AuditEntry(nil), s.d.Audit[n-maxAudit:]...)
	}
	s.mu.Unlock()
	select {
	case s.dirty <- struct{}{}:
	default:
	}
}

func (s *Store) writer() {
	for range s.dirty {
		time.Sleep(400 * time.Millisecond)
		if err := s.Flush(); err != nil {
			fmt.Fprintln(os.Stderr, "Speichern fehlgeschlagen:", err)
		}
	}
}

func (s *Store) Flush() error {
	s.mu.Lock()
	b, err := json.MarshalIndent(s.d, "", " ")
	s.mu.Unlock()
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (d *DBData) FindCommand(id string) *Command {
	for i := len(d.Commands) - 1; i >= 0; i-- {
		if d.Commands[i].ID == id {
			return d.Commands[i]
		}
	}
	return nil
}

func (d *DBData) FindAdmin(name string) *Admin {
	for _, a := range d.Admins {
		if a.Username == name {
			return a
		}
	}
	return nil
}

// newPCID vergibt eine eindeutige 9-stellige PC-ID (wie bei TeamViewer, gut am Telefon vorlesbar).
func (d *DBData) newPCID() string {
	for {
		b := randBytes(4)
		n := (uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])) % 900000000
		id := fmt.Sprintf("%09d", n+100000000)
		taken := false
		for _, dev := range d.Devices {
			if dev.PCID == id {
				taken = true
				break
			}
		}
		if !taken {
			return id
		}
	}
}
