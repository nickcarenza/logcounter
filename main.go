package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
)

import (
	"github.com/gosuri/uilive"
	"github.com/gosuri/uitable"
)

type counter struct {
	p *regexp.Regexp
	n int
}

func main() {
	patternArgs := os.Args[1:]
	counters := []counter{}
	for _, patternArg := range patternArgs {
		r, err := regexp.Compile(patternArg)
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}
		counters = append(counters, counter{r, 0})
	}

	// If no parameters are given, count each distinct line
	var useDistinct = false
	if len(counters) == 0 {
		useDistinct = true
	}

	writer := uilive.New()
	reader := bufio.NewReader(os.Stdin)

	writer.Start()

	var exitCode int

	var totalLinesRead = 0

	for {
		table := uitable.New()
		table.AddRow("PATTERN", "COUNT")

		line, err := reader.ReadBytes('\n')
		line = line[:len(line)-1]
		if err == io.EOF {
			exitCode = 0
			break
		}
		if err != nil {
			log.Println(err)
			exitCode = 1
			break
		}

		if useDistinct {
			p := fmt.Sprintf("^%s$", regexp.QuoteMeta(string(line)))
			var patternExists = false
			for _, c := range counters {
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
				c := counter{r, 0}
				counters = append(counters, c)
			}
		}

		for i, _ := range counters {
			c := &counters[i]
			indexMatches := c.p.FindAllIndex(line, -1)
			if indexMatches != nil {
				c.n = c.n + len(indexMatches)
			}
			table.AddRow(c.p.String(), c.n)
		}

		totalLinesRead++

		table.AddRow("Total Lines Read:", totalLinesRead)

		fmt.Fprintln(writer, table)
		writer.Flush()
	}

	writer.Stop()

	os.Exit(exitCode)
}
