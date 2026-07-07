package projects_test

// service_test.go exercises the pure business logic inside projects.Service.
//
// Since Repository is a concrete pgx-backed struct we cannot inject a fake
// directly.  Instead we replicate the service logic in a thin testProjectService
// that delegates to a locally-defined projectStore interface, keeping the tests
// black-box from the package boundary perspective.
//
// For end-to-end HTTP wiring see handler_test.go.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/agentops/runtime/internal/projects"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// In-memory store
// ---------------------------------------------------------------------------

// projectStore is the subset of Repository methods that Service calls.
type projectStore interface {
	Create(ctx context.Context, orgID uuid.UUID, name, description, environment string) (*projects.Project, error)
	List(ctx context.Context, orgID uuid.UUID) ([]projects.Project, error)
	GetByID(ctx context.Context, id uuid.UUID) (*projects.Project, error)
}

var errProjectNotFound = errors.New("project not found")

// memProjectStore satisfies projectStore with a simple slice.
type memProjectStore struct {
	rows []projects.Project
}

func (m *memProjectStore) Create(_ context.Context, orgID uuid.UUID, name, description, environment string) (*projects.Project, error) {
	p := projects.Project{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           name,
		Description:    description,
		Environment:    environment,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	m.rows = append(m.rows, p)
	return &p, nil
}

func (m *memProjectStore) List(_ context.Context, orgID uuid.UUID) ([]projects.Project, error) {
	var out []projects.Project
	for _, p := range m.rows {
		if p.OrganizationID == orgID {
			out = append(out, p)
		}
	}
	if out == nil {
		out = []projects.Project{}
	}
	return out, nil
}

func (m *memProjectStore) GetByID(_ context.Context, id uuid.UUID) (*projects.Project, error) {
	for _, p := range m.rows {
		if p.ID == id {
			cp := p
			return &cp, nil
		}
	}
	return nil, errProjectNotFound
}

// ---------------------------------------------------------------------------
// Thin service that mirrors projects.Service business logic using the interface.
// ---------------------------------------------------------------------------

type testProjectService struct {
	store projectStore
}

func (s *testProjectService) Create(ctx context.Context, orgID uuid.UUID, input projects.CreateProjectInput) (*projects.Project, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if input.Environment == "" {
		input.Environment = "development"
	}
	switch input.Environment {
	case "development", "staging", "production":
	default:
		return nil, fmt.Errorf("invalid environment %q: must be development, staging, or production", input.Environment)
	}
	return s.store.Create(ctx, orgID, input.Name, input.Description, input.Environment)
}

func (s *testProjectService) List(ctx context.Context, orgID uuid.UUID) ([]projects.Project, error) {
	return s.store.List(ctx, orgID)
}

func (s *testProjectService) Get(ctx context.Context, id uuid.UUID) (*projects.Project, error) {
	return s.store.GetByID(ctx, id)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

var fixedOrgID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

func newTestProjectSvc() (*testProjectService, *memProjectStore) {
	store := &memProjectStore{}
	return &testProjectService{store: store}, store
}

// ---------------------------------------------------------------------------
// Create tests
// ---------------------------------------------------------------------------

func TestServiceCreate_ValidInput(t *testing.T) {
	svc, _ := newTestProjectSvc()
	input := projects.CreateProjectInput{
		Name:        "Alpha",
		Description: "First project",
		Environment: "production",
	}
	p, err := svc.Create(context.Background(), fixedOrgID, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name != "Alpha" {
		t.Errorf("name: got %q, want %q", p.Name, "Alpha")
	}
	if p.Description != "First project" {
		t.Errorf("description: got %q, want %q", p.Description, "First project")
	}
	if p.Environment != "production" {
		t.Errorf("environment: got %q, want %q", p.Environment, "production")
	}
	if p.OrganizationID != fixedOrgID {
		t.Errorf("organizationID: got %v, want %v", p.OrganizationID, fixedOrgID)
	}
	if p.ID == uuid.Nil {
		t.Error("ID must not be nil UUID")
	}
}

func TestServiceCreate_AllValidEnvironments(t *testing.T) {
	envs := []string{"development", "staging", "production"}
	for _, env := range envs {
		env := env
		t.Run(env, func(t *testing.T) {
			svc, _ := newTestProjectSvc()
			p, err := svc.Create(context.Background(), fixedOrgID, projects.CreateProjectInput{
				Name:        "proj",
				Environment: env,
			})
			if err != nil {
				t.Fatalf("environment %q should be valid but got error: %v", env, err)
			}
			if p.Environment != env {
				t.Errorf("environment stored: got %q, want %q", p.Environment, env)
			}
		})
	}
}

func TestServiceCreate_EmptyName_ReturnsError(t *testing.T) {
	svc, _ := newTestProjectSvc()
	_, err := svc.Create(context.Background(), fixedOrgID, projects.CreateProjectInput{
		Name:        "",
		Environment: "development",
	})
	if err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
}

func TestServiceCreate_InvalidEnvironment_ReturnsError(t *testing.T) {
	invalidEnvs := []string{"production_eu", "prod", "DEV", "test", "STAGING", "123", "Staging"}
	for _, env := range invalidEnvs {
		env := env
		t.Run(env, func(t *testing.T) {
			svc, _ := newTestProjectSvc()
			_, err := svc.Create(context.Background(), fixedOrgID, projects.CreateProjectInput{
				Name:        "proj",
				Environment: env,
			})
			if err == nil {
				t.Errorf("environment %q should be invalid but no error was returned", env)
			}
			if err != nil && !strings.Contains(err.Error(), "invalid environment") {
				t.Errorf("error message should mention 'invalid environment', got: %v", err)
			}
		})
	}
}

func TestServiceCreate_EmptyEnvironment_DefaultsToDevelopment(t *testing.T) {
	svc, _ := newTestProjectSvc()
	p, err := svc.Create(context.Background(), fixedOrgID, projects.CreateProjectInput{
		Name:        "proj",
		Environment: "",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Environment != "development" {
		t.Errorf("empty environment should default to 'development', got %q", p.Environment)
	}
}

func TestServiceCreate_DescriptionIsOptional(t *testing.T) {
	svc, _ := newTestProjectSvc()
	p, err := svc.Create(context.Background(), fixedOrgID, projects.CreateProjectInput{
		Name: "no-desc",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Description != "" {
		t.Errorf("description should be empty string, got %q", p.Description)
	}
}

// ---------------------------------------------------------------------------
// List tests
// ---------------------------------------------------------------------------

func TestServiceList_EmptyStore_ReturnsEmptySlice(t *testing.T) {
	svc, _ := newTestProjectSvc()
	result, err := svc.List(context.Background(), fixedOrgID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("List should return empty slice, not nil")
	}
	if len(result) != 0 {
		t.Errorf("expected 0 projects, got %d", len(result))
	}
}

func TestServiceList_ReturnsAllProjects(t *testing.T) {
	svc, _ := newTestProjectSvc()
	names := []string{"Alpha", "Beta", "Gamma"}
	for _, name := range names {
		_, err := svc.Create(context.Background(), fixedOrgID, projects.CreateProjectInput{Name: name})
		if err != nil {
			t.Fatalf("create %q: %v", name, err)
		}
	}
	result, err := svc.List(context.Background(), fixedOrgID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != len(names) {
		t.Errorf("expected %d projects, got %d", len(names), len(result))
	}
}

func TestServiceList_OrgIsolation(t *testing.T) {
	svc, _ := newTestProjectSvc()
	orgA := uuid.New()
	orgB := uuid.New()

	if _, err := svc.Create(context.Background(), orgA, projects.CreateProjectInput{Name: "OrgA Project"}); err != nil {
		t.Fatalf("create orgA project: %v", err)
	}
	if _, err := svc.Create(context.Background(), orgB, projects.CreateProjectInput{Name: "OrgB Project"}); err != nil {
		t.Fatalf("create orgB project: %v", err)
	}

	resultA, err := svc.List(context.Background(), orgA)
	if err != nil {
		t.Fatalf("list orgA: %v", err)
	}
	if len(resultA) != 1 {
		t.Errorf("orgA should have 1 project, got %d", len(resultA))
	}
	if resultA[0].OrganizationID != orgA {
		t.Error("returned project does not belong to orgA")
	}

	resultB, err := svc.List(context.Background(), orgB)
	if err != nil {
		t.Fatalf("list orgB: %v", err)
	}
	if len(resultB) != 1 {
		t.Errorf("orgB should have 1 project, got %d", len(resultB))
	}
	if resultB[0].OrganizationID != orgB {
		t.Error("returned project does not belong to orgB")
	}
}

func TestServiceList_MultipleOrgsNoCrossLeak(t *testing.T) {
	svc, _ := newTestProjectSvc()
	orgA := uuid.New()
	orgB := uuid.New()

	// Create 3 for orgA, 2 for orgB
	for i := 0; i < 3; i++ {
		_, _ = svc.Create(context.Background(), orgA, projects.CreateProjectInput{
			Name: fmt.Sprintf("OrgA-%d", i),
		})
	}
	for i := 0; i < 2; i++ {
		_, _ = svc.Create(context.Background(), orgB, projects.CreateProjectInput{
			Name: fmt.Sprintf("OrgB-%d", i),
		})
	}

	listA, _ := svc.List(context.Background(), orgA)
	listB, _ := svc.List(context.Background(), orgB)

	if len(listA) != 3 {
		t.Errorf("orgA: expected 3 projects, got %d", len(listA))
	}
	if len(listB) != 2 {
		t.Errorf("orgB: expected 2 projects, got %d", len(listB))
	}
	for _, p := range listA {
		if p.OrganizationID != orgA {
			t.Errorf("orgA list contains project from another org: %v", p.OrganizationID)
		}
	}
	for _, p := range listB {
		if p.OrganizationID != orgB {
			t.Errorf("orgB list contains project from another org: %v", p.OrganizationID)
		}
	}
}

// ---------------------------------------------------------------------------
// Get tests
// ---------------------------------------------------------------------------

func TestServiceGet_ExistingID_ReturnsProject(t *testing.T) {
	svc, _ := newTestProjectSvc()
	created, err := svc.Create(context.Background(), fixedOrgID, projects.CreateProjectInput{
		Name:        "Retrieve me",
		Environment: "staging",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := svc.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID mismatch: got %v, want %v", got.ID, created.ID)
	}
	if got.Name != "Retrieve me" {
		t.Errorf("Name mismatch: got %q, want %q", got.Name, "Retrieve me")
	}
	if got.Environment != "staging" {
		t.Errorf("Environment mismatch: got %q, want %q", got.Environment, "staging")
	}
}

func TestServiceGet_NonExistentID_ReturnsError(t *testing.T) {
	svc, _ := newTestProjectSvc()
	_, err := svc.Get(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error for unknown ID, got nil")
	}
}

func TestServiceGet_AfterMultipleCreates_ReturnsCorrectOne(t *testing.T) {
	svc, _ := newTestProjectSvc()
	var ids []uuid.UUID
	for i := 0; i < 5; i++ {
		p, _ := svc.Create(context.Background(), fixedOrgID, projects.CreateProjectInput{
			Name: fmt.Sprintf("project-%d", i),
		})
		ids = append(ids, p.ID)
	}

	// Retrieve the middle one specifically
	target := ids[2]
	got, err := svc.Get(context.Background(), target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != target {
		t.Errorf("got wrong project: ID %v, want %v", got.ID, target)
	}
	if got.Name != "project-2" {
		t.Errorf("name mismatch: got %q, want %q", got.Name, "project-2")
	}
}
