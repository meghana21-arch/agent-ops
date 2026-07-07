package projects

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// defaultOrgID is used in Phase 1 before real auth is implemented.
var defaultOrgID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(r *gin.RouterGroup) {
	r.POST("/projects", h.create)
	r.GET("/projects", h.list)
	r.GET("/projects/:projectId", h.get)
}

func (h *Handler) create(c *gin.Context) {
	var input CreateProjectInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, err := h.svc.Create(c.Request.Context(), defaultOrgID, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, p)
}

func (h *Handler) list(c *gin.Context) {
	projects, err := h.svc.List(c.Request.Context(), defaultOrgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"projects": projects})
}

func (h *Handler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid projectId"})
		return
	}
	p, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}
	c.JSON(http.StatusOK, p)
}
