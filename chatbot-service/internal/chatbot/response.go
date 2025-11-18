package chatbot

import (
	"fmt"
	"strings"
)

const (
	languageEnglish    = "en"
	languageIndonesian = "id"
)

var (
	indonesianMarkers      = []string{"aku", "saya", "kamu", "gimana", "bagaimana", "dong", "tolong", "kerja", "belajar", "fokus", "produktif", "jadwal", "semangat", "capek", "istirahat"}
	productivityKeywordsEN = []string{"focus", "productive", "productivity", "task", "study", "learn", "project", "work", "routine", "habit", "schedule", "plan", "goal", "deadline", "energy", "rest", "burnout", "balance"}
	productivityKeywordsID = []string{"fokus", "produktif", "produktifitas", "tugas", "belajar", "kerja", "pekerjaan", "rutinitas", "kebiasaan", "jadwal", "rencana", "target", "deadline", "energi", "istirahat", "burnout", "seimbang", "semangat"}
)

var topicKeywords = map[string][]string{
	"focus":   {"focus", "fokus", "distract", "distraksi", "concentr", "deep work", "mendalam", "pomodoro"},
	"study":   {"study", "belajar", "ujian", "exam", "kuliah", "homework", "paper"},
	"work":    {"work", "project", "meeting", "kerja", "pekerjaan", "deadline"},
	"routine": {"habit", "rutinitas", "kebiasaan", "schedule", "jadwal", "pagi", "malam", "ritual"},
	"energy":  {"sleep", "tidur", "rest", "istirahat", "capek", "burnout", "stress", "energi"},
}

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

func isProductivityContext(question string, history []*ChatMessage) bool {
	if hasProductivityKeyword(strings.ToLower(question)) {
		return true
	}
	for _, utt := range lastUserUtterances(history, 3) {
		if hasProductivityKeyword(strings.ToLower(utt)) {
			return true
		}
	}
	return false
}

func buildProductivityResponse(question string, history []*ChatMessage, lang string) string {
	if !isProductivityContext(question, history) {
		return boundaryMessage(lang)
	}

	topic := detectTopic(question, history)
	focal := focusPhrase(question)
	tips := suggestionsForTopic(topic, lang, focal)

	if lang == languageIndonesian {
		return formatIndonesianResponse(focal, topic, tips)
	}
	return formatEnglishResponse(focal, topic, tips)
}

func detectTopic(question string, history []*ChatMessage) string {
	combined := strings.ToLower(question)
	for _, utt := range lastUserUtterances(history, 2) {
		combined += " " + strings.ToLower(utt)
	}
	bestTopic := "general"
	bestScore := 0
	for topic, keywords := range topicKeywords {
		score := keywordScore(combined, keywords)
		if score > bestScore {
			bestTopic = topic
			bestScore = score
		}
	}
	return bestTopic
}

func boundaryMessage(lang string) string {
	if lang == languageIndonesian {
		return "Aku pendamping FocusNest untuk hal-hal seputar fokus, produktivitas, dan kebiasaan sehat. Kita bisa ngobrol soal perencanaan, deep work, istirahat, atau motivasi—di luar itu belum bisa kubantu ya."
	}
	return "I'm your FocusNest guide for focus, healthy routines, and productivity. Let's stay on planning, deep work, rest, or motivation—anything outside that scope is out of bounds."
}

func suggestionsForTopic(topic, lang, focal string) []string {
	if lang == languageIndonesian {
		return suggestionsID(topic, focal)
	}
	return suggestionsEN(topic, focal)
}

func suggestionsEN(topic, focal string) []string {
	switch topic {
	case "focus":
		return []string{
			fmt.Sprintf("Silence notifications and run a 25-minute deep-work block just for \"%s\".", focal),
			"Write down the biggest distraction and decide how you'll block it before you start.",
			"End the block with a 3-minute checkout: breathe, log what worked, and reset.",
		}
	case "study":
		return []string{
			fmt.Sprintf("Split \"%s\" into three tiny checkpoints and tick them off as you go.", focal),
			"Teach the concept back out loud or in a short note to lock it in.",
			"Schedule a quick recovery break after 2 study rounds to avoid cramming fatigue.",
		}
	case "work":
		return []string{
			fmt.Sprintf("Clarify what 'done' means for \"%s\" before you touch your tools.", focal),
			"Batch similar tasks so meetings/communication don't fragment your flow.",
			"End the day by writing a two-line brief for tomorrow so you restart fast.",
		}
	case "routine":
		return []string{
			"Anchor the habit to something you already do (coffee, commute, evening shutdown).",
			fmt.Sprintf("Make \"%s\" visible—sticky note, calendar block, or reminder on your desk.", focal),
			"Celebrate tiny wins weekly so the routine feels rewarding, not endless.",
		}
	case "energy":
		return []string{
			"Protect a 7-8 hour sleep window and keep wake-up times consistent.",
			"Use a 5-minute mobility or breathing reset when your body says it's tired.",
			"Stack deep work after your highest-energy point and push admin to low-energy slots.",
		}
	default:
		return []string{
			fmt.Sprintf("Define the very next action for \"%s\" and put it on today's list.", focal),
			"Rate your energy from 1-5 and use that to decide the length of your next block.",
			"Close every block with a one-line journal so progress stays visible.",
		}
	}
}

