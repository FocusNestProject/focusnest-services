package chatbot

import (
	"strings"
)

const (
	languageEnglish    = "en"
	languageIndonesian = "id"
)

var (
	indonesianMarkers = []string{
		// Pronouns & address
		"aku", "saya", "kamu", "gue", "gw", "lo", "elo", "kalian", "mereka",
		// Common verbs
		"gimana", "bagaimana", "kenapa", "mengapa", "kapan", "dimana", "siapa",
		"mau", "bisa", "boleh", "harus", "perlu", "coba", "tolong", "bantu",
		"kerja", "belajar", "fokus", "istirahat", "tidur", "mulai", "selesai",
		"bikin", "buat", "lakukan", "lakuin", "kasih", "ngasih", "ambil",
		// Productivity & app-specific
		"produktif", "produktivitas", "jadwal", "target", "kebiasaan", "rutinitas",
		"semangat", "capek", "lelah", "stress", "stres", "motivasi", "goals",
		"pomodoro", "timer", "sesi", "streak", "progres", "progress",
		// Particles & discourse markers
		"dong", "sih", "deh", "nih", "lah", "kan", "ya", "yuk", "ayo",
		"juga", "kalau", "kalo", "tapi", "tapi", "terus", "habis", "udah",
		"sudah", "belum", "lagi", "aja", "saja", "banget", "sangat", "sekali",
		// Common nouns
		"hari", "minggu", "bulan", "waktu", "pagi", "siang", "malam", "besok",
		"kemarin", "sekarang", "nanti", "tugas", "pekerjaan", "kuliah", "sekolah",
	}
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
