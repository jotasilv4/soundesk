package store

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/iv4nz/soundesk/internal/models"
)

type Store struct {
	mu           sync.RWMutex
	sounds       map[string]models.Sound
	sessions     map[string]models.Session
	metadataPath string
	sessionsPath string
	audiosDir    string
}

func NewStore(audiosDir, metadataPath string) (*Store, error) {
	if err := os.MkdirAll(audiosDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create audios directory: %w", err)
	}

	s := &Store{
		sounds:       make(map[string]models.Sound),
		sessions:     make(map[string]models.Session),
		metadataPath: metadataPath,
		sessionsPath: filepath.Join(filepath.Dir(metadataPath), "sessions.json"),
		audiosDir:    audiosDir,
	}

	if err := s.loadMetadata(); err != nil {
		return nil, fmt.Errorf("failed to load metadata: %w", err)
	}

	if err := s.loadSessions(); err != nil {
		return nil, fmt.Errorf("failed to load sessions: %w", err)
	}

	s.ensureDefaultSession()

	return s, nil
}

func (s *Store) AudiosDir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.audiosDir
}

func (s *Store) loadMetadata() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.metadataPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(s.metadataPath)
	if err != nil {
		return err
	}

	var soundsList []models.Sound
	if err := json.Unmarshal(data, &soundsList); err != nil {
		return err
	}

	for _, snd := range soundsList {
		s.sounds[snd.ID] = snd
	}

	return nil
}

func (s *Store) saveMetadata() error {
	var soundsList []models.Sound
	for _, snd := range s.sounds {
		soundsList = append(soundsList, snd)
	}

	data, err := json.MarshalIndent(soundsList, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.metadataPath, data, 0644)
}

func (s *Store) AddSound(name, filename, filePath string) (models.Sound, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snd := models.Sound{
		ID:        uuid.New().String(),
		Name:      name,
		Filename:  filename,
		FilePath:  filePath,
		CreatedAt: time.Now(),
	}

	s.sounds[snd.ID] = snd

	if err := s.saveMetadata(); err != nil {
		return models.Sound{}, err
	}

	return snd, nil
}

func (s *Store) GetSounds() []models.Sound {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]models.Sound, 0, len(s.sounds))
	for _, snd := range s.sounds {
		list = append(list, snd)
	}
	return list
}

func (s *Store) GetSound(id string) (models.Sound, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snd, ok := s.sounds[id]
	return snd, ok
}

func (s *Store) CreateSession(name string) models.Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess := models.Session{
		ID:        uuid.New().String(),
		Name:      name,
		CreatedAt: time.Now(),
	}

	s.sessions[sess.ID] = sess
	_ = s.saveSessions()
	return sess
}

func (s *Store) GetSessions() []models.Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]models.Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		list = append(list, sess)
	}
	return list
}

func (s *Store) GetSession(id string) (models.Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, ok := s.sessions[id]
	return sess, ok
}

func (s *Store) DeleteSound(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	snd, ok := s.sounds[id]
	if !ok {
		return fmt.Errorf("sound not found")
	}

	// Try deleting the actual audio file
	if err := os.Remove(snd.FilePath); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: failed to delete file %s: %v", snd.FilePath, err)
	}

	delete(s.sounds, id)
	return s.saveMetadata()
}

func (s *Store) DeleteSession(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[id]; !ok {
		return false
	}

	delete(s.sessions, id)
	_ = s.saveSessions()
	return true
}

func (s *Store) loadSessions() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.sessionsPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(s.sessionsPath)
	if err != nil {
		return err
	}

	var sessionsList []models.Session
	if err := json.Unmarshal(data, &sessionsList); err != nil {
		return err
	}

	for _, sess := range sessionsList {
		s.sessions[sess.ID] = sess
	}

	return nil
}

func (s *Store) saveSessions() error {
	var sessionsList []models.Session
	for _, sess := range s.sessions {
		sessionsList = append(sessionsList, sess)
	}

	data, err := json.MarshalIndent(sessionsList, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.sessionsPath, data, 0644)
}

func (s *Store) ensureDefaultSession() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.sessions) == 0 {
		sess := models.Session{
			ID:        uuid.New().String(),
			Name:      "Geral",
			CreatedAt: time.Now(),
		}
		s.sessions[sess.ID] = sess
		
		var sessionsList = []models.Session{sess}
		data, err := json.MarshalIndent(sessionsList, "", "  ")
		if err == nil {
			_ = os.WriteFile(s.sessionsPath, data, 0644)
		}
	}
}
