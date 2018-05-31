package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

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

	toVisit   map[string]struct{}
	toVisitMu sync.RWMutex

	lastActiveTime int64 // time, but sync/atomic compatible

	numQueued      uint64
	numVisited     uint64
	numEncountered uint64
}

// FilterFunc is used to exclude urls from getting crawled
type FilterFunc func(*url.URL) bool

// Mapper is used to map a site structure
type Mapper interface {
	Add(string, ...string)
}

const (
	visitChanBuffer = 1000
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

func (c *Crawler) registerActivity() {
	atomic.StoreInt64(&c.lastActiveTime, time.Now().Unix())
}

func (c *Crawler) lastActivity() time.Time {
	return time.Unix(atomic.LoadInt64(&c.lastActiveTime), 0)
}

// Add adds one or more urls to crawler. source can be nil to indicate root. Returns a list of errors if any occured.
func (c *Crawler) Add(source *url.URL, uri ...*url.URL) []error {
	var errs []error

	for _, u := range uri {
		var err error

		u := u
		u.Fragment = "" // reset fragment, we don't want it messing our visited list

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

		if err == nil {
			c.logger.Debugf("Add(%v %v): OK", source, us)
			atomic.AddUint64(&c.numQueued, 1)
		} else if err != nil {
			atomic.AddUint64(&c.numEncountered, 1)
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

	// Here we create a new cancellable context to control goroutines and requests.
	// We can't just rely on the parent context because we want to shutdown goroutines if there's no activity for some time.
	// Closing the channel would introduce "send on closed chan" errors since channel consumers also produce new messages.
	ctx, cancel := context.WithCancel(c.ctx)

	c.wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go c.worker(ctx)
	}

	// Check every second if we're still actively crawling pages

	limit := 1.5 * c.client.Timeout.Seconds()
	for {
		if len(c.visitChan) == 0 && time.Since(c.lastActivity()).Seconds() > limit {
			break
		}

		select {
		case <-c.ctx.Done():
			goto endfor
		case <-time.After(time.Second):
		}

	}
endfor:
	cancel()           // cancel goroutines and in-flight requests (if any)
	c.wg.Wait()        // wait for shutdown
	close(c.visitChan) // close after we're done (not before) to prevent send on closed channel errors
}

func (c *Crawler) Stats() (uint64, uint64, uint64) {
	q := atomic.LoadUint64(&c.numQueued)
	v := atomic.LoadUint64(&c.numVisited)
	e := atomic.LoadUint64(&c.numEncountered)
	return q, v, e
}
