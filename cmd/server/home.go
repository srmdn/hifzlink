package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/srmdn/hifzlink/internal/search"
)

type continuationView struct {
	Prompt        search.Ayah
	Answer        search.Ayah
	PromptRef     string
	AnswerRef     string
	CompareURL    string
	TranslationEN string
	TranslationID string
}

func (s *server) continuation(surah int, lang string) *continuationView {
	ayah := 4
	if surah == 102 {
		ayah = 3
	}
	prompt, ok := s.quran.Get(surah, ayah)
	if !ok {
		return nil
	}
	answer, ok := s.quran.Get(surah, ayah+1)
	if !ok {
		return nil
	}
	return &continuationView{
		Prompt: prompt, Answer: answer,
		PromptRef: fmt.Sprintf("%d:%d", surah, ayah), AnswerRef: fmt.Sprintf("%d:%d", surah, ayah+1),
		CompareURL:    withLang("/compare?ayah1=78:5&ayah2=102:4", lang),
		TranslationEN: s.translationFor("en", surah, ayah+1), TranslationID: s.translationFor("id", surah, ayah+1),
	}
}

func (s *server) learningData(r *http.Request, data map[string]any) map[string]any {
	data = s.withCommonViewData(r, data)
	data["LearningUI"] = true
	data["LearningSection"] = ""
	switch {
	case strings.HasPrefix(r.URL.Path, "/dashboard"):
		data["LearningSection"] = "dashboard"
	case strings.HasPrefix(r.URL.Path, "/practice"):
		data["LearningSection"] = "practice"
	case strings.HasPrefix(r.URL.Path, "/surah"):
		data["LearningSection"] = "surah"
	case strings.HasPrefix(r.URL.Path, "/ayah/"):
		data["LearningSection"] = "ayah"
	case r.URL.Path == "/privacy", r.URL.Path == "/terms":
		data["LearningSection"] = "legal"
	case r.URL.Path == "/methodology":
		data["LearningSection"] = "legal"
	case strings.HasPrefix(r.URL.Path, "/juz"):
		data["LearningSection"] = "juz"
	case strings.HasPrefix(r.URL.Path, "/search"), strings.HasPrefix(r.URL.Path, "/compare"):
		data["LearningSection"] = "search"
		if strings.HasPrefix(r.URL.Path, "/compare") {
			data["LearningSection"] = "compare"
		}
	}
	data["UI"] = homeCopy(pageLang(r))
	data["PracticeURL"] = withLang("/practice", pageLang(r))
	data["WeeklyURL"] = withLang("/practice?surah=78", pageLang(r))
	data["OpeningURL"] = withLang("/compare?ayah1=78:4&ayah2=102:3", pageLang(r))
	data["ContinuationURL"] = withLang("/compare?ayah1=78:5&ayah2=102:4", pageLang(r))
	data["MethodologyURL"] = withLang("/methodology", pageLang(r))
	return data
}

type featuredPairView struct {
	Title, Ref1, Ref2, URL string
	Ayah1, Ayah2           search.Ayah
}

// Editorial selections always use the canonical verse text.
func (s *server) featuredPairs(lang string) []featuredPairView {
	copy := homeCopy(lang)
	var pairs []featuredPairView
	for i, title := range []string{copy["opening"], copy["continuations"]} {
		left, leftOK := s.quran.Get(78, 4+i)
		right, rightOK := s.quran.Get(102, 3+i)
		if !leftOK || !rightOK {
			continue
		}
		ref1, ref2 := fmt.Sprintf("78:%d", 4+i), fmt.Sprintf("102:%d", 3+i)
		pairs = append(pairs, featuredPairView{Title: title, Ref1: ref1, Ref2: ref2, Ayah1: left, Ayah2: right,
			URL: withLang("/compare?ayah1="+ref1+"&ayah2="+ref2, lang)})
	}
	return pairs
}

