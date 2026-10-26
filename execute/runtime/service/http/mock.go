package http

import (
	"context"
	"encoding/json"
	nativeerrors "errors"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/ygrebnov/errorc"

	"github.com/e2engine/core/execute/match"
	"github.com/e2engine/core/internal/util"
	"github.com/e2engine/core/model"
	"github.com/e2engine/core/model/matcher"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
	"github.com/e2engine/core/pkg/log"
)

type MockHTTPService struct {
	cfg    *MockHTTPServiceConfig
	logger log.Logger

	spec     *model.ServiceSpec
	fixtures []Fixture

	server         *http.Server
	runtimeAddress string
}

func NewMockHTTPService(
	cfg *MockHTTPServiceConfig,
	logger log.Logger,
	spec *model.ServiceSpec,
) *MockHTTPService {
	// The spec has already been validated higher up the stack, so we can assume it's valid here.
	return &MockHTTPService{
		cfg: cfg,
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
		Handler:           s,
		ReadHeaderTimeout: s.cfg.ReadHeaderTimeout,
		IdleTimeout:       s.cfg.IdleTimeout,
		MaxHeaderBytes:    s.cfg.MaxRequestHeaderBytes,
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
	if s.server == nil {
		return nil
	}

	err := s.server.Shutdown(ctx)
	if err != nil {
		return nativeerrors.Join(
			err,
			s.server.Close(),
		)
	}

	return nil
}

func (s *MockHTTPService) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	fixture, err := s.match(r)
	if err != nil {
		if nativeerrors.Is(err, errors.ErrHTTPRequestBodyTooLarge) {
			http.Error(
				w,
				http.StatusText(http.StatusRequestEntityTooLarge),
				http.StatusRequestEntityTooLarge,
			)
			return
		}
		http.Error(
			w,
			http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest,
		)
		return
	}

	if fixture == nil {
		w.Header().Set(string(keys.E2EngineMockMissHeader), "true")

		s.logger.Debug(
			"No fixture matched request",
			log.String(keys.RequestMethod, r.Method),
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
	body, err := io.ReadAll(
		io.LimitReader(r.Body, s.cfg.MaxRequestBodySize+1),
	)
	if err == nil && int64(len(body)) > s.cfg.MaxRequestBodySize {
		return nil, errors.ErrHTTPRequestBodyTooLarge
	}
	if err != nil {
		s.logger.Error(
			errors.ErrFailedToReadRequestBody.Error(),
			log.String(keys.RequestMethod, r.Method),
			log.Err(keys.Cause, err),
		)

		return nil, errorc.With(
			errors.ErrFailedToReadRequestBody,
			errorc.String(keys.RequestMethod, r.Method),
			errorc.String(keys.RequestURLPath, r.URL.Path),
			errorc.Error(keys.Cause, err),
		)
	}

	query := r.URL.Query()

	for i := range s.fixtures {
		fixture := &s.fixtures[i]

		if fixture.method == r.Method &&
			fixture.path == r.URL.Path &&
			areQueryParamsEqual(fixture.query, query) &&
			areHeadersEqual(fixture.headers, r.Header) {
			equalBody, errBodyEqual := isBodyEqual(
				fixture.body,
				fixture.bodyJSON,
				body,
			)
			if errBodyEqual != nil {
				return nil, errBodyEqual
			}
			if equalBody {
				return fixture, nil
			}
		}
	}

	return nil, nil
}

func isBodyEqual(
	expectedBody *matcher.String,
	expectedBodyJSON *matcher.String,
	actual []byte,
) (bool, error) {
	if expectedBody != nil {
		return match.IsStringMatched(actual, expectedBody), nil
	}

	if expectedBodyJSON != nil {
		if !json.Valid(actual) {
			return false, nil
		}

		return match.IsJSONMatched(actual, expectedBodyJSON)
	}

	return true, nil
}

func areHeadersEqual(
	expected map[string][]matcher.String,
	actual http.Header,
) bool {
	for key, expectedValues := range expected {
		canonicalKey := http.CanonicalHeaderKey(key)
		actualValues, exists := actual[canonicalKey]

		if !match.IsStringSliceMatched(
			actualValues,
			exists,
			expectedValues,
		) {
			return false
		}
	}

	return true
}

func areQueryParamsEqual(
	expected,
	actual url.Values,
) bool {
	for key, expectedValues := range expected {
		actualValues, exists := actual[key]
		if !exists || !util.IsStringSlicesEqual(actualValues, expectedValues) {
			return false
		}
	}

	return true
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
				log.String(keys.RequestMethod, fixture.method),
			)

			return
		}
	}
}

type Fixture struct {
	method  string
	path    string
	query   url.Values
	headers map[string][]matcher.String

	body     *matcher.String
	bodyJSON *matcher.String

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

			responseHeaders:  spec.Then.HTTP.Headers,
			responseBody:     spec.Then.HTTP.Body,
			responseBodyJSON: spec.Then.HTTP.BodyJSON,
		}

		if spec.When.HTTP.Body != nil {
			fixture.body = spec.When.HTTP.Body
		}

		if spec.When.HTTP.BodyJSON != nil {
			fixture.bodyJSON = spec.When.HTTP.BodyJSON
		}

		fixtures[i] = fixture
	}

	return fixtures
}
