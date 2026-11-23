package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Conn ist eine offene Server-Sent-Events-Verbindung (Agent, Bildschirm-Helfer oder Dashboard).
type Conn struct {
	ch     chan []byte
	closed chan struct{}
	once   sync.Once
}

func newConn(buf int) *Conn { return &Conn{ch: make(chan []byte, buf), closed: make(chan struct{})} }

func (c *Conn) Close() { c.once.Do(func() { close(c.closed) }) }

func (c *Conn) Send(msg []byte) bool {
	select {
	case <-c.closed:
		return false
	default:
	}
	select {
	case c.ch <- msg:
		return true
	default:
		return false
	}
}

func sseMsg(event string, v any) []byte {
	b, _ := json.Marshal(v)
	return []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event, b))
}

// ScreenSession ist eine laufende Bildschirm-Verbindung zu einem PC.
type ScreenSession struct {
	ID         string
	DeviceID   string
	State      string // requesting, active
	Quality    string
	By         string
	Message    string
	StartedAt  time.Time
	W, H       int
	frame      []byte
	viewers    map[chan []byte]struct{}
	lastViewer time.Time
}

func (s *ScreenSession) Info() map[string]any {
	if s == nil {
		return nil
	}
	return map[string]any{
		"id": s.ID, "state": s.State, "quality": s.Quality, "by": s.By, "message": s.Message,
		"startedAt": s.StartedAt, "w": s.W, "h": s.H, "viewers": len(s.viewers),
	}
}

type Hub struct {
	mu       sync.Mutex
	agents   map[string]*Conn
	screens  map[string]*Conn
	dash     map[*Conn]struct{}
	sessions map[string]*ScreenSession
	secrets  map[string]json.RawMessage
}

func NewHub() *Hub {
	return &Hub{
		agents: map[string]*Conn{}, screens: map[string]*Conn{}, dash: map[*Conn]struct{}{},
		sessions: map[string]*ScreenSession{}, secrets: map[string]json.RawMessage{},
	}
}

func (h *Hub) roleMap(role string) map[string]*Conn {
	if role == "screen" {
		return h.screens
	}
	return h.agents
}

func (h *Hub) RegisterAgent(devID, role string) *Conn {
	c := newConn(512)
	h.mu.Lock()
	m := h.roleMap(role)
	if old := m[devID]; old != nil {
		old.Close()
	}
	m[devID] = c
	h.mu.Unlock()
	return c
}

// UnregisterAgent meldet eine Verbindung ab. Gibt true zurück, wenn dabei eine
// Bildschirm-Sitzung beendet wurde.
func (h *Hub) UnregisterAgent(devID, role string, c *Conn) (endedSession bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	m := h.roleMap(role)
	if m[devID] != c {
		return false
	}
	delete(m, devID)
	c.Close()
	if role == "screen" {
		if s := h.sessions[devID]; s != nil {
			h.endSessionLocked(devID)
			return true
		}
	}
	return false
}

func (h *Hub) DropDevice(devID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, m := range []map[string]*Conn{h.agents, h.screens} {
		if c := m[devID]; c != nil {
			c.Close()
			delete(m, devID)
		}
	}
	h.endSessionLocked(devID)
}

func (h *Hub) Online(devID string) (agent, screen bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.agents[devID] != nil, h.screens[devID] != nil
}

type liveState struct {
	Agent, Screen bool
	Session       map[string]any
}

func (h *Hub) Live() map[string]liveState {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := map[string]liveState{}
	for id := range h.agents {
		ls := out[id]
		ls.Agent = true
		out[id] = ls
	}
	for id := range h.screens {
		ls := out[id]
		ls.Screen = true
		out[id] = ls
	}
	for id, s := range h.sessions {
		ls := out[id]
		ls.Session = s.Info()
		out[id] = ls
	}
	return out
}

func (h *Hub) SendToAgent(devID, role, event string, payload any) bool {
	h.mu.Lock()
	c := h.roleMap(role)[devID]
	h.mu.Unlock()
	if c == nil {
		return false
	}
	if !c.Send(sseMsg(event, payload)) {
		c.Close() // Puffer voll: Verbindung neu aufbauen lassen
		return false
	}
	return true
}

// ---------- Dashboard ----------

func (h *Hub) AddDash() *Conn {
	c := newConn(256)
	h.mu.Lock()
	h.dash[c] = struct{}{}
	h.mu.Unlock()
	return c
}

