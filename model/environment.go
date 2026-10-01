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

type Environment Resource[EnvironmentSpec]

var environmentBinding *modellib.Binding[Environment]

func (e *Environment) Validate(ctx context.Context) error {
	if e.Kind != ResourceKindEnvironment {
		return errorc.With(
			errors.ErrInvalidSpec,
			errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
			errorc.String(keys.Validation, "invalid resource kind"),
		)
	}

	if err := environmentBinding.Validate(ctx, e); err != nil {
		return nativeerrors.Join(
			errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
			),
			err,
		)
	}

	if len(e.Spec.Services) == 0 {
		return errorc.With(
			errors.ErrInvalidSpec,
			errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
			errorc.String(keys.Validation, "environment spec must contain at least one service"),
		)
	}

	serviceIDs := make(map[string]struct{}, len(e.Spec.Services))
	addresses := make(map[string]struct{}, len(e.Spec.Services))

	for i := range e.Spec.Services {
		service := &e.Spec.Services[i]

		if _, ok := serviceIDs[service.ID]; ok {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
				errorc.String(keys.EnvironmentServiceID, service.ID),
				errorc.String(keys.Validation, "service ids must be unique"),
			)
		}
		serviceIDs[service.ID] = struct{}{}

		if _, ok := addresses[service.Address]; ok {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
				errorc.String(keys.EnvironmentServiceID, service.ID),
				errorc.String(keys.EnvironmentServiceListenAddress, service.Address),
				errorc.String(keys.Validation, "service addresses must be unique"),
			)
		}
		addresses[service.Address] = struct{}{}

		if err := service.Validate(); err != nil {
			return err
		}
	}

	return nil
}

type EnvironmentSpec struct {
	Services []ServiceSpec `json:"services" yaml:"services" validateElem:"dive"`
}

type ServiceKind string

const (
	ServiceKindHTTP ServiceKind = "http"
	ServiceKindGRPC ServiceKind = "grpc"
)

type ServiceMode string

const (
	ServiceModeReal   ServiceMode = "real"
	ServiceModeMocked ServiceMode = "mocked"
)

type ServiceSpec struct {
	ID   string      `json:"id" yaml:"id" validate:"min(3),max(200)"`
	Kind ServiceKind `json:"kind" yaml:"kind" validate:"oneof(http,grpc)"`
	Mode ServiceMode `json:"mode" yaml:"mode" validate:"oneof(real,mocked)"`
	// Stable logical address used inside the environment.
	Address string `json:"address" yaml:"address" validate:"networkaddress"`

	// Actual destination of a real service.
	HTTPTarget string `json:"http_target,omitempty" yaml:"http_target,omitempty" validate:"omitempty,httpurl"`
	GRPCTarget string `json:"grpc_target,omitempty" yaml:"grpc_target,omitempty" validate:"omitempty,networkaddress"`

	RequestDefaults *ServiceDefaults `json:"request_defaults,omitempty" yaml:"request_defaults,omitempty" validateElem:"omitempty,dive"`

	Fixtures []FixtureSpec `json:"fixtures,omitempty" yaml:"fixtures,omitempty" validateElem:"dive"`

	// gRPC-only
	Proto *GRPCProtoSpec `json:"proto,omitempty" yaml:"proto,omitempty" validateElem:"omitempty,dive"`
}

