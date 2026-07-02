package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/iv4nz/soundesk/internal/audio"
	"github.com/iv4nz/soundesk/internal/store"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	SessionID string
	Conn      *websocket.Conn
	Send      chan []byte
	IsServer  bool
}

type WSMessage struct {
	Type      string `json:"type"` // "play", "sound_played", "stop", "stop_all", "error", "client_info"
	SoundID   string `json:"sound_id,omitempty"`
	SoundName string `json:"sound_name,omitempty"`
	FilePath  string `json:"file_path,omitempty"`
	IsServer  bool   `json:"is_server,omitempty"`
	Message   string `json:"message,omitempty"`
}

type Hub struct {
	mu         sync.RWMutex
	sessions   map[string]map[*Client]bool
	store      *store.Store
	register   chan *Client
	unregister chan *Client
}

func NewHub(s *store.Store) *Hub {
	h := &Hub{
		sessions:   make(map[string]map[*Client]bool),
		store:      s,
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
	go h.run()
	return h
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.sessions[client.SessionID] == nil {
				h.sessions[client.SessionID] = make(map[*Client]bool)
			}
			h.sessions[client.SessionID][client] = true
			log.Printf("[WS HUB] Client connected to session %s (Server: %v). Total clients in session: %d", 
				client.SessionID, client.IsServer, len(h.sessions[client.SessionID]))
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.sessions[client.SessionID]; ok {
				if _, exists := clients[client]; exists {
					delete(clients, client)
					close(client.Send)
					client.Conn.Close()
					log.Printf("[WS HUB] Client disconnected from session %s. Remaining: %d", 
						client.SessionID, len(clients))
					if len(clients) == 0 {
						delete(h.sessions, client.SessionID)
					}
				}
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request, sessionID string, isServer bool) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS] Upgrade failed: %v", err)
		return
	}

	client := &Client{
		SessionID: sessionID,
		Conn:      conn,
		Send:      make(chan []byte, 256),
		IsServer:  isServer,
	}

	h.register <- client

	go client.writePump()
	go client.readPump(h)
}

func (c *Client) readPump(h *Hub) {
	defer func() {
		h.unregister <- c
	}()

	c.Conn.SetReadLimit(512)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error { 
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil 
	})

	for {
		var msg WSMessage
		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[WS] Read error: %v", err)
			}
			break
		}

		if msg.Type == "play" {
			snd, ok := h.store.GetSound(msg.SoundID)
			if !ok {
				c.SendJSON(WSMessage{Type: "error", Message: "Sound not found"})
				continue
			}

			// Play the sound on the server host CLI
			err := audio.Play(snd.FilePath)
			if err != nil {
				log.Printf("[WS] Error playing sound on server CLI: %v", err)
			}

			// Broadcast event to all clients in the session
			h.BroadcastToSession(c.SessionID, WSMessage{
				Type:      "sound_played",
				SoundID:   snd.ID,
				SoundName: snd.Name,
				FilePath:  snd.FilePath,
				IsServer:  c.IsServer,
			})
		} else if msg.Type == "stop" {
			// Stop all running system command playbacks
			audio.StopAll()

			// Broadcast event to all clients in the session to stop HTML5 players
			h.BroadcastToSession(c.SessionID, WSMessage{
				Type:     "stop_all",
				IsServer: c.IsServer,
			})
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) SendJSON(msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[WS] Error encoding message: %v", err)
		return
	}
	select {
	case c.Send <- data:
	default:
	}
}

func (h *Hub) BroadcastToSession(sessionID string, msg WSMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients, ok := h.sessions[sessionID]
	if !ok {
		return
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[WS] Error broadcasting message: %v", err)
		return
	}

	for client := range clients {
		select {
		case client.Send <- data:
		default:
		}
	}
}

func (h *Hub) CloseSession(sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	clients, ok := h.sessions[sessionID]
	if !ok {
		return
	}

	for client := range clients {
		client.SendJSON(WSMessage{Type: "error", Message: "Sessão encerrada pelo administrador."})
		client.Conn.Close()
	}
	delete(h.sessions, sessionID)
}
