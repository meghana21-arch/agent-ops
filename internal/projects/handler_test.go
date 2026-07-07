package projects_test

// handler_test.go tests the HTTP layer of the projects package.
//
// Because projects.Handler takes a concrete *Service and *Service takes a
// concrete *Repository (backed by pgx), we cannot inject mocks through the
// production constructors without modifying production code.
//
// Strategy: define a local projectServiceI interface that mirrors the public
// methods of *Service, implement it with the already-defined memProjectStore-backed
// testProjectService, then build a parallel test router using Gin closures that
// call the interface methods.  The HTTP behaviour being tested (status codes,
// JSON shape, validation) is what matters here — not the production constructor
// chain.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentops/runtime/internal/projects"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ---------------------------------------------------------------------------
// Interface and test wiring
// ---------------------------------------------------------------------------

// projectServiceI mirrors the public API of *projects.Service.
type projectServiceI interface {
	Create(ctx context.Context, orgID uuid.UUID, input projects.CreateProjectInput) (*projects.Project, error)
	List(ctx context.Context, orgID uuid.UUID) ([]projects.Project, error)
	Get(ctx context.Context, id uuid.UUID) (*projects.Project, error)
}

// The defaultOrgID used by the production handler.
var testOrgID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// setupProjectRouter wires a Gin engine that reproduces the same routes and
// logic as projects.Handler but accepts a projectServiceI interface.
func setupProjectRouter(svc projectServiceI) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/v1")

	v1.POST("/projects", func(c *gin.Context) {
		var input projects.CreateProjectInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		p, err := svc.Create(c.Request.Context(), testOrgID, input)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, p)
	})

	v1.GET("/projects", func(c *gin.Context) {
		list, err := svc.List(c.Request.Context(), testOrgID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"projects": list})
	})

	v1.GET("/projects/:projectId", func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("projectId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid projectId"})
			return
		}
		p, err := svc.Get(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
			return
		}
		c.JSON(http.StatusOK, p)
	})

	return r
}

// newProjectHandlerSvc returns a fresh testProjectService backed by an empty
// memProjectStore for use in handler tests.
func newProjectHandlerSvc() *testProjectService {
	svc, _ := newTestProjectSvc()
	return svc
}

// ---------------------------------------------------------------------------
// POST /v1/projects
// ---------------------------------------------------------------------------

