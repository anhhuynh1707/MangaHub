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
	case "notify":
		handleNotify(os.Args[2:])
	case "chat":
		handleChat(os.Args[2:])
	case "grpc":
		handleGRPC(os.Args[2:])
	case "server":
		handleServer(os.Args[2:])
	case "review":
		handleReview(os.Args[2:])
	case "friend":
		handleFriend(os.Args[2:])
	case "sharedlist":
		handleSharedList(os.Args[2:])
	case "feed":
		handleFeed(os.Args[2:])
	case "export":
		handleExport(os.Args[2:])
	case "import":
		handleImport(os.Args[2:])
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
   manga       Manga operations (search, advanced, info, list, recommend)
   library     Library management (add, list, remove, update)
   progress    Reading progress (update, history)
   sync        TCP progress sync (connect, disconnect, status, monitor)
   notify      UDP notifications (subscribe, unsubscribe, test, send, send-ack, ack-stats)
   chat        WebSocket chat (join, send, history)
   grpc        gRPC service (manga get/search/stream, progress update, watch)
   server      Server management (status, start)
   review      User reviews and ratings (add, list, mine)
   friend      Friend system (add, accept, list, pending)
   sharedlist  Shared reading lists (create, mine, public)
   feed        Activity feed (view, mine)
   export      Export data (library, progress, all)
   import      Import data (library, progress, manga)

 EXAMPLES:
   mangahub auth register --username alice
   mangahub auth login --username alice

   mangahub manga search "one piece"
   mangahub manga advanced --genres action,adventure --sort rating --min-rating 8
   mangahub manga advanced "demon slayer" --sort popularity
   mangahub manga info one-piece
   mangahub manga recommend
   mangahub manga recommend --limit 5

   mangahub library add --manga-id one-piece --status reading
   mangahub library list
   mangahub progress update --manga-id one-piece --chapter 1095

   mangahub sync connect
   mangahub notify subscribe
   mangahub notify send-ack --type new_chapter --manga-id one-piece --message "Ch 1121!"
   mangahub notify ack-stats

   mangahub chat join general
   mangahub chat send one-piece "Hello One Piece fans!"

   mangahub grpc manga get --id one-piece
   mangahub grpc manga search --query "naruto"
   mangahub grpc manga stream --query "one" --limit 5
   mangahub grpc watch
   mangahub grpc watch --manga-id one-piece

   mangahub export library --format json --output library.json
   mangahub export all --output mangahub-backup.tar.gz
   mangahub import library --file library.json

 Use 'mangahub <command> --help' for more information about a command.`)
}
