![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)
![Tag](https://img.shields.io/github/tag/disq/crawler.svg)
[![godoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://godoc.org/github.com/disq/crawler)
[![Go Report](https://goreportcard.com/badge/github.com/disq/crawler)](https://goreportcard.com/report/github.com/disq/crawler)

# Crawler

Web crawler PoC

## Build / Deploy

    # Clone
    git clone https://github.com/disq/crawler.git
    cd crawler

    # Fetch dependencies
    dep ensure

    # Build
    go build ./cmd/crawler

## Usage

    Usage: ./crawler [options] [start url] [additional hosts to include...]
      -l string
            Log level (default "info")
      -t duration
            HTTP timeout (default 5s)
      -w int
            Number of worker goroutines. Negative numbers mean multiples of the CPU core count. (default 256)

# License

MIT.