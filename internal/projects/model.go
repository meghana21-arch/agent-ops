package projects

import (
	"time"

	"github.com/google/uuid"
)

type Project struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organizationId"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Environment    string    `json:"environment"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
