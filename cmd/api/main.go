package main

import (
	"embed"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/iv4nz/soundesk/audios"
	"github.com/iv4nz/soundesk/internal/handlers"
	"github.com/iv4nz/soundesk/internal/store"
	"github.com/iv4nz/soundesk/internal/websocket"
	"github.com/iv4nz/soundesk/web"
)

func main() {
	audiosDir := "audios"
	metadataPath := "audios/metadata.json"

	// If running on Vercel (read-only filesystem), use /tmp for writable storage and copy initial assets
	if os.Getenv("VERCEL") == "1" {
		audiosDir = "/tmp/audios"
		metadataPath = "/tmp/audios/metadata.json"
		if err := copyEmbeddedDir(audios.FS, ".", "/tmp/audios"); err != nil {
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
	if os.Getenv("VERCEL") == "1" {
		r.StaticFS("/web", http.FS(web.FS))
		r.GET("/", func(c *gin.Context) {
			c.FileFromFS("index.html", http.FS(web.FS))
		})
	} else {
		r.Static("/web", "./web")
		r.StaticFile("/", "./web/index.html")
	}
	
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

func copyEmbeddedDir(embedFS embed.FS, srcDir, dstDir string) error {
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}
	entries, err := embedFS.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		srcPath := srcDir + "/" + entry.Name()
		dstPath := filepath.Join(dstDir, entry.Name())
		
		data, err := embedFS.ReadFile(srcPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			return err
		}
	}
	return nil
}