package main

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/iv4nz/soundesk/internal/handlers"
	"github.com/iv4nz/soundesk/internal/store"
	"github.com/iv4nz/soundesk/internal/websocket"
)

func main() {
	audiosDir := "audios"
	metadataPath := "audios/metadata.json"

	// If running on Vercel (read-only filesystem), use /tmp for writable storage and copy initial assets
	if os.Getenv("VERCEL") == "1" {
		audiosDir = "/tmp/audios"
		metadataPath = "/tmp/audios/metadata.json"
		if err := copyDir("audios", "/tmp/audios"); err != nil {
			log.Printf("Warning: failed to copy initial audios to /tmp: %v", err)
		}
	}

	// Initialize Store
	s, err := store.NewStore(audiosDir, metadataPath)
	if err != nil {
		log.Fatalf("failed to initialize store: %v", err)
	}

	// Initialize Websocket Hub
	hub := websocket.NewHub(s)

	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())

	// Serve Frontend Static Files
	r.Static("/web", "./web")
	r.StaticFile("/", "./web/index.html")
	
	// Serve Audio Files Statically so the browser can play them
	r.Static("/audios", audiosDir)

	// API Group
	v1 := r.Group("/api/v1")

	// Sounds Routes
	soundHandler := handlers.NewSoundHandler(s)
	sounds := v1.Group("/sounds")
	{
		sounds.POST("", soundHandler.Create)
		sounds.GET("", soundHandler.List)
	}

	// Sessions Routes
	sessionHandler := handlers.NewSessionHandler(s, hub)
	sessions := v1.Group("/sessions")
	{
		sessions.POST("", sessionHandler.Create)
		sessions.GET("", sessionHandler.List)
		sessions.GET("/:id/ws", sessionHandler.Connect)
		sessions.POST("/:id/play", sessionHandler.Play)
		sessions.POST("/:id/stop", sessionHandler.Stop)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := port
	if len(addr) > 0 && addr[0] != ':' {
		addr = ":" + addr
	}

	log.Printf("SounDesk server starting on %s...", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}

func copyDir(src string, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		
		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}