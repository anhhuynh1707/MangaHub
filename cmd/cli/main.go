package main

import (
	"fmt"
	"os"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "auth":
		handleAuth(os.Args[2:])
	case "manga":
		handleManga(os.Args[2:])
	case "library":
		handleLibrary(os.Args[2:])
	case "progress":
		handleProgress(os.Args[2:])
	case "sync":
		handleSync(os.Args[2:])
	case "server":
		handleServer(os.Args[2:])
	case "help", "--help", "-h":
		printUsage()
	case "version", "--version", "-v":
		fmt.Printf("MangaHub CLI v%s\n", version)
	default:
		fmt.Printf("✗ Unknown command: '%s'\n", os.Args[1])
		fmt.Println("Run 'mangahub help' for usage information")
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`
 ╔══════════════════════════════════════════════╗
 ║          MangaHub CLI - v` + version + `               ║
 ║     Manga Tracking & Network Protocols       ║
 ╚══════════════════════════════════════════════╝

 USAGE:
   mangahub <command> <subcommand> [flags]

 COMMANDS:
   auth        Authentication (register, login, logout, status, change-password)
   manga       Manga operations (search, info, list)
   library     Library management (add, list, remove, update)
   progress    Reading progress (update, history)
   sync        TCP progress sync (connect, disconnect, status, monitor)
   server      Server management (status, start)

 EXAMPLES:
   mangahub auth register --username alice
   mangahub auth login --username alice
   mangahub manga search "one piece"
   mangahub manga info one-piece
   mangahub library add --manga-id one-piece --status reading
   mangahub library list
   mangahub progress update --manga-id one-piece --chapter 1095
   mangahub sync connect

 Use 'mangahub <command> --help' for more information about a command.`)
}
