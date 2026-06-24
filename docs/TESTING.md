# MangaHub — Testing Guide

> **Last Updated:** 2026-05-05  
> **Status:** Phase 3 (HTTP + TCP + UDP + WebSocket) Complete  
> **Ports:** API `:8080` | TCP Sync `:9090` | UDP Notify `:9091` | WebSocket Chat `:8080/ws/chat`

---

## Table of Contents

1. [Prerequisites & Setup](#1-prerequisites--setup)
2. [Server Health](#2-server-health)
3. [Authentication](#3-authentication)
4. [Manga API](#4-manga-api)
5. [Library Management](#5-library-management)
6. [Progress Tracking](#6-progress-tracking)
7. [TCP Sync Server](#7-tcp-sync-server)
8. [CLI Application](#8-cli-application)
9. [Data Collection](#9-data-collection)
10. [Multi-Terminal Sessions](#10-multi-terminal-sessions)
11. [End-to-End Workflow](#11-end-to-end-workflow)
12. [Social & Community Features](#12-social--community-features)

---

## 1. Prerequisites & Setup

### Build Everything

```powershell
cd C:\Users\Dell\Documents\Go\mangahub

# Build API server
go build ./cmd/api-server/

# Build CLI
go build -o mangahub.exe ./cmd/cli/

# Build TCP test client
go build ./cmd/tcp-client/

# Build standalone TCP server
go build ./cmd/tcp-server/

# Build standalone UDP server
go build ./cmd/udp-server/

# Build standalone gRPC server
go build ./cmd/grpc-server/
```

### Fresh Database Reset

```powershell
# Stop server first, then:
Remove-Item .\data\mangahub.db -Force -ErrorAction SilentlyContinue
```

### Start Server

```powershell
go run ./cmd/api-server/
```

**Logging is now structured (slog).** By default it prints readable text for
local dev; set `LOG_FORMAT=json` for machine-parseable output and `LOG_LEVEL`
(debug|info|warn|error) to control verbosity:

```bash
LOG_FORMAT=text LOG_LEVEL=info go run ./cmd/api-server/   # default (readable)
LOG_FORMAT=json go run ./cmd/api-server/                  # JSON (prod-style)
```

**Expected output (text mode):**
```
time=10:32:02 level=INFO msg="MangaHub API server starting" port=8080 ...
time=10:32:05 level=INFO msg=request request_id=c6e073da method=GET path=/health status=200 latency_ms=0 client_ip=::1
```
Each request logs a line with `request_id`, `method`, `path`, `status`,
`latency_ms`, and `user_id` (when authenticated). Every response also carries an
`X-Request-Id` header.

> **Note — rate limiting:** the API limits each IP to **100 req/min** (public) /
> **300 req/min** (authenticated); `/health*` and `/swagger*` are exempt. A burst
> of test calls beyond the limit returns **429 Too Many Requests** — expected,
> not a failure.

### Register Test Users

```powershell
# Register Alice
curl -s -X POST http://localhost:8080/auth/register `
  -H "Content-Type: application/json" `
  -d '{"username":"alice","password":"alice123"}'

# Register Bob
curl -s -X POST http://localhost:8080/auth/register `
  -H "Content-Type: application/json" `
  -d '{"username":"bob","password":"bob123"}'
```

### Login & Save Tokens

```powershell
# Login Alice -> save token
$alice = Invoke-RestMethod -Uri "http://localhost:8080/auth/login" `
  -Method POST -Body '{"username":"alice","password":"alice123"}' `
  -ContentType "application/json"
$ALICE_TOKEN = $alice.data.token
Write-Host "Alice token: $ALICE_TOKEN"

# Login Bob -> save token
$bob = Invoke-RestMethod -Uri "http://localhost:8080/auth/login" `
  -Method POST -Body '{"username":"bob","password":"bob123"}' `
  -ContentType "application/json"
$BOB_TOKEN = $bob.data.token
Write-Host "Bob token: $BOB_TOKEN"
```

---

## 2. Server Health

```powershell
# HTTP health check
curl -s http://localhost:8080/health | ConvertFrom-Json | ConvertTo-Json

# CLI health check
.\mangahub.exe server status
```

**Expected:**
```json
{
  "success": true,
  "message": "MangaHub API is running",
  "data": { "status": "healthy", "manga_count": 200 }
}
```

---

## 3. Authentication

### 3.1 Register

```powershell
curl -s -X POST http://localhost:8080/auth/register `
  -H "Content-Type: application/json" `
  -d '{"username":"testuser","password":"test123"}'
```

**Expected:** `201 Created` with user ID  
**Error case (duplicate):**
```json
{ "success": false, "error": "username already taken" }
```

### 3.2 Login

```powershell
curl -s -X POST http://localhost:8080/auth/login `
  -H "Content-Type: application/json" `
  -d '{"username":"alice","password":"alice123"}'
```

**Expected:** `200 OK` with JWT token  
**Error case (wrong password):**
```json
{ "success": false, "error": "invalid username or password" }
```

### 3.3 Auth Status

```powershell
curl -s http://localhost:8080/auth/status `
  -H "Authorization: Bearer $ALICE_TOKEN"
```

**Expected:** `200 OK` with user info

### 3.4 Logout

```powershell
curl -s -X POST http://localhost:8080/auth/logout `
  -H "Authorization: Bearer $ALICE_TOKEN"
```

**Expected:** `200 OK` — client discards token

### 3.5 Change Password

```powershell
curl -s -X PUT http://localhost:8080/auth/change-password `
  -H "Authorization: Bearer $ALICE_TOKEN" `
  -H "Content-Type: application/json" `
  -d '{"old_password":"alice123","new_password":"newpass789"}'
```

**Expected:** `200 OK`  
> Warning: After changing password, you must login again with the new password.

---

## 4. Manga API

### 4.1 Search

```powershell
# Search by keyword
curl -s "http://localhost:8080/manga?search=one" | ConvertFrom-Json | ConvertTo-Json -Depth 3

# Search with genre filter
curl -s "http://localhost:8080/manga?genre=Shounen&limit=5"

# Search with status filter
curl -s "http://localhost:8080/manga?status=ongoing&limit=5"
```

**Expected:** Returns `{ manga: [...], total, page, limit }`

### 4.2 Get by ID

```powershell
curl -s http://localhost:8080/manga/one-piece | ConvertFrom-Json | ConvertTo-Json -Depth 3
```

**Expected:**
```json
{
  "id": "one-piece",
  "title": "One Piece",
  "author": "Oda Eiichiro",
  "genres": ["Action", "Adventure", "Shounen"],
  "status": "ongoing",
  "total_chapters": 1120,
  "description": "A young pirate's adventure...",
  "cover_url": "https://example.com/covers/one-piece.jpg"
}
```

### 4.3 Create Manga (authenticated)

```powershell
curl -s -X POST http://localhost:8080/manga `
  -H "Authorization: Bearer $ALICE_TOKEN" `
  -H "Content-Type: application/json" `
  -d '{"id":"test-manga","title":"Test Manga","author":"Test Author","genres":["Action"],"status":"ongoing","total_chapters":10,"description":"A test manga."}'
```

### 4.4 Update Manga (authenticated)

```powershell
curl -s -X PUT http://localhost:8080/manga/test-manga `
  -H "Authorization: Bearer $ALICE_TOKEN" `
  -H "Content-Type: application/json" `
  -d '{"id":"test-manga","title":"Test Manga","author":"Test Author","genres":["Action"],"status":"ongoing","total_chapters":20,"description":"A test manga."}'
```

### 4.5 Delete Manga (authenticated)

```powershell
curl -s -X DELETE http://localhost:8080/manga/test-manga `
  -H "Authorization: Bearer $ALICE_TOKEN"
```

---

## 5. Library Management

### 5.1 Add to Library

```powershell
# Alice adds One Piece
curl -s -X POST http://localhost:8080/users/library `
  -H "Authorization: Bearer $ALICE_TOKEN" `
  -H "Content-Type: application/json" `
  -d '{"manga_id":"one-piece","status":"reading"}'

# Alice adds Naruto
curl -s -X POST http://localhost:8080/users/library `
  -H "Authorization: Bearer $ALICE_TOKEN" `
  -H "Content-Type: application/json" `
  -d '{"manga_id":"naruto","status":"completed"}'

# Bob adds Demon Slayer
curl -s -X POST http://localhost:8080/users/library `
  -H "Authorization: Bearer $BOB_TOKEN" `
  -H "Content-Type: application/json" `
  -d '{"manga_id":"demon-slayer","status":"reading"}'
```

### 5.2 View Library

```powershell
# Alice's library
curl -s http://localhost:8080/users/library `
  -H "Authorization: Bearer $ALICE_TOKEN" | ConvertFrom-Json | ConvertTo-Json -Depth 4

# Bob's library
curl -s http://localhost:8080/users/library `
  -H "Authorization: Bearer $BOB_TOKEN" | ConvertFrom-Json | ConvertTo-Json -Depth 4
```

**Expected:** Returns categorized lists (reading, completed, plan_to_read)

### 5.3 Remove from Library

```powershell
curl -s -X DELETE http://localhost:8080/users/library/naruto `
  -H "Authorization: Bearer $ALICE_TOKEN"
```

---

## 6. Progress Tracking

### 6.1 Update Progress (HTTP)

```powershell
curl -s -X PUT http://localhost:8080/users/progress `
  -H "Authorization: Bearer $ALICE_TOKEN" `
  -H "Content-Type: application/json" `
  -d '{"manga_id":"one-piece","current_chapter":1095,"status":"reading"}'
```

**Expected:** `200 OK` — also triggers TCP broadcast to connected clients

### 6.2 Verify Progress Persisted

```powershell
curl -s http://localhost:8080/users/library `
  -H "Authorization: Bearer $ALICE_TOKEN" | ConvertFrom-Json | ConvertTo-Json -Depth 4
```

**Check:** `current_chapter` should be `1095`

---

## 7. TCP Sync Server

### 7.1 Using the TCP Test Client

```powershell
# Save tokens to files first
$ALICE_TOKEN | Set-Content .\data\alice_token.txt -NoNewline
$BOB_TOKEN | Set-Content .\data\bob_token.txt -NoNewline

# Terminal 1: Connect Alice
go run ./cmd/tcp-client/ alice token-file:.\data\alice_token.txt

# Terminal 2: Connect Bob
go run ./cmd/tcp-client/ bob token-file:.\data\bob_token.txt
```

**Test commands inside TCP client:**
```
ping                          -> Should get Pong
progress one-piece 1095       -> Should broadcast to all clients
quit                          -> Graceful disconnect
```

**Expected cross-client behavior:**
- When Bob sends `progress one-piece 1095`, Alice sees:
  ```
  BROADCAST: user-bob -> one-piece ch.1095
  ```

### 7.2 TCP Protocol Messages

All messages are **newline-delimited JSON** (`\n` terminated).

| Direction | Message |
|-----------|---------|
| Client -> Server | `{"type":"auth","token":"jwt-token"}\n` |
| Client -> Server | `{"type":"connect","user_id":"user123"}\n` |
| Client -> Server | `{"type":"progress","manga_id":"one-piece","chapter":1095}\n` |
| Client -> Server | `{"type":"ping"}\n` |
| Client -> Server | `{"type":"status"}\n` |
| Client -> Server | `{"type":"disconnect"}\n` |
| Server -> Client | `{"type":"welcome","message":"Connected to MangaHub..."}\n` |
| Server -> Client | `{"type":"auth","user_id":"...","username":"..."}\n` |
| Server -> Client | `{"type":"broadcast","user_id":"...","manga_id":"...","chapter":N}\n` |
| Server -> Client | `{"type":"user_joined","username":"bob"}\n` |
| Server -> Client | `{"type":"user_left","username":"bob"}\n` |
| Server -> Client | `{"type":"pong"}\n` |
| Server -> Client | `{"type":"status","connected_users":3,"message":"..."}\n` |
| Server -> Client | `{"type":"error","message":"..."}\n` |

### 7.3 Verify TCP Progress Persists to DB

1. Connect via TCP client
2. Send: `progress one-piece 500`
3. Disconnect
4. Check via HTTP:
   ```powershell
   curl -s http://localhost:8080/users/library `
     -H "Authorization: Bearer $ALICE_TOKEN"
   ```
5. **Verify:** `current_chapter` is now `500`

---

## 8. CLI Application

### 8.1 Build CLI

```powershell
go build -o mangahub.exe ./cmd/cli/
```

### 8.2 Auth Commands

```powershell
# Register (prompts for password securely)
.\mangahub.exe auth register --username johndoe

# Login (saves token to profile)
.\mangahub.exe auth login --username alice

# Check status
.\mangahub.exe auth status

# Logout
.\mangahub.exe auth logout

# Change password
.\mangahub.exe auth change-password
```

### 8.3 Manga Commands

```powershell
# Search
.\mangahub.exe manga search "one piece"
.\mangahub.exe manga search naruto --limit 5
.\mangahub.exe manga search "" --genre Shounen

# View details
.\mangahub.exe manga info one-piece
.\mangahub.exe manga info demon-slayer

# List all
.\mangahub.exe manga list
.\mangahub.exe manga list --limit 10
.\mangahub.exe manga list --genre Romance
.\mangahub.exe manga list --status ongoing
```

**Expected search output:**
```
+-------------------+-------------------+------------------+-----------+----------+
| ID                | Title             | Author           | Status    | Chapters |
+-------------------+-------------------+------------------+-----------+----------+
| one-piece         | One Piece         | Oda Eiichiro     | ongoing   | 1120     |
| one-punch-man     | One Punch Man     | ONE              | ongoing   | 195      |
+-------------------+-------------------+------------------+-----------+----------+
```

### 8.4 Library Commands

```powershell
# Add manga to library
.\mangahub.exe library add --manga-id one-piece --status reading
.\mangahub.exe library add --manga-id death-note --status completed

# View library
.\mangahub.exe library list
.\mangahub.exe library list --status reading

# Remove from library
.\mangahub.exe library remove --manga-id death-note

# Update status
.\mangahub.exe library update --manga-id one-piece --status completed
```

### 8.5 Progress Commands

```powershell
# Update progress
.\mangahub.exe progress update --manga-id one-piece --chapter 1095

# View history
.\mangahub.exe progress history
```

### 8.6 Sync Commands

```powershell
# Check TCP server status (uses HTTP API /sync/status)
.\mangahub.exe sync status

# Interactive TCP connection
.\mangahub.exe sync connect
# Then type: progress one-piece 1095
# Then type: ping
# Then type: status
# Then type: quit

# Monitor mode (read-only, watch live updates)
.\mangahub.exe sync monitor
# Press Ctrl+C to exit
```

**Expected sync status output:**
```
TCP Sync Status:
  Connection: Active
  Server:     localhost:9090
  Uptime:     2m30s

Session Info:
  User:       alice
  User ID:    user-alice
  Profile:    alice

Connected Users: 2
  - user-alice
  - user-bob
```

### 8.7 Server Commands

```powershell
.\mangahub.exe server status
.\mangahub.exe server start
```

### 8.8 Notify Commands

```powershell
# Test UDP server is alive
.\mangahub.exe notify test

# Subscribe to notifications (stays connected, listens for updates)
.\mangahub.exe notify subscribe
# -> Receives notifications in real-time
# -> Ctrl+C to unsubscribe and exit

# Unsubscribe from notifications
.\mangahub.exe notify unsubscribe

# Send a notification to all subscribers (requires auth)
.\mangahub.exe notify send --type new_chapter --manga-id one-piece --message "Chapter 1121 released!"
.\mangahub.exe notify send --type system --message "Server maintenance at midnight"
```

**Expected subscribe output:**
```
✓ Subscribed to notifications!
  Registered for notifications. You are client #1.
  Listening on: 127.0.0.1:50207

Waiting for notifications... (Press Ctrl+C to exit)

[11:38:15] 📖 NEW CHAPTER: Chapter 1121 released!
[11:40:00] 🔔 SYSTEM: Server maintenance at midnight
```

### 8.9 Chat Commands (WebSocket)

```powershell
# Join a chat room (room name is required)
.\mangahub.exe chat join general
.\mangahub.exe chat join one-piece

# Send a one-shot message to a specific room
.\mangahub.exe chat send general "Hello everyone!"
.\mangahub.exe chat send one-piece "Hello One Piece fans!"

# View chat history for a room
.\mangahub.exe chat history general
.\mangahub.exe chat history one-piece --limit 50
```

**Interactive chat commands (inside `chat join`):**
```
/help             - Show available commands
/users            - List online users
/quit             - Leave chat
/pm <user> <msg>  - Private message
/history          - Show recent messages
/status           - Connection status
```

**Expected join output:**
```
Connecting to WebSocket chat server at ws://localhost:8080/ws/chat...
✓ Connected to One Piece

  Chat Room:  #one-piece
  Online:     2 users
  Your Name:  alice
  Profile:    alice
  Connected:  2026-05-05 15:37:58

─────────────────────────────────────────────────────────────
You are now in chat. Type your message and press Enter.
Type /help for commands or /quit to leave.

alice>
```

**Multi-user and multi-room chat test flow:**
```powershell
# Terminal 1 (Alice):
$env:MANGAHUB_PROFILE = "alice"
.\mangahub.exe chat join one-piece
# Type: Hello One Piece fans!
# Type: /users
# Type: /pm bob Secret message!

# Terminal 2 (Bob):
$env:MANGAHUB_PROFILE = "bob"
.\mangahub.exe chat join one-piece
# Bob sees: 📜 Recent messages (Alice's history)
# Alice sees: 👋 bob joined the chat
# Bob types: Hey Alice!
# Alice sees: [15:40] bob: Hey Alice!
# Bob types: /quit
# Alice sees: 👋 bob left the chat

# Terminal 3 (Admin):
$env:MANGAHUB_PROFILE = "admin"
.\mangahub.exe chat join general
# Admin connects to General Chat and does NOT see Alice's messages
# Type: /users
# Admin sees themselves in General Chat and Alice/Bob in One Piece
```

**Expected `/quit` output:**
```
Leaving chat...
✓ Disconnected from chat server
  Session: 4m12s | Sent: 4 | Received: 11
```

---

### 8.10 gRPC Internal Service

Ensure you have started the standalone gRPC server first:
```powershell
go run ./cmd/grpc-server/
```

Test the gRPC commands via CLI:
```powershell
# Get a manga by ID via gRPC
.\mangahub.exe grpc manga get --id one-piece

# Search manga via gRPC
.\mangahub.exe grpc manga search --query naruto
.\mangahub.exe grpc manga search --genre Shounen --limit 5

# Update reading progress via gRPC
.\mangahub.exe grpc progress update --user-id user-alice --manga-id one-piece --chapter 500
```

---

## 9. Data Collection

### 9.1 Web Scraping (quotes.toscrape.com)

```powershell
# Scrape quotes
curl -s -X POST "http://localhost:8080/data/scrape-quotes?pages=2" `
  -H "Authorization: Bearer $ALICE_TOKEN"

# View scraped quotes
curl -s http://localhost:8080/data/scraped-quotes `
  -H "Authorization: Bearer $ALICE_TOKEN" | ConvertFrom-Json | ConvertTo-Json -Depth 3
```

### 9.2 HTTPBin Test

```powershell
curl -s -X POST http://localhost:8080/data/test-httpbin `
  -H "Authorization: Bearer $ALICE_TOKEN"
```

### 9.3 MangaDex API Import

```powershell
curl -s -X POST "http://localhost:8080/data/fetch-mangadex?limit=10" `
  -H "Authorization: Bearer $ALICE_TOKEN"
```

### 9.4 JSON Export

```powershell
# Export manga database to JSON
curl -s -X POST http://localhost:8080/data/export-files `
  -H "Authorization: Bearer $ALICE_TOKEN"

# View exported JSON
curl -s http://localhost:8080/data/export-json `
  -H "Authorization: Bearer $ALICE_TOKEN" | ConvertFrom-Json | ConvertTo-Json -Depth 2
```

### 9.5 JSON Import

```powershell
curl -s -X POST http://localhost:8080/data/import-json `
  -H "Authorization: Bearer $ALICE_TOKEN" `
  -H "Content-Type: application/json" `
  -d '{"path":"data/manga.json"}'
```

---

## 10. Multi-Terminal Sessions

### Profile-Based Session Isolation

Each terminal sets `MANGAHUB_PROFILE` to isolate sessions:

**Terminal 1 (Alice):**
```powershell
$env:MANGAHUB_PROFILE = "alice"
.\mangahub.exe auth login --username alice
.\mangahub.exe library list              # Shows Alice's library
.\mangahub.exe sync connect              # Connects as Alice
```

**Terminal 2 (Bob):**
```powershell
$env:MANGAHUB_PROFILE = "bob"
.\mangahub.exe auth login --username bob
.\mangahub.exe library list              # Shows Bob's library
.\mangahub.exe sync connect              # Connects as Bob
```

**Terminal 3 (Charlie):**
```powershell
$env:MANGAHUB_PROFILE = "charlie"
.\mangahub.exe auth register --username charlie
.\mangahub.exe auth login --username charlie
.\mangahub.exe auth status               # Shows charlie profile
```

### Config File Locations

```
C:\Users\Dell\.mangahub\
  profiles\
    default.json     <- no profile set
    alice.json       <- MANGAHUB_PROFILE=alice
    bob.json         <- MANGAHUB_PROFILE=bob
    charlie.json     <- MANGAHUB_PROFILE=charlie
```

### Alternative: MANGAHUB_TOKEN env var

```powershell
# Direct token override (skips config file)
$env:MANGAHUB_TOKEN = "eyJhbGci..."
.\mangahub.exe library list
```

---

## 11. End-to-End Workflow

This is the full test sequence from scratch:

```powershell
# == SETUP ==
Remove-Item .\data\mangahub.db -Force -ErrorAction SilentlyContinue
# Start server in Terminal 0:
go run ./cmd/api-server/
# Build CLI:
go build -o mangahub.exe ./cmd/cli/

# == TERMINAL 1: ALICE ==
$env:MANGAHUB_PROFILE = "alice"
.\mangahub.exe auth register --username alice        # Password: alice123
.\mangahub.exe auth login --username alice            # Password: alice123
.\mangahub.exe auth status
.\mangahub.exe manga search "one piece"
.\mangahub.exe manga info one-piece
.\mangahub.exe library add --manga-id one-piece --status reading
.\mangahub.exe library add --manga-id naruto --status plan-to-read
.\mangahub.exe library list
.\mangahub.exe progress update --manga-id one-piece --chapter 500
.\mangahub.exe library list                          # Verify chapter = 500
.\mangahub.exe sync status

# == TERMINAL 2: BOB ==
$env:MANGAHUB_PROFILE = "bob"
.\mangahub.exe auth register --username bob           # Password: bob123
.\mangahub.exe auth login --username bob              # Password: bob123
.\mangahub.exe library add --manga-id demon-slayer --status reading
.\mangahub.exe sync connect
# In sync session: progress demon-slayer 100
# In sync session: quit
.\mangahub.exe library list                          # Verify chapter = 100

# == TERMINAL 1: ALICE MONITORS ==
.\mangahub.exe sync monitor
# Should see Bob's updates in real-time
# Ctrl+C to exit

# == TERMINAL 3: UDP NOTIFICATIONS ==
.\mangahub.exe notify test
.\mangahub.exe notify subscribe
# In another terminal:
$env:MANGAHUB_PROFILE = "alice"
.\mangahub.exe notify send --type new_chapter --manga-id one-piece --message "Chapter 1121!"
# Terminal 3 should show: 📖 NEW CHAPTER: Chapter 1121!

# == TERMINAL 4: WEBSOCKET CHAT ==
$env:MANGAHUB_PROFILE = "alice"
.\mangahub.exe chat join general
# Type: Hello from Alice!
# In another terminal:
$env:MANGAHUB_PROFILE = "bob"
.\mangahub.exe chat join general
# Bob sees Alice's history, Alice sees "bob joined the chat"
# Bob types: Hey Alice!
# Alice sees Bob's message in real-time
# Bob types: /pm alice Secret!
# Alice sees: (PM from bob): Secret!
# Bob types: /quit
# Alice sees: "bob left the chat"
# Alice types: /quit

# == VERIFY CHAT HISTORY ==
.\mangahub.exe chat history general    # Shows all messages from the session

# == VERIFY ISOLATION ==
# Terminal 1:
$env:MANGAHUB_PROFILE = "alice"
.\mangahub.exe library list    # Shows One Piece, Naruto
# Terminal 2:
$env:MANGAHUB_PROFILE = "bob"
.\mangahub.exe library list    # Shows Demon Slayer only
```

---

## 12. Social & Community Features

These tests demonstrate both the **CLI commands** and the underlying **curl commands** (which you can use to import into Postman) to verify the new social features. Ensure you have tokens and profiles configured for Alice and Bob from Section 1.

### 12.1 User Reviews & Ratings

**Using CLI:**
```powershell
$env:MANGAHUB_PROFILE = "alice"
# 1. Create a review for a manga
.\mangahub.exe review add --manga-id one-piece --rating 9 --text "Amazing adventure, highly recommended!"

# 2. Get all reviews for a manga
.\mangahub.exe review list --manga-id one-piece

# 3. Get Alice's own reviews
.\mangahub.exe review mine
```

**Using curl (for Postman):**
```powershell
# 1. Create a review for a manga
curl -s -X POST http://localhost:8080/manga/one-piece/reviews `
  -H "Authorization: Bearer $ALICE_TOKEN" `
  -H "Content-Type: application/json" `
  -d '{"rating":9,"text":"Amazing adventure, highly recommended!"}'

# 2. Get all reviews for a manga
curl -s "http://localhost:8080/manga/one-piece/reviews" | ConvertFrom-Json | ConvertTo-Json -Depth 3

# 3. Get rating statistics for a manga
curl -s "http://localhost:8080/manga/one-piece/rating-stats" | ConvertFrom-Json | ConvertTo-Json -Depth 3

# 4. Get Alice's own reviews
curl -s "http://localhost:8080/users/reviews" `
  -H "Authorization: Bearer $ALICE_TOKEN" | ConvertFrom-Json | ConvertTo-Json -Depth 3
```

### 12.2 Friend System

**Using CLI:**
```powershell
# Terminal 1 (Alice): Send friend request
$env:MANGAHUB_PROFILE = "alice"
.\mangahub.exe friend add --id user-bob

# Terminal 2 (Bob): View pending and accept
$env:MANGAHUB_PROFILE = "bob"
.\mangahub.exe friend pending
.\mangahub.exe friend accept --id user-alice

# Terminal 1 (Alice): View friends list
$env:MANGAHUB_PROFILE = "alice"
.\mangahub.exe friend list

# Terminal 1 (Alice): Remove friend
.\mangahub.exe friend remove --id user-bob
```

**Using curl (for Postman):**
```powershell
# 1. Get Bob's dynamic user ID
$bobInfo = curl -s -H "Authorization: Bearer $BOB_TOKEN" http://localhost:8080/auth/status | ConvertFrom-Json
$bobId = $bobInfo.data.user_id

# 2. Alice sends a friend request to Bob
curl -s -X POST http://localhost:8080/friends/add `
  -H "Authorization: Bearer $ALICE_TOKEN" `
  -H "Content-Type: application/json" `
  -d "{`"friend_id`":`"$bobId`"}"

# 3. Bob views pending friend requests
curl -s -H "Authorization: Bearer $BOB_TOKEN" http://localhost:8080/users/friends/pending | ConvertFrom-Json | ConvertTo-Json

# 4. Get Alice's dynamic user ID
$aliceInfo = curl -s -H "Authorization: Bearer $ALICE_TOKEN" http://localhost:8080/auth/status | ConvertFrom-Json
$aliceId = $aliceInfo.data.user_id

# 5. Bob accepts Alice's friend request
curl -s -X POST http://localhost:8080/friends/$aliceId/accept `
  -H "Authorization: Bearer $BOB_TOKEN"

# 6. Alice views her friends list
curl -s -H "Authorization: Bearer $ALICE_TOKEN" http://localhost:8080/users/friends | ConvertFrom-Json | ConvertTo-Json
```

### 12.3 Reading Lists Sharing

**Using CLI:**
```powershell
$env:MANGAHUB_PROFILE = "alice"
# 1. Create a shared list
.\mangahub.exe sharedlist create --name "Top Shounen" --manga-ids "one-piece,naruto" --public

# 2. View own lists
.\mangahub.exe sharedlist mine

# 3. View public lists
.\mangahub.exe sharedlist public
```

**Using curl (for Postman):**
```powershell
# 1. Alice creates a shared reading list
curl -s -X POST http://localhost:8080/reading-lists/create `
  -H "Authorization: Bearer $ALICE_TOKEN" `
  -H "Content-Type: application/json" `
  -d '{"name":"Top Shounen","manga_ids":["one-piece","naruto"],"is_public":true}'

# 2. View all public reading lists (Bob can see this)
curl -s "http://localhost:8080/reading-lists/public" | ConvertFrom-Json | ConvertTo-Json -Depth 3

# 3. Bob subscribes to Alice's list
# (Replace LIST_ID with the one from the 'public' response)
curl -s -X POST "http://localhost:8080/reading-lists/LIST_ID/subscribe" `
  -H "Authorization: Bearer $BOB_TOKEN"

# 4. Bob views his subscribed lists
curl -s "http://localhost:8080/reading-lists/subscribed" `
  -H "Authorization: Bearer $BOB_TOKEN" | ConvertFrom-Json | ConvertTo-Json

# 5. Alice adds another manga to her list
curl -s -X POST "http://localhost:8080/reading-lists/LIST_ID/manga" `
  -H "Authorization: Bearer $ALICE_TOKEN" `
  -H "Content-Type: application/json" `
  -d '{"manga_id":"berserk"}'

# 6. Alice removes a manga from her list
curl -s -X DELETE "http://localhost:8080/reading-lists/LIST_ID/manga/naruto" `
  -H "Authorization: Bearer $ALICE_TOKEN"

# 7. Bob unsubscribes from the list
curl -s -X DELETE "http://localhost:8080/reading-lists/LIST_ID/subscribe" `
  -H "Authorization: Bearer $BOB_TOKEN"
```

### 12.4 Activity Feed

**Using CLI:**
```powershell
# Bob views his friends' activities (will see Alice's)
$env:MANGAHUB_PROFILE = "bob"
.\mangahub.exe feed view

# Alice views her own activities
$env:MANGAHUB_PROFILE = "alice"
.\mangahub.exe feed mine

# Alice creates a custom activity post
$env:MANGAHUB_PROFILE = "alice"
.\mangahub.exe feed post "Just started watching the new anime adaptation!"
```

**Using curl (for Postman):**
```powershell
# 1. Create a custom activity post
curl -s -X POST http://localhost:8080/feed/activities `
  -H "Authorization: Bearer $ALICE_TOKEN" `
  -H "Content-Type: application/json" `
  -d '{"message":"Just started watching the new anime adaptation!"}'
# 2. View Bob's activity feed (will show Alice's activities since they are friends)
curl -s -H "Authorization: Bearer $BOB_TOKEN" "http://localhost:8080/feed/activities" | ConvertFrom-Json | ConvertTo-Json -Depth 3

# 3. View Alice's own activity history
curl -s -H "Authorization: Bearer $ALICE_TOKEN" "http://localhost:8080/users/$aliceId/activities" | ConvertFrom-Json | ConvertTo-Json -Depth 3
```

---

## 13. Enhanced TCP Synchronization — Conflict Resolution

### 13.1 View Current Strategy

```powershell
# CLI
.\mangahub.exe sync strategy

# curl
curl -s http://localhost:8080/sync/strategy `
  -H "Authorization: Bearer $ALICE_TOKEN" | ConvertFrom-Json | ConvertTo-Json
```

**Expected:** Shows `last_write_wins` as default strategy.

### 13.2 Change Strategy

```powershell
# CLI
.\mangahub.exe sync strategy merge

# curl
curl -s -X PUT http://localhost:8080/sync/strategy `
  -H "Authorization: Bearer $ALICE_TOKEN" `
  -H "Content-Type: application/json" `
  -d '{"strategy":"merge"}'
```

**Available strategies:** `last_write_wins`, `merge`, `user_choice`

### 13.3 Trigger a Conflict (via TCP sync connect)

```powershell
# Terminal 1 (Alice): Connect to sync
$env:MANGAHUB_PROFILE = "alice"
.\mangahub.exe sync connect

# Inside the sync session:
progress one-piece 500
# Then send a conflicting update:
progress one-piece 300

# With "merge" strategy → server keeps ch.500 (higher chapter)
# With "last_write_wins" → server accepts ch.300 (latest)
# With "user_choice" → server rejects ch.300 (conflict notification sent)

# Change strategy at runtime inside the session:
strategy merge
progress one-piece 200
# → Conflict auto-resolved: ch.500 kept (merge picks higher)
```

### 13.4 View Conflict Log

```powershell
# CLI
.\mangahub.exe sync conflicts

# curl
curl -s http://localhost:8080/sync/conflicts `
  -H "Authorization: Bearer $ALICE_TOKEN" | ConvertFrom-Json | ConvertTo-Json -Depth 4
```

**Expected:** Shows table of resolved conflicts with manga, existing/incoming chapters, devices, strategy used, and resolution.

### 13.5 Conflict Resolution Strategies Explained

| Strategy | Behavior |
|----------|----------|
| `last_write_wins` | Always accept the latest update, overwrite previous (default) |
| `merge` | Keep the higher chapter number (furthest reading progress) |
| `user_choice` | Reject conflicting updates, send conflict notification to user |

---

## 14. Data Export/Import

### 14.1 Export Library to JSON (CLI)

```powershell
# Export your library as JSON
.\mangahub.exe export library --format json --output library.json
```

**Expected output:**
```
Exporting library for alice...
✓ Library exported successfully!
  Entries:  3
  Format:   JSON
  File:     C:\Users\Dell\Documents\Go\mangahub\library.json

Import back: mangahub import library --file library.json
```

**Verify the exported file:**
```powershell
Get-Content library.json | ConvertFrom-Json | ConvertTo-Json -Depth 3
```

### 14.2 Export Library to CSV (CLI)

```powershell
# Export library as CSV
.\mangahub.exe export library --format csv --output library.csv
```

**Verify CSV:**
```powershell
Get-Content library.csv
```

**Expected CSV content:**
```
manga_id,current_chapter,status,updated_at
one-piece,1095,reading,2026-05-10T12:00:00+07:00
naruto,0,plan_to_read,2026-05-10T11:00:00+07:00
```

### 14.3 Export Reading Progress to CSV (CLI)

```powershell
# Export progress as CSV (default format)
.\mangahub.exe export progress --format csv --output progress.csv

# Or as JSON
.\mangahub.exe export progress --format json --output progress.json
```

**Expected output:**
```
Exporting progress for alice...
✓ Progress exported successfully!
  Entries:  3
  Format:   CSV
  File:     C:\Users\Dell\Documents\Go\mangahub\progress.csv

Import back: mangahub import progress --file progress.csv
```

### 14.4 Full Data Export as tar.gz Archive (CLI)

```powershell
# Create a full backup archive
.\mangahub.exe export all --output mangahub-backup.tar.gz
```

**Expected output:**
```
Creating full backup for alice...
  ✓ library.json
  ✓ reviews.json
  ✓ friends.json
  ✓ progress.csv
  ✓ metadata.json

✓ Full backup created successfully!
  Files:    5
  Size:     2.3 KB
  Archive:  C:\Users\Dell\Documents\Go\mangahub\mangahub-backup.tar.gz
```

### 14.5 Import Library from JSON (CLI)

```powershell
# Import library entries from a previously exported JSON file
.\mangahub.exe import library --file library.json
`````````

**Expected output:**
```
Importing library from library.json...

✓ Library import complete!
  Imported: 2 entries
  Skipped:  1 (already in library)
```
```

### 14.6 Import Progress from CSV (CLI)

```powershell
# Import progress from a CSV file
.\mangahub.exe import progress --file progress.csv
```

**Expected output:**
```
Importing progress from progress.csv...

✓ Progress import complete!
  Updated: 3 entries
```

### 14.7 Import Manga from JSON (CLI)

```powershell
# Import manga data from a JSON file
.\mangahub.exe import manga --file manga.json
```

### 14.8 Export via API (curl / Postman)

All export API endpoints require authentication and return downloadable files.
CSV exports also save a copy to the server's `./data/` directory using `csv_storage.go`.

| Endpoint | Format | Server File Saved |
|----------|--------|-------------------|
| `GET /data/export/library?format=json` | JSON | — |
| `GET /data/export/library?format=csv` | CSV | `./data/library.csv` |
| `GET /data/export/progress?format=csv` | CSV | `./data/progress.csv` |
| `GET /data/export/progress?format=json` | JSON | — |
| `GET /data/export/manga?format=json` | JSON | `./data/manga_export.json` |
| `GET /data/export/manga?format=csv` | CSV | `./data/manga.csv` |
| `GET /data/export/full` | JSON | — |

```powershell
# Export library as JSON (file download)
curl -s "http://localhost:8080/data/export/library?format=json" `
  -H "Authorization: Bearer $ALICE_TOKEN" -o library-api.json

# Export library as CSV (also saves ./data/library.csv on server)
curl -s "http://localhost:8080/data/export/library?format=csv" `
  -H "Authorization: Bearer $ALICE_TOKEN" -o library-api.csv

# Export progress as CSV (also saves ./data/progress.csv on server)
curl -s "http://localhost:8080/data/export/progress?format=csv" `
  -H "Authorization: Bearer $ALICE_TOKEN" -o progress-api.csv

# Export progress as JSON
curl -s "http://localhost:8080/data/export/progress?format=json" `
  -H "Authorization: Bearer $ALICE_TOKEN" -o progress-api.json

# Export manga database as JSON (also saves ./data/manga_export.json on server)
curl -s "http://localhost:8080/data/export/manga?format=json" `
  -H "Authorization: Bearer $ALICE_TOKEN" -o manga-api.json

# Export manga database as CSV (also saves ./data/manga.csv on server)
curl -s "http://localhost:8080/data/export/manga?format=csv" `
  -H "Authorization: Bearer $ALICE_TOKEN" -o manga-api.csv

# Full data export (user_id + library combined)
curl -s "http://localhost:8080/data/export/full" `
  -H "Authorization: Bearer $ALICE_TOKEN" -o full-export.json
```

**Verify saved server files:**
```powershell
# After calling the CSV export endpoints, check files on disk:
Get-Content ./data/library.csv
Get-Content ./data/progress.csv
Get-Content ./data/manga.csv
```

### 14.9 Complete Export → Import Workflow

This test verifies the full round-trip: export from one account, import into another.

```powershell
# == Step 1: Export Alice's data ==
$env:MANGAHUB_PROFILE = "alice"
.\mangahub.exe export library --format json --output alice-library.json
.\mangahub.exe export progress --format csv --output alice-progress.csv
.\mangahub.exe export all --output alice-backup.tar.gz

# == Step 2: Switch to Bob and import ==
$env:MANGAHUB_PROFILE = "bob"
.\mangahub.exe import library --file alice-library.json
.\mangahub.exe import progress --file alice-progress.csv

# == Step 3: Verify Bob now has Alice's manga ==
.\mangahub.exe library list
```

---

## Checklist Summary

### Social & Community Features (26 points)
| Feature | Test Section | Status |
|---------|-------------|--------|
| User Reviews & Ratings | S12.1 | Done |
| Friend System | S12.2 | Done |
| Reading Lists Sharing | S12.3 | Done |
| Activity Feed | S12.4 | Done |

### Task 1: Data Collection and JSON Storage
| Feature | Test Section | Status |
|---------|-------------|--------|
| 100 seeded manga with cover_url | S1 Setup | Done |
| MangaDex API integration | S9.3 | Done |
| Web scraping (quotes.toscrape.com) | S9.1 | Done |
| HTTPBin test | S9.2 | Done |
| JSON export/import | S9.4, S9.5 | Done |

### Task 2: TCP Progress Sync Server (20 points)
| Feature | Test Section | Status |
|---------|-------------|--------|
| internal/tcp/server.go | -- | Done |
| internal/tcp/handler.go | -- | Done |
| internal/tcp/protocol.go | -- | Done |
| cmd/tcp-server/main.go | -- | Done |
| Accept multiple TCP connections | S7.1 | Done |
| Broadcast to ALL clients | S7.1 | Done |
| Graceful connect/disconnect | S7.1 | Done |
| JSON message protocol (newline-delimited) | S7.2 | Done |
| Concurrent goroutines | -- | Done |
| HTTP PUT /users/progress triggers TCP broadcast | S6.1, S7.3 | Done |
| TCP progress persists to DB | S7.3 | Done |
| CLI: sync connect | S8.6 | Done |
| CLI: sync disconnect | S8.6 | Done |
| CLI: sync status | S8.6 | Done |
| CLI: sync monitor | S8.6 | Done |

### Auth and User Management
| Feature | Test Section | Status |
|---------|-------------|--------|
| Register | S3.1 | Done |
| Login (JWT) | S3.2 | Done |
| Auth Status | S3.3 | Done |
| Logout | S3.4 | Done |
| Change Password | S3.5 | Done |
| Per-terminal sessions (profiles) | S10 | Done |

### Task 3: UDP Notification System (15 points)
| Feature | Test Section | Status |
|---------|-------------|--------|
| internal/udp/server.go | -- | Done |
| internal/udp/notifier.go | -- | Done |
| cmd/udp-server/main.go | -- | Done |
| UDP server listening for registrations | S8.8 | Done |
| Broadcast notifications to all clients | S8.8 | Done |
| Client list management (add/remove) | S8.8 | Done |
| Fire-and-forget delivery (no ACK) | -- | Done |
| Basic error logging | -- | Done |
| CLI: notify subscribe | S8.8 | Done |
| CLI: notify unsubscribe | S8.8 | Done |
| CLI: notify test | S8.8 | Done |
| CLI: notify send (via HTTP) | S8.8 | Done |
| HTTP: POST /notify/broadcast | S8.8 | Done |
| HTTP: GET /notify/status | S8.8 | Done |

### Task 4: WebSocket Chat System (15 points)
| Feature | Test Section | Status |
|---------|-------------|--------|
| internal/websocket/hub.go | -- | Done |
| internal/websocket/client.go | -- | Done |
| GET /ws/chat upgrade handler | S8.9 | Done |
| GET /chat/history API endpoint | S8.9 | Done |
| Hub.Run() as central goroutine | -- | Done |
| Real-time message broadcasting | S8.9 | Done |
| User join/leave notifications | S8.9 | Done |
| Private messaging (/pm) | S8.9 | Done |
| Connection lifecycle management | S8.9 | Done |
| Per-client writePump (no concurrent writes) | -- | Done |
| Chat history (in-memory, last 50) | S8.9 | Done |
| Interactive commands (/help, /users, /quit, /status) | S8.9 | Done |
| CLI: chat join | S8.9 | Done |
| CLI: chat send | S8.9 | Done |
| CLI: chat history | S8.9 | Done |

### Task 5: gRPC Internal Service (10 points)
| Feature | Test Section | Status |
|---------|-------------|--------|
| proto/manga.proto | -- | Done |
| internal/grpc/pb/ | -- | Done |
| internal/grpc/server.go | -- | Done |
| internal/grpc/client.go | -- | Done |
| cmd/grpc-server/main.go | S8.10 | Done |
| Unary RPC: GetManga | S8.10 | Done |
| Unary RPC: SearchManga | S8.10 | Done |
| Unary RPC: UpdateProgress | S8.10 | Done |
| CLI: grpc manga get | S8.10 | Done |
| CLI: grpc manga search | S8.10 | Done |
| CLI: grpc progress update | S8.10 | Done |

### Remaining / Bonus Features
| Task | Status |
|------|--------|
| Docker Compose | Done |
| Input Sanitization (5 pts) | Done |
| OpenAPI / Swagger Documentation (5 pts) | Done |
| GitHub Actions CI/CD Pipeline (10 pts) | Done |
| UDP Delivery Confirmation — ACK system (5 pts) | Done |
| gRPC Server-side Streaming (10 pts) | Done |

### Data Export/Import (10 points)
| Feature | Test Section | Status |
|---------|-------------|--------|
| Export library to JSON | S14.1 | Done |
| Export library to CSV | S14.2 | Done |
| Export progress to CSV | S14.3 | Done |
| Export progress to JSON | S14.3 | Done |
| Full data export (tar.gz) | S14.4 | Done |
| Import library from JSON | S14.5 | Done |
| Import progress from CSV | S14.6 | Done |
| Import manga from JSON/CSV | S14.7 | Done |
| API export endpoints (JSON/CSV download) | S14.8 | Done |
| CSV storage functions | -- | Done |
| MangaDex external import | S9.3 | Done |

---

## S15. Input Sanitization

### S15.1 Manga Create — HTML Injection Rejected
```powershell
# Should return 400 "Invalid title: input must not contain < or >"
curl -s -X POST http://localhost:8080/manga `
  -H "Authorization: Bearer $ALICE_TOKEN" `
  -H "Content-Type: application/json" `
  -d '{"id":"test-xss","title":"<script>alert(1)</script>","author":"Hacker"}'
```

### S15.2 Manga Create — SQL Injection Rejected
```powershell
# Should return 400 — ID contains invalid character ";"
curl -s -X POST http://localhost:8080/manga `
  -H "Authorization: Bearer $ALICE_TOKEN" `
  -H "Content-Type: application/json" `
  -d '{"id":"id=1; DROP TABLE manga--","title":"Normal Title"}'
```

### S15.3 Review Text — Length Enforced
```powershell
# Should return 400 "Invalid review text: input exceeds maximum length of 2000 characters"
$longText = "a" * 2001
curl -s -X POST http://localhost:8080/manga/one-piece/reviews `
  -H "Authorization: Bearer $ALICE_TOKEN" `
  -H "Content-Type: application/json" `
  -d "{\"rating\":8,\"text\":\"$longText\"}"
```

### S15.4 Valid Request Still Works
```powershell
# Should return 201 Created
curl -s -X POST http://localhost:8080/manga `
  -H "Authorization: Bearer $ALICE_TOKEN" `
  -H "Content-Type: application/json" `
  -d '{"id":"my-manga","title":"My Manga & Brotherhood","author":"Author Name"}'
```

---

## S16. OpenAPI / Swagger Documentation

### S16.1 Access Swagger UI
Open in browser: **http://localhost:8080/swagger/index.html**

- Click **Authorize** (top-right) → enter `Bearer <your-token>`
- All 43 endpoints are listed across 7 tags: `auth`, `manga`, `users`, `reviews`, `friends`, `reading-lists`, `feed`
- Click any endpoint → **Try it out** → **Execute**

### S16.2 Verify Swagger JSON Spec
```powershell
# Should return valid JSON with paths object
curl -s http://localhost:8080/swagger/doc.json | python3 -c "
import sys, json
spec = json.load(sys.stdin)
print(f'Paths: {len(spec[\"paths\"])}')
print(f'Info: {spec[\"info\"][\"title\"]} v{spec[\"info\"][\"version\"]}')
"
```
Expected output:
```
Paths: 34
Info: MangaHub API v1.0
```

---

## S17. GitHub Actions CI/CD Pipeline

### S17.1 Trigger CI
Push any commit to `main` or open a PR:
```powershell
git push origin main
```
Go to **github.com/anhhuynh1707/MangaHub → Actions** tab.

### S17.2 Expected Jobs
| Job | Description | Trigger |
|-----|-------------|---------|
| **Build & Test** | Go build + unit tests + go vet | Push / PR |
| **Docker Build & Smoke Test** | `docker compose build` + `up -d` + health check | After Build & Test passes |
| **Publish to GHCR** | Pushes `ghcr.io/anhhuynh1707/mangahub:latest` | Push to main only |

### S17.3 Pull Published Docker Image
```powershell
docker pull ghcr.io/anhhuynh1707/mangahub:latest
docker run -p 8080:8080 ghcr.io/anhhuynh1707/mangahub:latest
```

---

## S18. UDP Delivery Confirmation (ACK System)

> The server sends a `notification_id` with each broadcast. Clients ACK within 3 seconds.
> After the window closes, a `DeliveryRecord` reports who acknowledged.

### S18.1 Register a Client (Terminal 1)
```bash
# Linux/macOS: listen on a random port and register
nc -u localhost 9091 <<< '{"type":"register"}'
# Keep this terminal open — it will receive broadcast + ack prompt
```

### S18.2 Send Broadcast-With-ACK (Terminal 2)
```powershell
# Returns delivery record after 3 seconds
curl -s -X POST http://localhost:8080/notify/broadcast-ack `
  -H "Authorization: Bearer $ALICE_TOKEN" `
  -H "Content-Type: application/json" `
  -d '{"type":"new_chapter","manga_id":"one-piece","message":"Chapter 1121!"}'
```

Expected response (after 3s):
```json
{
  "success": true,
  "data": {
    "notif_id": "notif-1717481234567890000",
    "message": "Chapter 1121!",
    "sent_to": ["127.0.0.1:54321"],
    "acked_by": [],
    "unacked": ["127.0.0.1:54321"],
    "ack_rate": 0.0,
    "timed_out": true
  }
}
```

### S18.3 Send ACK from Client
```bash
# In the nc terminal, send the ACK with the notification_id received
echo '{"type":"ack","notification_id":"notif-1717481234567890000"}' | nc -u localhost 9091
```

### S18.4 View Delivery History
```powershell
curl -s http://localhost:8080/notify/ack-stats `
  -H "Authorization: Bearer $ALICE_TOKEN"
```

---

## S19. gRPC Server-side Streaming

> Two new streaming RPCs: `StreamSearch` (streams results one-by-one) and
> `WatchMangaUpdates` (long-lived stream receiving live events).
> Requires `grpcurl` or the Go gRPC client.

### S19.1 Install grpcurl (if not installed)
```bash
brew install grpcurl          # macOS
# or: go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
```

### S19.2 StreamSearch — Stream Results One by One
```bash
# Each manga is sent as a separate message, not one big response
grpcurl -plaintext \
  -H "authorization: Bearer $ALICE_TOKEN" \
  -d '{"query":"one","limit":5}' \
  localhost:9092 mangahub.MangaService/StreamSearch
```

Expected: 5 separate JSON objects printed one after another as they stream.

### S19.3 WatchMangaUpdates — Live Event Stream (Terminal 1)
```bash
# Keep this running — it blocks waiting for events
grpcurl -plaintext \
  -H "authorization: Bearer $ALICE_TOKEN" \
  -d '{"manga_id":"","user_id":"user-alice"}' \
  localhost:9092 mangahub.MangaService/WatchMangaUpdates
```

Expected initial response:
```json
{
  "event_type": "connected",
  "message": "Watching manga updates (filter: \"\")",
  "timestamp": 1717481234
}
```

### S19.4 Trigger a Live Event (Terminal 2)
```powershell
# Update progress — this publishes to the gRPC event hub
curl -s -X PUT http://localhost:8080/users/progress `
  -H "Authorization: Bearer $ALICE_TOKEN" `
  -H "Content-Type: application/json" `
  -d '{"manga_id":"one-piece","current_chapter":1095,"status":"reading"}'
```

Terminal 1 should immediately receive:
```json
{
  "event_type": "progress_updated",
  "manga_id": "one-piece",
  "user_id": "user-alice",
  "chapter": 1095,
  "message": "User user-alice reached chapter 1095 of one-piece",
  "timestamp": 1717481290
}
```

### S19.5 Filter by Specific Manga
```bash
# Only receive events for "naruto", ignoring all others
grpcurl -plaintext \
  -H "authorization: Bearer $ALICE_TOKEN" \
  -d '{"manga_id":"naruto","user_id":"user-alice"}' \
  localhost:9092 mangahub.MangaService/WatchMangaUpdates
```

---

## S20. Advanced Search & Filtering CLI

### S20.1 Multi-Genre Filter
```powershell
# All manga with both action AND adventure genres
mangahub manga advanced --genres action,adventure
```

### S20.2 Keyword + Sort by Rating
```powershell
# Search "one" sorted by average review rating (highest first)
mangahub manga advanced one --sort rating
```

### S20.3 Minimum Rating Filter
```powershell
# Only manga with average review >= 8.0, sorted by popularity
mangahub manga advanced --min-rating 8 --sort popularity
```

### S20.4 Status + Genre + Pagination
```powershell
# Ongoing romance manga, page 2
mangahub manga advanced --genres romance --status ongoing --page 2 --limit 5
```

### S20.5 Equivalent HTTP Request (for reference)
```powershell
curl -s -X POST http://localhost:8080/manga/search `
  -H "Authorization: Bearer $ALICE_TOKEN" `
  -H "Content-Type: application/json" `
  -d '{
    "search": "one",
    "genres": ["action", "adventure"],
    "min_rating": 7.5,
    "sort_by": "rating",
    "page": 1,
    "limit": 10
  }'
```

---

## S21. Recommendation System CLI

### S21.1 Prerequisites — Add Manga to Library First
```powershell
# The engine needs reading history to generate recommendations
mangahub library add --manga-id one-piece --status reading
mangahub library add --manga-id naruto --status completed
mangahub library add --manga-id bleach --status completed
mangahub progress update --manga-id one-piece --chapter 1095
```

### S21.2 Get Recommendations (Default — Top 10)
```powershell
mangahub manga recommend
```

Expected output:
```
🤖 Generating personalised recommendations...
   (based on your reading history and similar users)

📚 Your Reading Profile:
   Read: 3 manga | Completed: 2 | Similar users found: 1
   Favourite genres: action, adventure, shounen

🌟 Top 10 Recommendations for user-alice:

   1. Attack on Titan              score: 1.75
      Author: Hajime Isayama       Status: completed
      Genres: action, drama, fantasy
      Reason: similar to naruto
      ID: attack-on-titan
   ...
```

### S21.3 Limit Results
```powershell
mangahub manga recommend --limit 3
```

### S21.4 Equivalent HTTP Request
```powershell
curl -s http://localhost:8080/users/recommendations?limit=10 `
  -H "Authorization: Bearer $ALICE_TOKEN"
```

---

## S22. UDP ACK CLI

### S22.1 Send Broadcast with Delivery Confirmation (Terminal 2)
> First subscribe in Terminal 1: `mangahub notify subscribe`

```powershell
# Blocks for up to 3s waiting for ACKs, then prints delivery report
mangahub notify send-ack `
  --type new_chapter `
  --manga-id one-piece `
  --message "Chapter 1121 released!"
```

Expected output after 3 seconds:
```
📡 Sending broadcast with ACK tracking (waiting up to 3s)...
   Type: new_chapter | Message: Chapter 1121 released!

📊 Delivery Report — notif-1717481234567890000
   Sent to:     1 client(s)
   ACK'd:       0 client(s)
   Unacked:     1 client(s)
   ACK rate:    0%
   ✗ No reply:  [127.0.0.1:54321]
   ⚠  Some clients did not ACK within 3s (fire-and-forget still delivered)
```

### S22.2 Subscriber Sends ACK (Terminal 1)
The `mangahub notify subscribe` command automatically ACKs tracked notifications.
To manually ACK via netcat:
```bash
# Replace the notification_id with the one received
echo '{"type":"ack","notification_id":"notif-1717481234567890000"}' | nc -u localhost 9091
```

### S22.3 View Delivery History
```powershell
mangahub notify ack-stats
```

Expected output:
```
📊 Delivery History (3 records):

  Notification ID                    | Message                   | Sent | ACK'd | Rate | Timeout
  notif-1717481234567890000          | Chapter 1121 released!    |  2   |  1    |  50% | Yes
  notif-1717481200000000000          | Test notification         |  1   |  1    | 100% | No
```

---

## S23. gRPC Streaming CLI

### S23.1 StreamSearch — Results Streamed One by One

```powershell
# Results arrive individually rather than as a single response
mangahub grpc manga stream --query "one" --limit 5
```

Expected output:
```
📡 Streaming search results for "one" (server-side streaming)...

  [ 1] One Piece                        Eiichiro Oda         ongoing      518 ch | adventure, action, ...
  [ 2] One Punch Man                    ONE                  ongoing       200 ch | action, comedy, ...
  [ 3] Monster                          Naoki Urasawa        completed     162 ch | mystery, drama, ...
  [ 4] Fullmetal Alchemist              Hiromu Arakawa       completed     116 ch | action, adventure, ...
  [ 5] One-Punch Man                    Yusuke Murata        ongoing       195 ch | action, comedy, ...

✓ Stream complete — received 5 results
```

### S23.2 WatchMangaUpdates — Live Event Stream (Terminal 1)

```powershell
# Blocks and streams live events — press Ctrl+C to stop
mangahub grpc watch
```

Expected after connection:
```
📺 Watching ALL manga update events (press Ctrl+C to stop)...
   Events stream live as users update progress or manga is changed.

[10:30:01] ✓ Connected — Watching manga updates (filter: "")
```

### S23.3 Trigger Live Events (Terminal 2)

```powershell
# Update progress — Terminal 1 will instantly receive this
mangahub progress update --manga-id one-piece --chapter 1096
```

Terminal 1 shows immediately:
```
[10:30:15] 📖 PROGRESS  manga=one-piece            ch=1096   user=user-alice
```

### S23.4 Watch a Specific Manga Only

```powershell
mangahub grpc watch --manga-id naruto
```

Only events for `naruto` will appear; all other manga updates are filtered out.

### S23.5 gRPC Watch vs grpcurl (both work)

```bash
# CLI way (above)
mangahub grpc watch --manga-id one-piece

# grpcurl way (equivalent)
grpcurl -plaintext \
  -H "authorization: Bearer $TOKEN" \
  -d '{"manga_id":"one-piece","user_id":"user-alice"}' \
  localhost:9092 mangahub.MangaService/WatchMangaUpdates
```

---

## Frontend E2E Tests (Playwright)

End-to-end tests live in `frontend/e2e/` and drive a real browser through the
full user journey: **register → login → add manga to library → update progress →
leave a review → join chat → clean up**. The test seeds its own manga via the API
(so it doesn't depend on the MangaDex seed) and deletes it (and its library entry,
review, and activity rows) at the end.

**Prerequisites:** the backend running on `:8080`. Playwright auto-starts the
Vite dev server itself.

```bash
# 1. Backend (must include the ClearActivityFeed fix used by the cleanup step):
go run ./cmd/api-server/
#    or: docker compose up -d --build mangahub-api redis

# 2. Run the tests (from frontend/):
cd frontend
npm run test:e2e          # headless
npm run test:e2e:ui       # interactive UI mode — watch each step
npx playwright show-report
```

First time only: `npx playwright install chromium`.

**Pointing at a different backend / running app:**

```bash
# Run against an already-running app (e.g. the Docker frontend on :3000):
E2E_BASE_URL=http://localhost:3000 E2E_API_URL=http://localhost:8080 npm run test:e2e
```

- `E2E_BASE_URL` — where the app is served (default `http://localhost:5173`,
  which the config auto-starts). When set, the Vite dev server is **not** started.
- `E2E_API_URL` — backend base the test calls directly for setup/cleanup
  (default `http://localhost:8080`).

**In CI:** the `e2e` job in `.github/workflows/ci.yml` builds + starts the Go
backend, installs the Chromium browser, runs `npm run test:e2e`, and uploads the
Playwright HTML report as an artifact.

---

## Frontend API types (generated)

The frontend's request types are generated from the backend Swagger spec:

```bash
cd frontend
npm run gen:api     # swagger2openapi (2.0→3.0) → openapi-typescript → src/api/schema.d.ts
```

This also runs automatically as a `prebuild` step (`npm run build`). If the spec
is missing/broken, the build falls back to the committed `src/api/schema.d.ts`.
When the backend API changes, re-copy the spec
(`cp docs/swagger.json frontend/openapi.json`) and re-run `gen:api`.

---

> **Note:** After resetting the database, all users must be re-registered.
> Passwords for test users: alice -> alice123, bob -> bob123
