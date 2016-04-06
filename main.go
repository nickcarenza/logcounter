package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"sync/atomic"
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

func main() {
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
	reader := bufio.NewReader(os.Stdin)

	writer.Start()

	var exitCode int

	var totalLinesRead = 0

	for {
		select {
		case <-repaintTicker.C:
			repaint(writer, logCounters, totalLinesRead)
		default:
			line, err := reader.ReadBytes('\n')
			if err == io.EOF {
				exitCode = 0
				break
			}
			if err != nil {
				log.Println(err)
				exitCode = 1
				break
			}
			line = line[:len(line)-1]

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
					r, err := regexp.Compile(p)
					if err != nil {
						log.Println(err)
						exitCode = 1
						break
					}
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

	writer.Stop()

	os.Exit(exitCode)
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