func (s *ServiceSpec) Validate() error {
	if s.Kind == ServiceKindHTTP {
		if s.Proto != nil {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
				errorc.String(keys.Validation, "proto spec not allowed for HTTP service"),
			)
		}
	}

	if s.Kind == ServiceKindGRPC {
		if s.Proto == nil {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
				errorc.String(keys.Validation, "proto spec is required for GRPC service"),
			)
		}

		if (s.Proto.External == nil) == (s.Proto.Internal == nil) {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
				errorc.String(
					keys.Validation,
					"exactly one of external or internal proto spec must be provided for GRPC service",
				),
			)
		}

		if s.Mode == ServiceModeReal && s.Proto.Internal != nil {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
				errorc.String(
					keys.Validation,
					"internal proto spec is not allowed for real GRPC service",
				),
			)
		}

		if s.Proto.Internal != nil &&
			len(s.Proto.Internal.Methods) == 0 {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
				errorc.String(
					keys.Validation,
					"internal proto spec must contain at least one method",
				),
			)
		}

		if s.Proto.External != nil {
			hasFile := s.Proto.External.File != ""
			hasBufModule := s.Proto.External.BufModule != ""

			if hasFile == hasBufModule {
				return errorc.With(
					errors.ErrInvalidSpec,
					errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
					errorc.String(
						keys.Validation,
						"exactly one of file or buf_module must be provided for external GRPC proto spec",
					),
				)
			}
		}
	}

	if s.Mode == ServiceModeMocked {
		if len(s.Fixtures) == 0 {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
				errorc.String(keys.Validation, "mocked service must have at least one fixture"),
			)
		}

		if s.HTTPTarget != "" || s.GRPCTarget != "" {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
				errorc.String(keys.Validation, "mocked service must not have a target"),
			)
		}
	}

	if s.Mode == ServiceModeReal {
		if (s.Kind == ServiceKindHTTP && s.HTTPTarget == "") ||
			(s.Kind == ServiceKindGRPC && s.GRPCTarget == "") {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
				errorc.String(keys.Validation, "target is required for real service"),
			)
		}

		if s.Kind == ServiceKindHTTP && s.GRPCTarget != "" {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
				errorc.String(keys.Validation, "GRPC target is not allowed for HTTP service"),
			)
		}

		if s.Kind == ServiceKindGRPC && s.HTTPTarget != "" {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
				errorc.String(keys.Validation, "HTTP target is not allowed for GRPC service"),
			)
		}

		if len(s.Fixtures) > 0 {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
				errorc.String(keys.Validation, "fixtures are not allowed for real service"),
			)
		}
	}

	for i := range s.Fixtures {
		fixture := &s.Fixtures[i]

		switch s.Kind {
		case ServiceKindHTTP:
			if fixture.When.HTTP == nil || fixture.Then.HTTP == nil {
				return errorc.With(
					errors.ErrInvalidSpec,
					errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
					errorc.String(keys.Validation, "HTTP service fixture must contain HTTP when and then"),
				)
			}
			if fixture.When.GRPC != nil || fixture.Then.GRPC != nil {
				return errorc.With(
					errors.ErrInvalidSpec,
					errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
					errorc.String(keys.Validation, "gRPC fixture is not allowed for HTTP service"),
				)
			}

			u, err := url.ParseRequestURI(fixture.When.HTTP.Path)
			if err != nil || u.Path == "" || !strings.HasPrefix(u.Path, "/") {
				return errorc.With(
					errors.ErrInvalidSpec,
					errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
					errorc.String(keys.Validation, "invalid HTTP fixture path"),
				)
			}

		case ServiceKindGRPC:
			if fixture.When.GRPC == nil || fixture.Then.GRPC == nil {
				return errorc.With(
					errors.ErrInvalidSpec,
					errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
					errorc.String(keys.Validation, "gRPC service fixture must contain gRPC when and then"),
				)
			}
			if fixture.When.HTTP != nil || fixture.Then.HTTP != nil {
				return errorc.With(
					errors.ErrInvalidSpec,
					errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
					errorc.String(keys.Validation, "HTTP fixture is not allowed for gRPC service"),
				)
			}
		}
	}

	return nil
}

type ServiceDefaults struct {
	Headers map[string][]string `json:"headers,omitempty" yaml:"headers,omitempty"`
}

type GRPCProtoSpec struct {
	External *GRPCExternalProtoSpec `json:"external,omitempty" yaml:"external,omitempty" validateElem:"omitempty,dive"`
	Internal *GRPCInternalProtoSpec `json:"internal,omitempty" yaml:"internal,omitempty" validateElem:"omitempty,dive"`
}

type GRPCExternalProtoSpec struct {
	File      string `json:"file,omitempty" yaml:"file,omitempty" validate:"min(1),omitempty"`
	BufModule string `json:"buf_module,omitempty" yaml:"buf_module,omitempty" validate:"min(1),omitempty"`
	Service   string `json:"service" yaml:"service" validate:"min(1)"`
}

