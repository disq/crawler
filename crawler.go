package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"

	"github.com/disq/yolo"
	"github.com/pkg/errors"
)

type visit struct {
	source, target *url.URL
}

// Crawler is our main struct
type Crawler struct {
	ctx    context.Context
	logger yolo.Logger
	client *http.Client
	wg     sync.WaitGroup

	filter FilterFunc
	mapper Mapper

	visitChan chan visit
	closer    sync.Once

	toVisit   map[string]struct{}
	toVisitMu sync.RWMutex

	numVisited uint64
}

// FilterFunc is used to exclude urls from getting crawled
type FilterFunc func(*url.URL) bool

// Mapper is used to map a site structure
type Mapper interface {
	Add(string, ...string)
}

const (
	visitChanBuffer = 10
)

var (
	ErrUnsupportedScheme = fmt.Errorf("Unsupported scheme")
	ErrFilteredOut       = fmt.Errorf("Filtered out")
	ErrAlreadyInList     = fmt.Errorf("Already in visit list")
)

// New creates a new crawler
func New(ctx context.Context, logger yolo.Logger, client *http.Client, filter FilterFunc, mapper Mapper) *Crawler {
	return &Crawler{
		ctx:    ctx,
		logger: logger,
		client: client,

		filter: filter,
		mapper: mapper,

		visitChan: make(chan visit, visitChanBuffer),
		toVisit:   make(map[string]struct{}),
	}
}

func (c *Crawler) Close() {
	//c.closer.Do(func() {
	//	close(c.visitChan)
	//})
}

// Add adds one or more urls to crawler. source can be nil to indicate root. Returns a list of errors if any occured.
func (c *Crawler) Add(source *url.URL, uri ...*url.URL) []error {
	var errs []error

	for _, u := range uri {
		var err error

		u := u

		if source != nil {
			if u.Scheme == "" {
				u.Scheme = source.Scheme
			}
			if u.Host == "" {
				u.Host = source.Host
			}
		}

		if u.Scheme != "http" && u.Scheme != "https" {
			err = ErrUnsupportedScheme
		} else if err == nil && c.filter != nil && !c.filter(u) {
			err = ErrFilteredOut
		}

		us := u.String()
		if err == nil {
			c.toVisitMu.RLock()
			if _, ok := c.toVisit[us]; ok {
				err = ErrAlreadyInList
			}
			c.toVisitMu.RUnlock()
		}

		c.logger.Debugf("Add(%v %v): %v", source, us, err)

		if err != nil {
			errs = append(errs, errors.Wrapf(err, "Invalid URL %v", u))
			continue
		}

		c.toVisitMu.Lock()
		c.toVisit[us] = struct{}{}
		c.toVisitMu.Unlock()

		{
			uu := *u
			uu.Scheme = ""
			if source != nil && source.Host == uu.Host {
				uu.Host = ""
			}
			if source == nil {
				c.mapper.Add("<root>", uu.String())
			} else {
				c.mapper.Add(source.String(), uu.String())
			}
		}

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

func (c *Crawler) NumVisited() uint64 {
	return atomic.LoadUint64(&c.numVisited)
}
