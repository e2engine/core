package call

import (
	nativeerrors "errors"
	"sync"
	"testing"

	"github.com/e2engine/core/pkg/errors"
)

func TestStoreGet(t *testing.T) {
	store := NewStoreFactory(&Config{
		MaxCalls: 3,
	}).New()

	call1 := Call{
		TestExecutionID: "execution-1",
		ServiceID:       "service-1",
	}

	call2 := Call{
		TestExecutionID: "execution-2",
		ServiceID:       "service-1",
	}

	call3 := Call{
		TestExecutionID: "execution-1",
		ServiceID:       "service-2",
	}

	for _, recorded := range []Call{call1, call2, call3} {
		if err := store.Record(recorded); err != nil {
			t.Fatalf("Record() error = %v", err)
		}
	}

	tests := []struct {
		name     string
		filter   Filter
		expected []Call
	}{
		{
			name:     "all",
			filter:   Filter{},
			expected: []Call{call1, call2, call3},
		},
		{
			name: "test execution",
			filter: Filter{
				TestExecutionID: "execution-1",
			},
			expected: []Call{call1, call3},
		},
		{
			name: "service",
			filter: Filter{
				ServiceID: "service-1",
			},
			expected: []Call{call1, call2},
		},
		{
			name: "test execution and service",
			filter: Filter{
				TestExecutionID: "execution-1",
				ServiceID:       "service-2",
			},
			expected: []Call{call3},
		},
		{
			name: "no match",
			filter: Filter{
				TestExecutionID: "execution-3",
			},
			expected: []Call{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := store.Get(tt.filter)

			if len(actual) != len(tt.expected) {
				t.Fatalf(
					"expected %d calls, got %d",
					len(tt.expected),
					len(actual),
				)
			}

			for i := range tt.expected {
				if actual[i].TestExecutionID !=
					tt.expected[i].TestExecutionID {
					t.Fatalf(
						"expected test execution ID %q, got %q",
						tt.expected[i].TestExecutionID,
						actual[i].TestExecutionID,
					)
				}

				if actual[i].ServiceID !=
					tt.expected[i].ServiceID {
					t.Fatalf(
						"expected service ID %q, got %q",
						tt.expected[i].ServiceID,
						actual[i].ServiceID,
					)
				}
			}
		})
	}
}

func TestStoreFactoryNew(t *testing.T) {
	f := NewStoreFactory(&Config{
		MaxCalls: 1,
	})

	store1 := f.New()
	store2 := f.New()

	if store1 == nil {
		t.Fatal("expected first store")
	}

	if store2 == nil {
		t.Fatal("expected second store")
	}

	if store1 == store2 {
		t.Fatal("expected independent stores")
	}

	if err := store1.Record(Call{
		TestExecutionID: "execution-1",
	}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	if len(store1.Get(Filter{})) != 1 {
		t.Fatal("expected call in first store")
	}

	if len(store2.Get(Filter{})) != 0 {
		t.Fatal("expected second store to be empty")
	}
}

func TestStoreRecordLimit(t *testing.T) {
	store := NewStoreFactory(&Config{
		MaxCalls: 2,
	}).New()

	for i := range 2 {
		if err := store.Record(Call{
			TestExecutionID: "execution-1",
		}); err != nil {
			t.Fatalf(
				"Record(%d) error = %v",
				i,
				err,
			)
		}
	}

	// Reaching the limit is valid.
	if err := store.Err(); err != nil {
		t.Fatalf(
			"Err() = %v, want nil at limit",
			err,
		)
	}

	// The next record exceeds the limit.
	err := store.Record(Call{
		TestExecutionID: "execution-2",
	})

	if !nativeerrors.Is(
		err,
		errors.ErrCallStoreLimitExceeded,
	) {
		t.Fatalf(
			"Record() error = %v, want ErrCallStoreLimitExceeded",
			err,
		)
	}

	// The store retains the failure.
	if err := store.Err(); !nativeerrors.Is(
		err,
		errors.ErrCallStoreLimitExceeded,
	) {
		t.Fatalf(
			"Err() = %v, want ErrCallStoreLimitExceeded",
			err,
		)
	}

	// Only the first two calls were recorded.
	if got := store.Get(Filter{}); len(got) != 2 {
		t.Fatalf(
			"Get() returned %d calls, want 2",
			len(got),
		)
	}

	// Subsequent recording attempts must also fail.
	if err := store.Record(Call{}); !nativeerrors.Is(
		err,
		errors.ErrCallStoreLimitExceeded,
	) {
		t.Fatalf(
			"subsequent Record() error = %v, want ErrCallStoreLimitExceeded",
			err,
		)
	}

	if got := store.Get(Filter{}); len(got) != 2 {
		t.Fatalf(
			"Get() returned %d calls after subsequent failure, want 2",
			len(got),
		)
	}
}

func TestStoreFactoryIndependentLimits(t *testing.T) {
	factory := NewStoreFactory(&Config{
		MaxCalls: 1,
	})

	first := factory.New()
	second := factory.New()

	if err := first.Record(Call{}); err != nil {
		t.Fatalf(
			"first.Record() error = %v",
			err,
		)
	}

	// Overflow the first store.
	if err := first.Record(Call{}); !nativeerrors.Is(
		err,
		errors.ErrCallStoreLimitExceeded,
	) {
		t.Fatalf(
			"first.Record() error = %v, want ErrCallStoreLimitExceeded",
			err,
		)
	}

	// The second store must remain unaffected.
	if err := second.Err(); err != nil {
		t.Fatalf(
			"second.Err() = %v, want nil",
			err,
		)
	}

	if err := second.Record(Call{}); err != nil {
		t.Fatalf(
			"second.Record() error = %v",
			err,
		)
	}
}

func TestStoreConcurrentRecording(t *testing.T) {
	const (
		maxCalls  = 100
		workers   = 20
		perWorker = 20
	)

	store := NewStoreFactory(&Config{
		MaxCalls: maxCalls,
	}).New()

	var wg sync.WaitGroup
	var mu sync.Mutex

	var succeeded int
	var rejected int
	var unexpected []error

	for range workers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for range perWorker {
				err := store.Record(Call{
					TestExecutionID: "execution-1",
				})

				// Exercise concurrent reads of the overflow state.
				_ = store.Err()

				mu.Lock()

				switch {
				case err == nil:
					succeeded++

				case nativeerrors.Is(
					err,
					errors.ErrCallStoreLimitExceeded,
				):
					rejected++

				default:
					unexpected = append(unexpected, err)
				}

				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if len(unexpected) != 0 {
		t.Fatalf(
			"unexpected Record errors: %v",
			unexpected,
		)
	}

	if succeeded != maxCalls {
		t.Errorf(
			"successful records = %d, want %d",
			succeeded,
			maxCalls,
		)
	}

	expectedRejected := workers*perWorker - maxCalls

	if rejected != expectedRejected {
		t.Errorf(
			"rejected records = %d, want %d",
			rejected,
			expectedRejected,
		)
	}

	if got := len(store.Get(Filter{})); got != maxCalls {
		t.Errorf(
			"stored calls = %d, want %d",
			got,
			maxCalls,
		)
	}

	if err := store.Err(); !nativeerrors.Is(
		err,
		errors.ErrCallStoreLimitExceeded,
	) {
		t.Errorf(
			"Err() = %v, want ErrCallStoreLimitExceeded",
			err,
		)
	}
}
