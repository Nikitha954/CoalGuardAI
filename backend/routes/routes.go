package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"coal-governance-backend/config"
	"coal-governance-backend/controllers"
	"coal-governance-backend/middleware"
	"coal-governance-backend/models"
)

// RegisterRoutes wires every REST endpoint defined in the API spec, applying
// JWT authentication and role-based access control (RBAC) as required.
func RegisterRoutes(router *gin.Engine, cfg *config.Config) {
	authController := controllers.NewAuthController(cfg)
	mineController := controllers.NewMineController()
	dashboardController := controllers.NewDashboardController()
	complianceController := controllers.NewComplianceController()
	inspectionController := controllers.NewInspectionsController(cfg)
	violationController := controllers.NewViolationsController()
	correctiveActionsController := controllers.NewCorrectiveActionsController()
	incidentsController := controllers.NewIncidentsController()
	documentsController := controllers.NewDocumentsController(cfg)
	notificationsController := controllers.NewNotificationsController()
	simulationController := controllers.NewSimulationController()
	reportsController := controllers.NewReportsController()
	analyticsController := controllers.NewAnalyticsController(cfg)
	attendanceController := controllers.NewAttendanceController()
	grievanceController := controllers.NewGrievanceController()
	auditController := controllers.NewAuditController()
	contractorController := controllers.NewContractorController()
	environmentalController := controllers.NewEnvironmentalController()
	operationalController := controllers.NewOperationalController()

	// Health check (unauthenticated) — useful for docker-compose healthchecks.
	router.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "coal-governance backend is running"})
	})

	api := router.Group("/api")
	{
		// ---------------- AUTH (public) ----------------
		auth := api.Group("/auth")
		{
			auth.POST("/login", authController.Login)
		}

		// ---------------- PROTECTED ROUTES ----------------
		protected := api.Group("/")
		protected.Use(middleware.AuthRequired(cfg))
		{
			// Authentication & User Management
			protected.POST("/auth/logout", authController.Logout)
			protected.GET("/auth/me", authController.Me)
			protected.GET("/users", authController.ListUsers)
			protected.POST("/users", middleware.RequireRoles(models.RoleSuperAdmin), authController.CreateUser)
			protected.PUT("/users/:id", middleware.RequireRoles(models.RoleSuperAdmin), authController.UpdateUser)
			protected.DELETE("/users/:id", middleware.RequireRoles(models.RoleSuperAdmin), authController.DeactivateUser)
			protected.GET("/roles", authController.ListRoles)

			// ---------------- MINES ----------------
			mines := protected.Group("/mines")
			{
				mines.GET("", mineController.ListMines)
				mines.GET("/:id", mineController.GetMine)
				mines.POST("", middleware.RequireRoles(models.RoleSuperAdmin), mineController.CreateMine)
				mines.PUT("/:id", middleware.RequireRoles(models.RoleSuperAdmin), mineController.UpdateMine)
				mines.DELETE("/:id", middleware.RequireRoles(models.RoleSuperAdmin), mineController.DeactivateMine)
			}
			protected.GET("/subsidiaries", mineController.ListSubsidiaries)

			// ---------------- ATTENDANCE & WORKERS ----------------
			attendance := protected.Group("/attendance")
			{
				attendance.GET("", attendanceController.ListAttendance)
				attendance.POST("", middleware.RequireRoles(models.RoleMineManager, models.RoleSafetyOfficer, models.RoleSuperAdmin), attendanceController.MarkAttendance)
				attendance.GET("/report", attendanceController.GetAttendanceReport)
			}
			protected.GET("/workers", attendanceController.ListWorkers)
			protected.POST("/workers", middleware.RequireRoles(models.RoleMineManager, models.RoleSafetyOfficer, models.RoleSuperAdmin), attendanceController.CreateWorker)

			// ---------------- GRIEVANCES ----------------
			grievances := protected.Group("/grievances")
			{
				grievances.GET("", grievanceController.ListGrievances)
				grievances.GET("/:id", grievanceController.GetGrievance)
				grievances.POST("", grievanceController.SubmitGrievance)
				grievances.PUT("/:id/assign", middleware.RequireRoles(models.RoleMineManager, models.RoleSafetyOfficer, models.RoleSuperAdmin), grievanceController.AssignGrievance)
				grievances.PUT("/:id/resolve", middleware.RequireRoles(models.RoleMineManager, models.RoleSafetyOfficer, models.RoleSuperAdmin), grievanceController.ResolveGrievance)
				grievances.PUT("/:id/escalate", middleware.RequireRoles(models.RoleMineManager, models.RoleSafetyOfficer, models.RoleInspector, models.RoleSuperAdmin), grievanceController.EscalateGrievance)
			}

			// ---------------- CONTRACTORS ----------------
			contractors := protected.Group("/contractors")
			{
				contractors.GET("", contractorController.ListContractors)
				contractors.POST("", middleware.RequireRoles(models.RoleSuperAdmin, models.RoleMineManager), contractorController.CreateContractor)
				contractors.PUT("/:id", middleware.RequireRoles(models.RoleSuperAdmin, models.RoleMineManager), contractorController.UpdateContractor)
				contractors.PUT("/:id/blacklist", middleware.RequireRoles(models.RoleSuperAdmin, models.RoleMineManager, models.RoleSafetyOfficer), contractorController.BlacklistContractor)
				contractors.POST("/check-expiries", middleware.RequireRoles(models.RoleSuperAdmin, models.RoleMineManager, models.RoleSafetyOfficer), contractorController.CheckContractExpiries)
			}

			// ---------------- ENVIRONMENTAL ----------------
			environmental := protected.Group("/environmental")
			{
				environmental.GET("", environmentalController.ListEnvironmentalData)
				environmental.POST("", middleware.RequireRoles(models.RoleSafetyOfficer, models.RoleInspector, models.RoleSuperAdmin), environmentalController.LogEnvironmentalReading)
			}

			// ---------------- OPERATIONAL / PRODUCTION ----------------
			operational := protected.Group("/operational")
			{
				operational.GET("", operationalController.ListOperationalData)
				operational.POST("", middleware.RequireRoles(models.RoleMineManager, models.RoleSuperAdmin), operationalController.LogOperationalData)
			}

			// ---------------- AUDIT TRAIL ----------------
			audit := protected.Group("/audit")
			{
				audit.GET("", middleware.RequireRoles(models.RoleSuperAdmin, models.RoleRegulatoryOffice, models.RoleCorporateManager), auditController.ListAuditLogs)
				audit.GET("/verify", middleware.RequireRoles(models.RoleSuperAdmin, models.RoleRegulatoryOffice, models.RoleCorporateManager), auditController.VerifyAuditChain)
			}

			// ---------------- COMPLIANCE ----------------
			compliance := protected.Group("/compliance")
			{
				compliance.GET("/rules", complianceController.ListRules)
				compliance.GET("/categories", complianceController.ListCategories)
				compliance.POST("/rules", middleware.RequireRoles(models.RoleSuperAdmin), complianceController.CreateRule)
				compliance.PUT("/rules/:id", middleware.RequireRoles(models.RoleSuperAdmin), complianceController.UpdateRule)
				compliance.DELETE("/rules/:id", middleware.RequireRoles(models.RoleSuperAdmin), complianceController.DeactivateRule)
			}

			// ---------------- INSPECTIONS ----------------
			inspections := protected.Group("/inspections")
			{
				inspections.GET("", inspectionController.ListInspections)
				inspections.GET("/:id", inspectionController.GetInspection)
				inspections.POST("", middleware.RequireRoles(models.RoleInspector, models.RoleSafetyOfficer, models.RoleSuperAdmin), inspectionController.CreateInspection)
				inspections.PUT("/:id/status", middleware.RequireRoles(models.RoleMineManager, models.RoleSafetyOfficer, models.RoleSuperAdmin), inspectionController.UpdateInspectionStatus)
				inspections.POST("/analyze-draft", inspectionController.AnalyzeInspectionDraft)
				inspections.POST("/:id/analyze", inspectionController.AnalyzeInspection)
			}

			// ---------------- VIOLATIONS ----------------
			violations := protected.Group("/violations")
			{
				violations.GET("", violationController.ListViolations)
				violations.GET("/:id", violationController.GetViolation)
				violations.POST("", middleware.RequireRoles(models.RoleSafetyOfficer, models.RoleSuperAdmin), violationController.CreateViolation)
				violations.PUT("/:id", middleware.RequireRoles(models.RoleMineManager, models.RoleSafetyOfficer, models.RoleSuperAdmin), violationController.UpdateViolation)
			}

			// ---------------- CORRECTIVE ACTIONS ----------------
			correctiveActions := protected.Group("/corrective-actions")
			{
				correctiveActions.GET("", correctiveActionsController.ListCorrectiveActions)
				correctiveActions.POST("", middleware.RequireRoles(models.RoleMineManager, models.RoleSafetyOfficer, models.RoleSuperAdmin), correctiveActionsController.CreateCorrectiveAction)
				correctiveActions.PUT("/:id/submit", correctiveActionsController.SubmitAction)
				correctiveActions.PUT("/:id/verify", middleware.RequireRoles(models.RoleMineManager, models.RoleSafetyOfficer, models.RoleSuperAdmin), correctiveActionsController.VerifyAction)
				correctiveActions.POST("/check-escalations", correctiveActionsController.TriggerEscalationCheck)
			}

			// ---------------- INCIDENTS ----------------
			incidents := protected.Group("/incidents")
			{
				incidents.GET("", incidentsController.ListIncidents)
				incidents.POST("", incidentsController.CreateIncident)
				incidents.POST("/emergency", incidentsController.TriggerEmergency)
				incidents.PUT("/:id/status", middleware.RequireRoles(models.RoleMineManager, models.RoleSafetyOfficer, models.RoleSuperAdmin), incidentsController.UpdateIncidentStatus)
			}

			// ---------------- DOCUMENTS & OCR WORKFLOW ----------------
			documents := protected.Group("/documents")
			{
				documents.GET("", documentsController.ListDocuments)
				documents.POST("", middleware.RequireRoles(models.RoleInspector, models.RoleSafetyOfficer, models.RoleMineManager, models.RoleRegulatoryOffice, models.RoleSuperAdmin), documentsController.UploadDocument)
				documents.PUT("/:id", middleware.RequireRoles(models.RoleSafetyOfficer, models.RoleMineManager, models.RoleRegulatoryOffice, models.RoleSuperAdmin), documentsController.UpdateDocument)
				documents.DELETE("/:id", middleware.RequireRoles(models.RoleSuperAdmin), documentsController.DeleteDocument)
				documents.POST("/:id/review", middleware.RequireRoles(models.RoleSafetyOfficer, models.RoleMineManager, models.RoleSuperAdmin), documentsController.ReviewDocument)
				documents.POST("/:id/approve", middleware.RequireRoles(models.RoleMineManager, models.RoleSuperAdmin), documentsController.ApproveDocument)
				documents.POST("/:id/verify-regulatory", middleware.RequireRoles(models.RoleRegulatoryOffice, models.RoleSuperAdmin), documentsController.RegulatoryVerifyDocument)
				documents.POST("/:id/flag-violation", middleware.RequireRoles(models.RoleSafetyOfficer, models.RoleMineManager, models.RoleRegulatoryOffice, models.RoleSuperAdmin), documentsController.FlagViolationFromDocument)
			}

			// ---------------- NOTIFICATIONS ----------------
			protected.GET("/notifications", notificationsController.ListNotifications)
			protected.PUT("/notifications/:id/read", notificationsController.MarkAsRead)

			// ---------------- SIMULATOR CONTROL ----------------
			protected.GET("/simulator/mode", simulationController.GetMode)
			protected.POST("/simulator/mode", middleware.RequireRoles(models.RoleSuperAdmin, models.RoleCorporateManager, models.RoleSafetyOfficer), simulationController.SetMode)
			protected.POST("/simulator/reset", middleware.RequireRoles(models.RoleSuperAdmin), simulationController.ResetDemo)

			// ---------------- REPORTS ----------------
			protected.GET("/reports", reportsController.ListReports)
			protected.POST("/reports/generate", reportsController.GenerateReport)

			// ---------------- ANALYTICS / DASHBOARD ----------------
			protected.GET("/analytics/dashboard", dashboardController.DashboardSummary)
			protected.GET("/analytics/charts", analyticsController.GetChartsData)
			protected.GET("/analytics/risk", analyticsController.GetRiskScores)
			protected.GET("/analytics/anomalies", analyticsController.GetAnomalies)
			protected.GET("/analytics/recurring-violations", analyticsController.GetRecurringViolations)
			protected.POST("/analytics/risk/recalculate", analyticsController.TriggerRiskRecalculate)
			protected.POST("/ai/voice-query", analyticsController.HandleVoiceQuery)
			protected.POST("/ai/translate", analyticsController.TranslateText)
		}
	}
}
