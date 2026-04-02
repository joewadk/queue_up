package main

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/lib/pq"
)

type problemEntry struct {
	slug       string
	title      string
	difficulty string
	url        string
	tags       []string
	queueRank  int
}

func main() {
	if err := godotenv.Load("../.env"); err != nil {
		log.Fatal("Error loading .env file from project root:", err)
	}

	password := os.Getenv("POSTGRES_PASSWORD")
	if password == "" {
		log.Fatal("POSTGRES_PASSWORD environment variable is required")
	}

	dbHost := "localhost"
	dbPort := 55432
	dbUser := "queue_up"
	dbName := "queue_up"

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, password, dbName)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Error connecting to DB:", err)
	}
	defer db.Close()

	entries, source, err := loadNC150Queue()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Using %d queue entries from %s\n", len(entries), source)

	stmt, err := db.Prepare(`
		INSERT INTO problems (slug, title, difficulty, url, tags, source_set, queue_rank)
		VALUES ($1, $2, $3, $4, $5, 'NEETCODE_150', $6)
		ON CONFLICT (slug) DO NOTHING
	`)
	if err != nil {
		log.Fatal("prepare statement:", err)
	}
	defer stmt.Close()

	inserted := 0
	for _, entry := range entries {
		res, err := stmt.Exec(entry.slug, entry.title, entry.difficulty, entry.url, pq.Array(entry.tags), entry.queueRank)
		if err != nil {
			log.Printf("Error inserting %s: %v\n", entry.slug, err)
			continue
		}
		if affected, err := res.RowsAffected(); err == nil && affected > 0 {
			inserted += int(affected)
		}
	}

	fmt.Printf("✅ NC150 problem queue populated: %d new rows inserted (source=%s)\n", inserted, source)
}

func loadNC150Queue() ([]problemEntry, string, error) {
	// Update to consider the correct CSV file in the project root
	const rootCSVPath = "../nc150 - Sheet2.csv"

	entries, err := parseNC150CSV(rootCSVPath)
	if err == nil {
		return entries, rootCSVPath, nil
	}
	if !os.IsNotExist(err) {
		return nil, "", fmt.Errorf("error parsing %s: %w", rootCSVPath, err)
	}

	return nil, "", fmt.Errorf("no valid CSV file found")
}

func parseNC150CSV(path string) ([]problemEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read nc150 csv: %w", err)
	}

	var result []problemEntry
	for idx, row := range records {
		if idx == 0 {
			continue
		}
		if len(row) < 6 {
			return nil, fmt.Errorf("row %d malformed, expected 6 columns", idx+1)
		}
		queueRank, err := strconv.Atoi(strings.TrimSpace(row[0]))
		if err != nil {
			return nil, fmt.Errorf("parse queue_rank on row %d: %w", idx+1, err)
		}
		tags := strings.Split(strings.TrimSpace(row[5]), ",")
		entry := problemEntry{
			queueRank:  queueRank,
			title:      strings.TrimSpace(row[1]),
			difficulty: strings.TrimSpace(row[2]),
			slug:       strings.TrimSpace(row[3]),
			url:        strings.TrimSpace(row[4]),
			tags:       tags,
		}
		if entry.slug == "" || entry.title == "" || entry.url == "" {
			return nil, fmt.Errorf("row %d missing required fields", idx+1)
		}
		result = append(result, entry)
	}
	return result, nil
}
