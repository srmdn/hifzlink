package main

func adminCategoryOptionsForLang(lang string) []adminCategoryOption {
	options := adminCategoryOptions()
	if lang != "id" {
		return options
	}
	translations := map[string][2]string{
		"lafzi":             {"Lafzi · seluruh ayat", "Seluruh lafaz sama atau setidaknya 90% kata pada masing-masing ayat selaras. Kesamaan satu frasa saja tidak cukup."},
		"addition_omission": {"Penambahan / penghilangan", "Urutan lengkap salah satu ayat dipertahankan dengan tambahan kata. Ayat pendek mencakup setidaknya 80% ayat panjang."},
		"word_swap":         {"Perubahan kata", "Beberapa kata diganti pada ayat yang selebihnya sama. Saran otomatis mensyaratkan panjang sama dan minimal 80% posisi kata identik."},
		"ending_variation":  {"Akhir berbeda", "Bagian awal yang sama mencakup minimal 80% masing-masing ayat; hanya bagian akhir berubah. Berbeda dari frasa akhir sama."},
		"order_change":      {"Perubahan urutan", "Kumpulan kata lengkap sama, tetapi urutannya berubah."},
		"pronoun_shift":     {"Perubahan kata ganti", "Perubahan utama berupa kata ganti. Tetapkan manual setelah membaca teks Arab."},
		"shared_opening":    {"Frasa awal sama", "Frasa yang sama berada di awal kedua ayat; bagian lainnya cukup berbeda."},
		"shared_phrase":     {"Frasa sama · tengah atau posisi berbeda", "Frasa sama ditemukan di tengah atau pada posisi berbeda. Keseluruhan ayat tidak hampir identik."},
		"shared_ending":     {"Frasa akhir sama", "Kedua ayat berakhir dengan frasa yang sama; bagian sebelumnya cukup berbeda. Contoh: 2:114 ↔ 5:41."},
		"structural":        {"Pola kalimat serupa", "Susunan gramatikal serupa dengan lafaz berbeda. Perlu penilaian manusia; tumpang tindih kata saja tidak cukup."},
		"other":             {"Lainnya · perlu penilaian", "Belum ada pola lafaz yang jelas. Tinjau manfaat pasangan untuk hafalan sebelum menetapkan hubungan."},
	}
	for i := range options {
		if t, ok := translations[options[i].Key]; ok {
			options[i].Label = t[0]
			options[i].Hint = t[1]
		}
	}
	return options
}

func categoryDisplayLabel(key, lang string) string {
	for _, option := range adminCategoryOptionsForLang(lang) {
		if option.Key == key {
			return option.Label
		}
	}
	return key
}

// Reader-facing descriptions explain the saved label without admin rule thresholds.
func categoryDescription(key, lang string) string {
	descriptions := map[string][2]string{
		"lafzi":             {"Lafaz kedua ayat sama atau hampir sama dari awal hingga akhir.", "The wording of both verses is identical or nearly identical from beginning to end."},
		"addition_omission": {"Salah satu ayat memuat kata atau frasa tambahan dalam rangkaian lafaz yang sama.", "One verse includes extra words or a phrase within an otherwise matching word sequence."},
		"word_swap":         {"Beberapa kata berbeda, sedangkan susunan dan sebagian besar lafaz kedua ayat sama.", "A few words differ, while the word order and most of the wording match."},
		"ending_variation":  {"Bagian awal kedua ayat sama, tetapi kata atau frasa penutupnya berbeda.", "Both verses share the same opening, but their closing words or phrases differ."},
		"order_change":      {"Kedua ayat menggunakan kata-kata yang sama dengan urutan berbeda.", "Both verses use the same words in a different order."},
		"pronoun_shift":     {"Perbedaan utama kedua ayat terdapat pada kata ganti yang digunakan.", "The main difference between the verses is the pronoun used."},
		"shared_opening":    {"Kedua ayat diawali frasa yang sama, sedangkan bagian setelahnya berbeda.", "Both verses begin with the same phrase, while the wording that follows differs."},
		"shared_phrase":     {"Kedua ayat memuat frasa yang sama di tengah ayat atau pada posisi berbeda. Lafaz di luar frasa tersebut berbeda.", "Both verses share a phrase within the verse or at different positions. The surrounding wording differs."},
		"shared_ending":     {"Kedua ayat memiliki frasa penutup yang sama, sedangkan bagian sebelumnya berbeda.", "Both verses share the same closing phrase, while the preceding wording differs."},
		"structural":        {"Kedua ayat memiliki pola kalimat serupa dengan pilihan kata yang berbeda.", "Both verses have a similar sentence structure with different wording."},
		"other":             {"Pasangan ini belum dikelompokkan ke dalam pola kemiripan yang lebih spesifik.", "This pair has not been assigned a more specific similarity pattern."},
	}
	if description, ok := descriptions[key]; ok {
		if lang == "id" {
			return description[0]
		}
		return description[1]
	}
	return ""
}
