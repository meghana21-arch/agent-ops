package runs_test

// handler_test.go tests the HTTP layer of the runs package.
//
// Since runs.Handler wraps *Service (concrete) and *Service wraps *Repository
// (concrete, pgx-backed), we cannot inject fakes through the production
// constructors without modifying production code.
//
// Strategy: define a local runServiceI interface that mirrors *Service's public
// methods; implement it with the already-defined testRunService / memRunStore;
// then wire a Gin router with closures that reproduce the same HTTP behaviour as
// runs.Handler.  We test status codes, JSON shapes, and error semantics.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentops/runtime/internal/runs"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ---------------------------------------------------------------------------
// Interface and test wiring
// ---------------------------------------------------------------------------

type runServiceI interface {
	Create(ctx context.Context, input runs.CreateRunInput) (*runs.Run, error)
	Get(ctx context.Context, runID uuid.UUID) (*runs.Run, error)
	List(ctx context.Context, projectID uuid.UUID) ([]runs.Run, error)
	ListSteps(ctx context.Context, runID uuid.UUID) ([]runs.Step, error)
	Cancel(ctx context.Context, runID uuid.UUID) error
	Resume(ctx context.Context, runID uuid.UUID) error
}

// setupRunRouter builds a Gin engine with the same routes and logic as
// runs.Handler but accepting runServiceI so we can inject a fake store.
func setupRunRouter(svc runServiceI) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/v1")

	v1.POST("/runs", func(c *gin.Context) {
		var input runs.CreateRunInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		run, err := svc.Create(c.Request.Context(), input)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, run)
	})

	v1.GET("/runs", func(c *gin.Context) {
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
		list, err := svc.List(c.Request.Context(), projectID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"runs": list})
	})

	v1.GET("/runs/:runId", func(c *gin.Context) {
		runID, err := uuid.Parse(c.Param("runId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid runId"})
			return
		}
		run, err := svc.Get(c.Request.Context(), runID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
			return
		}
		c.JSON(http.StatusOK, run)
	})

	v1.GET("/runs/:runId/steps", func(c *gin.Context) {
		runID, err := uuid.Parse(c.Param("runId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid runId"})
			return
		}
		steps, err := svc.ListSteps(c.Request.Context(), runID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"steps": steps})
	})

	v1.POST("/runs/:runId/cancel", func(c *gin.Context) {
		runID, err := uuid.Parse(c.Param("runId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid runId"})
			return
		}
		if err := svc.Cancel(c.Request.Context(), runID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "run cancelled"})
	})

	v1.POST("/runs/:runId/resume", func(c *gin.Context) {
		runID, err := uuid.Parse(c.Param("runId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid runId"})
			return
		}
		if err := svc.Resume(c.Request.Context(), runID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "run resumed"})
	})

	return r
}

func newRunHandlerSvc() *testRunService {
	svc, _ := newTestRunSvc()
	return svc
}

// newRunHandlerSvcWithStore returns both svc and store so tests can manipulate
// run state directly.
func newRunHandlerSvcWithStore() (*testRunService, *memRunStore) {
	return newTestRunSvc()
}

// seedRunWithStatus creates a run via the service and then force-sets its status
// in the underlying store, returning the run ID.
func seedRunWithStatus(t *testing.T, svc *testRunService, store *memRunStore, status runs.RunStatus) uuid.UUID {
	t.Helper()
	projectID := uuid.New()
	r, err := svc.Create(context.Background(), runs.CreateRunInput{
		ProjectID: projectID.String(),
		Goal:      "seeded goal",
		MaxSteps:  10,
	})
	if err != nil {
		t.Fatalf("seed run: %v", err)
	}
	for i := range store.runs {
		if store.runs[i].ID == r.ID {
			store.runs[i].Status = status
		}
	}
	return r.ID
}

// ---------------------------------------------------------------------------
// POST /v1/runs
// ---------------------------------------------------------------------------