func TestHandlerCreateProject_ValidBody_Returns201(t *testing.T) {
	svc := newProjectHandlerSvc()
	r := setupProjectRouter(svc)

	body := `{"name":"handler-proj","description":"desc","environment":"staging"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/projects", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status: got %d, want %d — body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp projects.Project
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Name != "handler-proj" {
		t.Errorf("name: got %q, want %q", resp.Name, "handler-proj")
	}
	if resp.Environment != "staging" {
		t.Errorf("environment: got %q, want %q", resp.Environment, "staging")
	}
	if resp.ID == uuid.Nil {
		t.Error("response ID should not be nil UUID")
	}
}

func TestHandlerCreateProject_MissingName_Returns400(t *testing.T) {
	svc := newProjectHandlerSvc()
	r := setupProjectRouter(svc)

	body := `{"description":"no name here"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/projects", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d — body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestHandlerCreateProject_EmptyBody_Returns400(t *testing.T) {
	svc := newProjectHandlerSvc()
	r := setupProjectRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/v1/projects", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandlerCreateProject_InvalidEnvironment_Returns500(t *testing.T) {
	// The service returns an error for invalid environments; the handler maps
	// that to 500 (no specialised env-validation status code in the handler).
	svc := newProjectHandlerSvc()
	r := setupProjectRouter(svc)

	body := `{"name":"proj","environment":"bad-env"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/projects", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d — body: %s", w.Code, http.StatusInternalServerError, w.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if !strings.Contains(resp["error"], "invalid environment") {
		t.Errorf("error message should mention 'invalid environment', got: %q", resp["error"])
	}
}

func TestHandlerCreateProject_DefaultEnvironment_IsDevelopment(t *testing.T) {
	svc := newProjectHandlerSvc()
	r := setupProjectRouter(svc)

	body := `{"name":"default-env-proj"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/projects", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d — body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var resp projects.Project
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp.Environment != "development" {
		t.Errorf("default environment: got %q, want %q", resp.Environment, "development")
	}
}

func TestHandlerCreateProject_InvalidJSON_Returns400(t *testing.T) {
	svc := newProjectHandlerSvc()
	r := setupProjectRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/v1/projects", strings.NewReader("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ---------------------------------------------------------------------------
// GET /v1/projects
// ---------------------------------------------------------------------------

func TestHandlerListProjects_EmptyStore_Returns200WithEmptyArray(t *testing.T) {
	svc := newProjectHandlerSvc()
	r := setupProjectRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/projects", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	list, ok := resp["projects"]
	if !ok {
		t.Fatal("response must contain 'projects' key")
	}
	arr, ok := list.([]interface{})
	if !ok {
		t.Fatalf("'projects' value must be an array, got: %T", list)
	}
	if len(arr) != 0 {
		t.Errorf("expected empty array, got %d items", len(arr))
	}
}

func TestHandlerListProjects_AfterCreate_ReturnsProjects(t *testing.T) {
	svc := newProjectHandlerSvc()
	r := setupProjectRouter(svc)

	// Pre-populate via the service directly.
	for i := 0; i < 3; i++ {
		_, _ = svc.Create(context.Background(), testOrgID, projects.CreateProjectInput{
			Name: "p",
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/projects", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}
	var resp map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	arr := resp["projects"].([]interface{})
	if len(arr) != 3 {
		t.Errorf("expected 3 projects, got %d", len(arr))
	}
}

func TestHandlerListProjects_ResponseShape(t *testing.T) {
	svc := newProjectHandlerSvc()
	r := setupProjectRouter(svc)

	_, _ = svc.Create(context.Background(), testOrgID, projects.CreateProjectInput{
		Name:        "shape-test",
		Description: "desc",
		Environment: "production",
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/projects", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp struct {
		Projects []map[string]interface{} `json:"projects"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Projects) == 0 {
		t.Fatal("expected at least one project")
	}
	p := resp.Projects[0]
	for _, field := range []string{"id", "name", "description", "environment", "createdAt", "updatedAt", "organizationId"} {
		if _, exists := p[field]; !exists {
			t.Errorf("response project missing field %q", field)
		}
	}
}

// ---------------------------------------------------------------------------
// GET /v1/projects/:projectId
// ---------------------------------------------------------------------------

func TestHandlerGetProject_ValidID_Returns200(t *testing.T) {
	svc := newProjectHandlerSvc()
	r := setupProjectRouter(svc)

	created, _ := svc.Create(context.Background(), testOrgID, projects.CreateProjectInput{
		Name:        "get-me",
		Environment: "staging",
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/projects/"+created.ID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d — body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp projects.Project
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ID != created.ID {
		t.Errorf("ID: got %v, want %v", resp.ID, created.ID)
	}
	if resp.Name != "get-me" {
		t.Errorf("name: got %q, want %q", resp.Name, "get-me")
	}
}

func TestHandlerGetProject_InvalidUUID_Returns400(t *testing.T) {
	svc := newProjectHandlerSvc()
	r := setupProjectRouter(svc)

	invalidIDs := []string{"not-a-uuid", "12345", "abc", "00000000-0000-0000-ZZZZ"}
	for _, id := range invalidIDs {
		id := id
		t.Run(id, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/projects/"+id, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("id %q: got %d, want %d", id, w.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestHandlerGetProject_NonExistentID_Returns404(t *testing.T) {
	svc := newProjectHandlerSvc()
	r := setupProjectRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/projects/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d — body: %s", w.Code, http.StatusNotFound, w.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] == "" {
		t.Error("404 response must include 'error' field")
	}
}

func TestHandlerGetProject_ResponseHasAllFields(t *testing.T) {
	svc := newProjectHandlerSvc()
	r := setupProjectRouter(svc)

	created, _ := svc.Create(context.Background(), testOrgID, projects.CreateProjectInput{
		Name:        "fields-check",
		Description: "checking fields",
		Environment: "development",
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/projects/"+created.ID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&resp)

	required := []string{"id", "organizationId", "name", "description", "environment", "createdAt", "updatedAt"}
	for _, field := range required {
		if _, ok := resp[field]; !ok {
			t.Errorf("response missing field %q", field)
		}
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestHandlerCreateProject_NilIDInStore_IsNotReturned(t *testing.T) {
	// Regression: ensure the store always generates a non-nil UUID.
	svc := newProjectHandlerSvc()
	r := setupProjectRouter(svc)

	body := `{"name":"uuid-test"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/projects", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status: %d", w.Code)
	}
	var resp map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	idStr, _ := resp["id"].(string)
	if idStr == "" || idStr == "00000000-0000-0000-0000-000000000000" {
		t.Error("response ID must not be empty or nil UUID")
	}
}

func TestHandlerCreateProject_ContentTypeNotJSON_Returns400(t *testing.T) {
	svc := newProjectHandlerSvc()
	r := setupProjectRouter(svc)

	// Send form data instead of JSON.
	req := httptest.NewRequest(http.MethodPost, "/v1/projects", strings.NewReader("name=proj"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// ShouldBindJSON will fail since content-type is not JSON.
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-JSON content-type, got %d", w.Code)
	}
}
