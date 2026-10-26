package model

import (
	"context"
	"strconv"

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
			errorc.String(keys.Fields, "Kind"),
			errorc.String(keys.Validation, "invalid kind"),
		)
	}

	if err := environmentBinding.Validate(ctx, e); err != nil {
		return handleValidationError(err, EntityKindEnvironment)
	}

	if len(e.Spec.Services) == 0 {
		return errorc.With(
			errors.ErrInvalidSpec,
			errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
			errorc.String(keys.Fields, "Spec.Services"),
			errorc.String(keys.Validation, "environment spec must contain at least one service"),
		)
	}

	serviceIDs := make(map[string]struct{}, len(e.Spec.Services))
	ports := make(map[int]struct{}, len(e.Spec.Services))

	for i := range e.Spec.Services {
		service := &e.Spec.Services[i]

		if _, ok := serviceIDs[service.ID]; ok {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
				errorc.String(keys.Fields, "Spec.Services["+strconv.Itoa(i)+"].ID"),
				errorc.String(keys.Validation, "service ids must be unique"),
			)
		}
		serviceIDs[service.ID] = struct{}{}

		if _, ok := ports[service.E2EnginePort]; ok {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
				errorc.String(keys.Fields, "Spec.Services["+strconv.Itoa(i)+"].E2EnginePort"),
				errorc.String(
					keys.Validation,
					"E2Engine ports must be unique within an environment",
				),
			)
		}

		ports[service.E2EnginePort] = struct{}{}

		if err := service.Validate(i); err != nil {
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

	E2EnginePort int `json:"e2engine_port" yaml:"e2engine_port" validate:"min(1),max(65535)"`

	// Required for real services, forbidden for mocked services.
	// The hostname may be overridden by runtime hostname mappings.
	Target string `json:"target,omitempty" yaml:"target,omitempty"`

	RequestDefaults *RequestDefaults `json:"request_defaults,omitempty" yaml:"request_defaults,omitempty" validateElem:"omitempty,dive"`
	Fixtures        []FixtureSpec    `json:"fixtures,omitempty" yaml:"fixtures,omitempty" validateElem:"dive"`
	Proto           *GRPCProtoSpec   `json:"proto,omitempty" yaml:"proto,omitempty" validateElem:"omitempty,dive"`
}