func TestRunHandlerCreate_ValidBody_Returns201(t *testing.T) {
	svc := newRunHandlerSvc()
	r := setupRunRouter(svc)

	projectID := uuid.New()
	body := fmt.Sprintf(`{"projectId":%q,"goal":"test goal","maxSteps":5}`, projectID)
	req := httptest.NewRequest(http.MethodPost, "/v1/runs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status: got %d, want %d — body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var resp runs.Run
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ID == uuid.Nil {
		t.Error("response ID must not be nil UUID")
	}
	if resp.Goal != "test goal" {
		t.Errorf("goal: got %q, want %q", resp.Goal, "test goal")
	}
	if resp.Status != runs.StatusCreated {
		t.Errorf("status: got %q, want %q", resp.Status, runs.StatusCreated)
	}
}

func TestRunHandlerCreate_MissingProjectID_Returns400(t *testing.T) {
	svc := newRunHandlerSvc()
	r := setupRunRouter(svc)

	body := `{"goal":"no project"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/runs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d — body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestRunHandlerCreate_MissingGoal_Returns400(t *testing.T) {
	svc := newRunHandlerSvc()
	r := setupRunRouter(svc)

	body := fmt.Sprintf(`{"projectId":%q}`, uuid.New())
	req := httptest.NewRequest(http.MethodPost, "/v1/runs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d — body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestRunHandlerCreate_InvalidProjectID_Returns500(t *testing.T) {
	// UUID parse happens in the service; the handler maps service errors to 500.
	svc := newRunHandlerSvc()
	r := setupRunRouter(svc)

	body := `{"projectId":"not-a-uuid","goal":"goal"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/runs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d — body: %s", w.Code, http.StatusInternalServerError, w.Body.String())
	}
}

func TestRunHandlerCreate_MaxStepsZero_Defaults20(t *testing.T) {
	svc := newRunHandlerSvc()
	r := setupRunRouter(svc)

	body := fmt.Sprintf(`{"projectId":%q,"goal":"goal","maxSteps":0}`, uuid.New())
	req := httptest.NewRequest(http.MethodPost, "/v1/runs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status: got %d", w.Code)
	}
	var resp runs.Run
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp.MaxSteps != 20 {
		t.Errorf("maxSteps: got %d, want 20", resp.MaxSteps)
	}
}

