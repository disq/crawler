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

	// Flags
	var (
		flagTimeout    time.Duration
		flagNumWorkers int
		flagLogLevel   string
	)

	{
		const defaultNumWorkers = 256

		flag.DurationVar(&flagTimeout, "t", 5*time.Second, "HTTP timeout")
		flag.IntVar(&flagNumWorkers, "w", defaultNumWorkers, fmt.Sprintf("Number of worker goroutines. Negative numbers mean multiples of the CPU core count."))
		flag.StringVar(&flagLogLevel, "l", "info", "Log level")

		flag.Usage = func() {
			fmt.Printf("Usage: %v [options] [start url] [additional hosts to include...]\n", os.Args[0])
			flag.PrintDefaults()
		}

		flag.Parse()
	}

	var logger yolo.Logger

	// Logger or bust
	{
		lvl, err := yolo.LevelFromString(flagLogLevel)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		logger = yolo.New(yolo.WithLevel(lvl))
	}

	// Number of workers
	{
		if flagNumWorkers < 0 {
			flagNumWorkers = runtime.NumCPU() * -flagNumWorkers
		}
		if flagNumWorkers < 1 {
			flagNumWorkers = 1
		}
	}

	// Start URL
	startParam := flag.Arg(0)
	if startParam == "" {
		flag.Usage()
		os.Exit(1)
	}

	// Parse start URL...
	startURL, err := url.Parse(startParam)
	if err != nil {
		logger.Errorf("%v", err)
		os.Exit(1)
	}

	// Initialize filter, add start url host by default
	fil := urlfilter.New()
	fil.AddHost(startURL.Host)

	// Add the trailing slash
	if startURL.Path == "" {
		startURL.Path = "/"
	}

	// Additional host whitelist...
	for i := 1; i < flag.NArg(); i++ {
		fil.AddHost(flag.Arg(i))
	}

	// More set up
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	client := &http.Client{Timeout: flagTimeout}

	mpr := mapper.New()
	c := crawler.New(ctx, logger, client, fil.Match, mpr)

	// Attempt to queue up start URL as the root URL
	errs := c.Add(nil, startURL)
	if len(errs) != 0 {
		logger.Errorf("%v", errs[0])
		os.Exit(1)
	}

	// Handle signals, cancel context
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM, syscall.SIGPIPE)
		<-ch
		logger.Infof("Got signal, cleaning up...")
		cancelFunc()
	}()

	// Crawl
	startTime := time.Now()
	c.Run(flagNumWorkers)

	// Show sitemap
	mpr.List(os.Stdout)

	// Show stats
	{
		q, v, e := c.Stats()
		if q != v {
			logger.Infof("Visited pages so far: %v (queued: %v)", v, q)
		} else {
			logger.Infof("Visited pages: %v", v)
		}
		logger.Infof("Encountered HREFs: %v", e)

		logger.Infof("Time took: %v", time.Since(startTime))
	}
}
