package grpc

import (
	"context"
	nativeerrors "errors"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
)

func TestProtoMethodResolverResolveExternalModule(t *testing.T) {
	descriptorSet := testDescriptorSet()

	loader := &testDescriptorLoader{
		descriptorSet: descriptorSet,
	}

	resolver := NewProtoMethodResolver(loader)

	spec := &model.GRPCProtoSpec{
		External: &model.GRPCExternalProtoSpec{
			BufModule: "buf.build/acme/users",
			Service:   "users.v1.UserService",
		},
	}

	method, err := resolver.ResolveMethod(
		context.Background(),
		spec,
		"GetUser",
	)
	if err != nil {
		t.Fatalf("ResolveMethod() error = %v", err)
	}

	if method.Name() != "GetUser" {
		t.Errorf(
			"expected method %q, got %q",
			"GetUser",
			method.Name(),
		)
	}

	if loader.calls != 1 {
		t.Errorf(
			"expected loader to be called once, got %d",
			loader.calls,
		)
	}

	if loader.module != "buf.build/acme/users" {
		t.Errorf(
			"expected module %q, got %q",
			"buf.build/acme/users",
			loader.module,
		)
	}

	// Resolve the same contract again. The descriptor registry should
	// come from the module cache rather than invoking the loader.
	method, err = resolver.ResolveMethod(
		context.Background(),
		spec,
		"GetUser",
	)
	if err != nil {
		t.Fatalf(
			"second ResolveMethod() error = %v",
			err,
		)
	}

	if method.Name() != "GetUser" {
		t.Errorf(
			"expected method %q, got %q",
			"GetUser",
			method.Name(),
		)
	}

	if loader.calls != 1 {
		t.Errorf(
			"expected cached module to avoid another load, got %d calls",
			loader.calls,
		)
	}
}

func TestProtoMethodResolverResolveExternalModuleLoadError(
	t *testing.T,
) {
	loader := &testDescriptorLoader{
		err: nativeerrors.New("load failed"),
	}

	resolver := NewProtoMethodResolver(loader)

	spec := &model.GRPCProtoSpec{
		External: &model.GRPCExternalProtoSpec{
			BufModule: "buf.build/acme/users",
			Service:   "users.v1.UserService",
		},
	}

	_, err := resolver.ResolveMethod(
		context.Background(),
		spec,
		"GetUser",
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !nativeerrors.Is(
		err,
		errors.ErrCannotLoadGRPCProto,
	) {
		t.Errorf(
			"expected error %v, got %v",
			errors.ErrCannotLoadGRPCProto,
			err,
		)
	}
}

func TestProtoMethodResolverResolveExternalFile(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "users.proto")

	data := []byte(`
syntax = "proto3";

package users.v1;

service UserService {
	rpc GetUser(GetUserRequest) returns (GetUserResponse);
}

message GetUserRequest {
	string id = 1;
}

message GetUserResponse {
	string name = 1;
}
`)

	if err := os.WriteFile(
		path,
		data,
		0o600,
	); err != nil {
		t.Fatalf("cannot write proto file: %v", err)
	}

	resolver := NewProtoMethodResolver(nil)

	spec := &model.GRPCProtoSpec{
		External: &model.GRPCExternalProtoSpec{
			File:    path,
			Service: "users.v1.UserService",
		},
	}

	method, err := resolver.ResolveMethod(
		context.Background(),
		spec,
		"GetUser",
	)
	if err != nil {
		t.Fatalf("ResolveMethod() error = %v", err)
	}

	if method.Name() != "GetUser" {
		t.Errorf(
			"expected method %q, got %q",
			"GetUser",
			method.Name(),
		)
	}

	// Exercise the file cache.
	method, err = resolver.ResolveMethod(
		context.Background(),
		spec,
		"GetUser",
	)
	if err != nil {
		t.Fatalf(
			"second ResolveMethod() error = %v",
			err,
		)
	}

	if method.Name() != "GetUser" {
		t.Errorf(
			"expected method %q, got %q",
			"GetUser",
			method.Name(),
		)
	}
}

