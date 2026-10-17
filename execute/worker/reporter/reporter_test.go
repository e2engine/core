package reporter

import (
	"errors"
	"testing"

	"github.com/e2engine/core/model"
	"github.com/e2engine/core/model/matcher"
)

func TestNew(t *testing.T) {
	r := newReporterForTest(
		&Config{},
		&model.TestSpec{},
	)

	result := r.Result()

	if result.ExecutionID != "execution-1" {
		t.Errorf(
			"expected execution ID %q, got %q",
			"execution-1",
			result.ExecutionID,
		)
	}

	if result.TestSuiteExecutionID != "suite-execution-1" {
		t.Errorf(
			"expected test suite execution ID %q, got %q",
			"suite-execution-1",
			result.TestSuiteExecutionID,
		)
	}

	if result.Summary == nil {
		t.Fatal("expected summary")
	}

	if !result.FinishedAt.IsZero() {
		t.Errorf(
			"expected zero finished time, got %v",
			result.FinishedAt,
		)
	}
}

func TestReporterSetError(t *testing.T) {
	r := newReporterForTest(
		&Config{},
		&model.TestSpec{},
	)

	expectedErr := errors.New("execution failed")

	r.SetError(expectedErr)

	result := r.Result()

	if result.Status != model.ExecutionStatusError {
		t.Errorf(
			"expected status %q, got %q",
			model.ExecutionStatusError,
			result.Status,
		)
	}

	if result.Summary.Error != expectedErr.Error() {
		t.Errorf(
			"expected error %q, got %q",
			expectedErr.Error(),
			result.Summary.Error,
		)
	}

	if result.FinishedAt.IsZero() {
		t.Error("expected non-zero finished time")
	}

	mustPanic(t, func() {
		r.AddDeviation(&model.TestExecutionDeviation{})
	})

	mustPanic(t, func() {
		r.AddPayloadDeviation(&model.PayloadDeviation{})
	})
}

func TestReporterFinalize(t *testing.T) {
	tests := []struct {
		name         string
		addDeviation bool
		expected     model.ExecutionStatus
	}{
		{
			name:     "passed",
			expected: model.ExecutionStatusPassed,
		},
		{
			name:         "failed",
			addDeviation: true,
			expected:     model.ExecutionStatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newReporterForTest(
				&Config{},
				&model.TestSpec{},
			)

			if tt.addDeviation {
				r.AddDeviation(
					&model.TestExecutionDeviation{
						Field: "status",
					},
				)
			}

			r.Finalize()

			result := r.Result()

			if result.Status != tt.expected {
				t.Errorf(
					"expected status %q, got %q",
					tt.expected,
					result.Status,
				)
			}

			if result.FinishedAt.IsZero() {
				t.Error("expected non-zero finished time")
			}

			mustPanic(t, func() {
				r.Finalize()
			})
		})
	}
}

func TestReporterAddDeviation(t *testing.T) {
	r := newReporterForTest(
		&Config{},
		&model.TestSpec{},
	)

	r.AddDeviation(nil)

	if len(r.Result().Summary.Deviations) != 0 {
		t.Fatal("expected nil deviation to be ignored")
	}

	deviation := &model.TestExecutionDeviation{
		Field:    "body",
		Expected: "expected",
		Actual:   "actual",
		Message:  "message",
	}

	r.AddDeviation(deviation)

	if len(r.Result().Summary.Deviations) != 1 {
		t.Fatalf(
			"expected 1 deviation, got %d",
			len(r.Result().Summary.Deviations),
		)
	}

	actual := r.Result().Summary.Deviations[0]

	if actual != *deviation {
		t.Errorf(
			"expected deviation %+v, got %+v",
			*deviation,
			actual,
		)
	}

	// Reporter stores the deviation by value.
	deviation.Field = "changed"

	if r.Result().Summary.Deviations[0].Field != "body" {
		t.Error("stored deviation changed with source")
	}
}

func TestReporterAddPayloadDeviation(t *testing.T) {
	tests := []struct {
		name string

		dumpPayload bool
		expected    *matcher.String
		actual      []byte

		wantExpected string
		wantActual   string
	}{
		{
			name: "payload disabled",
			expected: &matcher.String{
				Operator: matcher.StringMatchOperatorEquals,
				Value:    "expected payload",
			},
			actual:       []byte("actual payload"),
			wantExpected: "<dump disabled>",
			wantActual:   "<dump disabled>",
		},
		{
			name:        "payload enabled",
			dumpPayload: true,
			expected: &matcher.String{
				Operator: matcher.StringMatchOperatorEquals,
				Value:    "expected payload",
			},
			actual:       []byte("actual payload"),
			wantExpected: "expected payload",
			wantActual:   "actual payload",
		},
		{
			name: "nonempty matcher with dumping disabled",
			expected: &matcher.String{
				Operator: matcher.StringMatchOperatorNonEmpty,
			},
			actual:       []byte("actual payload"),
			wantExpected: "<dump disabled>",
			wantActual:   "<dump disabled>",
		},
		{
			name:        "nonempty matcher with dumping enabled",
			dumpPayload: true,
			expected: &matcher.String{
				Operator: matcher.StringMatchOperatorNonEmpty,
			},
			actual:       []byte("actual payload"),
			wantExpected: "nonempty",
			wantActual:   "actual payload",
		},
		{
			name: "empty actual payload",
			expected: &matcher.String{
				Operator: matcher.StringMatchOperatorNonEmpty,
			},
			wantExpected: "<dump disabled>",
		},
		{
			name:        "empty actual payload with dumping enabled",
			dumpPayload: true,
			expected: &matcher.String{
				Operator: matcher.StringMatchOperatorNonEmpty,
			},
			wantExpected: "nonempty",
		},
		{
			name:         "nil expected matcher",
			actual:       []byte("actual payload"),
			wantExpected: "<dump disabled>",
			wantActual:   "<dump disabled>",
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

			r.AddPayloadDeviation(nil)

			r.AddPayloadDeviation(&model.PayloadDeviation{
				Field:    "body",
				Expected: tt.expected,
				Actual:   tt.actual,
				Message:  "payload differs",
			})

			deviations := r.Result().Summary.Deviations

			if len(deviations) != 1 {
				t.Fatalf(
					"expected 1 deviation, got %d",
					len(deviations),
				)
			}

			actual := deviations[0]

			if actual.Field != "body" {
				t.Errorf(
					"expected field %q, got %q",
					"body",
					actual.Field,
				)
			}

			if actual.Message != "payload differs" {
				t.Errorf(
					"expected message %q, got %q",
					"payload differs",
					actual.Message,
				)
			}

			if actual.Expected != tt.wantExpected {
				t.Errorf(
					"expected Expected %q, got %q",
					tt.wantExpected,
					actual.Expected,
				)
			}

			if actual.Actual != tt.wantActual {
				t.Errorf(
					"expected Actual %q, got %q",
					tt.wantActual,
					actual.Actual,
				)
			}
		})
	}
}

func newReporterForTest(
	cfg *Config,
	spec *model.TestSpec,
) *Reporter {
	return New(
		cfg,
		spec,
		"execution-1",
		"suite-execution-1",
	)
}

func mustPanic(
	t *testing.T,
	fn func(),
) {
	t.Helper()

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()

	fn()
}
