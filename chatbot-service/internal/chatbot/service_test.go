package chatbot

import "testing"

func TestDeriveChatTitle(t *testing.T) {
	tests := []struct {
		prompt     string
		wantPrefix string
	}{
		{"Need focus for math exam", "Need Focus For"},
		{"", "Focus Chat"},
	}
	for _, tt := range tests {
		got := deriveChatTitle(tt.prompt)
		if tt.prompt == "" {
			if got[:10] != tt.wantPrefix {
				t.Fatalf("expected fallback title to start with %q, got %s", tt.wantPrefix, got)
			}
			continue
		}
		if got[:len(tt.wantPrefix)] != tt.wantPrefix {
			t.Fatalf("unexpected title: %s", got)
		}
	}
}
