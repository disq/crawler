package crawler

func (c *Crawler) worker() {
	defer c.wg.Done()

	for uri := range c.visitChan {
		addr := uri.target.String()

		c.logger.Printf("Crawl(%v): (from %v)", addr, uri.source)

		resp, err := c.client.Get(addr)
		if err != nil {
			c.logger.Printf("Crawl(%v): %v", addr, err)
		}

		if err := c.parse(uri.source, resp.Body); err != nil {
			c.logger.Printf("Crawl(%v): Parse: %v", addr, err)
		}

		resp.Body.Close()
	}

}
