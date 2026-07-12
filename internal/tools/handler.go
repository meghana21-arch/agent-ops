package tools

import (
	"encoding/json"
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
	rg.POST("/projects/:projectId/tools", h.register)
	rg.GET("/projects/:projectId/tools", h.list)
	rg.GET("/tools", h.listBuiltins)
}

type registerInput struct {
	ToolName string          `json:"toolName" binding:"required"`
	Enabled  *bool           `json:"enabled"`
	Config   json.RawMessage `json:"config"`
}

func (h *Handler) register(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid projectId"})
		return
	}
	var in registerInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	pt, err := h.svc.Register(c.Request.Context(), projectID, in.ToolName, enabled, in.Config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, pt)
}

func (h *Handler) list(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid projectId"})
		return
	}
	tools, err := h.svc.List(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tools)
}

func (h *Handler) listBuiltins(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.Defs())
}