type GRPCInternalProtoSpec struct {
	Package string           `json:"package" yaml:"package" validate:"min(1)"`
	Service string           `json:"service" yaml:"service" validate:"min(1)"`
	Methods []GRPCMethodSpec `json:"methods" yaml:"methods" validateElem:"dive"`
}

type GRPCMethodSpec struct {
	Name     string          `json:"name" yaml:"name" validate:"min(1)"`
	Request  GRPCMessageSpec `json:"request" yaml:"request" validateElem:"dive"`
	Response GRPCMessageSpec `json:"response" yaml:"response" validateElem:"dive"`
}

type GRPCMessageSpec struct {
	Fields []GRPCFieldSpec `json:"fields,omitempty" yaml:"fields,omitempty" validateElem:"dive"`
}

type GRPCFieldSpec struct {
	Name     string        `json:"name" yaml:"name" validate:"min(1)"`
	Type     GRPCFieldType `json:"type" yaml:"type" validate:"oneof(string,bool,int32,int64,uint32,uint64,float,double,bytes)"`
	Repeated bool          `json:"repeated,omitempty" yaml:"repeated,omitempty"`
}

type GRPCFieldType string

const (
	GRPCFieldTypeString GRPCFieldType = "string"
	GRPCFieldTypeBool   GRPCFieldType = "bool"
	GRPCFieldTypeInt32  GRPCFieldType = "int32"
	GRPCFieldTypeInt64  GRPCFieldType = "int64"
	GRPCFieldTypeUint32 GRPCFieldType = "uint32"
	GRPCFieldTypeUint64 GRPCFieldType = "uint64"
	GRPCFieldTypeFloat  GRPCFieldType = "float"
	GRPCFieldTypeDouble GRPCFieldType = "double"
	GRPCFieldTypeBytes  GRPCFieldType = "bytes"
)

type FixtureSpec struct {
	When FixtureWhen `json:"when" yaml:"when"`
	Then FixtureThen `json:"then" yaml:"then"`
}

type FixtureWhen struct {
	HTTP *HTTPFixtureWhen `json:"http,omitempty" yaml:"http,omitempty" validateElem:"omitempty,dive"`
	GRPC *GRPCFixtureWhen `json:"grpc,omitempty" yaml:"grpc,omitempty" validateElem:"omitempty,dive"`
}

type FixtureThen struct {
	HTTP *HTTPFixtureThen `json:"http,omitempty" yaml:"http,omitempty" validateElem:"omitempty,dive"`
	GRPC *GRPCFixtureThen `json:"grpc,omitempty" yaml:"grpc,omitempty" validateElem:"omitempty,dive"`
}

type HTTPFixtureWhen struct {
	Method  string              `json:"method" yaml:"method" validate:"oneof(GET,POST,PUT,DELETE,PATCH,OPTIONS,HEAD)"`
	Path    string              `json:"path" yaml:"path" validate:"min(1)"`
	Headers map[string][]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Body    string              `json:"body,omitempty" yaml:"body,omitempty" validate:"omitempty,validjson"`
}

type HTTPFixtureThen struct {
	Status  int                 `json:"status" yaml:"status" validate:"min(100),max(599)"`
	Headers map[string][]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Body    string              `json:"body,omitempty" yaml:"body,omitempty" validate:"omitempty,validjson"`
}

type GRPCFixtureWhen struct {
	Method  string         `json:"method" yaml:"method" validate:"min(1)"`
	Message map[string]any `json:"message,omitempty" yaml:"message,omitempty"`
}

type GRPCFixtureThen struct {
	Status  string         `json:"status,omitempty" yaml:"status,omitempty" validate:"omitempty,oneof(OK,Canceled,Unknown,InvalidArgument,DeadlineExceeded,NotFound,AlreadyExists,PermissionDenied,ResourceExhausted,FailedPrecondition,Aborted,OutOfRange,Unimplemented,Internal,Unavailable,DataLoss,Unauthenticated)"`
	Message map[string]any `json:"message,omitempty" yaml:"message,omitempty"`
}

type EnvironmentsPage = Page[Environment]
