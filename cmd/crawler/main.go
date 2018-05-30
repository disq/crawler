package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"net/url"

	"github.com/disq/crawler"
	"github.com/disq/crawler/urlfilter"
)

func main() {
	timeout := flag.Duration("timeout", 5*time.Second, "HTTP timeout")
	nw := flag.Int("workers", runtime.NumCPU(), "Number of workers")

	flag.Usage = func() {
		fmt.Printf("Usage: %v [options] [start url] [additional hosts to include...]\n", os.Args[0])
	}

	flag.Parse()

	startURL := flag.Arg(0)

	if startURL == "" {
		flag.Usage()
		os.Exit(1)
	}

	fil := urlfilter.New()

	if u, err := url.Parse(startURL); err == nil {
		fil.AddHost(u.Host)
	}

	for i := 1; i < flag.NArg(); i++ {
		fil.AddHost(flag.Arg(i))
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	client := &http.Client{Timeout: *timeout}

	logger := log.New(os.Stderr, "", log.LstdFlags|log.LUTC)
	c := crawler.New(ctx, logger, client, fil.Match)

	errs := c.Add(startURL)
	if len(errs) != 0 {
		logger.Fatal(errs[0])
	}

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM, syscall.SIGPIPE)
		<-ch
		logger.Print("Got signal, cleaning up...")
		cancelFunc()
	}()

	go c.Run(*nw)

	<-ctx.Done()
	c.Close()

	// TODO show sitemap
}
