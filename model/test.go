package model

import (
	"context"
	"strconv"

	"github.com/ygrebnov/errorc"
	modellib "github.com/ygrebnov/model"

	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
)

type Test Resource[TestSpec]

var testBinding *modellib.Binding[Test]

func (t *Test) Validate(ctx context.Context) error {
	if t.Kind != ResourceKindTest {
		return invalidTestSpec(errorc.New("invalid kind"), "Kind")
	}

	if err := testBinding.Validate(ctx, t); err != nil {
		return handleValidationError(err, EntityKindTest)
	}

	if index, err := t.Spec.validateTags(); err != nil {
		return invalidTestSpec(err, "Spec.Tags["+strconv.Itoa(index)+"]")
	}

	fields, err := t.Spec.validateProtocol()
	if err != nil {
		return invalidTestSpec(err, fields)
	}

	if t.Spec.Request.HTTP != nil {
		if err := t.Spec.Request.HTTP.Validate(); err != nil {
			return invalidTestSpec(err, "Spec.Request.HTTP")
		}
	}

	if t.Spec.Expect.HTTP != nil {
		if err := t.Spec.Expect.HTTP.Validate(); err != nil {
			return invalidTestSpec(err, "Spec.Expect.HTTP")
		}
	}

	for i := range t.Spec.Expect.Calls {
		if err := t.Spec.Expect.Calls[i].Validate(); err != nil {
			return invalidTestSpec(err, "Spec.Expect.Calls["+strconv.Itoa(i)+"]")
		}
	}

	return nil
}

type TestSpec struct {
	Tags    []string    `json:"tags,omitempty" yaml:"tags,omitempty" validateElem:"min(1),max(200)"`
	Request RequestSpec `json:"request" yaml:"request"`
	Expect  ExpectSpec  `json:"expect" yaml:"expect"`
}

func (s *TestSpec) validateTags() (int, error) {
	tags := make(map[string]struct{}, len(s.Tags))

	for i, tag := range s.Tags {
		if _, ok := tags[tag]; ok {
			return i, errorc.With(
				errorc.New("test tags must be unique"),
				errorc.String(keys.TestTag, tag),
			)
		}

		tags[tag] = struct{}{}
	}

	return 0, nil
}

func (s *TestSpec) validateProtocol() (string, error) {
	requestHTTP := s.Request.HTTP != nil
	requestGRPC := s.Request.GRPC != nil

	// Both nil or both non-nil are invalid.
	if requestHTTP == requestGRPC {
		return "Spec.Request.HTTP, Spec.Request.GRPC",
			errorc.New("request must contain exactly one of http or grpc")
	}

	expectHTTP := s.Expect.HTTP != nil
	expectGRPC := s.Expect.GRPC != nil

	// Both nil or both non-nil are invalid.
	if expectHTTP == expectGRPC {
		return "Spec.Expect.HTTP, Spec.Expect.GRPC",
			errorc.New("expect must contain exactly one of http or grpc")
	}

	if requestHTTP != expectHTTP {
		return "Spec.Request.HTTP, Spec.Expect.HTTP",
			errorc.New("request and expect protocols must match")
	}

	if requestGRPC != expectGRPC {
		return "Spec.Request.GRPC, Spec.Expect.GRPC",
			errorc.New("request and expect protocols must match")
	}

	return "", nil
}

func invalidTestSpec(validation error, fields string) error {
	return errorc.With(
		errors.ErrInvalidSpec,
		errorc.String(keys.SpecKind, string(EntityKindTest)),
		errorc.String(keys.Fields, fields),
		errorc.String(keys.Validation, validation.Error()),
	)
}

type RequestSpec struct {
	HTTP *HTTPRequest `json:"http,omitempty" yaml:"http,omitempty" validateElem:"omitempty,dive"`
	GRPC *GRPCRequest `json:"grpc,omitempty" yaml:"grpc,omitempty" validateElem:"omitempty,dive"`
}

type ExpectSpec struct {
	HTTP  *HTTPResponsePattern   `json:"http,omitempty" yaml:"http,omitempty" validateElem:"omitempty,dive"`
	GRPC  *GRPCResponsePattern   `json:"grpc,omitempty" yaml:"grpc,omitempty" validateElem:"omitempty,dive"`
	Calls []ServiceExpectedCalls `json:"calls,omitempty" yaml:"calls,omitempty" validateElem:"dive"`
}

type ServiceExpectedCalls struct {
	ServiceID string              `json:"service_id" yaml:"service_id" validate:"min(3),max(200)"`
	Count     *int                `json:"count,omitempty" yaml:"count,omitempty" validate:"omitempty,min(0)"`
	HTTP      *HTTPRequestPattern `json:"http,omitempty" yaml:"http,omitempty" validateElem:"omitempty,dive"`
	GRPC      *GRPCRequestPattern `json:"grpc,omitempty" yaml:"grpc,omitempty" validateElem:"omitempty,dive"`
}

func (c *ServiceExpectedCalls) Validate() error {
	httpSet := c.HTTP != nil
	grpcSet := c.GRPC != nil

	if httpSet == grpcSet {
		return errorc.New(
			"call expectation must contain exactly one of http or grpc",
		)
	}

	if c.HTTP != nil {
		return c.HTTP.Validate()
	}

	return nil
}

type TestsPage = Page[Test]
