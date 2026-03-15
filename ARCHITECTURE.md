# Architecture

This document describes the system architecture.

---

# System Overview

The application consists of three main parts:

1. Quran dataset
2. Go backend server
3. server-rendered frontend

---

# Quran Dataset

Location:

data/quran.json

Loaded into memory at server startup.

Each record:

* surah
* ayah
* juz
* text_ar

---

# Backend

The backend is a Go HTTP server.

Responsibilities:

* load Quran dataset
* handle API requests
* store verse relations
* serve HTML pages

---

# Database

SQLite database.

Stores only verse relations.

Table:

relations

Fields:

* id
* ayah1_surah
* ayah1_ayah
* ayah2_surah
* ayah2_ayah
* note

---

# Folder Structure

cmd/server

main.go

internal

db
relations
search

web

templates
static

data

quran.json

---

# Request Flow

Example: user searches for verse.

User → HTTP request

GET /ayah/60/8

Server:

1. find verse in dataset
2. find relations in database
3. render HTML page

---

# Compare Mode

Compare view renders two verses side-by-side.

Used to highlight wording differences.

---

# Performance

The Quran dataset is small.

Load entire dataset into memory.

This makes verse lookup extremely fast.

---

# Deployment

The server should run with a single command:

go run ./cmd/server

No additional services required.

---

# Design Philosophy

Keep everything simple.

Avoid unnecessary dependencies.

Prioritize clarity and maintainability.