func TestProtoMethodResolverResolveInvalidContract(t *testing.T) {
	resolver := NewProtoMethodResolver(nil)

	tests := []struct {
		name string
		spec *model.GRPCProtoSpec
	}{
		{
			name: "no proto source",
			spec: &model.GRPCProtoSpec{},
		},
		{
			name: "external without source",
			spec: &model.GRPCProtoSpec{
				External: &model.GRPCExternalProtoSpec{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolver.ResolveMethod(
				context.Background(),
				tt.spec,
				"GetUser",
			)
			if err == nil {
				t.Fatal("expected error")
			}

			if !nativeerrors.Is(
				err,
				errors.ErrInvalidGRPCProtoContract,
			) {
				t.Errorf(
					"expected error %v, got %v",
					errors.ErrInvalidGRPCProtoContract,
					err,
				)
			}
		})
	}
}

func TestProtoMethodResolverResolveMethodNotFound(t *testing.T) {
	loader := &testDescriptorLoader{
		descriptorSet: testDescriptorSet(),
	}

	resolver := NewProtoMethodResolver(loader)

	tests := []struct {
		name    string
		service string
		method  string
	}{
		{
			name:    "service not found",
			service: "users.v1.MissingService",
			method:  "GetUser",
		},
		{
			name:    "method not found",
			service: "users.v1.UserService",
			method:  "MissingMethod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolver.ResolveMethod(
				context.Background(),
				&model.GRPCProtoSpec{
					External: &model.GRPCExternalProtoSpec{
						BufModule: "buf.build/acme/users",
						Service:   tt.service,
					},
				},
				tt.method,
			)
			if err == nil {
				t.Fatal("expected error")
			}

			if !nativeerrors.Is(
				err,
				errors.ErrCannotResolveGRPCMethod,
			) {
				t.Errorf(
					"expected error %v, got %v",
					errors.ErrCannotResolveGRPCMethod,
					err,
				)
			}
		})
	}
}

func TestProtoMethodResolverResolveInternal(t *testing.T) {
	resolver := NewProtoMethodResolver(nil)

	spec := &model.GRPCInternalProtoSpec{
		Package: "users.v1",
		Service: "UserService",
		Methods: []model.GRPCMethodSpec{
			{
				Name: "GetUser",
				Request: model.GRPCMessageSpec{
					Fields: []model.GRPCFieldSpec{
						{
							Name: "id",
							Type: model.GRPCFieldTypeString,
						},
						{
							Name:     "roles",
							Type:     model.GRPCFieldTypeString,
							Repeated: true,
						},
					},
				},
				Response: model.GRPCMessageSpec{
					Fields: []model.GRPCFieldSpec{
						{
							Name: "name",
							Type: model.GRPCFieldTypeString,
						},
						{
							Name: "active",
							Type: model.GRPCFieldTypeBool,
						},
					},
				},
			},
		},
	}

	protoSpec := &model.GRPCProtoSpec{
		Internal: spec,
	}

	method, err := resolver.ResolveMethod(
		context.Background(),
		protoSpec,
		"GetUser",
	)
	if err != nil {
		t.Fatalf("ResolveMethod() error = %v", err)
	}

	if method.FullName() != "users.v1.UserService.GetUser" {
		t.Errorf(
			"expected full name %q, got %q",
			"users.v1.UserService.GetUser",
			method.FullName(),
		)
	}

	request := method.Input()

	if request.FullName() != "users.v1.GetUserRequest" {
		t.Errorf(
			"expected request %q, got %q",
			"users.v1.GetUserRequest",
			request.FullName(),
		)
	}

	id := request.Fields().ByName("id")
	if id == nil {
		t.Fatal("expected id field")
	}

	if id.Kind() != protoreflect.StringKind {
		t.Errorf(
			"expected id kind %v, got %v",
			protoreflect.StringKind,
			id.Kind(),
		)
	}

	if id.Number() != 1 {
		t.Errorf(
			"expected id field number 1, got %d",
			id.Number(),
		)
	}

	roles := request.Fields().ByName("roles")
	if roles == nil {
		t.Fatal("expected roles field")
	}

	if !roles.IsList() {
		t.Error("expected roles to be repeated")
	}

	if roles.Number() != 2 {
		t.Errorf(
			"expected roles field number 2, got %d",
			roles.Number(),
		)
	}

	response := method.Output()

	if response.FullName() != "users.v1.GetUserResponse" {
		t.Errorf(
			"expected response %q, got %q",
			"users.v1.GetUserResponse",
			response.FullName(),
		)
	}

	active := response.Fields().ByName("active")
	if active == nil {
		t.Fatal("expected active field")
	}

	if active.Kind() != protoreflect.BoolKind {
		t.Errorf(
			"expected active kind %v, got %v",
			protoreflect.BoolKind,
			active.Kind(),
		)
	}

	// Resolve again to exercise the inline cache.
	method2, err := resolver.ResolveMethod(
		context.Background(),
		protoSpec,
		"GetUser",
	)
	if err != nil {
		t.Fatalf(
			"second ResolveMethod() error = %v",
			err,
		)
	}

	if method2.FullName() != method.FullName() {
		t.Errorf(
			"expected cached method %q, got %q",
			method.FullName(),
			method2.FullName(),
		)
	}
}

