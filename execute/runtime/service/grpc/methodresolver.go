package grpc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"sync"

	"github.com/bufbuild/protocompile"
	"github.com/ygrebnov/errorc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
	"github.com/e2engine/core/util"
)

type DescriptorLoader interface {
	Load(
		ctx context.Context,
		bufModule string,
	) (*descriptorpb.FileDescriptorSet, error)
}

type ProtoMethodResolver struct {
	loader DescriptorLoader

	mu      sync.RWMutex
	modules map[string]*protoregistry.Files
	files   map[string]*protoregistry.Files
	inline  map[string]*protoregistry.Files
}

func NewProtoMethodResolver(
	loader DescriptorLoader,
) *ProtoMethodResolver {
	return &ProtoMethodResolver{
		loader:  loader,
		modules: make(map[string]*protoregistry.Files),
		files:   make(map[string]*protoregistry.Files),
		inline:  make(map[string]*protoregistry.Files),
	}
}

func (r *ProtoMethodResolver) ResolveMethod(
	ctx context.Context,
	spec *model.GRPCProtoSpec,
	method string,
) (protoreflect.MethodDescriptor, error) {
	switch {
	case spec.External != nil:
		return r.resolveExternal(
			ctx,
			spec.External,
			method,
		)

	case spec.Internal != nil:
		return r.resolveInternal(
			spec.Internal,
			method,
		)

	default:
		return nil, errorc.With(
			errors.ErrInvalidGRPCProtoContract,
			errorc.String(keys.Operation, "resolve method"),
			errorc.String(keys.GRPCMethod, method),
		)
	}
}

func (r *ProtoMethodResolver) resolveExternal(
	ctx context.Context,
	spec *model.GRPCExternalProtoSpec,
	method string,
) (protoreflect.MethodDescriptor, error) {
	var (
		files *protoregistry.Files
		err   error
	)

	switch {
	case spec.File != "":
		files, err = r.loadFile(
			ctx,
			spec.File,
		)

	case spec.BufModule != "":
		files, err = r.loadModule(
			ctx,
			spec.BufModule,
		)

	default:
		return nil, errorc.With(
			errors.ErrInvalidGRPCProtoContract,
			errorc.String(keys.Operation, "resolve external proto"),
			errorc.String(keys.GRPCMethod, method),
		)
	}

	if err != nil {
		return nil, err
	}

	return findMethod(
		files,
		spec.Service,
		method,
	)
}

func (r *ProtoMethodResolver) loadFile(
	ctx context.Context,
	path string,
) (*protoregistry.Files, error) {
	if err := util.ValidateFilePath(path); err != nil {
		return nil, err
	}

	r.mu.RLock()
	files := r.files[path]
	r.mu.RUnlock()

	if files != nil {
		return files, nil
	}

	// TODO: use a controlled resolver that opens and validates protobuf files
	// through the shared file-loading abstraction, including imported files.
	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(
			&protocompile.SourceResolver{},
		),
	}

	compiled, err := compiler.Compile(
		ctx,
		path,
	)
	if err != nil {
		return nil, errorc.With(
			errors.ErrCannotLoadGRPCProto,
			errorc.String(keys.FilePath, path),
			errorc.String(keys.Operation, "compile protobuf file"),
			errorc.Error(keys.Cause, err),
		)
	}

	files = new(protoregistry.Files)

	for _, file := range compiled {
		if err := files.RegisterFile(file); err != nil {
			return nil, errorc.With(
				errors.ErrCannotLoadGRPCProto,
				errorc.String(keys.FilePath, path),
				errorc.String(
					keys.Operation,
					"register protobuf file",
				),
				errorc.Error(keys.Cause, err),
			)
		}
	}

	r.mu.Lock()

	if existing := r.files[path]; existing != nil {
		files = existing
	} else {
		r.files[path] = files
	}

	r.mu.Unlock()

	return files, nil
}

func (r *ProtoMethodResolver) resolveInternal(
	spec *model.GRPCInternalProtoSpec,
	method string,
) (protoreflect.MethodDescriptor, error) {
	files, err := r.loadInternal(spec)
	if err != nil {
		return nil, err
	}

	service := spec.Service
	if spec.Package != "" {
		service = spec.Package + "." + spec.Service
	}

	return findMethod(
		files,
		service,
		method,
	)
}

