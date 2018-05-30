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

type visit struct {
	source, target *url.URL
}

// Crawler is our main struct
type Crawler struct {
	ctx    context.Context
	logger *log.Logger
	client *http.Client
	wg     sync.WaitGroup

	filter FilterFunc

	visitChan chan visit
	closer    sync.Once

	toVisit   map[string]struct{}
	toVisitMu sync.Mutex
}

// FilterFunc is used to exclude urls from getting crawled
type FilterFunc func(*url.URL) bool

const (
	visitChanBuffer = 10
)

var (
	ErrUnsupportedScheme = fmt.Errorf("Unsupported scheme")
	ErrFilteredOut       = fmt.Errorf("Filtered out")
	ErrAlreadyInList     = fmt.Errorf("Already in visit list")
)

// New creates a new crawler
func New(ctx context.Context, logger *log.Logger, client *http.Client, filter FilterFunc) *Crawler {
	return &Crawler{
		ctx:    ctx,
		logger: logger,
		client: client,

		filter: filter,

		visitChan: make(chan visit, visitChanBuffer),
		toVisit:   make(map[string]struct{}),
	}
}

func (c *Crawler) Close() {
	c.closer.Do(func() {
		close(c.visitChan)
	})
}

// Add adds one or more urls to crawler. source can be nil. Returns a list of errors if any occured.
func (c *Crawler) Add(source *url.URL, uri ...*url.URL) []error {
	var errs []error

	c.toVisitMu.Lock()
	defer c.toVisitMu.Unlock()

	for _, u := range uri {
		var err error

		if u.Scheme == "" {
			u.Scheme = source.Scheme
		}
		if u.Host == "" {
			u.Host = source.Host
		}

		if u.Scheme != "http" && u.Scheme != "https" {
			err = ErrUnsupportedScheme
		} else if err == nil && c.filter != nil && !c.filter(u) {
			err = ErrFilteredOut
		}

		us := u.String()
		if err == nil {
			if _, ok := c.toVisit[us]; ok {
				err = ErrAlreadyInList
			}
		}

		c.logger.Printf("Add(%v %v): %v", source, us, err)

		if err != nil {
			errs = append(errs, errors.Wrapf(err, "Invalid URL %v", u))
			continue
		}

		c.toVisit[us] = struct{}{}

		v := visit{
			source: source,
			target: u,
		}

		select {
		case c.visitChan <- v:
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
