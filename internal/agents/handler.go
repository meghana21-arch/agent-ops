package agents

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

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.POST("/projects/:projectId/agent-configs", h.create)
	rg.GET("/projects/:projectId/agent-configs", h.list)
	rg.GET("/agent-configs/:configId", h.get)
}

func (h *Handler) create(c *gin.Context) {
	projectID := c.Param("projectId")
	var in CreateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	in.ProjectID = projectID

	cfg, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, cfg)
}

func (h *Handler) list(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid projectId"})
		return
	}
	cfgs, err := h.svc.List(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfgs)
}

func (h *Handler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("configId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid configId"})
		return
	}
	cfg, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent config not found"})
		return
	}
	c.JSON(http.StatusOK, cfg)
}
