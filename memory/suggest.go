package memory

import (
	"context"
	"regexp"
	"strings"
)

type SuggestRequest struct {
	Subject           string  `json:"subject"`
	Scope             Scope   `json:"scope"`
	UserPrompt        string  `json:"user_prompt"`
	AssistantResponse string  `json:"assistant_response"`
	LLMInference      string  `json:"llm_inference"`
	MaxCandidates     int     `json:"max_candidates"`
	MinConfidence     float64 `json:"min_confidence"`
}

type MemoryCandidate struct {
	Memory               Memory `json:"memory"`
	Reason               string `json:"reason"`
	RequiresConfirmation bool   `json:"requires_confirmation"`
}

type SuggestResponse struct {
	Candidates []MemoryCandidate `json:"candidates"`
}

func (s *Store) SuggestMemories(ctx context.Context, req SuggestRequest) (SuggestResponse, error) {
	limit := req.MaxCandidates
	if limit <= 0 || limit > 10 {
		limit = 5
	}
	min := req.MinConfidence
	if min <= 0 {
		min = 0.65
	}
	subject := strings.TrimSpace(req.Subject)
	if subject == "" {
		subject = "user"
	}
	scope := req.Scope
	if scope == "" {
		scope = ScopeGlobal
	}

	texts := []struct {
		kind string
		text string
	}{
		{"prompt", req.UserPrompt},
		{"assistant", req.AssistantResponse},
		{"llm_inference", req.LLMInference},
	}

	seen := map[string]bool{}
	var out []MemoryCandidate
	for _, item := range texts {
		for _, c := range suggestFromText(subject, scope, item.kind, item.text) {
			if c.Memory.Confidence < min {
				continue
			}
			key := strings.ToLower(string(c.Memory.Type) + "|" + c.Memory.Subject + "|" + c.Memory.Content)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, c)
			if len(out) >= limit {
				return SuggestResponse{Candidates: out}, nil
			}
		}
	}
	return SuggestResponse{Candidates: out}, nil
}

func suggestFromText(subject string, scope Scope, sourceKind string, text string) []MemoryCandidate {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	lower := strings.ToLower(text)
	var out []MemoryCandidate
	add := func(typ MemoryType, content, reason string, conf float64) {
		content = compactWhitespace(content)
		if len([]rune(content)) < 12 {
			return
		}
		out = append(out, MemoryCandidate{
			Memory: Memory{Type: typ, Subject: subject, Content: content, Source: Source{Kind: "suggestion", Ref: sourceKind}, Scope: scope, Confidence: conf, Tags: []string{"suggested"}, EmbeddingRefs: EmbeddingRefs{}},
			Reason: reason, RequiresConfirmation: true,
		})
	}

	for _, sentence := range splitSentences(text) {
		l := strings.ToLower(sentence)
		switch {
		case containsAny(l, "prefiro", "eu prefiro", "gosto de", "não gosto", "nao gosto", "quero que", "responda", "me chame", "call me", "i prefer", "i like", "i don't like"):
			add(TypePreference, sentence, "explicit preference/style instruction", 0.9)
		case containsAny(l, "decidimos", "decidi", "decisão", "decisao", "vamos usar", "fica definido", "decision", "we decided", "use "):
			add(TypeDecision, sentence, "project decision or implementation choice", 0.82)
		case containsAny(l, "preciso", "todo", "pendente", "lembrar de", "fazer depois", "next step", "need to", "follow up"):
			add(TypeTask, sentence, "possible durable task or follow-up", 0.74)
		case strings.Contains(l, " é ") || strings.Contains(l, " eh ") || strings.Contains(l, " is "):
			if containsAny(l, "projeto", "project", "usuário", "usuario", "user", "sistema", "system") {
				add(TypeFact, sentence, "possible stable fact", 0.68)
			}
		}
	}

	if strings.TrimSpace(sourceKind) == "llm_inference" && containsAny(lower, "should remember", "memory candidate", "aprendizado", "guardar", "remember that") {
		add(TypeNote, text, "LLM-provided memory inference", 0.76)
	}
	return out
}

var sentenceSplit = regexp.MustCompile(`[.!?\n]+`)

func splitSentences(s string) []string {
	parts := sentenceSplit.Split(s, -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func containsAny(s string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}
