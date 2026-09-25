package main

import (
	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"coal-governance-backend/config"
	"coal-governance-backend/database"
	"coal-governance-backend/routes"
)

func main() {
	cfg := config.Load()

	if err := database.Connect(cfg.DatabaseURL); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.DB.Close()

	router := gin.Default()

	// CORS is intentionally permissive for local hackathon demo purposes
	// (frontend served statically from a different port). Tighten
	// AllowOrigins for any real deployment.
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	routes.RegisterRoutes(router, cfg)

	// Serve uploaded evidence photos and documents statically
	router.Static("/uploads", "./uploads")

	// Serve frontend directly from Go backend for single-port convenience
	router.StaticFile("/", "../frontend/index.html")
	router.StaticFile("/index.html", "../frontend/index.html")
	router.StaticFile("/login.html", "../frontend/login.html")
	router.StaticFile("/dashboard.html", "../frontend/dashboard.html")
	router.StaticFile("/mines.html", "../frontend/mines.html")
	router.StaticFile("/inspections.html", "../frontend/inspections.html")
	router.StaticFile("/violations.html", "../frontend/violations.html")
	router.StaticFile("/analytics.html", "../frontend/analytics.html")
	router.StaticFile("/corrective-actions.html", "../frontend/corrective-actions.html")
	router.StaticFile("/compliance.html", "../frontend/compliance.html")
	router.StaticFile("/documents.html", "../frontend/documents.html")
	router.StaticFile("/attendance.html", "../frontend/attendance.html")
	router.StaticFile("/audit-logs.html", "../frontend/audit-logs.html")
	router.StaticFile("/contractors.html", "../frontend/contractors.html")
	router.StaticFile("/environmental.html", "../frontend/environmental.html")
	router.StaticFile("/grievances.html", "../frontend/grievances.html")
	router.StaticFile("/incidents.html", "../frontend/incidents.html")
	router.StaticFile("/notifications.html", "../frontend/notifications.html")
	router.StaticFile("/production.html", "../frontend/production.html")
	router.StaticFile("/reports.html", "../frontend/reports.html")
	router.StaticFile("/simulation-control.html", "../frontend/simulation-control.html")
	router.StaticFile("/users.html", "../frontend/users.html")
	router.StaticFile("/manifest.json", "../frontend/manifest.json")
	router.StaticFile("/service-worker.js", "../frontend/service-worker.js")
	router.StaticFile("/favicon.ico", "../frontend/icons/icon-192x192.png")
	router.Static("/js", "../frontend/js")
	router.Static("/css", "../frontend/css")
	router.Static("/icons", "../frontend/icons")
	router.Static("/assets", "../frontend/assets")

	log.Printf("Coal Governance backend starting on port %s\n", cfg.AppPort)
	if err := router.Run(":" + cfg.AppPort); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}