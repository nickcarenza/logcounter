package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"
)

import (
	"github.com/gosuri/uilive"
	"github.com/gosuri/uitable"
	// copied counters from: github.com/paulbellamy/ratecounter
)

var config = map[string]string{
	"repaint_interval": os.Getenv("LOGC_REPAINT_INTERVAL"),
}

type LogCounter struct {
	p    *regexp.Regexp
	c    Counter
	rsec *RateCounter
	rmin *RateCounter
	rhr  *RateCounter
}

// A Counter is a thread-safe counter implementation
type Counter int64

// Increment the counter by some value
func (c *Counter) Incr(val int64) {
	atomic.AddInt64((*int64)(c), val)
}

// Return the counter's current value
func (c *Counter) Value() int64 {
	return atomic.LoadInt64((*int64)(c))
}

// Reset the counter's value
func (c *Counter) Reset() {
	atomic.StoreInt64((*int64)(c), 0)
}

// A RateCounter is a thread-safe counter which returns the number of times
// 'Incr' has been called in the last interval
type RateCounter struct {
	counter  Counter
	interval time.Duration
}

// Constructs a new RateCounter, for the interval provided
func NewRateCounter(intrvl time.Duration) *RateCounter {
	return &RateCounter{
		interval: intrvl,
	}
}

// Add an event into the RateCounter
func (r *RateCounter) Incr(val int64) {
	r.counter.Incr(val)
	time.AfterFunc(r.interval, func() {
		r.counter.Incr(-1 * val)
	})
	//go r.scheduleDecrement(val)
}

// func (r *RateCounter) scheduleDecrement(amount int64) {
// 	time.Sleep(r.interval)
// 	r.counter.Incr(-1 * amount)
// }

// Return the current number of events in the last interval
func (r *RateCounter) Rate() int64 {
	return r.counter.Value()
}

func (r *RateCounter) Reset() {
	r.counter.Reset()
}

func (r *RateCounter) String() string {
	return strconv.FormatInt(r.counter.Value(), 10) + "/" + r.interval.String()
}

func NewLogCounter(p *regexp.Regexp) *LogCounter {
	return &LogCounter{
		p:    p,
		c:    Counter(0),
		rsec: NewRateCounter(1 * time.Second),
		rmin: NewRateCounter(1 * time.Minute),
		rhr:  NewRateCounter(1 * time.Hour),
	}
}

func (c *LogCounter) Incr(delta int64) {
	c.c.Incr(delta)
	c.rsec.Incr(delta)
	c.rmin.Incr(delta)
	c.rhr.Incr(delta)
}

func (c *LogCounter) Reset() {
	c.c.Reset()
	c.rsec.Reset()
	c.rmin.Reset()
	c.rhr.Reset()
}

func main() {
	// USR1 Will change mode to passthough to behave just like tail -f, counts and rates continue in this mode
	var passthroughChan = make(chan os.Signal, 1)
	signal.Notify(passthroughChan, syscall.SIGUSR1)

	// USR2 Will reset the counters
	var resetChan = make(chan os.Signal, 1)
	signal.Notify(resetChan, syscall.SIGUSR2)

	patternArgs := os.Args[1:]
	logCounters := []*LogCounter{}
	for _, patternArg := range patternArgs {
		r, err := regexp.Compile(patternArg)
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}
		logCounters = append(logCounters, NewLogCounter(r))
	}

	// If no parameters are given, count each distinct line
	var useDistinct = false
	if len(logCounters) == 0 {
		useDistinct = true
	}

	repaintInterval, err := time.ParseDuration(config["repaint_interval"])
	if err != nil {
		repaintInterval = 1 * time.Second
	}

	repaintTicker := time.NewTicker(repaintInterval)

	writer := uilive.New()
	// reader := bufio.NewReader(os.Stdin)
	scanner := bufio.NewScanner(os.Stdin)
	lineChan := make(chan []byte)

	go func(scanner *bufio.Scanner, lineChan chan []byte) {
		for scanner.Scan() {
			var scannerBytes = scanner.Bytes()
			var line = make([]byte, len(scannerBytes))
			copy(line, scannerBytes)
			lineChan <- line
		}
		close(lineChan)
	}(scanner, lineChan)

	writer.Start()
	defer writer.Stop()

	var totalLinesRead = 0

	var passthrough = false

	defer repaint(writer, logCounters, totalLinesRead)

	for {
		select {
		case <-resetChan:
			for _, cnt := range logCounters {
				cnt.Reset()
				totalLinesRead = 0
				repaint(writer, logCounters, totalLinesRead)
			}
		case <-passthroughChan:
			passthrough = !passthrough
		case <-repaintTicker.C:
			if !passthrough {
				repaint(writer, logCounters, totalLinesRead)
			}
		case line, ok := <-lineChan:
			if !ok {
				return
			}

			if passthrough {
				log.Println(string(line))
			}

			if useDistinct {
				p := fmt.Sprintf("^%s$", regexp.QuoteMeta(string(line)))
				var patternExists = false
				for _, c := range logCounters {
					if c.p.String() == p {
						patternExists = true
						break
					}
				}
				if !patternExists {
					r := regexp.MustCompile(p)
					c := NewLogCounter(r)
					logCounters = append(logCounters, c)
				}
			}

			for _, c := range logCounters {
				indexMatches := c.p.FindAllIndex(line, -1)
				if indexMatches != nil {
					c.Incr(int64(len(indexMatches)))
				}
			}

			totalLinesRead++
		}
	}
}

func repaint(w *uilive.Writer, logCounters []*LogCounter, totalLinesRead int) {
	table := uitable.New()
	table.AddRow("PATTERN", "COUNT", "RATE/s", "RATE/m", "RATE/hr")
	for _, c := range logCounters {
		table.AddRow(c.p.String(), c.c.Value(), c.rsec.String(), c.rmin.String(), c.rhr.String())
	}

	table.AddRow("Total Lines Read:", totalLinesRead, "--", "--", "--")
	fmt.Fprintln(w, table)
	w.Flush()
}
