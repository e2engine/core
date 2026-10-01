package call

import "sync"

type Store struct {
	mu    sync.RWMutex
	calls []Call
}

var _ Recorder = (*Store)(nil)
var _ Reader = (*Store)(nil)

func (c *Store) Record(call Call) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.calls = append(c.calls, call)
}

type Filter struct {
	TestExecutionID string
	ServiceID       string
}

func (c *Store) Get(
	f Filter,
) []Call {
	c.mu.RLock()
	defer c.mu.RUnlock()

	calls := make([]Call, 0)

	for i := range c.calls {
		if f.TestExecutionID != "" && c.calls[i].TestExecutionID != f.TestExecutionID {
			continue
		}
		if f.ServiceID != "" && c.calls[i].ServiceID != f.ServiceID {
			continue
		}

		calls = append(calls, c.calls[i])
	}

	return calls
}

type StoreFactory interface {
	New() *Store
}

type factory struct{}

func (f *factory) New() *Store {
	return &Store{}
}

func NewStoreFactory() StoreFactory {
	return &factory{}
}
