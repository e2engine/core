package call

import "testing"

func TestStoreGet(t *testing.T) {
	store := &Store{}

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

	store.Record(call1)
	store.Record(call2)
	store.Record(call3)

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
	factory := NewStoreFactory()

	store1 := factory.New()
	store2 := factory.New()

	if store1 == nil {
		t.Fatal("expected first store")
	}

	if store2 == nil {
		t.Fatal("expected second store")
	}

	if store1 == store2 {
		t.Fatal("expected independent stores")
	}

	store1.Record(Call{
		TestExecutionID: "execution-1",
	})

	if len(store1.Get(Filter{})) != 1 {
		t.Fatal("expected call in first store")
	}

	if len(store2.Get(Filter{})) != 0 {
		t.Fatal("expected second store to be empty")
	}
}
