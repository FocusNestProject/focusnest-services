package chatbot

import (
	"strings"
)

const (
	languageEnglish    = "en"
	languageIndonesian = "id"
)

var (
	indonesianMarkers = []string{"aku", "saya", "kamu", "gimana", "bagaimana", "dong", "tolong", "kerja", "belajar", "fokus", "produktif", "jadwal", "semangat", "capek", "istirahat"}
)

func detectLanguage(question string, history []*ChatMessage) string {
	text := strings.ToLower(question)
	for _, utt := range lastUserUtterances(history, 2) {
		text += " " + strings.ToLower(utt)
	}
	for _, marker := range indonesianMarkers {
		if strings.Contains(text, marker) {
			return languageIndonesian
		}
	}
	return languageEnglish
}

func lastUserUtterances(history []*ChatMessage, limit int) []string {
	if limit <= 0 {
		return nil
	}
	var phrases []string
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "user" {
			phrases = append(phrases, history[i].Content)
			if len(phrases) == limit {
				break
			}
		}
	}
	return phrases
}
