package crawler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"

	"github.com/pkg/errors"
)

// Crawler is our main struct
type Crawler struct {
	ctx    context.Context
	logger *log.Logger
	client *http.Client
	wg     sync.WaitGroup

	filter FilterFunc

	visitChan chan string
	closer    sync.Once

	visited   map[string]struct{}
	visitedMu sync.RWMutex
}

// FilterFunc is used to exclude urls from getting crawled
type FilterFunc func(*url.URL) bool

const (
	visitChanBuffer = 10
)

var (
	ErrUnsupportedScheme = fmt.Errorf("Unsupported scheme")
	ErrFilteredOut       = fmt.Errorf("Filtered out")
)

// New creates a new crawler
func New(ctx context.Context, logger *log.Logger, client *http.Client, filter FilterFunc) *Crawler {
	return &Crawler{
		ctx:    ctx,
		logger: logger,
		client: client,

		filter: filter,

		visitChan: make(chan string, visitChanBuffer),
		visited:   make(map[string]struct{}),
	}
}

func (c *Crawler) Close() {
	c.closer.Do(func() {
		close(c.visitChan)
	})
}

// Add adds one or more urls to crawler. Returns a list of errors if any occured.
func (c *Crawler) Add(uri ...string) []error {
	var errs []error

	for _, u := range uri {
		res, err := url.Parse(u)
		if err == nil && res.Scheme != "http" && res.Scheme != "https" {
			err = ErrUnsupportedScheme
		} else if err == nil && c.filter != nil && !c.filter(res) {
			err = ErrFilteredOut
		}
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "Invalid URL %v", u))
		}

		select {
		case c.visitChan <- u:
		case <-c.ctx.Done():
			return append(errs, c.ctx.Err())
		}
	}

	return errs
}

// Run launches the worker pool and blocks until they all finish.
func (c *Crawler) Run(numWorkers int) {
	c.wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go c.worker()
	}
	c.wg.Wait()
}
