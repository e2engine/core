package http

import (
	"context"
	"encoding/json"
	nativeerrors "errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"

	"github.com/ygrebnov/errorc"

	"github.com/e2engine/core/model"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
	"github.com/e2engine/core/pkg/log"
)

type MockHTTPService struct {
	logger log.Logger

	spec     *model.ServiceSpec
	fixtures []Fixture

	server         *http.Server
	runtimeAddress string
}

func NewMockHTTPService(logger log.Logger, spec *model.ServiceSpec) *MockHTTPService {
	// The spec has already been validated higher up the stack, so we can assume it's valid here.
	return &MockHTTPService{
		logger: logger.With(
			log.String(keys.EnvironmentServiceID, spec.ID),
			log.String(keys.EnvironmentServiceKind, string(spec.Kind)),
			log.String(keys.EnvironmentServiceMode, string(spec.Mode)),
		),
		spec:     spec,
		fixtures: compileFixtures(spec.Fixtures),
	}
}

func (s *MockHTTPService) Mount(ctx context.Context) error {
	listenerConfig := net.ListenConfig{}
	listener, err := listenerConfig.Listen(
		ctx,
		"tcp",
		"127.0.0.1:0",
	)
	if err != nil {
		return err
	}

	s.runtimeAddress = listener.Addr().String()

	s.server = &http.Server{
		Handler: s,
	}

	go func() {
		err := s.server.Serve(listener)
		if err != nil && !nativeerrors.Is(err, http.ErrServerClosed) {
			s.logger.Error(
				"HTTP mock service stopped unexpectedly",
				log.Err(keys.Cause, err),
			)
		}
	}()

	s.logger.Debug(
		"Mounted service",
		log.String(
			keys.EnvironmentServiceRuntimeAddress,
			s.runtimeAddress,
		),
	)

	return nil
}

func (s *MockHTTPService) GetRuntimeAddress() string {
	return s.runtimeAddress
}

func (s *MockHTTPService) Unmount(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *MockHTTPService) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	fixture, err := s.match(r)
	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusBadRequest,
		)
		return
	}

	if fixture == nil {
		w.Header().Set(string(keys.E2EngineMockMissHeader), "true")

		s.logger.Debug(
			"No fixture matched request",
			log.String(keys.RequestMethod, r.Method),
			log.String(keys.RequestURLPath, r.URL.Path),
		)

		http.Error(
			w,
			errors.ErrNoFixtureMatched.Error(),
			http.StatusNotImplemented,
		)
		return
	}

	s.respond(w, fixture)
}

func (s *MockHTTPService) match(
	r *http.Request,
) (*Fixture, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Error(
			errors.ErrFailedToReadRequestBody.Error(),
			log.String(keys.RequestMethod, r.Method),
			log.String(keys.RequestURLPath, r.URL.Path),
			log.Err(keys.Cause, err),
		)

		return nil, errorc.With(
			errors.ErrFailedToReadRequestBody,
			errorc.String(keys.RequestMethod, r.Method),
			errorc.String(keys.RequestURLPath, r.URL.Path),
			errorc.Error(keys.Cause, err),
		)
	}

	for i := range s.fixtures {
		fixture := &s.fixtures[i]

		if fixture.method == r.Method &&
			fixture.path == r.URL.Path &&
			areQueryParamsEqual(
				fixture.query,
				r.URL.Query(),
			) &&
			areHeadersEqual(
				fixture.headers,
				r.Header,
			) &&
			isBodyEqual(
				fixture.body,
				fixture.bodyJSON,
				body,
			) {
			return fixture, nil
		}
	}

	return nil, nil
}

func isBodyEqual(
	expectedBody *string,
	expectedBodyJSON *string,
	actual []byte,
) bool {
	if expectedBody != nil {
		return *expectedBody == string(actual)
	}

	if expectedBodyJSON != nil {
		var expectedValue any
		if err := json.Unmarshal(
			[]byte(*expectedBodyJSON),
			&expectedValue,
		); err != nil {
			return false
		}

		var actualValue any
		if err := json.Unmarshal(
			actual,
			&actualValue,
		); err != nil {
			return false
		}

		return reflect.DeepEqual(
			expectedValue,
			actualValue,
		)
	}

	return true
}

func areHeadersEqual(expected, actual http.Header) bool {
	for key, expectedValues := range expected {
		actualValues := actual.Values(key)
		if len(expectedValues) != len(actualValues) {
			return false
		}
		expectedCounts := make(map[string]int, len(expectedValues))
		for _, value := range expectedValues {
			expectedCounts[value]++
		}
		for _, value := range actualValues {
			if expectedCounts[value] == 0 {
				return false
			}
			expectedCounts[value]--
		}
	}
	return true
}

func areQueryParamsEqual(
	expected,
	actual url.Values,
) bool {
	return reflect.DeepEqual(expected, actual)
}

func (s *MockHTTPService) respond(
	w http.ResponseWriter,
	fixture *Fixture,
) {
	for k, values := range fixture.responseHeaders {
		for _, v := range values {
			w.Header().Add(k, v)
		}
	}

	var body []byte

	if fixture.responseBody != "" {
		body = []byte(
			fixture.responseBody,
		)
	} else if fixture.responseBodyJSON != "" {
		body = []byte(
			fixture.responseBodyJSON,
		)
	}

	w.WriteHeader(fixture.status)

	if len(body) > 0 {
		if _, err := w.Write(body); err != nil {
			s.logger.Error(
				errors.ErrFailedToWriteResponseBody.Error(),
				log.String(
					keys.RequestMethod,
					fixture.method,
				),
				log.String(
					keys.RequestURLPath,
					fixture.path,
				),
				log.Err(keys.Cause, err),
			)

			return
		}
	}
}

type Fixture struct {
	method  string
	path    string
	query   url.Values
	headers map[string][]string

	body     *string
	bodyJSON *string

	status          int
	responseHeaders map[string][]string

	responseBody     string
	responseBodyJSON string
}

func compileFixtures(
	specs []model.FixtureSpec,
) []Fixture {
	fixtures := make(
		[]Fixture,
		len(specs),
	)

	for i, spec := range specs {
		// Path is validated by ServiceSpec.Validate.
		u, _ := url.ParseRequestURI(
			spec.When.HTTP.Path,
		)

		fixture := Fixture{
			method:  spec.When.HTTP.Method,
			path:    u.Path,
			query:   u.Query(),
			headers: spec.When.HTTP.Headers,

			status: spec.Then.HTTP.StatusCode,

			responseHeaders: spec.Then.HTTP.Headers,
		}

		if spec.When.HTTP.Body != nil {
			fixture.body = spec.When.HTTP.Body
		}

		if spec.When.HTTP.BodyJSON != nil {
			fixture.bodyJSON = spec.When.HTTP.BodyJSON
		}

		if spec.Then.HTTP.Body != nil {
			fixture.responseBody = *spec.Then.HTTP.Body
		}

		if spec.Then.HTTP.BodyJSON != nil {
			fixture.responseBodyJSON = *spec.Then.HTTP.BodyJSON
		}

		fixtures[i] = fixture
	}

	return fixtures
}
