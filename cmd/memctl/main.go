package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/salemarsm/llm-memory/memory"
)

func main() {
	addr := flag.String("addr", envDefault("LLM_MEMORY_ADDR", "http://127.0.0.1:8787"), "llm-memory server URL")
	subject := flag.String("subject", "", "memory subject")
	scope := flag.String("scope", "global", "memory scope")
	typ := flag.String("type", "note", "memory type")
	maxTokens := flag.Int("max-tokens", 1200, "context token budget")
	jsonOut := flag.Bool("json", false, "print raw JSON")
	flag.Parse()

	if flag.NArg() < 1 {
		usage()
		os.Exit(2)
	}

	cmd := flag.Arg(0)
	switch cmd {
	case "search":
		query := strings.Join(flag.Args()[1:], " ")
		var out []memory.Memory
		post(*addr+"/api/search", memory.Query{Text: query, Subject: *subject, Scopes: scopes(*scope), Limit: 20}, &out)
		printJSONOrText(*jsonOut, out, func() { printMemories(out) })
	case "context":
		query := strings.Join(flag.Args()[1:], " ")
		var out memory.ContextResponse
		post(*addr+"/api/context", memory.ContextRequest{Query: query, Subject: *subject, Scopes: scopes(*scope), MaxTokens: *maxTokens}, &out)
		printJSONOrText(*jsonOut, out, func() { fmt.Println(out.Context) })
	case "suggest":
		prompt := strings.Join(flag.Args()[1:], " ")
		if prompt == "" {
			b, _ := io.ReadAll(os.Stdin)
			prompt = strings.TrimSpace(string(b))
		}
		var out memory.SuggestResponse
		post(*addr+"/api/suggest", memory.SuggestRequest{Subject: *subject, Scope: memory.Scope(*scope), UserPrompt: prompt, MaxCandidates: 5}, &out)
		printJSONOrText(*jsonOut, out, func() {
			for _, c := range out.Candidates {
				fmt.Printf("[%s %.2f] %s\nreason: %s\n\n", c.Memory.Type, c.Memory.Confidence, c.Memory.Content, c.Reason)
			}
		})
	case "remember":
		content := strings.Join(flag.Args()[1:], " ")
		if content == "" {
			b, _ := io.ReadAll(os.Stdin)
			content = strings.TrimSpace(string(b))
		}
		m := memory.Memory{Type: memory.MemoryType(*typ), Subject: *subject, Content: content, Source: memory.Source{Kind: "memctl", Ref: "cli"}, Scope: memory.Scope(*scope), Confidence: 0.9, Tags: []string{"cli"}, EmbeddingRefs: memory.EmbeddingRefs{}}
		var out memory.Memory
		post(*addr+"/api/memories", m, &out)
		printJSONOrText(*jsonOut, out, func() { fmt.Println(out.ID) })
	case "forget":
		if flag.NArg() < 2 {
			log.Fatal("forget requires id")
		}
		id := flag.Arg(1)
		req, _ := http.NewRequest(http.MethodDelete, strings.TrimRight(*addr, "/")+"/api/memories/"+id, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			b, _ := io.ReadAll(resp.Body)
			log.Fatalf("%s: %s", resp.Status, b)
		}
		fmt.Println(id)
	default:
		usage()
		os.Exit(2)
	}
}

func post(url string, in any, out any) {
	b, _ := json.Marshal(in)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		log.Fatalf("%s: %s", resp.Status, body)
	}
	if err := json.Unmarshal(body, out); err != nil {
		log.Fatal(err)
	}
}

func printJSONOrText(jsonOut bool, v any, text func()) {
	if jsonOut {
		b, _ := json.MarshalIndent(v, "", "  ")
		fmt.Println(string(b))
		return
	}
	text()
}

func printMemories(items []memory.Memory) {
	for _, m := range items {
		fmt.Printf("%s [%s/%s %.2f] %s\n", m.ID, m.Type, m.Scope, m.Confidence, m.Content)
	}
}

func scopes(s string) []memory.Scope {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]memory.Scope, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, memory.Scope(p))
		}
	}
	return out
}

func envDefault(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

func usage() {
	fmt.Fprintln(os.Stderr, `memctl [flags] <command> [args]

Commands:
  search <query>       search raw memories
  context <query>      print token-budgeted context for an agent prompt
  remember <content>   store a memory; reads stdin when content is empty
  suggest <text>       suggest durable memories/learnings from text
  forget <id>          delete a memory

Flags:
  -addr URL            default http://127.0.0.1:8787 or LLM_MEMORY_ADDR
  -subject NAME        restrict/create subject
  -scope LIST          comma scopes, default global
  -type TYPE           memory type for remember, default note
  -max-tokens N        context budget, default 1200
  -json                raw JSON output`)
}
