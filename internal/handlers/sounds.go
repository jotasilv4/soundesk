package handlers

import (
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iv4nz/soundesk/internal/store"
)

type SoundHandler struct {
	store *store.Store
}

func NewSoundHandler(s *store.Store) *SoundHandler {
	return &SoundHandler{store: s}
}

func (h *SoundHandler) Create(c *gin.Context) {
	name := c.PostForm("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	// Create a unique file name
	filename := file.Filename
	ext := filepath.Ext(filename)
	
	uniqueID := uuid.New().String()
	destName := uniqueID + ext
	destPath := filepath.Join("audios", destName)

	if err := c.SaveUploadedFile(file, destPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file: " + err.Error()})
		return
	}

	snd, err := h.store.AddSound(name, filename, destPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add sound to store: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, snd)
}

func (h *SoundHandler) List(c *gin.Context) {
	sounds := h.store.GetSounds()
	c.JSON(http.StatusOK, sounds)
}