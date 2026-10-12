package main

func ayahCopy(lang string) map[string]string {
	if lang == "id" {
		return map[string]string{
			"back": "Kembali ke surah", "kicker": "BACAAN AYAT", "surah": "Surah", "juz": "Juz",
			"translation": "Terjemahan", "audio": "Dengarkan ayat", "audioSource": "Tilawah dari Quran Foundation",
			"share": "Bagikan ayat", "shareText": "Baca ayat Quran ini di HifzLink.", "shareCopied": "Tautan ayat tersalin.",
			"shareFailed": "Tautan belum bisa disalin. Salin dari bilah alamat browser.",
			"saveHeading": "Simpan untuk diulang", "saveLogin": "Masuk dengan Quran.Foundation untuk menyimpan ayat ini ke koleksi.",
			"saveLabel": "Simpan ke koleksi", "selectCollection": "Pilih koleksi", "notePlaceholder": "Catatan (opsional)", "saveAyah": "Simpan ayat",
			"noCollections": "Kamu belum punya koleksi.", "createCollection": "Buat koleksi", "tafsir": "Tafsir", "hide": "Sembunyikan", "show": "Tampilkan",
			"relatedHeading": "Ayat dengan lafaz mirip", "relatedLead": "Baca ayat lengkap, lalu bandingkan bagian yang sering tertukar.",
			"compare": "Bandingkan", "noRelated": "Belum ada pasangan ayat yang dikurasi untuk ayat ini.",
			"saveCreated": "Koleksi dibuat.", "saveSuccess": "Ayat tersimpan ke koleksi.", "saveDuplicate": "Ayat ini sudah ada di koleksi.", "saveRemoved": "Ayat dihapus dari koleksi.",
		}
	}
	return map[string]string{
		"back": "Back to surah", "kicker": "READING AYAH", "surah": "Surah", "juz": "Juz",
		"translation": "Translation", "audio": "Listen to this ayah", "audioSource": "Recitation via Quran Foundation",
		"share": "Share ayah", "shareText": "Read this Quran verse on HifzLink.", "shareCopied": "Ayah link copied.",
		"shareFailed": "Could not copy the link. Copy it from your browser address bar.",
		"saveHeading": "Save for review", "saveLogin": "Sign in with Quran.Foundation to save this ayah to a collection.",
		"saveLabel": "Save to collection", "selectCollection": "Choose a collection", "notePlaceholder": "Optional note", "saveAyah": "Save ayah",
		"noCollections": "You don’t have a collection yet.", "createCollection": "Create a collection", "tafsir": "Tafsir", "hide": "Hide", "show": "Show",
		"relatedHeading": "Verses with similar wording", "relatedLead": "Read each complete verse, then compare the wording that is easy to mix up.",
		"compare": "Compare", "noRelated": "No curated similar verses are linked to this ayah yet.",
		"saveCreated": "Collection created.", "saveSuccess": "Ayah saved to collection.", "saveDuplicate": "This ayah is already in the collection.", "saveRemoved": "Ayah removed from collection.",
	}
}

func ayahSaveStatus(code, lang string) string {
	copy := ayahCopy(lang)
	switch code {
	case "created":
		return copy["saveCreated"]
	case "saved":
		return copy["saveSuccess"]
	case "duplicate":
		return copy["saveDuplicate"]
	case "removed":
		return copy["saveRemoved"]
	default:
		return ""
	}
}
