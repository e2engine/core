package reporter

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/e2engine/core/model"
	"github.com/e2engine/core/model/matcher"
)

func TestReporterAddHTTPRequest(t *testing.T) {
	tests := []struct {
		name string

		dumpPayload bool

		expectedBody     string
		expectedBodyJSON string
	}{
		{
			name:             "payload disabled",
			expectedBody:     "<dump disabled>",
			expectedBodyJSON: "<dump disabled>",
		},
		{
			name:             "payload enabled",
			dumpPayload:      true,
			expectedBody:     "request body",
			expectedBodyJSON: `{"name":"test"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &model.TestSpec{
				Request: model.RequestSpec{
					HTTP: &model.HTTPRequest{
						Body: "request body",
						BodyJSON: `{
							"name": "test"
						}`,
					},
				},
			}

			r := newReporterForTest(
				&Config{
					DumpPayload: tt.dumpPayload,
				},
				spec,
			)

			u, err := url.Parse(
				"http://example.com/resource?a=1",
			)
			if err != nil {
				t.Fatal(err)
			}

			req := &http.Request{
				Method: http.MethodPost,
				URL:    u,
				Header: http.Header{
					"X-Test": {"one", "two"},
				},
			}

			r.AddHTTPRequest(req)

			if r.Result().Summary.Request == nil {
				t.Fatal("expected request summary")
			}

			actual := r.Result().Summary.Request.HTTP

			if actual == nil {
				t.Fatal("expected HTTP request summary")
			}

			if actual.Method != http.MethodPost {
				t.Errorf(
					"expected method %q, got %q",
					http.MethodPost,
					actual.Method,
				)
			}

			if actual.URL != u.String() {
				t.Errorf(
					"expected URL %q, got %q",
					u.String(),
					actual.URL,
				)
			}

			if !reflect.DeepEqual(
				actual.Headers,
				map[string][]string(req.Header),
			) {
				t.Errorf(
					"expected headers %+v, got %+v",
					req.Header,
					actual.Headers,
				)
			}

			if actual.Body != tt.expectedBody {
				t.Errorf(
					"expected body %q, got %q",
					tt.expectedBody,
					actual.Body,
				)
			}

			if actual.BodyJSON != tt.expectedBodyJSON {
				t.Errorf(
					"expected JSON body %q, got %q",
					tt.expectedBodyJSON,
					actual.BodyJSON,
				)
			}

			req.Header["X-Test"][0] = "changed"

			if actual.Headers["X-Test"][0] != "one" {
				t.Error("stored headers changed with source")
			}
		})
	}
}

func TestReporterAddHTTPRequestWithoutBody(t *testing.T) {
	spec := &model.TestSpec{
		Request: model.RequestSpec{
			HTTP: &model.HTTPRequest{},
		},
	}

	r := newReporterForTest(
		&Config{},
		spec,
	)

	req, err := http.NewRequest(
		http.MethodGet,
		"http://example.com/resource",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	r.AddHTTPRequest(req)

	actual := r.Result().Summary.Request.HTTP

	if actual.Body != "" || actual.BodyJSON != "" {
		t.Errorf(
			"expected omitted body fields, got body=%q body_json=%q",
			actual.Body,
			actual.BodyJSON,
		)
	}
}

func TestReporterAddHeadersDeviation(t *testing.T) {
	tests := []struct {
		name         string
		header       string
		cfg          Config
		wantExpected string
		wantActual   string
	}{
		{
			name:         "default sensitive header",
			header:       "Authorization",
			wantExpected: redacted,
			wantActual:   redacted,
		},
		{
			name:   "custom sensitive header",
			header: "X-Private",
			cfg: Config{
				RedactHeaders: []string{" x-private "},
			},
			wantExpected: redacted,
			wantActual:   redacted,
		},
		{
			name:         "ordinary header",
			header:       "X-Public",
			wantExpected: "expected-token",
			wantActual:   "actual-token",
		},
		{
			name:   "redaction explicitly disabled",
			header: "authorization",
			cfg: Config{
				SkipRedactingHeadersMetadata: true,
			},
			wantExpected: "expected-token",
			wantActual:   "actual-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newReporterForTest(
				&tt.cfg,
				&model.TestSpec{},
			)

			original := &model.TestExecutionDeviation{
				Field:    "headers." + tt.header,
				Expected: "expected-token",
				Actual:   "actual-token",
			}

			r.AddHeadersDeviation(tt.header, original)

			deviations := r.Result().Summary.Deviations
			if len(deviations) != 1 {
				t.Fatalf(
					"expected 1 deviation, got %d",
					len(deviations),
				)
			}

			got := deviations[0]

			if got.Field != original.Field ||
				got.Expected != tt.wantExpected ||
				got.Actual != tt.wantActual {
				t.Errorf(
					"unexpected deviation: %+v",
					got,
				)
			}

			// Redaction must not modify the original deviation.
			if original.Expected != "expected-token" ||
				original.Actual != "actual-token" {
				t.Errorf(
					"original deviation was modified: %+v",
					original,
				)
			}
		})
	}
}

func TestReporterAddHeadersDeviationNil(t *testing.T) {
	r := newReporterForTest(
		&Config{},
		&model.TestSpec{},
	)

	r.AddHeadersDeviation("Authorization", nil)

	if len(r.Result().Summary.Deviations) != 0 {
		t.Errorf(
			"expected no deviations, got %+v",
			r.Result().Summary.Deviations,
		)
	}
}

func TestReporterAddHTTPRequestQueryRedaction(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		wantQuery url.Values
	}{
		{
			name: "default sensitive query parameters",
			wantQuery: url.Values{
				"access_token": {redacted},
				"API_KEY":      {redacted, redacted},
				"page":         {"2"},
				"private":      {"custom-secret"},
			},
		},
		{
			name: "custom sensitive query parameter",
			cfg: Config{
				RedactQueryParams: []string{" PRIVATE "},
			},
			wantQuery: url.Values{
				"access_token": {redacted},
				"API_KEY":      {redacted, redacted},
				"page":         {"2"},
				"private":      {redacted},
			},
		},
		{
			name: "redaction explicitly disabled",
			cfg: Config{
				RedactQueryParams:        []string{"private"},
				SkipRedactingQueryParams: true,
			},
			wantQuery: url.Values{
				"access_token": {"access-secret"},
				"API_KEY":      {"key-one", "key-two"},
				"page":         {"2"},
				"private":      {"custom-secret"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newReporterForTest(
				&tt.cfg,
				&model.TestSpec{
					Request: model.RequestSpec{
						HTTP: &model.HTTPRequest{},
					},
				},
			)

			req, err := http.NewRequest(
				http.MethodGet,
				"http://example.com/resource?access_token=access-secret&API_KEY=key-one&API_KEY=key-two&page=2&private=custom-secret",
				nil,
			)
			if err != nil {
				t.Fatal(err)
			}

			originalURL := req.URL.String()

			r.AddHTTPRequest(req)

			// Reporting must not modify the outgoing request.
			if got := req.URL.String(); got != originalURL {
				t.Errorf(
					"request URL modified: got %q, want %q",
					got,
					originalURL,
				)
			}

			reportedURL, err := url.Parse(
				r.Result().Summary.Request.HTTP.URL,
			)
			if err != nil {
				t.Fatalf("invalid reported URL: %v", err)
			}

			// Redaction must preserve the URL destination and path.
			if reportedURL.Scheme != "http" ||
				reportedURL.Host != "example.com" ||
				reportedURL.Path != "/resource" {
				t.Errorf(
					"unexpected reported URL: %q",
					reportedURL.String(),
				)
			}

			if got := reportedURL.Query(); !reflect.DeepEqual(got, tt.wantQuery) {
				t.Errorf(
					"reported query: got %#v, want %#v",
					got,
					tt.wantQuery,
				)
			}
		})
	}
}

func TestReporterAddHTTPRequestURLWithoutQuery(t *testing.T) {
	r := newReporterForTest(
		&Config{},
		&model.TestSpec{
			Request: model.RequestSpec{
				HTTP: &model.HTTPRequest{},
			},
		},
	)

	req, err := http.NewRequest(
		http.MethodGet,
		"http://example.com/resource",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	r.AddHTTPRequest(req)

	if got := r.Result().Summary.Request.HTTP.URL; got != req.URL.String() {
		t.Errorf(
			"reported URL: got %q, want %q",
			got,
			req.URL.String(),
		)
	}
}

func TestReporterAddHTTPResponseStatus(t *testing.T) {
	r := newReporterForTest(
		&Config{},
		&model.TestSpec{},
	)

	r.AddHTTPResponseStatus(http.StatusCreated)

	response := r.Result().Summary.Response

	if response == nil || response.HTTP == nil {
		t.Fatal("expected HTTP response summary")
	}

	if response.HTTP.StatusCode != http.StatusCreated {
		t.Errorf(
			"expected status %d, got %d",
			http.StatusCreated,
			response.HTTP.StatusCode,
		)
	}
}

func TestReporterAddHTTPResponseHeaders(t *testing.T) {
	spec := &model.TestSpec{
		Expect: model.ExpectSpec{
			HTTP: &model.HTTPResponsePattern{
				Headers: map[string][]matcher.String{
					"X-Included": {
						{
							Operator: matcher.StringMatchOperatorEquals,
							Value:    "expected",
						},
					},
					"X-Missing": {
						{
							Operator: matcher.StringMatchOperatorExists,
						},
					},
				},
			},
		},
	}

	r := newReporterForTest(
		&Config{},
		spec,
	)

	headers := http.Header{
		"X-Included":  {"actual-1", "actual-2"},
		"X-Unrelated": {"secret"},
	}

	r.AddHTTPResponseHeaders(headers)

	actual := r.Result().Summary.Response.HTTP.Headers

	expected := map[string][]string{
		"X-Included": {"actual-1", "actual-2"},
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf(
			"expected headers %+v, got %+v",
			expected,
			actual,
		)
	}

	headers["X-Included"][0] = "changed"

	if actual["X-Included"][0] != "actual-1" {
		t.Error("stored headers changed with source")
	}
}

func TestReporterAddHTTPRequestHeaderRedaction(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want map[string][]string
	}{
		{
			name: "default sensitive headers",
			want: http.Header{
				"Authorization": {redacted},
				"authorization": {redacted},
				"Cookie":        {redacted},
				"X-Api-Key":     {redacted},
				"X-Test":        {"visible"},
			},
		},
		{
			name: "custom header",
			cfg: Config{
				RedactHeaders: []string{" X-Test "},
			},
			want: http.Header{
				"Authorization": {redacted},
				"authorization": {redacted},
				"Cookie":        {redacted},
				"X-Api-Key":     {redacted},
				"X-Test":        {redacted},
			},
		},
		{
			name: "redaction explicitly disabled",
			cfg: Config{
				SkipRedactingHeadersMetadata: true,
			},
			want: http.Header{
				"Authorization": {"Bearer secret"},
				"authorization": {"Bearer another-secret"},
				"Cookie":        {"sid=secret"},
				"X-Api-Key":     {"api-secret"},
				"X-Test":        {"visible"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &model.TestSpec{
				Request: model.RequestSpec{
					HTTP: &model.HTTPRequest{},
				},
			}

			r := newReporterForTest(
				&tt.cfg,
				spec,
			)

			req, err := http.NewRequest(
				http.MethodGet,
				"http://example.com/",
				nil,
			)
			if err != nil {
				t.Fatal(err)
			}

			req.Header = http.Header{
				"Authorization": {"Bearer secret"},
				"authorization": {"Bearer another-secret"},
				"Cookie":        {"sid=secret"},
				"X-Api-Key":     {"api-secret"},
				"X-Test":        {"visible"},
			}

			original := req.Header.Clone()

			r.AddHTTPRequest(req)

			got := r.Result().Summary.Request.HTTP.Headers

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf(
					"expected headers %#v, got %#v",
					tt.want,
					got,
				)
			}

			// Reporting must not modify the outgoing request.
			if !reflect.DeepEqual(req.Header, original) {
				t.Error("request headers modified by reporting")
			}

			// The report must own its copy of the header values.
			req.Header["X-Test"][0] = "changed"

			if got["X-Test"][0] != tt.want["X-Test"][0] {
				t.Error("reported headers changed with source")
			}
		})
	}
}

func TestReporterAddHTTPResponseHeadersRedaction(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want map[string][]string
	}{
		{
			name: "default and custom redaction",
			cfg: Config{
				RedactHeaders: []string{" x-private "},
			},
			want: map[string][]string{
				"set-cookie": {redacted},
				"x-private":  {redacted},
				"X-Public":   {"visible"},
			},
		},
		{
			name: "redaction explicitly disabled",
			cfg: Config{
				RedactHeaders:                []string{"x-private"},
				SkipRedactingHeadersMetadata: true,
			},
			want: map[string][]string{
				"set-cookie": {"sid=secret"},
				"x-private":  {"secret"},
				"X-Public":   {"visible"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists := matcher.String{
				Operator: matcher.StringMatchOperatorExists,
			}

			spec := &model.TestSpec{
				Expect: model.ExpectSpec{
					HTTP: &model.HTTPResponsePattern{
						Headers: map[string][]matcher.String{
							"set-cookie": {exists},
							"x-private":  {exists},
							"X-Public":   {exists},
						},
					},
				},
			}

			r := newReporterForTest(
				&tt.cfg,
				spec,
			)

			headers := http.Header{
				"Set-Cookie":   {"sid=secret"},
				"X-Private":    {"secret"},
				"X-Public":     {"visible"},
				"X-Unselected": {"do not include"},
			}

			original := headers.Clone()

			r.AddHTTPResponseHeaders(headers)

			got := r.Result().Summary.Response.HTTP.Headers

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf(
					"expected headers %#v, got %#v",
					tt.want,
					got,
				)
			}

			if !reflect.DeepEqual(headers, original) {
				t.Error("response headers modified by reporting")
			}
		})
	}
}

func TestSelectHeaders(t *testing.T) {
	equals := func(value string) matcher.String {
		return matcher.String{
			Operator: matcher.StringMatchOperatorEquals,
			Value:    value,
		}
	}

	exists := matcher.String{
		Operator: matcher.StringMatchOperatorExists,
	}

	nonempty := matcher.String{
		Operator: matcher.StringMatchOperatorNonEmpty,
	}

	tests := []struct {
		name string

		actual   http.Header
		expected map[string][]matcher.String

		want map[string][]string
	}{
		{
			name: "no expected headers",
			actual: http.Header{
				"X-Test": {"value"},
			},
		},
		{
			name: "expected header absent",
			actual: http.Header{
				"X-Test": {"value"},
			},
			expected: map[string][]matcher.String{
				"X-Missing": {equals("expected")},
			},
		},
		{
			name: "selected headers",
			actual: http.Header{
				"X-One":   {"1"},
				"X-Two":   {"2"},
				"X-Other": {"other"},
			},
			expected: map[string][]matcher.String{
				"X-One":     {equals("expected")},
				"X-Two":     {equals("expected")},
				"X-Missing": {equals("expected")},
			},
			want: map[string][]string{
				"X-One": {"1"},
				"X-Two": {"2"},
			},
		},
		{
			name: "exists matcher",
			actual: http.Header{
				"X-Test": {""},
			},
			expected: map[string][]matcher.String{
				"X-Test": {exists},
			},
			want: map[string][]string{
				"X-Test": {""},
			},
		},
		{
			name: "nonempty matcher",
			actual: http.Header{
				"X-Test": {"value"},
			},
			expected: map[string][]matcher.String{
				"X-Test": {nonempty},
			},
			want: map[string][]string{
				"X-Test": {"value"},
			},
		},
		{
			name: "case insensitive header lookup",
			actual: http.Header{
				"X-Request-Id": {"123"},
			},
			expected: map[string][]matcher.String{
				"x-request-id": {nonempty},
			},
			want: map[string][]string{
				"x-request-id": {"123"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := selectHeaders(
				tt.actual,
				tt.expected,
			)

			if !reflect.DeepEqual(actual, tt.want) {
				t.Errorf(
					"expected %+v, got %+v",
					tt.want,
					actual,
				)
			}
		})
	}
}

func TestReporterAddHTTPResponseBody(t *testing.T) {
	tests := []struct {
		name        string
		dumpPayload bool
		body        []byte
		want        string
	}{
		{
			name: "payload disabled",
			body: []byte("body"),
			want: "<dump disabled>",
		},
		{
			name:        "payload enabled",
			dumpPayload: true,
			body:        []byte("body"),
			want:        "body",
		},
		{
			name: "empty body with dumping disabled",
			body: []byte{},
		},
		{
			name:        "empty body with dumping enabled",
			dumpPayload: true,
			body:        []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newReporterForTest(
				&Config{
					DumpPayload: tt.dumpPayload,
				},
				&model.TestSpec{},
			)

			r.AddHTTPResponseBody(tt.body)

			response := r.Result().Summary.Response

			if response == nil || response.HTTP == nil {
				t.Fatal("expected HTTP response summary")
			}

			if response.HTTP.Body != tt.want {
				t.Errorf(
					"expected body %q, got %q",
					tt.want,
					response.HTTP.Body,
				)
			}
		})
	}
}

func TestReporterAddHTTPResponseBodyJSON(t *testing.T) {
	tests := []struct {
		name        string
		dumpPayload bool
		body        []byte
		want        string
	}{
		{
			name: "payload disabled",
			body: []byte(`{"name":"test"}`),
			want: "<dump disabled>",
		},
		{
			name:        "payload enabled",
			dumpPayload: true,
			body:        []byte(`{"name":"test"}`),
			want:        `{"name":"test"}`,
		},
		{
			name: "empty body with dumping disabled",
			body: []byte{},
		},
		{
			name:        "empty body with dumping enabled",
			dumpPayload: true,
			body:        []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newReporterForTest(
				&Config{
					DumpPayload: tt.dumpPayload,
				},
				&model.TestSpec{},
			)

			r.AddHTTPResponseBodyJSON(tt.body)

			response := r.Result().Summary.Response

			if response == nil || response.HTTP == nil {
				t.Fatal("expected HTTP response summary")
			}

			if response.HTTP.BodyJSON != tt.want {
				t.Errorf(
					"expected JSON body %q, got %q",
					tt.want,
					response.HTTP.BodyJSON,
				)
			}
		})
	}
}
