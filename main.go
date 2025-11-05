package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

var (
	workers  = flag.Int("j", 16, "number of concurrent workers")
	hashType = flag.String("type", "sha256", "hash algorithm: md5, sha1, sha256, sha512, blake3")
	doUnpack = flag.Bool("unpack", false, "unpack archive and compute NAR hash")
	jsonOut  = flag.Bool("json", false, "output JSON format")
	timeout  = flag.Int("timeout", 300, "download timeout in seconds")
	failFast = flag.Bool("fail-fast", false, "exit on first error")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS] < urls.txt\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")

		type flagInfo struct {
			name    string
			usage   string
			def     string
		}

		var flags []flagInfo
		flag.VisitAll(func(f *flag.Flag) {
			prefix := "--"
			if len(f.Name) == 1 {
				prefix = "-"
			}
			flags = append(flags, flagInfo{
				name:  prefix + f.Name,
				usage: f.Usage,
				def:   f.DefValue,
			})
		})

		sort.Slice(flags, func(i, j int) bool {
			return flags[i].name < flags[j].name
		})

		w := tabwriter.NewWriter(os.Stderr, 0, 0, 2, ' ', 0)
		for _, f := range flags {
			fmt.Fprintf(w, "  %s\t%s\t(default %s)\n", f.name, f.usage, f.def)
		}
		w.Flush()
	}
}

func main() {
	flag.Parse()

	urls, err := readURLs(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
		os.Exit(2)
	}

	if len(urls) == 0 {
		fmt.Fprintf(os.Stderr, "no URLs provided\n")
		os.Exit(2)
	}

	opts := Options{
		Workers:  *workers,
		HashType: *hashType,
		Unpack:   *doUnpack,
		Timeout:  *timeout,
		Retries:  3,
		FailFast: *failFast,
	}

	results := WorkerPool(urls, opts)
	outputResults(results, *jsonOut)

	allSuccess := true
	anySuccess := false
	for _, r := range results {
		if r.Error != nil {
			allSuccess = false
		} else {
			anySuccess = true
		}
	}

	if allSuccess {
		os.Exit(0)
	} else if anySuccess {
		os.Exit(1)
	} else {
		os.Exit(2)
	}
}

func readURLs(r io.Reader) ([]string, error) {
	var urls []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		urls = append(urls, line)
	}
	return urls, scanner.Err()
}

func outputResults(results []Result, jsonMode bool) {
	if jsonMode {
		type jsonItem struct {
			URL   string `json:"url"`
			Hash  string `json:"hash,omitempty"`
			Error string `json:"error,omitempty"`
		}
		items := make([]jsonItem, 0, len(results))
		for _, r := range results {
			item := jsonItem{URL: r.URL}
			if r.Error != nil {
				item.Error = r.Error.Error()
			} else {
				item.Hash = r.Hash
			}
			items = append(items, item)
		}
		json.NewEncoder(os.Stdout).Encode(items)
	} else {
		for _, r := range results {
			if r.Error != nil {
				fmt.Fprintf(os.Stderr, "error: %s: %v\n", r.URL, r.Error)
			} else {
				fmt.Println(r.Hash)
			}
		}
	}
}
