package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iv4nz/soundesk/internal/audio"
	"github.com/iv4nz/soundesk/internal/store"
	"github.com/iv4nz/soundesk/internal/websocket"
)

type SessionHandler struct {
	store *store.Store
	hub   *websocket.Hub
}

func NewSessionHandler(s *store.Store, h *websocket.Hub) *SessionHandler {
	return &SessionHandler{
		store: s,
		hub:   h,
	}
}

type CreateSessionInput struct {
	Name string `json:"name" binding:"required"`
}

type PlaySoundInput struct {
	SoundID string `json:"sound_id" binding:"required"`
}

func (h *SessionHandler) Create(c *gin.Context) {
	var input CreateSessionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		// Fallback to query param or default name if body is empty or malformed
		name := c.Query("name")
		if name == "" {
			name = "Nova Sessão"
		}
		input.Name = name
	}

	sess := h.store.CreateSession(input.Name)
	c.JSON(http.StatusCreated, sess)
}

func (h *SessionHandler) List(c *gin.Context) {
	sessions := h.store.GetSessions()
	c.JSON(http.StatusOK, sessions)
}

func (h *SessionHandler) Connect(c *gin.Context) {
	sessionID := c.Param("id")
	_, ok := h.store.GetSession(sessionID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	role := c.Query("role")
	isServer := role == "server"

	h.hub.HandleWS(c.Writer, c.Request, sessionID, isServer)
}

func (h *SessionHandler) Play(c *gin.Context) {
	sessionID := c.Param("id")
	_, ok := h.store.GetSession(sessionID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	var input PlaySoundInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	snd, ok := h.store.GetSound(input.SoundID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "sound not found"})
		return
	}

	// Play on the server host CLI
	if err := audio.Play(snd.FilePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to play audio: " + err.Error()})
		return
	}

	// Broadcast sound played event to all clients connected to this session
	h.hub.BroadcastToSession(sessionID, websocket.WSMessage{
		Type:      "sound_played",
		SoundID:   snd.ID,
		SoundName: snd.Name,
		FilePath:  snd.FilePath,
		IsServer:  false, // Rest trigger
	})

	c.JSON(http.StatusOK, gin.H{"status": "playing", "sound": snd})
}

func (h *SessionHandler) Stop(c *gin.Context) {
	sessionID := c.Param("id")
	_, ok := h.store.GetSession(sessionID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	// Stop CLI playbacks
	audio.StopAll()

	// Broadcast stop event to all WS clients
	h.hub.BroadcastToSession(sessionID, websocket.WSMessage{
		Type:     "stop_all",
		IsServer: false,
	})

	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}
