package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/iv4nz/soundesk/internal/handlers"
	"github.com/iv4nz/soundesk/internal/store"
	"github.com/iv4nz/soundesk/internal/websocket"
)

func main() {
	// Initialize Store
	s, err := store.NewStore("audios", "audios/metadata.json")
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
	r.Static("/audios", "./audios")

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