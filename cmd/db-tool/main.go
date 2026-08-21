package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"mangahub/internal/dbops"
)

const (
	databasePath = "/app/data/mangahub.db"
	backupDir    = "/backups"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: db-tool <backup|verify|restore|list|prune> [options]")
	}
	switch args[0] {
	case "backup":
		return backupCommand(args[1:])
	case "verify":
		return verifyCommand(args[1:])
	case "restore":
		return restoreCommand(args[1:])
	case "list":
		return listCommand(args[1:])
	case "prune":
		return pruneCommand(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func listCommand(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("list accepts no options")
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return fmt.Errorf("read backup directory: %w", err)
	}
	var names []string
	for _, entry := range entries {
		if dbops.ValidBackupName(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	if len(names) == 0 {
		fmt.Println("No MangaHub backups found")
		return nil
	}
	for _, name := range names {
		metadata, err := dbops.VerifyBackup(filepath.Join(backupDir, name))
		if err != nil {
			return fmt.Errorf("verify %s: %w", name, err)
		}
		fmt.Printf("%s\t%d bytes\tsha256=%s\n", name, metadata.SizeBytes, metadata.SHA256)
	}
	return nil
}

func backupCommand(args []string) error {
	flags := flag.NewFlagSet("backup", flag.ContinueOnError)
	name := flags.String("name", "", "timestamped backup filename")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || !dbops.ValidBackupName(*name) {
		return fmt.Errorf("backup name must be a timestamped MangaHub full-SHA .db filename")
	}
	metadata, err := dbops.Backup(databasePath, filepath.Join(backupDir, *name))
	if err != nil {
		return err
	}
	fmt.Printf("Backup verified: %s (%d bytes, sha256=%s)\n", *name, metadata.SizeBytes, metadata.SHA256)
	return nil
}

func verifyCommand(args []string) error {
	flags := flag.NewFlagSet("verify", flag.ContinueOnError)
	name := flags.String("name", "", "backup filename")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || !dbops.ValidBackupName(*name) {
		return fmt.Errorf("invalid backup name")
	}
	metadata, err := dbops.VerifyBackup(filepath.Join(backupDir, *name))
	if err != nil {
		return err
	}
	fmt.Printf("Backup integrity passed: %s (%d bytes, sha256=%s)\n", *name, metadata.SizeBytes, metadata.SHA256)
	return nil
}

func restoreCommand(args []string) error {
	flags := flag.NewFlagSet("restore", flag.ContinueOnError)
	name := flags.String("name", "", "backup filename")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || !dbops.ValidBackupName(*name) {
		return fmt.Errorf("invalid backup name")
	}
	metadata, err := dbops.Restore(filepath.Join(backupDir, *name), databasePath)
	if err != nil {
		return err
	}
	fmt.Printf("Database restored from verified backup: %s (sha256=%s)\n", *name, metadata.SHA256)
	return nil
}

func pruneCommand(args []string) error {
	flags := flag.NewFlagSet("prune", flag.ContinueOnError)
	keep := flags.Int("keep", 7, "number of newest backup pairs to retain")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("prune accepts only --keep")
	}
	removed, err := dbops.Prune(backupDir, *keep)
	if err != nil {
		return err
	}
	fmt.Printf("Backup retention passed: kept %d newest backup(s), removed %d\n", *keep, len(removed))
	return nil
}