func TestRunHandlerCreate_EmptyBody_Returns400(t *testing.T) {
	svc := newRunHandlerSvc()
	r := setupRunRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/v1/runs", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRunHandlerCreate_InvalidJSON_Returns400(t *testing.T) {
	svc := newRunHandlerSvc()
	r := setupRunRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/v1/runs", strings.NewReader("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ---------------------------------------------------------------------------
// GET /v1/runs
// ---------------------------------------------------------------------------

func TestRunHandlerList_MissingProjectIDParam_Returns400(t *testing.T) {
	svc := newRunHandlerSvc()
	r := setupRunRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/runs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d — body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestRunHandlerList_InvalidProjectIDParam_Returns400(t *testing.T) {
	svc := newRunHandlerSvc()
	r := setupRunRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/runs?projectId=not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRunHandlerList_ValidProjectID_Returns200WithRunsKey(t *testing.T) {
	svc := newRunHandlerSvc()
	r := setupRunRouter(svc)

	projectID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/runs?projectId="+projectID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d — body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := resp["runs"]; !ok {
		t.Error("response must contain 'runs' key")
	}
}

func TestRunHandlerList_EmptyProject_ReturnsEmptyArray(t *testing.T) {
	svc := newRunHandlerSvc()
	r := setupRunRouter(svc)

	projectID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/runs?projectId="+projectID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	arr, ok := resp["runs"].([]interface{})
	if !ok {
		t.Fatalf("'runs' should be array, got %T", resp["runs"])
	}
	if len(arr) != 0 {
		t.Errorf("expected empty array, got %d items", len(arr))
	}
}

func TestRunHandlerList_PopulatedProject_ReturnsAllRuns(t *testing.T) {
	svc := newRunHandlerSvc()
	r := setupRunRouter(svc)

	projectID := uuid.New()
	for i := 0; i < 4; i++ {
		_, _ = svc.Create(context.Background(), runs.CreateRunInput{
			ProjectID: projectID.String(),
			Goal:      fmt.Sprintf("goal-%d", i),
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/runs?projectId="+projectID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	arr := resp["runs"].([]interface{})
	if len(arr) != 4 {
		t.Errorf("expected 4 runs, got %d", len(arr))
	}
}

// ---------------------------------------------------------------------------
// GET /v1/runs/:runId
// ---------------------------------------------------------------------------

func TestRunHandlerGet_ValidID_Returns200(t *testing.T) {
	svc, store := newRunHandlerSvcWithStore()
	r := setupRunRouter(svc)

	id := seedRunWithStatus(t, svc, store, runs.StatusCreated)
	req := httptest.NewRequest(http.MethodGet, "/v1/runs/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d — body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp runs.Run
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ID != id {
		t.Errorf("ID: got %v, want %v", resp.ID, id)
	}
}

func TestRunHandlerGet_InvalidUUID_Returns400(t *testing.T) {
	svc := newRunHandlerSvc()
	r := setupRunRouter(svc)

	invalidIDs := []string{"bad", "12345", "not-a-uuid"}
	for _, id := range invalidIDs {
		id := id
		t.Run(id, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/runs/"+id, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("id %q: got %d, want %d", id, w.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestRunHandlerGet_NonExistentID_Returns404(t *testing.T) {
	svc := newRunHandlerSvc()
	r := setupRunRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/runs/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d — body: %s", w.Code, http.StatusNotFound, w.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] == "" {
		t.Error("404 must include 'error' field")
	}
}

func TestRunHandlerGet_ResponseHasRequiredFields(t *testing.T) {
	svc, store := newRunHandlerSvcWithStore()
	r := setupRunRouter(svc)

	id := seedRunWithStatus(t, svc, store, runs.StatusCreated)
	req := httptest.NewRequest(http.MethodGet, "/v1/runs/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	required := []string{"id", "projectId", "goal", "status", "maxSteps", "createdAt", "updatedAt"}
	for _, f := range required {
		if _, ok := resp[f]; !ok {
			t.Errorf("response missing field %q", f)
		}
	}
}

// ---------------------------------------------------------------------------
// GET /v1/runs/:runId/steps
// ---------------------------------------------------------------------------

func TestRunHandlerListSteps_ValidRunID_Returns200WithStepsKey(t *testing.T) {
	svc, store := newRunHandlerSvcWithStore()
	r := setupRunRouter(svc)

	id := seedRunWithStatus(t, svc, store, runs.StatusCreated)
	req := httptest.NewRequest(http.MethodGet, "/v1/runs/"+id.String()+"/steps", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d — body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := resp["steps"]; !ok {
		t.Error("response must contain 'steps' key")
	}
}

func TestRunHandlerListSteps_NoSteps_ReturnsEmptyArray(t *testing.T) {
	svc, store := newRunHandlerSvcWithStore()
	r := setupRunRouter(svc)

	id := seedRunWithStatus(t, svc, store, runs.StatusCreated)
	req := httptest.NewRequest(http.MethodGet, "/v1/runs/"+id.String()+"/steps", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	arr, ok := resp["steps"].([]interface{})
	if !ok {
		t.Fatalf("'steps' must be array, got %T", resp["steps"])
	}
	if len(arr) != 0 {
		t.Errorf("expected empty steps array, got %d", len(arr))
	}
}

func TestRunHandlerListSteps_InvalidRunID_Returns400(t *testing.T) {
	svc := newRunHandlerSvc()
	r := setupRunRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/runs/invalid-uuid/steps", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ---------------------------------------------------------------------------
// POST /v1/runs/:runId/cancel
// ---------------------------------------------------------------------------

func TestRunHandlerCancel_RunningRun_Returns200(t *testing.T) {
	svc, store := newRunHandlerSvcWithStore()
	r := setupRunRouter(svc)

	id := seedRunWithStatus(t, svc, store, runs.StatusRunning)
	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+id.String()+"/cancel", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d — body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["message"] != "run cancelled" {
		t.Errorf("message: got %q, want %q", resp["message"], "run cancelled")
	}
}

func TestRunHandlerCancel_CreatedRun_Returns200(t *testing.T) {
	svc, store := newRunHandlerSvcWithStore()
	r := setupRunRouter(svc)

	id := seedRunWithStatus(t, svc, store, runs.StatusCreated)
	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+id.String()+"/cancel", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRunHandlerCancel_CompletedRun_Returns400(t *testing.T) {
	svc, store := newRunHandlerSvcWithStore()
	r := setupRunRouter(svc)

	id := seedRunWithStatus(t, svc, store, runs.StatusCompleted)
	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+id.String()+"/cancel", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d — body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] == "" {
		t.Error("400 response must include 'error' field")
	}
}

func TestRunHandlerCancel_FailedRun_Returns400(t *testing.T) {
	svc, store := newRunHandlerSvcWithStore()
	r := setupRunRouter(svc)

	id := seedRunWithStatus(t, svc, store, runs.StatusFailed)
	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+id.String()+"/cancel", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRunHandlerCancel_CancelledRun_Returns400(t *testing.T) {
	svc, store := newRunHandlerSvcWithStore()
	r := setupRunRouter(svc)

	id := seedRunWithStatus(t, svc, store, runs.StatusCancelled)
	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+id.String()+"/cancel", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRunHandlerCancel_InvalidRunID_Returns400(t *testing.T) {
	svc := newRunHandlerSvc()
	r := setupRunRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/v1/runs/not-uuid/cancel", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRunHandlerCancel_NonExistentRun_Returns400(t *testing.T) {
	svc := newRunHandlerSvc()
	r := setupRunRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+uuid.New().String()+"/cancel", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// The handler maps service errors (run not found) to 400.
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ---------------------------------------------------------------------------
// POST /v1/runs/:runId/resume
// ---------------------------------------------------------------------------

func TestRunHandlerResume_FailedRun_Returns200(t *testing.T) {
	svc, store := newRunHandlerSvcWithStore()
	r := setupRunRouter(svc)

	id := seedRunWithStatus(t, svc, store, runs.StatusFailed)
	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+id.String()+"/resume", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d — body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["message"] != "run resumed" {
		t.Errorf("message: got %q, want %q", resp["message"], "run resumed")
	}
}

func TestRunHandlerResume_WaitingForApprovalRun_Returns200(t *testing.T) {
	svc, store := newRunHandlerSvcWithStore()
	r := setupRunRouter(svc)

	id := seedRunWithStatus(t, svc, store, runs.StatusWaitingForApproval)
	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+id.String()+"/resume", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d — body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestRunHandlerResume_RunningRun_Returns400(t *testing.T) {
	svc, store := newRunHandlerSvcWithStore()
	r := setupRunRouter(svc)

	id := seedRunWithStatus(t, svc, store, runs.StatusRunning)
	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+id.String()+"/resume", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d — body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestRunHandlerResume_CompletedRun_Returns400(t *testing.T) {
	svc, store := newRunHandlerSvcWithStore()
	r := setupRunRouter(svc)

	id := seedRunWithStatus(t, svc, store, runs.StatusCompleted)
	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+id.String()+"/resume", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRunHandlerResume_CancelledRun_Returns400(t *testing.T) {
	svc, store := newRunHandlerSvcWithStore()
	r := setupRunRouter(svc)

	id := seedRunWithStatus(t, svc, store, runs.StatusCancelled)
	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+id.String()+"/resume", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRunHandlerResume_CreatedRun_Returns400(t *testing.T) {
	svc, store := newRunHandlerSvcWithStore()
	r := setupRunRouter(svc)

	id := seedRunWithStatus(t, svc, store, runs.StatusCreated)
	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+id.String()+"/resume", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRunHandlerResume_InvalidRunID_Returns400(t *testing.T) {
	svc := newRunHandlerSvc()
	r := setupRunRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/v1/runs/bad-uuid/resume", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRunHandlerResume_NonExistentRun_Returns400(t *testing.T) {
	svc := newRunHandlerSvc()
	r := setupRunRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+uuid.New().String()+"/resume", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ---------------------------------------------------------------------------
// Table-driven cancel/resume state tests via HTTP
// ---------------------------------------------------------------------------

func TestRunHandlerCancelResumeStatusCodes_Table(t *testing.T) {
	type testCase struct {
		status  runs.RunStatus
		action  string
		wantCode int
	}
	cases := []testCase{
		// Cancel
		{runs.StatusCreated, "cancel", http.StatusOK},
		{runs.StatusPlanning, "cancel", http.StatusOK},
		{runs.StatusRunning, "cancel", http.StatusOK},
		{runs.StatusWaitingForApproval, "cancel", http.StatusOK},
		{runs.StatusRetrying, "cancel", http.StatusOK},
		{runs.StatusCompleted, "cancel", http.StatusBadRequest},
		{runs.StatusFailed, "cancel", http.StatusBadRequest},
		{runs.StatusCancelled, "cancel", http.StatusBadRequest},
		// Resume
		{runs.StatusFailed, "resume", http.StatusOK},
		{runs.StatusWaitingForApproval, "resume", http.StatusOK},
		{runs.StatusCreated, "resume", http.StatusBadRequest},
		{runs.StatusPlanning, "resume", http.StatusBadRequest},
		{runs.StatusRunning, "resume", http.StatusBadRequest},
		{runs.StatusRetrying, "resume", http.StatusBadRequest},
		{runs.StatusCompleted, "resume", http.StatusBadRequest},
		{runs.StatusCancelled, "resume", http.StatusBadRequest},
	}

	for _, tc := range cases {
		tc := tc
		name := string(tc.status) + "_" + tc.action
		t.Run(name, func(t *testing.T) {
			svc, store := newRunHandlerSvcWithStore()
			r := setupRunRouter(svc)
			id := seedRunWithStatus(t, svc, store, tc.status)

			url := fmt.Sprintf("/v1/runs/%s/%s", id, tc.action)
			req := httptest.NewRequest(http.MethodPost, url, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.wantCode {
				t.Errorf("[%s] status: got %d, want %d — body: %s",
					name, w.Code, tc.wantCode, w.Body.String())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestRunHandlerCreate_ResponseBodyIsNotNilUUID(t *testing.T) {
	svc := newRunHandlerSvc()
	r := setupRunRouter(svc)

	body := fmt.Sprintf(`{"projectId":%q,"goal":"check uuid"}`, uuid.New())
	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	idStr, _ := resp["id"].(string)
	if idStr == "" || idStr == "00000000-0000-0000-0000-000000000000" {
		t.Error("response ID must be a valid non-nil UUID")
	}
}

func TestRunHandlerList_IsolatesProjectRuns(t *testing.T) {
	svc := newRunHandlerSvc()
	r := setupRunRouter(svc)

	projectA := uuid.New()
	projectB := uuid.New()

	for i := 0; i < 2; i++ {
		_, _ = svc.Create(context.Background(), runs.CreateRunInput{
			ProjectID: projectA.String(),
			Goal:      fmt.Sprintf("a-%d", i),
		})
	}
	_, _ = svc.Create(context.Background(), runs.CreateRunInput{
		ProjectID: projectB.String(),
		Goal:      "b-0",
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/runs?projectId="+projectA.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	arr := resp["runs"].([]interface{})
	if len(arr) != 2 {
		t.Errorf("expected 2 runs for projectA, got %d", len(arr))
	}
}

func TestRunHandlerGet_AfterCancel_StatusIsCancelled(t *testing.T) {
	svc, store := newRunHandlerSvcWithStore()
	r := setupRunRouter(svc)

	id := seedRunWithStatus(t, svc, store, runs.StatusRunning)

	// Cancel
	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+id.String()+"/cancel", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("cancel: unexpected status %d", w.Code)
	}

	// Get
	req2 := httptest.NewRequest(http.MethodGet, "/v1/runs/"+id.String(), nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	var resp runs.Run
	_ = json.NewDecoder(w2.Body).Decode(&resp)
	if resp.Status != runs.StatusCancelled {
		t.Errorf("status after cancel: got %q, want %q", resp.Status, runs.StatusCancelled)
	}
}
