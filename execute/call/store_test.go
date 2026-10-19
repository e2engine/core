package call

import (
	nativeerrors "errors"
	"sync"
	"testing"

	"github.com/e2engine/core/pkg/errors"
)

func TestStoreGet(t *testing.T) {
	store := NewStoreFactory(&Config{
		MaxCalls:         3,
		MaxRecordedBytes: 1 << 20,
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
		MaxCalls:         1,
		MaxRecordedBytes: 1 << 20,
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
		MaxCalls:         2,
		MaxRecordedBytes: 1 << 20,
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
	f := NewStoreFactory(&Config{
		MaxCalls:         1,
		MaxRecordedBytes: 1 << 20,
	})

	first := f.New()
	second := f.New()

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
		MaxCalls:         maxCalls,
		MaxRecordedBytes: 1 << 20,
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

func TestStoreRecordedBytesLimit(t *testing.T) {
	tests := []struct {
		name         string
		maxBytes     int64
		calls        []Call
		wantAccepted int
		wantExceeded bool
	}{
		{
			name:     "exact boundary",
			maxBytes: 5,
			calls: []Call{
				{
					HTTP: &HTTPCall{
						Request: HTTPRequest{
							Body: []byte("ab"),
						},
					},
				},
				{
					HTTP: &HTTPCall{
						Response: HTTPResponse{
							Body: []byte("cde"),
						},
					},
				},
			},
			wantAccepted: 2,
		},
		{
			name:     "cumulative overflow",
			maxBytes: 4,
			calls: []Call{
				{
					HTTP: &HTTPCall{
						Request: HTTPRequest{
							Body: []byte("ab"),
						},
					},
				},
				{
					HTTP: &HTTPCall{
						Response: HTTPResponse{
							Body: []byte("cde"),
						},
					},
				},
			},
			wantAccepted: 1,
			wantExceeded: true,
		},
		{
			name:     "single call too large",
			maxBytes: 2,
			calls: []Call{
				{
					GRPC: &GRPCCall{
						Request: GRPCRequest{
							Message: []byte{1, 2, 3},
						},
					},
				},
			},
			wantAccepted: 0,
			wantExceeded: true,
		},
		{
			name:     "zero-sized record",
			maxBytes: 1,
			calls: []Call{
				{},
			},
			wantAccepted: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewStoreFactory(&Config{
				MaxCalls:         10,
				MaxRecordedBytes: tt.maxBytes,
			}).New()

			var accepted int

			for _, record := range tt.calls {
				err := store.Record(record)

				switch {
				case err == nil:
					accepted++

				case nativeerrors.Is(
					err,
					errors.ErrCallStoreLimitExceeded,
				):
					// Expected when the byte limit is exceeded.

				default:
					t.Fatalf(
						"Record() unexpected error = %v",
						err,
					)
				}
			}

			if accepted != tt.wantAccepted {
				t.Errorf(
					"accepted calls = %d, want %d",
					accepted,
					tt.wantAccepted,
				)
			}

			recorded := store.Get(Filter{})

			if len(recorded) != tt.wantAccepted {
				t.Errorf(
					"retained calls = %d, want %d",
					len(recorded),
					tt.wantAccepted,
				)
			}

			var expectedBytes int64

			for _, retained := range recorded {
				expectedBytes += retained.Size()
			}

			if store.recordedBytes != expectedBytes {
				t.Errorf(
					"recordedBytes = %d, want %d",
					store.recordedBytes,
					expectedBytes,
				)
			}

			if tt.wantExceeded {
				if err := store.Err(); !nativeerrors.Is(
					err,
					errors.ErrCallStoreLimitExceeded,
				) {
					t.Errorf(
						"Err() = %v, want ErrCallStoreLimitExceeded",
						err,
					)
				}

				// Once exceeded, even a smaller call is rejected.
				if err := store.Record(Call{}); !nativeerrors.Is(
					err,
					errors.ErrCallStoreLimitExceeded,
				) {
					t.Errorf(
						"subsequent Record() error = %v, want ErrCallStoreLimitExceeded",
						err,
					)
				}

				if got := len(store.Get(Filter{})); got != tt.wantAccepted {
					t.Errorf(
						"retained calls after overflow = %d, want %d",
						got,
						tt.wantAccepted,
					)
				}
			} else {
				if err := store.Err(); err != nil {
					t.Errorf(
						"Err() = %v, want nil",
						err,
					)
				}
			}
		})
	}
}

func TestStoreConcurrentRecordedBytesLimit(t *testing.T) {
	const (
		maxCalls  = 1000
		maxBytes  = 100
		workers   = 20
		perWorker = 20
		callSize  = 3
	)

	store := NewStoreFactory(&Config{
		MaxCalls:         maxCalls,
		MaxRecordedBytes: maxBytes,
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
					HTTP: &HTTPCall{
						Request: HTTPRequest{
							Body: []byte("abc"),
						},
					},
				})

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

	expectedAccepted := maxBytes / callSize

	if len(unexpected) != 0 {
		t.Fatalf("unexpected Record() errors: %v", unexpected)
	}

	if succeeded != expectedAccepted {
		t.Errorf(
			"successful records = %d, want %d",
			succeeded,
			expectedAccepted,
		)
	}

	if rejected != workers*perWorker-expectedAccepted {
		t.Errorf(
			"rejected records = %d, want %d",
			rejected,
			workers*perWorker-expectedAccepted,
		)
	}

	if got := len(store.Get(Filter{})); got != expectedAccepted {
		t.Errorf(
			"stored calls = %d, want %d",
			got,
			expectedAccepted,
		)
	}

	if want := int64(expectedAccepted * callSize); store.recordedBytes != want {
		t.Errorf(
			"recordedBytes = %d, want %d",
			store.recordedBytes,
			want,
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

func TestStoreCombinedLimits(t *testing.T) {
	tests := []struct {
		name         string
		maxCalls     int
		maxBytes     int64
		recordSizes  []int
		wantAccepted int
		wantBytes    int64
	}{
		{
			name:         "call count limit reached first",
			maxCalls:     2,
			maxBytes:     100,
			recordSizes:  []int{3, 3, 3},
			wantAccepted: 2,
			wantBytes:    6,
		},
		{
			name:         "byte limit reached first",
			maxCalls:     100,
			maxBytes:     5,
			recordSizes:  []int{3, 3},
			wantAccepted: 1,
			wantBytes:    3,
		},
		{
			name:         "both limits reached exactly",
			maxCalls:     2,
			maxBytes:     6,
			recordSizes:  []int{3, 3, 3},
			wantAccepted: 2,
			wantBytes:    6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewStoreFactory(&Config{
				MaxCalls:         tt.maxCalls,
				MaxRecordedBytes: tt.maxBytes,
			}).New()

			var accepted int

			for _, size := range tt.recordSizes {
				err := store.Record(Call{
					HTTP: &HTTPCall{
						Request: HTTPRequest{
							Body: make([]byte, size),
						},
					},
				})

				if err == nil {
					accepted++
					continue
				}

				if !nativeerrors.Is(
					err,
					errors.ErrCallStoreLimitExceeded,
				) {
					t.Fatalf("Record() unexpected error = %v", err)
				}
			}

			if accepted != tt.wantAccepted {
				t.Errorf(
					"accepted = %d, want %d",
					accepted,
					tt.wantAccepted,
				)
			}

			if store.recordedBytes != tt.wantBytes {
				t.Errorf(
					"recordedBytes = %d, want %d",
					store.recordedBytes,
					tt.wantBytes,
				)
			}

			if got := len(store.Get(Filter{})); got != tt.wantAccepted {
				t.Errorf(
					"recorded calls = %d, want %d",
					got,
					tt.wantAccepted,
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
		})
	}
}