func suggestionsID(topic, focal string) []string {
	switch topic {
	case "focus":
		return []string{
			fmt.Sprintf("Matikan notif dan jalankan sesi 25 menit khusus untuk \"%s\".", focal),
			"Catat distraksi terbesar lalu tentukan cara mengamankannya sebelum mulai.",
			"Tutup sesi dengan jeda 3 menit: tarik napas, evaluasi singkat, lalu lanjut.",
		}
	case "study":
		return []string{
			fmt.Sprintf("Bagi \"%s\" jadi tiga langkah kecil dan centang tiap selesai.", focal),
			"Coba jelaskan materi dengan kata-katamu sendiri agar lebih nempel.",
			"Sisihkan jeda pemulihan singkat tiap dua sesi belajar supaya otak nggak penuh.",
		}
	case "work":
		return []string{
			fmt.Sprintf("Tentukan arti 'selesai' untuk \"%s\" sebelum eksekusi.", focal),
			"Kelompokkan tugas sejenis supaya meeting atau chat nggak pecah fokus.",
			"Tutup hari dengan dua kalimat rencana besok biar start-nya cepat.",
		}
	case "routine":
		return []string{
			"Tempelkan kebiasaan baru ke rutinitas lama—misal setelah kopi pagi atau sebelum tidur.",
			fmt.Sprintf("Bikin \"%s\" terlihat: sticky note, blok kalender, atau pengingat di meja.", focal),
			"Rayakan kemenangan kecil tiap minggu supaya rutinitas terasa menyenangkan.",
		}
	case "energy":
		return []string{
			"Jaga jam tidur-bangun tetap konsisten minimal 7 jam per malam.",
			"Gunakan peregangan atau napas 5 menit saat tubuh mulai protes.",
			"Letakkan kerja mendalam di jam energi puncak, administrasi di jam rendah.",
		}
	default:
		return []string{
			fmt.Sprintf("Tentukan aksi paling dekat untuk \"%s\" dan kerjakan hari ini juga.", focal),
			"Nilai energimu skala 1-5 lalu sesuaikan durasi sesi fokusnya.",
			"Tutup sesi dengan catatan satu kalimat supaya progres tetap terlihat.",
		}
	}
}

func formatEnglishResponse(focal, topic string, tips []string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Here’s a productivity check-in for \"%s\" (%s).\n\n", focal, topicDisplayName(topic, languageEnglish)))
	for i, tip := range tips {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, tip))
	}
	b.WriteString("\nTell me what sticks and we'll tweak the next block together.")
	return b.String()
}

func formatIndonesianResponse(focal, topic string, tips []string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Ini rencana fokus untuk \"%s\" (%s).\n\n", focal, topicDisplayName(topic, languageIndonesian)))
	for i, tip := range tips {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, tip))
	}
	b.WriteString("\nKasih tahu apa yang berhasil, nanti kita atur ulang sesi berikutnya.")
	return b.String()
}

func topicDisplayName(topic, lang string) string {
	switch topic {
	case "focus":
		if lang == languageIndonesian {
			return "Fokus Mendalam"
		}
		return "Deep Focus"
	case "study":
		if lang == languageIndonesian {
			return "Belajar"
		}
		return "Study"
	case "work":
		if lang == languageIndonesian {
			return "Pekerjaan"
		}
		return "Work"
	case "routine":
		if lang == languageIndonesian {
			return "Rutinitas"
		}
		return "Routines"
	case "energy":
		if lang == languageIndonesian {
			return "Energi & Pemulihan"
		}
		return "Energy & Recovery"
	default:
		if lang == languageIndonesian {
			return "Produktivitas Umum"
		}
		return "General Productivity"
	}
}

func hasProductivityKeyword(text string) bool {
	return containsKeyword(text, productivityKeywordsEN) || containsKeyword(text, productivityKeywordsID)
}

func containsKeyword(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func keywordScore(text string, keywords []string) int {
	score := 0
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			score++
		}
	}
	return score
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

func focusPhrase(text string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	if trimmed == "" {
		return "your next task"
	}
	runes := []rune(trimmed)
	if len(runes) > 80 {
		return string(runes[:80]) + "…"
	}
	return trimmed
}
