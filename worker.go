package crawler

import "io/ioutil"

func (c *Crawler) worker() {
	defer c.wg.Done()

	for addr := range c.visitChan {
		resp, err := c.client.Get(addr)
		if err != nil {
			c.logger.Printf("Crawl(%v): %v", addr, err)
		}
		b, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			c.logger.Printf("Crawl(%v): Error reading body: %v", addr, err)
			continue
		}

		// TODO use regex to parse urls and call c.Add
		_ = b

	}

}
