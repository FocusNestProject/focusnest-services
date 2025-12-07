package chatbot

import (
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
