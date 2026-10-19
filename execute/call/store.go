package call

import (
	"sync"

	"github.com/e2engine/core/pkg/errors"
)

type Store struct {
	mu            sync.RWMutex
	calls         []Call
	maxCalls      int
	limitExceeded bool
}

var _ Recorder = (*Store)(nil)
var _ Reader = (*Store)(nil)

func (c *Store) Record(call Call) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.limitExceeded || len(c.calls) >= c.maxCalls {
		c.limitExceeded = true
		return errors.ErrCallStoreLimitExceeded
	}

	c.calls = append(c.calls, call)
	return nil
}

func (c *Store) Err() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.limitExceeded {
		return errors.ErrCallStoreLimitExceeded
	}

	return nil
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

type factory struct {
	cfg *Config
}

func (f *factory) New() *Store {
	return &Store{
		maxCalls: f.cfg.MaxCalls,
	}
}

func NewStoreFactory(cfg *Config) StoreFactory {
	return &factory{
		cfg: cfg,
	}
}
