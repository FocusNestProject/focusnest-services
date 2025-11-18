package chatbot

import (
	"strings"
	"testing"
)

func TestDetectLanguage(t *testing.T) {
	history := []*ChatMessage{{Role: "user", Content: "Bisa bantu saya fokus?"}}
	if got := detectLanguage("Gimana cara fokus kerja?", history); got != languageIndonesian {
		t.Fatalf("expected Indonesian detection, got %s", got)
	}
	if got := detectLanguage("How do I plan tomorrow?", nil); got != languageEnglish {
		t.Fatalf("expected English detection, got %s", got)
	}
}

func TestIsProductivityContext(t *testing.T) {
	history := []*ChatMessage{{Role: "user", Content: "I need help with productivity"}}
	if !isProductivityContext("Any tips?", history) {
		t.Fatalf("expected productivity context to be true when history mentions productivity")
	}
	if isProductivityContext("What's your favorite movie?", nil) {
		t.Fatalf("expected non productivity prompt to be rejected")
	}
}

func TestBuildProductivityResponseBoundary(t *testing.T) {
	resp := buildProductivityResponse("Tell me a joke", nil, languageEnglish)
	if !strings.Contains(resp, "FocusNest") {
		t.Fatalf("expected boundary message to reference FocusNest, got %s", resp)
	}
}

func TestBuildProductivityResponseIndonesian(t *testing.T) {
	history := []*ChatMessage{{Role: "user", Content: "Butuh tips fokus belajar"}}
	resp := buildProductivityResponse("Gimana cara menjaga fokus belajar?", history, languageIndonesian)
	if !strings.Contains(resp, "rencana fokus") {
		t.Fatalf("expected Indonesian response mentioning rencana fokus, got %s", resp)
	}
}
