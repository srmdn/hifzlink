# Quran Murojaah

An open source tool to help Quran memorizers track and review similar verses (mutashabihat).

---

# Project Goal

Many Quran verses contain similar wording. During memorization (murojaah), memorizers often confuse one verse with another similar verse.

This project provides a simple tool to:

* search a verse
* view related similar verses
* compare verses side-by-side
* browse similar verses by surah or juz

The goal is to make Quran memorization easier and reduce confusion caused by similar verses.

---

# Target Users

* Quran memorizers (huffazh)
* students learning Quran memorization
* Quran teachers

---

# Core Idea

The core idea of the project is **verse relations**.

Example:

Surah Al-Mumtahanah 8 ↔ Surah Al-Mumtahanah 9

These verses have very similar wording and are often confused during memorization.

The system stores relations between verses and allows users to quickly explore them.

---

# Design Principles

This project must remain:

* simple
* lightweight
* easy to run locally
* minimal dependencies
* open source friendly

Avoid overengineering.

---

# Tech Stack

Backend

* Go
* Go standard library only
* net/http
* SQLite database
* no ORM
* no backend frameworks

Frontend

* HTML
* minimal JavaScript
* server-rendered templates
* no Node.js
* no npm
* no frontend frameworks required

---

# Quran Data Source

The Quran text will be stored locally using a JSON dataset.

File location:

data/quran.json

Each record contains:

* surah
* ayah
* juz
* text_ar

Example:

{
"surah": 60,
"ayah": 8,
"juz": 28,
"text_ar": "لا يَنْهَاكُمُ اللَّهُ عَنِ الَّذِينَ..."
}

The dataset will be loaded into memory when the server starts.

---

# Database Schema

The database stores only verse relations.

Table: relations

id INTEGER PRIMARY KEY

ayah1_surah INTEGER
ayah1_ayah INTEGER

ayah2_surah INTEGER
ayah2_ayah INTEGER

note TEXT

Example relation:

60:8 ↔ 60:9

---

# API Design

Get verse

GET /api/ayah/{surah}/{ayah}

Response

{
"surah": 60,
"ayah": 8,
"text": "...",
"juz": 28
}

---

Get related verses

GET /api/ayah/{surah}/{ayah}/relations

Response

{
"ayah": "60:8",
"related": [
{
"surah": 60,
"ayah": 9,
"text": "..."
}
]
}

---

Add relation

POST /api/relations

Body

{
"ayah1": "60:8",
"ayah2": "60:9",
"note": "mutashabihat"
}

---

List relations by surah

GET /api/surah/{surah}/relations

---

List relations by juz

GET /api/juz/{juz}/relations

---

# Frontend Requirements

The UI must prioritize readability for Quran memorization.

Rules:

* Always display full verses (never truncated)
* Arabic text must be clear and large
* Support side-by-side verse comparison

---

# Main Pages

Home

Search for verse.

Example input:

60:8

---

Ayah Page

Displays:

* full verse
* related verses

---

Compare Page

Displays two verses side-by-side.

Example:

Al-Mumtahanah 8 | Al-Mumtahanah 9

Full verse text | Full verse text

---

Surah Index

List relations inside a surah.

Example:

2:191 ↔ 2:217
2:25 ↔ 2:82

---

Juz Index

List relations inside a juz.

Example:

60:8 ↔ 60:9

---

# MVP Scope

The first version must support:

* search verse
* view full verse
* view similar verses
* compare verses
* add verse relation
* list relations by surah
* list relations by juz

---

# Future Features

* word difference highlighting
* quiz mode for memorization practice
* automatic detection of similar verses
* public dataset of mutashabihat verses

---

# Open Source Goal

The project should be easy to run.

Clone and run:

go run ./cmd/server

