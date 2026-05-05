package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/salemarsm/llm-memory/memory"
)

func doIngest(args []string) {
	fs := flag.NewFlagSet("ingest", flag.ExitOnError)
	recursive := fs.Bool("recursive", true, "ingest directories recursively")
	jsonOut := fs.Bool("json", false, "output JSON")
	noRecursive := fs.Bool("no-recursive", false, "do not recurse into subdirectories")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: ginko ingest <path> [--no-recursive] [--json]")
		os.Exit(2)
	}
	if *noRecursive {
		*recursive = false
	}
	path := fs.Arg(0)

	db := ginkoDBPath()
	store, err := memory.Open(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ginko ingest: open db %s: %v\n", db, err)
		os.Exit(1)
	}
	defer store.Close()

	resp, err := store.IngestPath(context.Background(), memory.IngestRequest{
		Path:      path,
		Recursive: *recursive,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "ginko ingest: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(resp)
		return
	}

	fmt.Printf("run:       %s\n", resp.Run.ID)
	fmt.Printf("status:    %s\n", resp.Run.Status)
	fmt.Printf("parser:    %s\n", resp.Run.Parser)
	fmt.Printf("documents: %d\n", len(resp.Documents))
	fmt.Printf("chunks:    %d\n", len(resp.Chunks))
	if len(resp.Skipped) > 0 {
		fmt.Printf("skipped:   %d\n", len(resp.Skipped))
		for _, s := range resp.Skipped {
			fmt.Printf("  - %s\n", s)
		}
	}
	if resp.Run.Error != "" {
		fmt.Fprintf(os.Stderr, "error: %s\n", resp.Run.Error)
		os.Exit(1)
	}
}
