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

func main() {
	patternArgs := os.Args[1:]
	patterns := map[*regexp.Regexp]int{}
	for _, patternArg := range patternArgs {
		r, err := regexp.Compile(patternArg)
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}
		patterns[r] = 0
	}

	writer := uilive.New()
	reader := bufio.NewReader(os.Stdin)

	writer.Start()

	var exitCode int

	for {
		table := uitable.New()
		table.AddRow("PATTERN", "COUNT")

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

		for r, i := range patterns {
			indexMatches := r.FindAllIndex(line, -1)
			if indexMatches != nil {
				patterns[r] = i + len(indexMatches)
			}
			table.AddRow(r.String(), patterns[r])
		}

		fmt.Fprintln(writer, table)
		writer.Flush()
	}

	writer.Stop()

	os.Exit(exitCode)
}
