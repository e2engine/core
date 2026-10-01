package model

type OrderBy string

const (
	OrderByID        OrderBy = "id"
	OrderByName      OrderBy = "name"
	OrderByVersion   OrderBy = "version"
	OrderByCreatedAt OrderBy = "createdAt"
	OrderByUpdatedAt OrderBy = "updatedAt"

	OrderByStartedAt       OrderBy = "startedAt"
	OrderByFinishedAt      OrderBy = "finishedAt"
	OrderByStatus          OrderBy = "status"
	OrderByEnvironmentID   OrderBy = "environmentId"
	OrderByEnvironmentName OrderBy = "environmentName"
	OrderByTestID          OrderBy = "testId"
	OrderByTestName        OrderBy = "testName"
)

type OrderDirection string

const (
	OrderDirectionAsc  OrderDirection = "asc"
	OrderDirectionDesc OrderDirection = "desc"
)

type Page[T Entity] struct {
	Items []T `json:"items" yaml:"items"`

	Before string `json:"before,omitempty" yaml:"before,omitempty"`
	After  string `json:"after,omitempty" yaml:"after,omitempty"`
}
