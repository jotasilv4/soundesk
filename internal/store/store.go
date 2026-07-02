package store

import (
	"encoding/json"
	"fmt"
	"os"
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
		audiosDir:    audiosDir,
	}

	if err := s.loadMetadata(); err != nil {
		return nil, fmt.Errorf("failed to load metadata: %w", err)
	}

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