func findMethod(
	files *protoregistry.Files,
	service string,
	method string,
) (protoreflect.MethodDescriptor, error) {
	descriptor, err := files.FindDescriptorByName(
		protoreflect.FullName(service),
	)
	if err != nil {
		return nil, errorc.With(
			errors.ErrCannotResolveGRPCMethod,
			errorc.String(keys.GRPCService, service),
			errorc.String(keys.GRPCMethod, method),
			errorc.Error(keys.Cause, err),
		)
	}

	serviceDescriptor, ok :=
		descriptor.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, errorc.With(
			errors.ErrCannotResolveGRPCMethod,
			errorc.String(keys.GRPCService, service),
			errorc.String(keys.GRPCMethod, method),
			errorc.String(
				keys.Operation,
				"resolve service descriptor",
			),
		)
	}

	methodDescriptor := serviceDescriptor.Methods().ByName(
		protoreflect.Name(method),
	)
	if methodDescriptor == nil {
		return nil, errorc.With(
			errors.ErrCannotResolveGRPCMethod,
			errorc.String(keys.GRPCService, service),
			errorc.String(keys.GRPCMethod, method),
		)
	}

	return methodDescriptor, nil
}

func (r *ProtoMethodResolver) loadModule(
	ctx context.Context,
	bufModule string,
) (*protoregistry.Files, error) {
	// TODO: review concurrent cache behavior
	r.mu.RLock()
	files := r.modules[bufModule]
	r.mu.RUnlock()

	if files != nil {
		return files, nil
	}

	descriptorSet, err := r.loader.Load(
		ctx,
		bufModule,
	)
	if err != nil {
		return nil, errorc.With(
			errors.ErrCannotLoadGRPCProto,
			errorc.String(keys.BufModule, bufModule),
			errorc.String(keys.Operation, "load protobuf module"),
			errorc.Error(keys.Cause, err),
		)
	}

	files, err = protodesc.NewFiles(descriptorSet)
	if err != nil {
		return nil, errorc.With(
			errors.ErrCannotLoadGRPCProto,
			errorc.String(keys.BufModule, bufModule),
			errorc.String(
				keys.Operation,
				"build protobuf descriptor registry",
			),
			errorc.Error(keys.Cause, err),
		)
	}

	r.mu.Lock()

	if existing := r.modules[bufModule]; existing != nil {
		files = existing
	} else {
		r.modules[bufModule] = files
	}

	r.mu.Unlock()

	return files, nil
}

func (r *ProtoMethodResolver) loadInternal(
	spec *model.GRPCInternalProtoSpec,
) (*protoregistry.Files, error) {
	key := internalContractKey(spec)

	r.mu.RLock()
	files := r.inline[key]
	r.mu.RUnlock()

	if files != nil {
		return files, nil
	}

	descriptorSet, err :=
		buildInternalDescriptorSet(spec)
	if err != nil {
		return nil, err
	}

	files, err = protodesc.NewFiles(descriptorSet)
	if err != nil {
		return nil, errorc.With(
			errors.ErrCannotLoadGRPCProto,
			errorc.String(
				keys.GRPCService,
				spec.Service,
			),
			errorc.String(
				keys.Operation,
				"build internal protobuf descriptor registry",
			),
			errorc.Error(keys.Cause, err),
		)
	}

	r.mu.Lock()

	if existing := r.inline[key]; existing != nil {
		files = existing
	} else {
		r.inline[key] = files
	}

	r.mu.Unlock()

	return files, nil
}

func buildInternalDescriptorSet(
	spec *model.GRPCInternalProtoSpec,
) (*descriptorpb.FileDescriptorSet, error) {
	fileName := spec.Service + ".proto"

	file := &descriptorpb.FileDescriptorProto{
		Name:    proto.String(fileName),
		Package: proto.String(spec.Package),
		Syntax:  proto.String("proto3"),
	}

	service := &descriptorpb.ServiceDescriptorProto{
		Name: proto.String(spec.Service),
	}

	for _, method := range spec.Methods {
		requestName := method.Name + "Request"
		responseName := method.Name + "Response"

		request, err := buildMessageDescriptor(
			requestName,
			method.Request,
		)
		if err != nil {
			return nil, errorc.With(
				errors.ErrCannotBuildGRPCDescriptor,
				errorc.String(keys.GRPCMethod, method.Name),
				errorc.String(
					keys.Operation,
					"build request descriptor",
				),
				errorc.Error(keys.Cause, err),
			)
		}

		response, err := buildMessageDescriptor(
			responseName,
			method.Response,
		)
		if err != nil {
			return nil, errorc.With(
				errors.ErrCannotBuildGRPCDescriptor,
				errorc.String(keys.GRPCMethod, method.Name),
				errorc.String(
					keys.Operation,
					"build response descriptor",
				),
				errorc.Error(keys.Cause, err),
			)
		}

		file.MessageType = append(
			file.MessageType,
			request,
			response,
		)

		service.Method = append(
			service.Method,
			&descriptorpb.MethodDescriptorProto{
				Name: proto.String(method.Name),
				InputType: proto.String(
					fullMessageName(
						spec.Package,
						requestName,
					),
				),
				OutputType: proto.String(
					fullMessageName(
						spec.Package,
						responseName,
					),
				),
			},
		)
	}

	file.Service = append(
		file.Service,
		service,
	)

	return &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{
			file,
		},
	}, nil
}