func TestProtoMethodResolverInternalUnsupportedFieldType(
	t *testing.T,
) {
	resolver := NewProtoMethodResolver(nil)

	spec := &model.GRPCProtoSpec{
		Internal: &model.GRPCInternalProtoSpec{
			Service: "UserService",
			Methods: []model.GRPCMethodSpec{
				{
					Name: "GetUser",
					Request: model.GRPCMessageSpec{
						Fields: []model.GRPCFieldSpec{
							{
								Name: "id",
								Type: model.GRPCFieldType(
									"unsupported",
								),
							},
						},
					},
				},
			},
		},
	}

	_, err := resolver.ResolveMethod(
		context.Background(),
		spec,
		"GetUser",
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !nativeerrors.Is(
		err,
		errors.ErrCannotBuildGRPCDescriptor,
	) {
		t.Errorf(
			"expected error %v, got %v",
			errors.ErrCannotBuildGRPCDescriptor,
			err,
		)
	}
}

func TestProtobufFieldType(t *testing.T) {
	tests := []struct {
		name      string
		fieldType model.GRPCFieldType
		expected  descriptorpb.FieldDescriptorProto_Type
	}{
		{
			name:      "string",
			fieldType: model.GRPCFieldTypeString,
			expected:  descriptorpb.FieldDescriptorProto_TYPE_STRING,
		},
		{
			name:      "bool",
			fieldType: model.GRPCFieldTypeBool,
			expected:  descriptorpb.FieldDescriptorProto_TYPE_BOOL,
		},
		{
			name:      "int32",
			fieldType: model.GRPCFieldTypeInt32,
			expected:  descriptorpb.FieldDescriptorProto_TYPE_INT32,
		},
		{
			name:      "int64",
			fieldType: model.GRPCFieldTypeInt64,
			expected:  descriptorpb.FieldDescriptorProto_TYPE_INT64,
		},
		{
			name:      "uint32",
			fieldType: model.GRPCFieldTypeUint32,
			expected:  descriptorpb.FieldDescriptorProto_TYPE_UINT32,
		},
		{
			name:      "uint64",
			fieldType: model.GRPCFieldTypeUint64,
			expected:  descriptorpb.FieldDescriptorProto_TYPE_UINT64,
		},
		{
			name:      "float",
			fieldType: model.GRPCFieldTypeFloat,
			expected:  descriptorpb.FieldDescriptorProto_TYPE_FLOAT,
		},
		{
			name:      "double",
			fieldType: model.GRPCFieldTypeDouble,
			expected:  descriptorpb.FieldDescriptorProto_TYPE_DOUBLE,
		},
		{
			name:      "bytes",
			fieldType: model.GRPCFieldTypeBytes,
			expected:  descriptorpb.FieldDescriptorProto_TYPE_BYTES,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := protobufFieldType(
				tt.fieldType,
			)
			if err != nil {
				t.Fatalf(
					"protobufFieldType() error = %v",
					err,
				)
			}

			if actual != tt.expected {
				t.Errorf(
					"expected type %v, got %v",
					tt.expected,
					actual,
				)
			}
		})
	}
}

func TestProtobufFieldTypeUnsupported(t *testing.T) {
	_, err := protobufFieldType(
		model.GRPCFieldType("unsupported"),
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !nativeerrors.Is(
		err,
		errors.ErrUnsupportedGRPCFieldType,
	) {
		t.Errorf(
			"expected error %v, got %v",
			errors.ErrUnsupportedGRPCFieldType,
			err,
		)
	}
}

func TestFullMessageName(t *testing.T) {
	tests := []struct {
		name     string
		pkg      string
		message  string
		expected string
	}{
		{
			name:     "without package",
			message:  "GetUserRequest",
			expected: ".GetUserRequest",
		},
		{
			name:     "with package",
			pkg:      "users.v1",
			message:  "GetUserRequest",
			expected: ".users.v1.GetUserRequest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := fullMessageName(
				tt.pkg,
				tt.message,
			)

			if actual != tt.expected {
				t.Errorf(
					"expected %q, got %q",
					tt.expected,
					actual,
				)
			}
		})
	}
}

