package crawler

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
)

func (c *Crawler) worker(ctx context.Context) {
	defer c.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case uri, ok := <-c.visitChan:
			if !ok {
				return
			}

			c.registerActivity()

			addr := uri.target.String()

			c.logger.Debugf("Crawl(%v) from %v", addr, uri.source)

			req, err := http.NewRequest("GET", addr, nil)
			if err != nil {
				c.logger.Errorf("Crawl(%v): %v", addr, err)
				continue
			}
			req = req.WithContext(ctx)

			req.Header.Add("Accept", "text/*")

			resp, err := c.client.Do(req)
			if err != nil {
				c.logger.Errorf("Crawl(%v): %v", addr, err)
				continue
			}

			atomic.AddUint64(&c.numVisited, 1)

			if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/") {
				c.logger.Warnf("Crawl(%v): Unhandled content-type %v", addr, ct)
			} else if err := c.parse(uri.target, resp.Body); err != nil {
				c.logger.Errorf("Crawl(%v): Parse: %v", addr, err)
			}

			resp.Body.Close()
		}
	}
}