func (h *Hub) RemoveDash(c *Conn) {
	h.mu.Lock()
	delete(h.dash, c)
	h.mu.Unlock()
	c.Close()
}

func (h *Hub) Broadcast(event string, v any) {
	msg := sseMsg(event, v)
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.dash {
		c.Send(msg)
	}
}

// ---------- Geheimnisse (Passwörter) nur im Speicher ----------

func (h *Hub) PutSecret(cmdID string, full json.RawMessage) {
	h.mu.Lock()
	h.secrets[cmdID] = full
	h.mu.Unlock()
}

func (h *Hub) TakeSecret(cmdID string, remove bool) json.RawMessage {
	h.mu.Lock()
	defer h.mu.Unlock()
	v := h.secrets[cmdID]
	if remove {
		delete(h.secrets, cmdID)
	}
	return v
}

// ---------- Bildschirm-Sitzungen ----------

func (h *Hub) Session(devID string) map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sessions[devID].Info()
}

func (h *Hub) StartSession(devID, quality, by string) (s map[string]any, isNew bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if cur := h.sessions[devID]; cur != nil {
		return cur.Info(), false
	}
	ss := &ScreenSession{
		ID: randHex(8), DeviceID: devID, State: "requesting", Quality: quality, By: by,
		StartedAt: time.Now(), viewers: map[chan []byte]struct{}{}, lastViewer: time.Now(),
	}
	h.sessions[devID] = ss
	return ss.Info(), true
}

func (h *Hub) SetQuality(devID, sessionID, q string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.sessions[devID]
	if s == nil || s.ID != sessionID {
		return false
	}
	s.Quality = q
	return true
}

func (h *Hub) endSessionLocked(devID string) {
	s := h.sessions[devID]
	if s == nil {
		return
	}
	for ch := range s.viewers {
		close(ch)
	}
	s.viewers = nil
	delete(h.sessions, devID)
}

func (h *Hub) EndSession(devID, sessionID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.sessions[devID]
	if s == nil || (sessionID != "" && s.ID != sessionID) {
		return false
	}
	h.endSessionLocked(devID)
	return true
}

func (h *Hub) UpdateSession(devID, sessionID, state, msg string, w, hh int) (bool, map[string]any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.sessions[devID]
	if s == nil || s.ID != sessionID {
		return false, nil
	}
	s.State, s.Message = state, msg
	if w > 0 && hh > 0 {
		s.W, s.H = w, hh
	}
	return true, s.Info()
}

func (h *Hub) ActiveSessionID(devID string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s := h.sessions[devID]; s != nil && s.State == "active" {
		return s.ID
	}
	return ""
}

// PushFrame verteilt ein neues Bild an alle Zuschauer. false = Sitzung gibt es nicht mehr.
func (h *Hub) PushFrame(devID, sessionID string, frame []byte, w, hh int) (ok, becameActive bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.sessions[devID]
	if s == nil || s.ID != sessionID {
		return false, false
	}
	if s.State != "active" {
		s.State, becameActive = "active", true
	}
	if w > 0 && hh > 0 {
		s.W, s.H = w, hh
	}
	s.frame = frame
	for ch := range s.viewers {
		select {
		case ch <- frame:
		default:
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- frame:
			default:
			}
		}
	}
	return true, becameActive
}

func (h *Hub) AddViewer(devID, sessionID string) (chan []byte, []byte, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.sessions[devID]
	if s == nil || s.ID != sessionID {
		return nil, nil, false
	}
	ch := make(chan []byte, 1)
	s.viewers[ch] = struct{}{}
	s.lastViewer = time.Now()
	return ch, s.frame, true
}

func (h *Hub) RemoveViewer(devID string, ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.sessions[devID]
	if s == nil || s.viewers == nil {
		return
	}
	if _, ok := s.viewers[ch]; ok {
		delete(s.viewers, ch)
		s.lastViewer = time.Now()
	}
}

// ExpireSessions beendet Sitzungen ohne Antwort oder ohne Zuschauer.
func (h *Hub) ExpireSessions() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var ended []string
	for id, s := range h.sessions {
		noAnswer := s.State == "requesting" && time.Since(s.StartedAt) > 90*time.Second
		noViewer := len(s.viewers) == 0 && time.Since(s.lastViewer) > 30*time.Second
		if noAnswer || noViewer {
			h.endSessionLocked(id)
			ended = append(ended, id)
		}
	}
	return ended
}
