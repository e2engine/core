package keys

import "github.com/ygrebnov/keys"

var (
	file                  = keys.Factory(keys.WithSegments("file"))
	entity                = keys.Factory(keys.WithSegments("entity"))
	cursor                = keys.Factory(keys.WithSegments("cursor"))
	spec                  = keys.Factory(keys.WithSegments("spec"))
	environment           = keys.Factory(keys.WithSegments("environment"))
	environmentService    = keys.Factory(keys.WithSegments("environment", "service"))
	request               = keys.Factory(keys.WithSegments("request"))
	execution             = keys.Factory(keys.WithSegments("execution"))
	test                  = keys.Factory(keys.WithSegments("test"))
	testsuite             = keys.Factory(keys.WithSegments("testsuite"))
	http                  = keys.Factory(keys.WithSegments("http"))
	grpc                  = keys.Factory(keys.WithSegments("grpc"))
	e2engineTestExecution = keys.Factory(
		keys.WithSegments("e2engine", "test", "execution"),
		keys.WithSeparator('-'),
	)
	message = keys.Factory(keys.WithSegments("message"))
	socket  = keys.Factory(keys.WithSegments("socket"))
)

var (
	SpecKind                         = spec("kind")
	FilePath                         = file("path")
	FileFormat                       = file("format")
	EntityID                         = entity("id")
	EntityName                       = entity("name")
	EntityRef                        = entity("ref")
	EntityKind                       = entity("kind")
	EntityIDs                        = entity("ids")
	RequestKind                      = request("kind")
	CursorVersion                    = cursor("version")
	CursorTimestampValue             = cursor("timestamp.value")
	CursorOrderBy                    = cursor("orderBy")
	EnvironmentID                    = environment("id")
	EnvironmentServiceID             = environmentService("id")
	EnvironmentServiceKind           = environmentService("kind")
	EnvironmentServiceMode           = environmentService("mode")
	EnvironmentServiceListenAddress  = environmentService("listen.address")
	EnvironmentServiceRuntimeAddress = environmentService("runtime.address")

	RequestMethod  = request("method")
	RequestURLPath = request("url.path")

	HTTPStatusCode   = http("status.code")
	HTTPResponseBody = http("response.body")

	GRPCMethod               = grpc("method")
	GRPCStatus               = grpc("status")
	GRPCService              = grpc("service")
	GRPCFieldType            = grpc("field.type")
	GRPCServiceTargetAddress = grpc("service.target.address")

	FieldBody = keys.New("body")

	ExecutionID                  = execution("id")
	ExecutionStatus              = execution("status")
	TestID                       = test("id")
	TestTag                      = test("tag")
	TestSuiteSelector            = testsuite("selector")
	TestSuiteMaxTestsNumber      = testsuite("max.tests.number")
	TestSuiteResolvedTestsNumber = testsuite("resolved.tests.number")

	E2EngineTestExecutionID = e2engineTestExecution("id")

	MessageType         = message("type")
	SocketAddress       = socket("address")
	SocketReadDeadline  = socket("read.deadline")
	SocketWriteDeadline = socket("write.deadline")

	BufModule = keys.New("buf.module")

	Component  = keys.New("component")
	Cause      = keys.New("cause")
	Validation = keys.New("validation")
	Value      = keys.New("value")
	Operation  = keys.New("operation")

	E2EngineMockMissHeader   = keys.New("X-E2Engine-Mock-Miss")
	E2EngineMockMissMetadata = keys.New("x-e2engine-mock-miss")

	HeaderAccept              = keys.New("Accept")
	HeaderAuthorization       = keys.New("Authorization")
	MediaTypeOctetStream      = keys.New("application/octet-stream")
	AuthorizationSchemeBearer = keys.New("Bearer")

	OrderBy        = keys.New("OrderBy")
	OrderDirection = keys.New("OrderDirection")
)
