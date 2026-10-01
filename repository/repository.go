package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ygrebnov/errorc"
)

var (
	ErrNotFound             = errorc.New("not found")
	ErrConflict             = errorc.New("conflict")
	ErrAlreadyExists        = errorc.New("already exists")
	ErrInvalidPayload       = errorc.New("invalid payload")
	ErrInvalidPageTraversal = errorc.New("invalid page traversal")
	ErrInvalidPageLimit     = errorc.New("invalid page limit")
)

type EntityRepository[E Entity] interface {
	List(ctx context.Context, params *ListParams) ([]E, error)
	Create(ctx context.Context, params *E) (*E, error)
	Get(ctx context.Context, id string) (*E, error)
	Delete(ctx context.Context, id string) (*E, error)
	GetIDsByID(ctx context.Context, id string) ([]string, error)
}

type ResourceRepository[E ResourceEntity] interface {
	EntityRepository[E]
	GetIDsByName(ctx context.Context, name string) ([]string, error)
}

type EnvironmentRepository = ResourceRepository[Environment]
type TestSuiteRepository = ResourceRepository[TestSuite]

type TestRepository interface {
	ResourceRepository[Test]
	GetByTag(ctx context.Context, tag string) ([]Test, error)
}

type SetRunningParams struct {
	ID        string    `json:"id"`
	StartedAt time.Time `json:"started_at"`
}

type SetCompletedParams struct {
	ID         string          `json:"id"`
	FinishedAt time.Time       `json:"finished_at"`
	Status     string          `json:"status"`
	Summary    json.RawMessage `json:"summary"`
}

type TestExecutionRepository interface {
	EntityRepository[TestExecution]
	GetByTestSuiteExecutionID(ctx context.Context, id string) ([]TestExecution, error)
	SetRunning(ctx context.Context, params *SetRunningParams) error
	SetCompleted(ctx context.Context, params *SetCompletedParams) error
	GetStatus(ctx context.Context, id string) (string, error)
}

type TestSuiteExecutionRepository interface {
	EntityRepository[TestSuiteExecution]
	SetRunning(ctx context.Context, params *SetRunningParams) error
	SetCompleted(ctx context.Context, params *SetCompletedParams) error
	GetStatus(ctx context.Context, id string) (string, error)
}

type ListParams struct {
	Limit          int            `json:"limit"`
	ValueString    string         `json:"value_string"`
	ValueTimestamp time.Time      `json:"value_timestamp"`
	ID             string         `json:"id"`
	Position       Position       `json:"position"`
	OrderBy        OrderBy        `json:"order_by"`
	OrderDirection OrderDirection `json:"order_direction"`
}

type Entity = interface {
	Environment | Test | TestSuite | TestExecution | TestSuiteExecution
}

type ResourceEntity interface {
	Environment | Test | TestSuite
}

type Environment Resource
type Test Resource
type TestSuite Resource

type Resource struct {
	ID          string          `json:"id" yaml:"id"`
	Kind        string          `json:"kind" yaml:"kind"`
	Version     string          `json:"version" yaml:"version"`
	Name        string          `json:"name" yaml:"name"`
	Description string          `json:"description,omitempty" yaml:"description,omitempty"`
	Spec        json.RawMessage `json:"spec" yaml:"spec"`
	CreatedAt   time.Time       `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" yaml:"updated_at"`
}

type TestExecution struct {
	ID                   string    `json:"id" yaml:"id"`
	TestSuiteExecutionID string    `json:"test_suite_execution_id" yaml:"testsuite_execution_id"`
	StartedAt            time.Time `json:"started_at" yaml:"started_at"`
	FinishedAt           time.Time `json:"finished_at,omitzero" yaml:"finished_at,omitzero"`
	Status               string    `json:"status" yaml:"status"`

	EnvironmentID   string `json:"environment_id" yaml:"environment_id"`
	EnvironmentName string `json:"environment_name,omitempty" yaml:"environment_name,omitempty"`

	TestID   string `json:"test_id" yaml:"test_id"`
	TestName string `json:"test_name,omitempty" yaml:"test_name,omitempty"`

	Summary json.RawMessage `json:"summary,omitempty" yaml:"summary,omitempty"`
}

type TestSuiteExecution struct {
	ID         string    `json:"id" yaml:"id"`
	StartedAt  time.Time `json:"started_at" yaml:"started_at"`
	FinishedAt time.Time `json:"finished_at,omitzero" yaml:"finished_at,omitzero"`
	Status     string    `json:"status" yaml:"status"`

	EnvironmentID   string `json:"environment_id" yaml:"environment_id"`
	EnvironmentName string `json:"environment_name,omitempty" yaml:"environment_name,omitempty"`

	TestSuiteID   string `json:"test_suite_id" yaml:"test_suite_id"`
	TestSuiteName string `json:"test_suite_name,omitempty" yaml:"test_suite_name,omitempty"`

	TestsCount int `json:"tests_count" yaml:"tests_count"`

	Summary json.RawMessage `json:"summary,omitempty" yaml:"summary,omitempty"`
}

type OrderBy string

const (
	OrderByID         OrderBy = "id"
	OrderByName       OrderBy = "name"
	OrderByVersion    OrderBy = "version"
	OrderByCreatedAt  OrderBy = "createdAt"
	OrderByUpdatedAt  OrderBy = "updatedAt"
	OrderByStartedAt  OrderBy = "startedAt"
	OrderByFinishedAt OrderBy = "finishedAt"
	OrderByStatus     OrderBy = "status"
)

type OrderDirection string

const (
	OrderDirectionAsc  OrderDirection = "asc"
	OrderDirectionDesc OrderDirection = "desc"
)

type Position string

const (
	PositionAfter  Position = "after"
	PositionBefore Position = "before"
)
