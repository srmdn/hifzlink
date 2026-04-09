package search

import "strings"

var surahNames = [...]string{
	"",
	"Al-Fatihah",
	"Al-Baqarah",
	"Ali 'Imran",
	"An-Nisa",
	"Al-Ma'idah",
	"Al-An'am",
	"Al-A'raf",
	"Al-Anfal",
	"At-Tawbah",
	"Yunus",
	"Hud",
	"Yusuf",
	"Ar-Ra'd",
	"Ibrahim",
	"Al-Hijr",
	"An-Nahl",
	"Al-Isra",
	"Al-Kahf",
	"Maryam",
	"Ta-Ha",
	"Al-Anbiya",
	"Al-Hajj",
	"Al-Mu'minun",
	"An-Nur",
	"Al-Furqan",
	"Ash-Shu'ara",
	"An-Naml",
	"Al-Qasas",
	"Al-'Ankabut",
	"Ar-Rum",
	"Luqman",
	"As-Sajdah",
	"Al-Ahzab",
	"Saba",
	"Fatir",
	"Ya-Sin",
	"As-Saffat",
	"Sad",
	"Az-Zumar",
	"Ghafir",
	"Fussilat",
	"Ash-Shura",
	"Az-Zukhruf",
	"Ad-Dukhan",
	"Al-Jathiyah",
	"Al-Ahqaf",
	"Muhammad",
	"Al-Fath",
	"Al-Hujurat",
	"Qaf",
	"Adh-Dhariyat",
	"At-Tur",
	"An-Najm",
	"Al-Qamar",
	"Ar-Rahman",
	"Al-Waqi'ah",
	"Al-Hadid",
	"Al-Mujadilah",
	"Al-Hashr",
	"Al-Mumtahanah",
	"As-Saff",
	"Al-Jumu'ah",
	"Al-Munafiqun",
	"At-Taghabun",
	"At-Talaq",
	"At-Tahrim",
	"Al-Mulk",
	"Al-Qalam",
	"Al-Haqqah",
	"Al-Ma'arij",
	"Nuh",
	"Al-Jinn",
	"Al-Muzzammil",
	"Al-Muddaththir",
	"Al-Qiyamah",
	"Al-Insan",
	"Al-Mursalat",
	"An-Naba",
	"An-Nazi'at",
	"'Abasa",
	"At-Takwir",
	"Al-Infitar",
	"Al-Mutaffifin",
	"Al-Inshiqaq",
	"Al-Buruj",
	"At-Tariq",
	"Al-A'la",
	"Al-Ghashiyah",
	"Al-Fajr",
	"Al-Balad",
	"Ash-Shams",
	"Al-Layl",
	"Ad-Duha",
	"Ash-Sharh",
	"At-Tin",
	"Al-'Alaq",
	"Al-Qadr",
	"Al-Bayyinah",
	"Az-Zalzalah",
	"Al-'Adiyat",
	"Al-Qari'ah",
	"At-Takathur",
	"Al-'Asr",
	"Al-Humazah",
	"Al-Fil",
	"Quraysh",
	"Al-Ma'un",
	"Al-Kawthar",
	"Al-Kafirun",
	"An-Nasr",
	"Al-Masad",
	"Al-Ikhlas",
	"Al-Falaq",
	"An-Nas",
}

