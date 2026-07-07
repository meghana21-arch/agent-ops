package projects

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, orgID uuid.UUID, name, description, environment string) (*Project, error) {
	p := &Project{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO projects (organization_id, name, description, environment)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, organization_id, name, description, environment, created_at, updated_at`,
		orgID, name, description, environment,
	).Scan(&p.ID, &p.OrganizationID, &p.Name, &p.Description, &p.Environment, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *Repository) List(ctx context.Context, orgID uuid.UUID) ([]Project, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, organization_id, name, description, environment, created_at, updated_at
		 FROM projects WHERE organization_id = $1 ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.OrganizationID, &p.Name, &p.Description, &p.Environment, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	if projects == nil {
		projects = []Project{}
	}
	return projects, rows.Err()
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*Project, error) {
	p := &Project{}
	err := r.db.QueryRow(ctx,
		`SELECT id, organization_id, name, description, environment, created_at, updated_at
		 FROM projects WHERE id = $1`,
		id,
	).Scan(&p.ID, &p.OrganizationID, &p.Name, &p.Description, &p.Environment, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}
