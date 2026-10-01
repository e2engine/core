package model

import (
	"time"
)

type EntityKind string

const (
	EntityKindEnvironment        EntityKind = "environment"
	EntityKindTest               EntityKind = "test"
	EntityKindTestSuite          EntityKind = "testsuite"
	EntityKindTestExecution      EntityKind = "testexecution"
	EntityKindTestSuiteExecution EntityKind = "testsuiteexecution"
)

type Entity = interface {
	Environment | Test | TestSuite | TestExecution | TestSuiteExecution
}

type ResourceSpec = interface {
	EnvironmentSpec | TestSpec | TestSuiteSpec
}

type ResourceKind string

const (
	ResourceKindEnvironment ResourceKind = "Environment"
	ResourceKindTest        ResourceKind = "Test"
	ResourceKindTestSuite   ResourceKind = "TestSuite"
)

type Resource[S ResourceSpec] struct {
	Kind        ResourceKind `json:"kind" yaml:"kind" validate:"oneof(Environment,Test,TestSuite)"`
	ID          string       `json:"id" yaml:"id" validate:"id,omitempty"`
	Name        string       `json:"name" yaml:"name" validate:"min(3),max(200)"`
	Version     string       `json:"version" yaml:"version" validate:"semver"`
	Description string       `json:"description,omitempty" yaml:"description,omitempty" validate:"omitempty,min(3),max(2000)"`
	Spec        S            `json:"spec" yaml:"spec"`
	CreatedAt   time.Time    `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at" yaml:"updated_at"`
}