func TestInternalContractKey(t *testing.T) {
	base := &model.GRPCInternalProtoSpec{
		Package: "users.v1",
		Service: "UserService",
		Methods: []model.GRPCMethodSpec{
			{
				Name: "GetUser",
				Request: model.GRPCMessageSpec{
					Fields: []model.GRPCFieldSpec{
						{
							Name: "id",
							Type: model.GRPCFieldTypeString,
						},
					},
				},
				Response: model.GRPCMessageSpec{
					Fields: []model.GRPCFieldSpec{
						{
							Name: "name",
							Type: model.GRPCFieldTypeString,
						},
					},
				},
			},
		},
	}

	baseKey := internalContractKey(base)

	if actual := internalContractKey(base); actual != baseKey {
		t.Errorf(
			"expected stable key %q, got %q",
			baseKey,
			actual,
		)
	}

	tests := []struct {
		name   string
		mutate func(*model.GRPCInternalProtoSpec)
	}{
		{
			name: "package",
			mutate: func(spec *model.GRPCInternalProtoSpec) {
				spec.Package = "other.v1"
			},
		},
		{
			name: "service",
			mutate: func(spec *model.GRPCInternalProtoSpec) {
				spec.Service = "OtherService"
			},
		},
		{
			name: "method",
			mutate: func(spec *model.GRPCInternalProtoSpec) {
				spec.Methods[0].Name = "CreateUser"
			},
		},
		{
			name: "request field name",
			mutate: func(spec *model.GRPCInternalProtoSpec) {
				spec.Methods[0].
					Request.
					Fields[0].
					Name = "user_id"
			},
		},
		{
			name: "request field type",
			mutate: func(spec *model.GRPCInternalProtoSpec) {
				spec.Methods[0].
					Request.
					Fields[0].
					Type = model.GRPCFieldTypeBytes
			},
		},
		{
			name: "request repeated",
			mutate: func(spec *model.GRPCInternalProtoSpec) {
				spec.Methods[0].
					Request.
					Fields[0].
					Repeated = true
			},
		},
		{
			name: "response field",
			mutate: func(spec *model.GRPCInternalProtoSpec) {
				spec.Methods[0].
					Response.
					Fields[0].
					Name = "display_name"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := cloneInternalProtoSpec(base)
			tt.mutate(spec)

			actual := internalContractKey(spec)

			if actual == baseKey {
				t.Errorf(
					"expected contract key to change after changing %s",
					tt.name,
				)
			}
		})
	}
}

func cloneInternalProtoSpec(
	spec *model.GRPCInternalProtoSpec,
) *model.GRPCInternalProtoSpec {
	result := *spec

	result.Methods = make(
		[]model.GRPCMethodSpec,
		len(spec.Methods),
	)

	for i, method := range spec.Methods {
		result.Methods[i] = method

		result.Methods[i].Request.Fields = append(
			[]model.GRPCFieldSpec(nil),
			method.Request.Fields...,
		)

		result.Methods[i].Response.Fields = append(
			[]model.GRPCFieldSpec(nil),
			method.Response.Fields...,
		)
	}

	return &result
}

type testDescriptorLoader struct {
	descriptorSet *descriptorpb.FileDescriptorSet
	err           error

	calls  int
	module string
}

func (l *testDescriptorLoader) Load(
	_ context.Context,
	module string,
) (*descriptorpb.FileDescriptorSet, error) {
	l.calls++
	l.module = module

	return l.descriptorSet, l.err
}

func testDescriptorSet() *descriptorpb.FileDescriptorSet {
	return &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{
			{
				Name:    proto.String("users.proto"),
				Package: proto.String("users.v1"),
				Syntax:  proto.String("proto3"),
				MessageType: []*descriptorpb.DescriptorProto{
					{
						Name: proto.String(
							"GetUserRequest",
						),
					},
					{
						Name: proto.String(
							"GetUserResponse",
						),
					},
				},
				Service: []*descriptorpb.ServiceDescriptorProto{
					{
						Name: proto.String(
							"UserService",
						),
						Method: []*descriptorpb.MethodDescriptorProto{
							{
								Name: proto.String(
									"GetUser",
								),
								InputType: proto.String(
									".users.v1.GetUserRequest",
								),
								OutputType: proto.String(
									".users.v1.GetUserResponse",
								),
							},
						},
					},
				},
			},
		},
	}
}
