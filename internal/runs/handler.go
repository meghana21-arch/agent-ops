package runs

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(r *gin.RouterGroup) {
	r.POST("/runs", h.create)
	r.GET("/runs", h.list)
	r.GET("/runs/:runId", h.get)
	r.GET("/runs/:runId/steps", h.listSteps)
	r.POST("/runs/:runId/cancel", h.cancel)
	r.POST("/runs/:runId/resume", h.resume)
}

func (h *Handler) create(c *gin.Context) {
	var input CreateRunInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	run, err := h.svc.Create(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, run)
}

func (h *Handler) list(c *gin.Context) {
	projectIDStr := c.Query("projectId")
	if projectIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "projectId query param required"})
		return
	}
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid projectId"})
		return
	}
	runsResult, err := h.svc.List(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"runs": runsResult})
}

func (h *Handler) get(c *gin.Context) {
	runID, err := uuid.Parse(c.Param("runId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid runId"})
		return
	}
	run, err := h.svc.Get(c.Request.Context(), runID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return
	}
	c.JSON(http.StatusOK, run)
}

func (h *Handler) listSteps(c *gin.Context) {
	runID, err := uuid.Parse(c.Param("runId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid runId"})
		return
	}
	steps, err := h.svc.ListSteps(c.Request.Context(), runID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"steps": steps})
}

func (h *Handler) cancel(c *gin.Context) {
	runID, err := uuid.Parse(c.Param("runId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid runId"})
		return
	}
	if err := h.svc.Cancel(c.Request.Context(), runID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "run cancelled"})
}

func (h *Handler) resume(c *gin.Context) {
	runID, err := uuid.Parse(c.Param("runId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid runId"})
		return
	}
	if err := h.svc.Resume(c.Request.Context(), runID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "run resumed"})
}
