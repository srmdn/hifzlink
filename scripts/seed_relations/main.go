package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/srmdn/hifzlink/internal/db"
	"github.com/srmdn/hifzlink/internal/relations"
	"github.com/srmdn/hifzlink/internal/search"
)

type seedRelation struct {
	Ayah1 string `json:"ayah1"`
	Ayah2 string `json:"ayah2"`
	Note  string `json:"note"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "seed failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	quran, err := search.Load(filepath.Join("data", "quran.json"))
	if err != nil {
		return fmt.Errorf("load quran dataset: %w", err)
	}

	dbStore, err := db.Open(filepath.Join("data", "relations.db"))
	if err != nil {
		return fmt.Errorf("open relations db: %w", err)
	}
	defer dbStore.Close()

	svc := relations.NewService(dbStore, quran)

	b, err := os.ReadFile(filepath.Join("data", "relations.seed.json"))
	if err != nil {
		return fmt.Errorf("read seed file: %w", err)
	}

	var seeds []seedRelation
	if err := json.Unmarshal(b, &seeds); err != nil {
		return fmt.Errorf("decode seed file: %w", err)
	}

	for i, rel := range seeds {
		if err := svc.Add(rel.Ayah1, rel.Ayah2, rel.Note); err != nil {
			return fmt.Errorf("seed entry %d (%s <-> %s): %w", i+1, rel.Ayah1, rel.Ayah2, err)
		}
	}

	fmt.Printf("Seeded %d relation entries into data/relations.db\n", len(seeds))
	return nil
}
