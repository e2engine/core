package model

import (
	"context"
	nativeerrors "errors"

	"github.com/ygrebnov/errorc"
	modellib "github.com/ygrebnov/model"

	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
)

type Test Resource[TestSpec]

var testBinding *modellib.Binding[Test]

func (t *Test) Validate(ctx context.Context) error {
	if t.Kind != ResourceKindTest {
		return errorc.With(
			errors.ErrInvalidSpec,
			errorc.String(keys.SpecKind, string(EntityKindTest)),
			errorc.String(keys.Validation, "invalid resource kind"),
		)
	}

	if err := testBinding.Validate(ctx, t); err != nil {
		return nativeerrors.Join(
			errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindTest)),
			),
			err,
		)
	}

	if err := t.Spec.validateTags(); err != nil {
		return invalidTestSpec(err)
	}

	if err := t.Spec.validateProtocol(); err != nil {
		return invalidTestSpec(err)
	}

	if t.Spec.Request.HTTP != nil {
		if err := t.Spec.Request.HTTP.Validate(); err != nil {
			return invalidTestSpec(err)
		}
	}

	if t.Spec.Expect.HTTP != nil {
		if err := t.Spec.Expect.HTTP.Validate(); err != nil {
			return invalidTestSpec(err)
		}
	}

	for i := range t.Spec.Expect.Calls {
		if err := t.Spec.Expect.Calls[i].Validate(); err != nil {
			return invalidTestSpec(err)
		}
	}

	return nil
}

type TestSpec struct {
	Tags    []string    `json:"tags,omitempty" yaml:"tags,omitempty" validateElem:"min(1),max(200)"`
	Request RequestSpec `json:"request" yaml:"request"`
	Expect  ExpectSpec  `json:"expect" yaml:"expect"`
}

func (s *TestSpec) validateTags() error {
	tags := make(map[string]struct{}, len(s.Tags))

	for _, tag := range s.Tags {
		if _, ok := tags[tag]; ok {
			return errorc.With(
				errorc.New("test tags must be unique"),
				errorc.String(keys.TestTag, tag),
			)
		}

		tags[tag] = struct{}{}
	}

	return nil
}

func (s *TestSpec) validateProtocol() error {
	requestHTTP := s.Request.HTTP != nil
	requestGRPC := s.Request.GRPC != nil

	// Both nil or both non-nil are invalid.
	if requestHTTP == requestGRPC {
		return errorc.New(
			"request must contain exactly one of http or grpc",
		)
	}

	expectHTTP := s.Expect.HTTP != nil
	expectGRPC := s.Expect.GRPC != nil

	// Both nil or both non-nil are invalid.
	if expectHTTP == expectGRPC {
		return errorc.New(
			"expect must contain exactly one of http or grpc",
		)
	}

	if requestHTTP != expectHTTP ||
		requestGRPC != expectGRPC {
		return errorc.New(
			"request and expect protocols must match",
		)
	}

	return nil
}

func invalidTestSpec(validation error) error {
	return nativeerrors.Join(
		errorc.With(
			errors.ErrInvalidSpec,
			errorc.String(keys.SpecKind, string(EntityKindTest)),
		),
		validation,
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