func (s *server) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		copy := homeCopy(pageLang(r))
		s.renderNotFound(w, r, copy["notFoundHeading"], copy["notFoundMessage"])
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	title := "hifzlink: Quran mutashabihat review"
	description := "Practice verse continuations and learn to tell similar Quran ayahs apart on HifzLink."
	if pageLang(r) == "id" {
		title = "hifzlink: latihan ayat mutasyabihat"
		description = "Latih lanjutan ayat dan pelajari perbedaan ayat Quran yang mirip di HifzLink."
	}
	relationCount, countErr := s.db.CountRelations()
	if countErr != nil {
		log.Printf("count relations for homepage: %v", countErr)
	}
	s.render(w, "home.html", s.learningData(r, map[string]any{
		"Title":            title,
		"Description":      description,
		"Continuation":     s.continuation(78, pageLang(r)),
		"FeaturedPairs":    s.featuredPairs(pageLang(r)),
		"RelationCount":    relationCount,
		"HasRelationCount": countErr == nil,
		"JSONLD":           websiteJSONLD("https://" + r.Host),
	}))
}

func (s *server) handleMethodology(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	lang := pageLang(r)
	title := "Sources & methodology — hifzlink"
	description := "How HifzLink defines, groups, sources, and reviews similar Quran verse pairs."
	if lang == "id" {
		title = "Sumber & metodologi — hifzlink"
		description = "Definisi, pengelompokan, sumber, dan proses kurasi pasangan ayat di HifzLink."
	}
	relationCount, countErr := s.db.CountRelations()
	if countErr != nil {
		log.Printf("count relations for methodology page: %v", countErr)
	}
	s.render(w, "methodology.html", s.learningData(r, map[string]any{
		"Title":            title,
		"Description":      description,
		"RelationCount":    relationCount,
		"HasRelationCount": countErr == nil,
	}))
}

func (s *server) handlePractice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data := map[string]any{"Title": "Practice — hifzlink"}
	if value := r.URL.Query().Get("surah"); value != "" {
		var surah int
		switch value {
		case "78":
			surah = 78
		case "102":
			surah = 102
		default:
			http.Error(w, "unsupported practice passage", http.StatusBadRequest)
			return
		}
		view := s.continuation(surah, pageLang(r))
		if view == nil {
			http.NotFound(w, r)
			return
		}
		data["Continuation"] = view
	}
	s.render(w, "practice.html", s.learningData(r, data))
}