func buildMessageDescriptor(
	name string,
	spec model.GRPCMessageSpec,
) (*descriptorpb.DescriptorProto, error) {
	message := &descriptorpb.DescriptorProto{
		Name: proto.String(name),
	}

	for i, field := range spec.Fields {
		fieldType, err :=
			protobufFieldType(field.Type)
		if err != nil {
			return nil, err
		}

		label :=
			descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL

		if field.Repeated {
			label =
				descriptorpb.FieldDescriptorProto_LABEL_REPEATED
		}

		message.Field = append(
			message.Field,
			&descriptorpb.FieldDescriptorProto{
				Name:   proto.String(field.Name),
				Number: proto.Int32(int32(i + 1)),
				Label:  label.Enum(),
				Type:   fieldType.Enum(),
			},
		)
	}

	return message, nil
}

func protobufFieldType(
	fieldType model.GRPCFieldType,
) (descriptorpb.FieldDescriptorProto_Type, error) {
	switch fieldType {
	case model.GRPCFieldTypeString:
		return descriptorpb.FieldDescriptorProto_TYPE_STRING, nil

	case model.GRPCFieldTypeBool:
		return descriptorpb.FieldDescriptorProto_TYPE_BOOL, nil

	case model.GRPCFieldTypeInt32:
		return descriptorpb.FieldDescriptorProto_TYPE_INT32, nil

	case model.GRPCFieldTypeInt64:
		return descriptorpb.FieldDescriptorProto_TYPE_INT64, nil

	case model.GRPCFieldTypeUint32:
		return descriptorpb.FieldDescriptorProto_TYPE_UINT32, nil

	case model.GRPCFieldTypeUint64:
		return descriptorpb.FieldDescriptorProto_TYPE_UINT64, nil

	case model.GRPCFieldTypeFloat:
		return descriptorpb.FieldDescriptorProto_TYPE_FLOAT, nil

	case model.GRPCFieldTypeDouble:
		return descriptorpb.FieldDescriptorProto_TYPE_DOUBLE, nil

	case model.GRPCFieldTypeBytes:
		return descriptorpb.FieldDescriptorProto_TYPE_BYTES, nil

	default:
		return 0, errorc.With(
			errors.ErrUnsupportedGRPCFieldType,
			errorc.String(
				keys.GRPCFieldType,
				string(fieldType),
			),
		)
	}
}

func fullMessageName(
	pkg string,
	message string,
) string {
	if pkg == "" {
		return "." + message
	}

	return "." + pkg + "." + message
}

func internalContractKey(
	spec *model.GRPCInternalProtoSpec,
) string {
	h := sha256.New()

	writeHashString(h, spec.Package)
	writeHashString(h, spec.Service)

	for _, method := range spec.Methods {
		writeHashString(h, method.Name)
		writeMessageHash(h, method.Request)
		writeMessageHash(h, method.Response)
	}

	return hex.EncodeToString(h.Sum(nil))
}

func writeMessageHash(
	h hash.Hash,
	spec model.GRPCMessageSpec,
) {
	for _, field := range spec.Fields {
		writeHashString(h, field.Name)
		writeHashString(h, string(field.Type))

		if field.Repeated {
			writeHashString(h, "repeated")
		} else {
			writeHashString(h, "optional")
		}
	}
}

func writeHashString(
	h hash.Hash,
	value string,
) {
	h.Write([]byte(value))
	h.Write([]byte{0})
}