// surahArabicNames holds the Arabic script name for each surah (index = surah number).
var surahArabicNames = [...]string{
	"",
	"الفاتحة", "البقرة", "آل عمران", "النساء", "المائدة",
	"الأنعام", "الأعراف", "الأنفال", "التوبة", "يونس",
	"هود", "يوسف", "الرعد", "إبراهيم", "الحجر",
	"النحل", "الإسراء", "الكهف", "مريم", "طه",
	"الأنبياء", "الحج", "المؤمنون", "النور", "الفرقان",
	"الشعراء", "النمل", "القصص", "العنكبوت", "الروم",
	"لقمان", "السجدة", "الأحزاب", "سبأ", "فاطر",
	"يس", "الصافات", "ص", "الزمر", "غافر",
	"فصلت", "الشورى", "الزخرف", "الدخان", "الجاثية",
	"الأحقاف", "محمد", "الفتح", "الحجرات", "ق",
	"الذاريات", "الطور", "النجم", "القمر", "الرحمن",
	"الواقعة", "الحديد", "المجادلة", "الحشر", "الممتحنة",
	"الصف", "الجمعة", "المنافقون", "التغابن", "الطلاق",
	"التحريم", "الملك", "القلم", "الحاقة", "المعارج",
	"نوح", "الجن", "المزمل", "المدثر", "القيامة",
	"الإنسان", "المرسلات", "النبأ", "النازعات", "عبس",
	"التكوير", "الانفطار", "المطففين", "الانشقاق", "البروج",
	"الطارق", "الأعلى", "الغاشية", "الفجر", "البلد",
	"الشمس", "الليل", "الضحى", "الشرح", "التين",
	"العلق", "القدر", "البينة", "الزلزلة", "العاديات",
	"القارعة", "التكاثر", "العصر", "الهمزة", "الفيل",
	"قريش", "الماعون", "الكوثر", "الكافرون", "النصر",
	"المسد", "الإخلاص", "الفلق", "الناس",
}

// surahRevelationPlaces holds where each surah was revealed ("Mecca" or "Medina").
var surahRevelationPlaces = [...]string{
	"",
	"Mecca", "Medina", "Medina", "Medina", "Medina",
	"Mecca", "Mecca", "Medina", "Medina", "Mecca",
	"Mecca", "Mecca", "Medina", "Mecca", "Mecca",
	"Mecca", "Mecca", "Mecca", "Mecca", "Mecca",
	"Mecca", "Medina", "Mecca", "Medina", "Mecca",
	"Mecca", "Mecca", "Mecca", "Mecca", "Mecca",
	"Mecca", "Mecca", "Medina", "Mecca", "Mecca",
	"Mecca", "Mecca", "Mecca", "Mecca", "Mecca",
	"Mecca", "Mecca", "Mecca", "Mecca", "Mecca",
	"Mecca", "Medina", "Medina", "Medina", "Mecca",
	"Mecca", "Mecca", "Mecca", "Mecca", "Mecca",
	"Mecca", "Medina", "Medina", "Medina", "Medina",
	"Medina", "Medina", "Medina", "Medina", "Medina",
	"Medina", "Mecca", "Mecca", "Mecca", "Mecca",
	"Mecca", "Mecca", "Mecca", "Mecca", "Mecca",
	"Medina", "Mecca", "Mecca", "Mecca", "Mecca",
	"Mecca", "Mecca", "Mecca", "Mecca", "Mecca",
	"Mecca", "Mecca", "Mecca", "Mecca", "Mecca",
	"Mecca", "Mecca", "Mecca", "Medina", "Medina",
	"Mecca", "Mecca", "Mecca", "Mecca", "Mecca",
	"Mecca", "Mecca", "Mecca", "Mecca", "Medina",
	"Mecca", "Mecca", "Mecca", "Mecca",
}

func lookupSurahArabicName(surah int) string {
	if surah > 0 && surah < len(surahArabicNames) {
		return surahArabicNames[surah]
	}
	return ""
}

func lookupRevelationPlace(surah int) string {
	if surah > 0 && surah < len(surahRevelationPlaces) {
		return surahRevelationPlaces[surah]
	}
	return ""
}

func lookupSurahName(surah int) string {
	if surah > 0 && surah < len(surahNames) {
		return surahNames[surah]
	}
	return ""
}

// SurahByName returns the surah number for a given name (case-insensitive,
// partial prefix match). Returns 0 if not found.
func SurahByName(name string) int {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return 0
	}
	for i := 1; i < len(surahNames); i++ {
		if strings.HasPrefix(strings.ToLower(surahNames[i]), name) {
			return i
		}
	}
	return 0
}
