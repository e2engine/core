package model

import (
	"context"
	nativeerrors "errors"
	"net/url"
	"strings"

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
	HTTP *HTTPRequestSpec `json:"http,omitempty" yaml:"http,omitempty" validateElem:"omitempty,dive"`
	GRPC *GRPCRequestSpec `json:"grpc,omitempty" yaml:"grpc,omitempty" validateElem:"omitempty,dive"`
}

type ExpectSpec struct {
	HTTP  *HTTPExpectSpec   `json:"http,omitempty" yaml:"http,omitempty" validateElem:"omitempty,dive"`
	GRPC  *GRPCExpectSpec   `json:"grpc,omitempty" yaml:"grpc,omitempty" validateElem:"omitempty,dive"`
	Calls []CallExpectation `json:"calls,omitempty" yaml:"calls,omitempty" validateElem:"dive"`
}

type CallExpectation struct {
	ServiceID string               `json:"service_id" yaml:"service_id" validate:"min(3),max(200)"`
	Count     *int                 `json:"count,omitempty" yaml:"count,omitempty" validate:"omitempty,min(0)"`
	HTTP      *HTTPCallExpectation `json:"http,omitempty" yaml:"http,omitempty" validateElem:"omitempty,dive"`
	GRPC      *GRPCCallExpectation `json:"grpc,omitempty" yaml:"grpc,omitempty" validateElem:"omitempty,dive"`
}

func (c *CallExpectation) Validate() error {
	httpSet := c.HTTP != nil
	grpcSet := c.GRPC != nil

	if httpSet == grpcSet {
		return errorc.New(
			"call expectation must contain exactly one of http or grpc",
		)
	}

	return nil
}

type HTTPCallExpectation struct {
	Method  string              `json:"method" yaml:"method" validate:"oneof(GET,POST,PUT,DELETE,PATCH,OPTIONS,HEAD)"`
	Path    string              `json:"path" yaml:"path" validate:"min(1)"`
	Query   map[string][]string `json:"query,omitempty" yaml:"query,omitempty"`
	Headers map[string][]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Body    string              `json:"body,omitempty" yaml:"body,omitempty" validate:"omitempty,validjson"`
}

type GRPCCallExpectation struct {
	Service  string              `json:"service" yaml:"service" validate:"min(1)"`
	Method   string              `json:"method" yaml:"method" validate:"min(1)"`
	Metadata map[string][]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Message  map[string]any      `json:"message,omitempty" yaml:"message,omitempty"`
}

type HTTPRequestSpec struct {
	Method  string              `json:"method" yaml:"method" validate:"oneof(GET,POST,PUT,DELETE,PATCH,OPTIONS,HEAD)"`
	URL     string              `json:"url" yaml:"url" validate:"min(1)"`
	Query   map[string]string   `json:"query,omitempty" yaml:"query,omitempty"`
	Headers map[string][]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Body    string              `json:"body,omitempty" yaml:"body,omitempty" validate:"omitempty,validjson"`
}

func (s *HTTPRequestSpec) Validate() error {
	u, err := url.Parse(s.URL)
	if err != nil {
		return errorc.With(
			errorc.New("invalid HTTP request URL"),
			errorc.Error(keys.Cause, err),
		)
	}

	if u.Scheme != "http" &&
		u.Scheme != "https" {
		return errorc.New(
			"HTTP request URL scheme must be http or https",
		)
	}

	if strings.TrimSpace(u.Host) == "" {
		return errorc.New(
			"HTTP request URL must contain a host",
		)
	}

	return nil
}

type HTTPExpectSpec struct {
	Status int    `json:"status" yaml:"status" validate:"min(100),max(599)"`
	Body   string `json:"body,omitempty" yaml:"body,omitempty" validate:"omitempty,validjson"`
}

type GRPCRequestSpec struct {
	Target   string              `json:"target" yaml:"target" validate:"networkaddress"`
	Service  string              `json:"service" yaml:"service" validate:"min(1)"`
	Method   string              `json:"method" yaml:"method" validate:"min(1)"`
	Metadata map[string][]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Message  map[string]any      `json:"message,omitempty" yaml:"message,omitempty"`
}

type GRPCExpectSpec struct {
	Status  string         `json:"status" yaml:"status" validate:"oneof(OK,Canceled,Unknown,InvalidArgument,DeadlineExceeded,NotFound,AlreadyExists,PermissionDenied,ResourceExhausted,FailedPrecondition,Aborted,OutOfRange,Unimplemented,Internal,Unavailable,DataLoss,Unauthenticated)"`
	Message map[string]any `json:"message,omitempty" yaml:"message,omitempty"`
}

type TestsPage = Page[Test]
