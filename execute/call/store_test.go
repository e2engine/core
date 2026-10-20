package call

import (
	nativeerrors "errors"
	"net/http"
	"reflect"
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

func TestStoreRecordOwnsSnapshot(t *testing.T) {
	store := NewStoreFactory(&Config{
		MaxCalls:         10,
		MaxRecordedBytes: 1 << 20,
	}).New()

	original := ownershipTestCall()
	want := original.Clone()
	wantBytes := want.Size()

	if err := store.Record(original); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	// Modify the original after recording.
	mutateOwnershipTestCall(&original)

	got := store.Get(Filter{})
	if len(got) != 1 {
		t.Fatalf("Get() returned %d calls, want 1", len(got))
	}

	if !reflect.DeepEqual(got[0], want) {
		t.Errorf(
			"recorded call changed after input mutation: got %#v, want %#v",
			got[0],
			want,
		)
	}

	if store.recordedBytes != wantBytes {
		t.Errorf(
			"recordedBytes = %d, want %d",
			store.recordedBytes,
			wantBytes,
		)
	}

	if err := store.Err(); err != nil {
		t.Errorf("Err() = %v, want nil", err)
	}
}

func TestStoreGetReturnsIndependentSnapshots(t *testing.T) {
	store := NewStoreFactory(&Config{
		MaxCalls:         10,
		MaxRecordedBytes: 1 << 20,
	}).New()

	original := ownershipTestCall()
	want := original.Clone()
	wantBytes := want.Size()

	if err := store.Record(original); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	first := store.Get(Filter{})
	if len(first) != 1 {
		t.Fatalf("first Get() returned %d calls, want 1", len(first))
	}

	// Modify the returned call.
	mutateOwnershipTestCall(&first[0])

	// A subsequent read must return the original data.
	second := store.Get(Filter{})
	if len(second) != 1 {
		t.Fatalf("second Get() returned %d calls, want 1", len(second))
	}

	if !reflect.DeepEqual(second[0], want) {
		t.Errorf(
			"stored call changed after Get() result mutation: got %#v, want %#v",
			second[0],
			want,
		)
	}

	if store.recordedBytes != wantBytes {
		t.Errorf(
			"recordedBytes = %d, want %d",
			store.recordedBytes,
			wantBytes,
		)
	}

	if err := store.Err(); err != nil {
		t.Errorf("Err() = %v, want nil", err)
	}
}

func ownershipTestCall() Call {
	return Call{
		TestExecutionID:    "execution-1",
		ServiceID:          "service-1",
		MockFixtureMatched: true,

		HTTP: &HTTPCall{
			Request: HTTPRequest{
				Method: "POST",
				Path:   "/test",
				Query: map[string][]string{
					"q": {"original"},
				},
				Headers: http.Header{
					"X-Request": {"original"},
				},
				Body:         []byte("request"),
				BodyComplete: true,
			},
			Response: HTTPResponse{
				StatusCode: http.StatusOK,
				Headers: http.Header{
					"X-Response": {"original"},
				},
				Body: []byte("response"),
			},
		},

		GRPC: &GRPCCall{
			Request: GRPCRequest{
				Service: "service",
				Method:  "Method",
				Metadata: map[string][]string{
					"request": {"original"},
				},
				Message: []byte("request"),
			},
			Response: GRPCResponse{
				Status: "OK",
				Metadata: map[string][]string{
					"response": {"original"},
				},
				Message: []byte("response"),
			},
		},
	}
}

func mutateOwnershipTestCall(c *Call) {
	c.TestExecutionID = "modified-execution"
	c.ServiceID = "modified-service"
	c.MockFixtureMatched = false

	// HTTP request.
	c.HTTP.Request.Method = "GET"
	c.HTTP.Request.Query["q"][0] = "modified"
	c.HTTP.Request.Query["extra"] = []string{"added"}
	c.HTTP.Request.Headers["X-Request"][0] = "modified"
	c.HTTP.Request.Headers.Set("X-Extra", "added")
	c.HTTP.Request.Body[0] = 'X'

	// HTTP response.
	c.HTTP.Response.StatusCode = http.StatusCreated
	c.HTTP.Response.Headers["X-Response"][0] = "modified"
	c.HTTP.Response.Body[0] = 'X'

	// gRPC request.
	c.GRPC.Request.Service = "modified-service"
	c.GRPC.Request.Metadata["request"][0] = "modified"
	c.GRPC.Request.Metadata["extra"] = []string{"added"}
	c.GRPC.Request.Message[0] = 'X'

	// gRPC response.
	c.GRPC.Response.Status = "Internal"
	c.GRPC.Response.Metadata["response"][0] = "modified"
	c.GRPC.Response.Message[0] = 'X'
}
