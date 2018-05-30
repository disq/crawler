package crawler

import (
	"io"
	"net/url"

	"golang.org/x/net/html"
)

func (c *Crawler) parse(source *url.URL, r io.Reader) error {
	var toVisit []*url.URL

	htmlTokens := html.NewTokenizer(r)
loop:
	for {
		tt := htmlTokens.Next()
		switch tt {
		case html.ErrorToken:
			break loop
		case html.StartTagToken:
			t := htmlTokens.Token()
			if t.Data == "a" {
				for _, at := range t.Attr {
					if at.Key == "href" {
						u, err := url.Parse(at.Val)
						if err != nil {
							c.logger.Printf("Invalid URL %v: %v", at.Val, err)
							continue
						}

						toVisit = append(toVisit, u)
					}
				}

			}
		}
	}

	c.Add(source, toVisit...)

	return nil
}