func (s *ServiceSpec) Validate(index int) error {
	if s.Kind == ServiceKindHTTP {
		if s.Proto != nil {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
				errorc.String(
					keys.Fields,
					"Spec.Services["+strconv.Itoa(index)+"].Proto",
				),
				errorc.String(keys.Validation, "proto spec is not allowed for HTTP service"),
			)
		}

		if s.RequestDefaults != nil && len(s.RequestDefaults.Metadata) > 0 {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
				errorc.String(
					keys.Fields,
					"Spec.Services["+strconv.Itoa(index)+"].RequestDefaults.Metadata",
				),
				errorc.String(keys.Validation, "metadata is not allowed for HTTP service"),
			)
		}
	}

	if s.Kind == ServiceKindGRPC {
		if s.RequestDefaults != nil && len(s.RequestDefaults.Headers) > 0 {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
				errorc.String(
					keys.Fields,
					"Spec.Services["+strconv.Itoa(index)+"].RequestDefaults.Headers",
				),
				errorc.String(keys.Validation, "headers are not allowed for GRPC service"),
			)
		}

		if s.Proto == nil {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
				errorc.String(
					keys.Fields,
					"Spec.Services["+strconv.Itoa(index)+"].Proto",
				),
				errorc.String(keys.Validation, "proto spec is required for GRPC service"),
			)
		}

		if (s.Proto.External == nil) == (s.Proto.Internal == nil) {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
				errorc.String(
					keys.Fields,
					"Spec.Services["+strconv.Itoa(index)+"].Proto.External, "+
						"Spec.Services["+strconv.Itoa(index)+"].Proto.Internal",
				),
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
					keys.Fields,
					"Spec.Services["+strconv.Itoa(index)+"].Mode, "+
						"Spec.Services["+strconv.Itoa(index)+"].Proto.Internal",
				),
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
					keys.Fields,
					"Spec.Services["+strconv.Itoa(index)+"].Proto.Internal.Methods",
				),
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
						keys.Fields,
						"Spec.Services["+strconv.Itoa(index)+"].Proto.External.File, "+
							"Spec.Services["+strconv.Itoa(index)+"].Proto.External.BufModule",
					),
					errorc.String(
						keys.Validation,
						"exactly one of file or buf_module must be provided for external GRPC proto spec",
					),
				)
			}
		}
	}

	if s.Mode == ServiceModeMocked {
		if s.Target != "" {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
				errorc.String(
					keys.Fields,
					"Spec.Services["+strconv.Itoa(index)+"].Mode, "+
						"Spec.Services["+strconv.Itoa(index)+"].Target",
				),
				errorc.String(keys.Validation, "target is not allowed for mocked service"),
			)
		}

		if len(s.Fixtures) == 0 {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
				errorc.String(
					keys.Fields,
					"Spec.Services["+strconv.Itoa(index)+"].Mode, "+
						"Spec.Services["+strconv.Itoa(index)+"].Fixtures",
				),
				errorc.String(keys.Validation, "mocked service must contain at least one fixture"),
			)
		}
	}

	if s.Mode == ServiceModeReal {
		if s.Target == "" {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
				errorc.String(
					keys.Fields,
					"Spec.Services["+strconv.Itoa(index)+"].Target",
				),
				errorc.String(keys.Validation, "target is required for real service"),
			)
		}

		if len(s.Fixtures) > 0 {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
				errorc.String(
					keys.Fields,
					"Spec.Services["+strconv.Itoa(index)+"].Mode, "+
						"Spec.Services["+strconv.Itoa(index)+"].Fixtures",
				),
				errorc.String(keys.Validation, "fixtures are not allowed for real service"),
			)
		}

		switch s.Kind {
		case ServiceKindHTTP:
			if err := validateHTTPURL(s.Target); err != nil {
				return errorc.With(
					errors.ErrInvalidSpec,
					errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
					errorc.String(
						keys.Fields,
						"Spec.Services["+strconv.Itoa(index)+"].Target",
					),
					errorc.String(keys.Validation, err.Error()),
				)
			}
		case ServiceKindGRPC:
			if err := validateNetworkAddress(s.Target); err != nil {
				return errorc.With(
					errors.ErrInvalidSpec,
					errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
					errorc.String(
						keys.Fields,
						"Spec.Services["+strconv.Itoa(index)+"].Target",
					),
					errorc.String(keys.Validation, err.Error()),
				)
			}
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
					errorc.String(
						keys.Fields,
						"Spec.Services["+strconv.Itoa(index)+"].Fixtures["+strconv.Itoa(i)+"].When.HTTP, "+
							"Spec.Services["+strconv.Itoa(index)+"].Fixtures["+strconv.Itoa(i)+"].Then.HTTP",
					),
					errorc.String(keys.Validation, "HTTP service fixture must contain HTTP when and then"),
				)
			}

			if fixture.When.GRPC != nil || fixture.Then.GRPC != nil {
				return errorc.With(
					errors.ErrInvalidSpec,
					errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
					errorc.String(
						keys.Fields,
						"Spec.Services["+strconv.Itoa(index)+"].Fixtures["+strconv.Itoa(i)+"].When.GRPC, "+
							"Spec.Services["+strconv.Itoa(index)+"].Fixtures["+strconv.Itoa(i)+"].Then.GRPC",
					),
					errorc.String(keys.Validation, "gRPC fixture is not allowed for HTTP service"),
				)
			}

			if err := fixture.When.HTTP.Validate(); err != nil {
				return errorc.With(
					errors.ErrInvalidSpec,
					errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
					errorc.String(
						keys.Fields,
						"Spec.Services["+strconv.Itoa(index)+"].Fixtures["+strconv.Itoa(i)+"].When.HTTP",
					),
					errorc.String(keys.Validation, err.Error()),
				)
			}

			if err := fixture.Then.HTTP.Validate(); err != nil {
				return errorc.With(
					errors.ErrInvalidSpec,
					errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
					errorc.String(
						keys.Fields,
						"Spec.Services["+strconv.Itoa(index)+"].Fixtures["+strconv.Itoa(i)+"].Then.HTTP",
					),
					errorc.String(keys.Validation, err.Error()),
				)
			}

		case ServiceKindGRPC:
			if fixture.When.GRPC == nil || fixture.Then.GRPC == nil {
				return errorc.With(
					errors.ErrInvalidSpec,
					errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
					errorc.String(
						keys.Fields,
						"Spec.Services["+strconv.Itoa(index)+"].Fixtures["+strconv.Itoa(i)+"].When.GRPC, "+
							"Spec.Services["+strconv.Itoa(index)+"].Fixtures["+strconv.Itoa(i)+"].Then.GRPC"),
					errorc.String(keys.Validation, "gRPC service fixture must contain gRPC when and then"),
				)
			}

			if fixture.When.HTTP != nil || fixture.Then.HTTP != nil {
				return errorc.With(
					errors.ErrInvalidSpec,
					errorc.String(keys.SpecKind, string(EntityKindEnvironment)),
					errorc.String(
						keys.Fields,
						"Spec.Services["+strconv.Itoa(index)+"].Fixtures["+strconv.Itoa(i)+"].When.HTTP, "+
							"Spec.Services["+strconv.Itoa(index)+"].Fixtures["+strconv.Itoa(i)+"].Then.HTTP",
					),
					errorc.String(keys.Validation, "HTTP fixture is not allowed for gRPC service"),
				)
			}
		}
	}

	return nil
}

type RequestDefaults struct {
	Headers  map[string][]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Metadata map[string][]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
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
	HTTP *HTTPRequestPattern `json:"http,omitempty" yaml:"http,omitempty" validateElem:"omitempty,dive"`
	GRPC *GRPCRequestPattern `json:"grpc,omitempty" yaml:"grpc,omitempty" validateElem:"omitempty,dive"`
}

type FixtureThen struct {
	HTTP *HTTPResponse `json:"http,omitempty" yaml:"http,omitempty" validateElem:"omitempty,dive"`
	GRPC *GRPCResponse `json:"grpc,omitempty" yaml:"grpc,omitempty" validateElem:"omitempty,dive"`
}

type EnvironmentsPage = Page[Environment]
