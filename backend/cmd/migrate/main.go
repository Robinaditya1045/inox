package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/inox/inox/backend/internal/config"
	"github.com/inox/inox/backend/pkg/logger"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MigrationFile struct {
	Version int
	Name    string
	Path    string
}

func main() {
	dirFlag := flag.String("dir", "migrations", "Directory containing SQL migration files")
	commandFlag := flag.String("command", "up", "Migration command: up, down, or status")
	stepsFlag := flag.Int("steps", 0, "Number of migrations to execute (0 for all)")
	dbFlag := flag.String("db", "", "Database URL connection string (defaults to DATABASE_URL or config)")
	flag.Parse()

	// Load config if db URL not passed
	dbURL := *dbFlag
	if dbURL == "" {
		cfg, err := config.Load()
		if err != nil {
			dbURL = os.Getenv("DATABASE_URL")
		} else {
			dbURL = cfg.DatabaseURL
		}
	}

	if dbURL == "" {
		fmt.Fprintf(os.Stderr, "error: DATABASE_URL not provided via -db flag or environment\n")
		os.Exit(1)
	}

	log := logger.New("info")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Error("failed to connect to PostgreSQL for migrations", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// 1. Ensure schema_migrations table exists
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`
	if _, err := pool.Exec(ctx, createTableSQL); err != nil {
		log.Error("failed to ensure schema_migrations table exists", "error", err)
		os.Exit(1)
	}

	// 2. Scan migration directory
	files, err := os.ReadDir(*dirFlag)
	if err != nil {
		log.Error("failed to read migrations directory", "dir", *dirFlag, "error", err)
		os.Exit(1)
	}

	var migrations []MigrationFile
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".sql") {
			continue
		}
		// Skip .down.sql when applying up migrations
		if strings.HasSuffix(f.Name(), ".down.sql") {
			continue
		}
		parts := strings.SplitN(f.Name(), "_", 2)
		if len(parts) < 2 {
			continue
		}
		ver, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		migrations = append(migrations, MigrationFile{
			Version: ver,
			Name:    f.Name(),
			Path:    filepath.Join(*dirFlag, f.Name()),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	// 3. Fetch applied versions from DB
	appliedVersions := make(map[int]time.Time)
	rows, err := pool.Query(ctx, "SELECT version, applied_at FROM schema_migrations")
	if err != nil {
		log.Error("failed to query schema_migrations", "error", err)
		os.Exit(1)
	}
	for rows.Next() {
		var v int
		var t time.Time
		if err := rows.Scan(&v, &t); err == nil {
			appliedVersions[v] = t
		}
	}
	rows.Close()

	// 4. Execute Command
	cmd := strings.ToLower(*commandFlag)
	switch cmd {
	case "status":
		fmt.Printf("\n%-8s | %-45s | %-10s | %s\n", "VERSION", "MIGRATION NAME", "STATUS", "APPLIED AT")
		fmt.Println(strings.Repeat("-", 90))
		for _, m := range migrations {
			t, applied := appliedVersions[m.Version]
			status := "Pending"
			appliedAt := "-"
			if applied {
				status = "Applied"
				appliedAt = t.Format("2006-01-02 15:04:05 UTC")
			}
			fmt.Printf("%-803d | %-45s | %-10s | %s\n", m.Version, m.Name, status, appliedAt)
		}
		fmt.Println()

	case "up":
		appliedCount := 0
		for _, m := range migrations {
			if _, alreadyApplied := appliedVersions[m.Version]; alreadyApplied {
				continue
			}

			content, err := os.ReadFile(m.Path)
			if err != nil {
				log.Error("failed to read migration file", "path", m.Path, "error", err)
				os.Exit(1)
			}

			log.Info("applying migration...", "version", m.Version, "name", m.Name)
			err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
				if _, err := tx.Exec(ctx, string(content)); err != nil {
					return fmt.Errorf("sql execution failed: %w", err)
				}
				_, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version, name) VALUES ($1, $2)", m.Version, m.Name)
				return err
			})

			if err != nil {
				log.Error("migration failed", "version", m.Version, "name", m.Name, "error", err)
				os.Exit(1)
			}

			log.Info("successfully applied migration", "version", m.Version, "name", m.Name)
			appliedCount++
			if *stepsFlag > 0 && appliedCount >= *stepsFlag {
				break
			}
		}
		log.Info("migrations check complete", "newly_applied", appliedCount)

	case "down":
		log.Error("down migrations require manual rollback or paired .down.sql files to prevent accidental production data loss")
		os.Exit(1)

	default:
		log.Error("unknown migration command", "command", cmd)
		os.Exit(1)
	}
}