func homeCopy(lang string) map[string]string {
	if lang == "id" {
		return map[string]string{
			"practice": "Latihan", "browse": "Baca per surah", "similar": "Ayat mirip", "language": "Bahasa antarmuka", "menuOpen": "Buka menu", "menuClose": "Tutup menu", "methodology": "Sumber & metodologi",
			"notFoundKicker": "KESALAHAN 404", "notFoundHeading": "Halaman tidak ditemukan", "notFoundMessage": "Alamat ini tidak tersedia. Periksa tautannya atau cari ayat lain.", "searchAnother": "Cari ayat lain", "privacy": "Privasi", "terms": "Ketentuan",
			"storyLabel": "Kenapa gue membuat ini", "storyQuote": "Saat murojaah, kadang pikiranku berpindah ke ayat lain yang terdengar mirip. Aku ingin melihat hubungan itu dengan jelas, lalu segera ingat ayat mana yang sedang kubaca dan di mana lafaznya berbeda.", "storyBody": "Mutasyabihat adalah ayat-ayat dengan lafaz mirip yang mudah tertukar saat dihafal. HifzLink membantu menampilkan persamaan dan perbedaannya agar murojaah bisa lebih terarah.",
			"eyebrow": "Hafalan Quran · Mutasyabihat", "titleFirst": "Ayat yang familiar.", "titleSecond": "Apa lanjutannya?",
			"lead": "Ingat ayat berikutnya, lalu bandingkan dengan bagian yang sering tertukar dalam hafalanmu.", "start": "Mulai latihan", "noAccount": "Bisa langsung dicoba. Tanpa akun.",
			"try": "Coba sambung ayat", "continue": "Lanjutkan ayat berikutnya.", "recall": "Ucapkan dari hafalan sebelum membuka jawaban.", "reveal": "Tampilkan", "hide": "Sembunyikan",
			"fullAnswer": "Ayat lengkap", "translation": "Terjemahan", "similarHint": "Ada ayat lain dengan lafaz yang mirip.", "compare": "Lihat perbedaannya", "sample": "Contoh latihan",
			"choose": "Mulai dari bagian hafalanmu.", "bySurah": "Per surah", "byJuz": "Per juz", "chooseSub": "Pilih bagian yang sedang kamu hafal.", "yourPractice": "Latihan pilihanmu", "practiceSub": "Coba sambung ayat, lalu periksa lafaz yang sering tertukar.",
			"read": "Jelajahi per surah", "readSub": "Temukan ayat dan pasangan yang mirip dalam bagian hafalanmu.", "weekly": "Latihan Minggu Ini", "editor": "Pilihan editor",
			"weeklyName": "Jaga lanjutan ayat tetap pada surahnya.", "weeklyDescription": "Dua bagian pendek yang mudah tertukar. Baca berdampingan dan perhatikan lafaz yang berbeda.", "weeklyStart": "Coba latihan ini",
			"featured": "Pasangan Pilihan", "featuredSub": "Beberapa tempat untuk memulai", "opening": "Awal ayat yang mirip", "openingSub": "Perhatikan lafaz setelah كَلَّا.",
			"continuations": "Lanjutan yang mirip", "continuationsSub": "Baca kedua ayat sebelum menguji hafalan.", "explore": "Pelajari pasangan", "footer": "Memahami perbedaan. Menguatkan hafalan.", "free": "Gratis digunakan", "source": "Lihat kode sumber", "navHome": "Beranda", "surahTitle": "Pilih surah", "surahDescription": "Cari nama atau nomor, lalu buka daftar ayat mirip dalam surah itu.", "surahFilter": "Cari surah", "surahPlaceholder": "Nama atau nomor surah", "surahs": "surah", "shown": "ditampilkan", "withPairs": "dengan pasangan ayat", "pairs": "pasangan", "noPairs": "Belum ada pasangan", "noSurahMatch": "Tidak ada surah yang cocok.", "trySurah": "Coba", "juzTitle": "Pilih juz", "juzDescription": "Buka satu juz untuk menelusuri pasangan ayat di dalamnya.", "juz": "Juz", "searchTitle": "Cari ayat atau surah", "searchDescription": "Masukkan nomor ayat, nomor surah, atau nama surah untuk menemukan pasangan ayat yang mirip.", "searchInput": "Referensi ayat atau surah", "searchPlaceholder": "Contoh: 60:8, 60, atau Al-Mumtahanah", "searchSubmit": "Cari", "searchExamples": "Coba pencarian ini", "searchResults": "hasil untuk", "searchHint": "Klik satu nomor ayat untuk memilih koneksi lainnya dan membandingkannya bersama dalam satu halaman.", "searchEmpty": "Tidak ada pasangan untuk pencarian ini.", "searchEmptyHint": "Periksa kembali ejaan atau coba nomor ayat seperti 60:8.", "all": "Semua", "ayahs": "Ayat", "revelation": "Turun di", "surahNavigation": "Navigasi surah", "similarPairs": "Pasangan ayat mirip", "pairHint": "Klik satu nomor ayat untuk memilih koneksi lainnya dan membandingkannya bersama dalam satu halaman.", "backToSurahs": "Semua surah", "backToJuz": "Semua juz", "beginsAt": "Dimulai dari", "noSurahPairs": "Belum ada pasangan ayat di surah ini.", "noSurahPairsHint": "Daftar pasangan akan bertambah seiring kurasi data.", "noJuzPairs": "Belum ada pasangan ayat di juz ini.", "noJuzPairsHint": "Coba buka surah atau cari pasangan dari halaman lain.", "compareBack": "Kembali ke ayat mirip", "compareHeading": "Apa yang membedakan ayat ini?", "compareLead": "Bandingkan lafaz kedua ayat. Kata yang berbeda ditandai dengan sorotan.", "verseOne": "Ayat pertama", "verseTwo": "Ayat kedua", "highlightLegend": "Bagian berwarna menunjukkan lafaz yang berbeda.", "showHighlights": "Tampilkan perbedaan", "hideHighlights": "Sembunyikan perbedaan", "sharePage": "Bagikan halaman", "sharePageText": "Jelajahi HifzLink dan bagikan halaman ini.", "sharePageCopied": "Tautan halaman tersalin.", "sharePair": "Bagikan pasangan", "shareCopied": "Tautan pasangan tersalin.", "shareFailed": "Tautan belum bisa disalin. Salin dari bilah alamat browser.", "shareText": "Bandingkan dua ayat Quran di HifzLink.", "startPractice": "Coba dari hafalan", "practiceControls": "Kontrol latihan", "switchSide": "Ganti ayat yang disembunyikan", "nextPair": "Pasangan berikutnya", "exitPractice": "Akhiri latihan", "relatedPairs": "Pasangan terkait", "lastUpdated": "Diperbarui", "relationNote": "Mengapa ayat ini mirip",
			"back": "Kembali", "setup": "Bagian mana yang ingin kamu latih?", "setupLead": "Mulai dengan salah satu dari dua contoh ini.", "passage": "Bagian hafalan", "mode": "Cara latihan", "textMode": "Sambung ayat dari teks",
			"practiceHeading": "Lanjutkan", "rating": "Bagaimana hafalanmu tadi?", "remembered": "Ingat", "hesitated": "Ragu", "mixed": "Tertukar", "ratingStatus": "Penilaianmu", "again": "Coba lagi", "change": "Ganti bagian",
		}
	}
	return map[string]string{
		"practice": "Practice", "browse": "Browse Quran", "similar": "Similar verses", "language": "Interface language", "menuOpen": "Open menu", "menuClose": "Close menu", "methodology": "Sources & methodology",
		"notFoundKicker": "ERROR 404", "notFoundHeading": "Page not found", "notFoundMessage": "This address isn't available. Check the link or search for another ayah.", "searchAnother": "Search another ayah", "privacy": "Privacy", "terms": "Terms",
		"storyLabel": "Why I built this", "storyQuote": "When I'm reciting and my mind slips toward a similar ayah, I want to see that connection clearly. The next time I meet it, I can remember which verse sounds familiar and exactly where the wording changes.", "storyBody": "Mutashabihat are verses with similar wording that can be easy to mix up during memorization. HifzLink makes their shared words and differences easier to review.",
		"eyebrow": "Quran memorization · Mutashabihat", "titleFirst": "A familiar ayah.", "titleSecond": "What follows?",
		"lead": "Recall the next ayah, then compare it with the passage you might confuse it with.", "start": "Start practicing", "noAccount": "Free to practise. No account needed.",
		"try": "Try a continuation", "continue": "Continue with the next verse.", "recall": "Recite from memory before revealing the answer.", "reveal": "Reveal", "hide": "Hide",
		"fullAnswer": "Complete verse", "translation": "Translation", "similarHint": "Another verse has similar wording.", "compare": "See the difference", "sample": "Sample practice",
		"choose": "Start from your own reading.", "bySurah": "By surah", "byJuz": "By juz", "chooseSub": "Choose the section you are memorizing.", "yourPractice": "Your practice", "practiceSub": "Try verse continuations, then check the wording you mix up.",
		"read": "Browse by surah", "readSub": "Find ayahs and similar pairs in the section you are memorizing.", "weekly": "This Week’s Practice", "editor": "Editor’s pick",
		"weeklyName": "Keep the continuation in the right surah.", "weeklyDescription": "These short passages are easy to mix up. Read them together and notice where the wording changes.", "weeklyStart": "Try this practice",
		"featured": "Featured Pairs", "featuredSub": "A few places to start", "opening": "Similar openings", "openingSub": "Notice the wording after كَلَّا.",
		"continuations": "Similar continuations", "continuationsSub": "Read both verses before testing your recall.", "explore": "Explore pair", "footer": "Understand the difference. Strengthen your recall.", "free": "Free to use", "source": "View source code", "navHome": "Home", "surahTitle": "Choose a surah", "surahDescription": "Search by name or number, then open the similar verses in that surah.", "surahFilter": "Find a surah", "surahPlaceholder": "Surah name or number", "surahs": "surahs", "shown": "shown", "withPairs": "with similar pairs", "pairs": "pairs", "noPairs": "No pairs yet", "trySurah": "Try", "juzTitle": "Choose a juz", "juzDescription": "Open a juz to browse the similar verse pairs in it.", "juz": "Juz", "searchTitle": "Find an ayah or surah", "searchDescription": "Enter an ayah reference, surah number, or surah name to find similar verses.", "searchInput": "Ayah reference or surah", "searchPlaceholder": "For example: 60:8, 60, or Al-Mumtahanah", "searchSubmit": "Search", "searchExamples": "Try a search", "searchResults": "results for", "searchHint": "Click one ayah reference to choose its other connections and compare them together on one page.", "searchEmpty": "No pairs match this search.", "searchEmptyHint": "Check the spelling or try an ayah reference such as 60:8.", "all": "All", "ayahs": "Ayahs", "revelation": "Revelation", "surahNavigation": "Surah navigation", "similarPairs": "Similar verse pairs", "pairHint": "Click one ayah reference to choose its other connections and compare them together on one page.", "backToSurahs": "All surahs", "backToJuz": "All juz", "beginsAt": "Begins at", "noSurahMatch": "No surahs match your search.", "noSurahPairs": "No similar verse pairs in this surah yet.", "noSurahPairsHint": "More pairs will appear as the dataset is curated.", "noJuzPairs": "No similar verse pairs in this juz yet.", "noJuzPairsHint": "Browse a surah or search for pairs from another section.", "compareBack": "Back to similar verses", "compareHeading": "What makes these verses different?", "compareLead": "Compare the wording of both verses. Highlighted words show the differences.", "verseOne": "First verse", "verseTwo": "Second verse", "highlightLegend": "Highlighted wording marks the differences.", "showHighlights": "Show differences", "hideHighlights": "Hide differences", "sharePage": "Share page", "sharePageText": "Explore HifzLink and share this page.", "sharePageCopied": "Page link copied.", "sharePair": "Share pair", "shareCopied": "Pair link copied.", "shareFailed": "Could not copy the link. Copy it from your browser address bar.", "shareText": "Compare these Quran verses on HifzLink.", "startPractice": "Recall from memory", "practiceControls": "Practice controls", "switchSide": "Hide the other verse", "nextPair": "Next pair", "exitPractice": "Exit practice", "relatedPairs": "Related pairs", "lastUpdated": "Updated", "relationNote": "Why these verses are similar",
		"back": "Back", "setup": "What would you like to practice?", "setupLead": "Start with one of these two sample passages.", "passage": "Your passage", "mode": "Practice mode", "textMode": "Continue from a written verse",
		"practiceHeading": "Continue", "rating": "How did you do?", "remembered": "Remembered", "hesitated": "Hesitated", "mixed": "Mixed up", "ratingStatus": "Your self-rating", "again": "Try again", "change": "Change passage",
	}
}
