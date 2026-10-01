package errors

import (
	errorsPkg "errors"

	"github.com/ygrebnov/errorc"
)

var (
	ErrCannotEncodeCursor                   = errorc.New("cannot encode cursor")
	ErrCannotDecodeCursor                   = errorc.New("cannot decode cursor")
	ErrInvalidCursor                        = errorc.New("invalid cursor")
	ErrCannotComposeCursorPayload           = errorc.New("cannot compose cursor payload")
	ErrCannotConstructCursorPayloadBinding  = errorc.New("cannot construct cursor payload binding")
	ErrInvalidRequestParameters             = errorc.New("invalid request parameters")
	ErrInaccessiblePath                     = errorc.New("the specified path is inaccessible")
	ErrCannotParseSpec                      = errorc.New("cannot parse the spec")
	ErrCannotParseTestExecutionSummary      = errorc.New("cannot parse the test execution summary")
	ErrCannotParseTestSuiteExecutionSummary = errorc.New("cannot parse the test suite execution summary")
	ErrInvalidSpec                          = errorc.New("invalid spec")
	ErrCannotConvertSpecToJSON              = errorc.New("cannot convert the spec to JSON")
	ErrEntityNotFound                       = errorc.New("entity not found")
	ErrEntityPrefixAmbiguous                = errorc.New("entity prefix is ambiguous")
	ErrEntityAlreadyExists                  = errorc.New("entity already exists")
	ErrEntityConflict                       = errorc.New("entity conflict")
	ErrCannotInitializeLogger               = errorc.New("cannot initialize logger")
	ErrFailedToScheduleJob                  = errorc.New("failed to schedule job")
	ErrNilScheduler                         = errorc.New("scheduler is nil")
	ErrNilRuntimeConfig                     = errorc.New("runtime config is nil")
	ErrExceededTestsLimit                   = errorc.New("exceeded tests limit")

	ErrInvalidTestExecutionInput                  = errorc.New("invalid test execution input")
	ErrUnsupportedEnvironmentServiceConfiguration = errorc.New("unsupported environment service configuration")
	ErrUnsupportedTestProtocol                    = errorc.New("unsupported test protocol")
	ErrCannotMountEnvironmentService              = errorc.New("cannot mount environment service")
	ErrCannotUnmountEnvironmentService            = errorc.New("cannot unmount environment service")
	ErrFailedToMountRouter                        = errorc.New("failed to mount router")
	ErrFailedToCaptureEnvironmentServiceCall      = errorc.New("failed to capture environment service call")
	ErrFailedToReadRequestBody                    = errorc.New("failed to read request body")
	ErrNoFixtureMatched                           = errorc.New("no fixture matched")
	ErrFailedToMarshalResponseBody                = errorc.New("failed to marshal response body")
	ErrFailedToWriteResponseBody                  = errorc.New("failed to write response body")
	ErrFailedToCreateTestExecution                = errorc.New("failed to create test execution")
	ErrFailedToCreateTestJobs                     = errorc.New("failed to create test jobs")

	ErrCannotLoadGRPCDescriptor   = errorc.New("cannot load gRPC descriptor")
	ErrInvalidBufModule           = errorc.New("invalid buf module")
	ErrInvalidGRPCProtoContract   = errorc.New("invalid gPRC proto contract")
	ErrCannotLoadGRPCProto        = errorc.New("cannot load gRPC proto")
	ErrCannotResolveGRPCMethod    = errorc.New("cannot resolve gRPC method")
	ErrCannotBuildGRPCDescriptor  = errorc.New("cannot build gRPC descriptor")
	ErrUnsupportedGRPCFieldType   = errorc.New("unsupported gRPC field type")
	ErrCannotBuildGRPCMessage     = errorc.New("cannot build gRPC message")
	ErrUnsupportedGRPCMessageType = errorc.New("unsupported gRPC message type")
	ErrCannotBuildGRPCRequest     = errorc.New("cannot build gRPC request")
	ErrGRPCTargetServiceNotFound  = errorc.New("gRPC target service not found")

	ErrNoTestsResolved = errorc.New("no tests resolved for test suite")

	ErrUnsupportedFileType = errorc.New("unsupported file type")
)

// Is wraps native errors package Is.
func Is(err, target error) bool {
	return errorsPkg.Is(err, target)
}
