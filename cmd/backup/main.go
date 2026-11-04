package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"almak-back/internal/config"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("A private backup directory is required")
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Could not read the database configuration")
	}
	dir, err := os.MkdirTemp(os.Args[1], "pre-deploy-")
	if err != nil {
		log.Fatal("Could not create the backup directory")
	}
	archive := filepath.Join(dir, "database.dump")
	dump := exec.Command("pg_dump", "--no-password", "--format=custom", "--file", archive)
	dump.Env = append(os.Environ(), "PGHOST="+cfg.DBHost, "PGPORT="+cfg.DBPort,
		"PGUSER="+cfg.DBUser, "PGDATABASE="+cfg.DBName, "PGPASSWORD="+cfg.DBPass)
	if err := dump.Run(); err != nil {
		log.Fatal("PostgreSQL backup failed")
	}
	if err := exec.Command("pg_restore", "--file="+os.DevNull, archive).Run(); err != nil {
		log.Fatal("PostgreSQL backup could not be read")
	}
	log.Print("PostgreSQL backup saved and verified")
}
