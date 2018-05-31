package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/disq/crawler"
	"github.com/disq/crawler/mapper"
	"github.com/disq/crawler/urlfilter"
	"github.com/disq/yolo"
)

func main() {
	timeout := flag.Duration("timeout", 5*time.Second, "HTTP timeout")
	nw := flag.Int("workers", runtime.NumCPU(), "Number of workers")
	ll := flag.String("log", "debug", "Log level")

	flag.Usage = func() {
		fmt.Printf("Usage: %v [options] [start url] [additional hosts to include...]\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	lvl, err := yolo.LevelFromString(*ll)
	if err != nil {
		panic(err)
	}

	startParam := flag.Arg(0)

	if startParam == "" {
		flag.Usage()
		os.Exit(1)
	}

	logger := yolo.New(yolo.WithLevel(lvl))

	fil := urlfilter.New()

	startURL, err := url.Parse(startParam)
	if err != nil {
		logger.Errorf("%v", err)
		os.Exit(1)
	}
	fil.AddHost(startURL.Host)

	if startURL.Path == "" {
		startURL.Path = "/"
	}

	for i := 1; i < flag.NArg(); i++ {
		fil.AddHost(flag.Arg(i))
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	client := &http.Client{Timeout: *timeout}

	mpr := mapper.New()
	c := crawler.New(ctx, logger, client, fil.Match, mpr)

	errs := c.Add(nil, startURL)
	if len(errs) != 0 {
		logger.Errorf("%v", errs[0])
		os.Exit(1)
	}

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM, syscall.SIGPIPE)
		<-ch
		logger.Infof("Got signal, cleaning up...")
		cancelFunc()
	}()

	c.Run(*nw)

	mpr.List(os.Stdout)
	logger.Infof("Visited pages: %v", c.NumVisited())
}
