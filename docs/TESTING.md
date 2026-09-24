# MangaHub — Testing Guide

> **Last updated:** 2026-08-31
> **Status:** application, CI, and local production-style gates implemented;
> manual AWS/EC2 evidence is still pending
> **Development ports:** frontend `:3000` | API/WebSocket/SSE `:8080` |
> TCP `:9090` | UDP `:9091/udp` | gRPC `:9092`
> **Production-style port:** edge `:8088` locally or `:80` on the planned EC2 host

---

## Table of Contents

0. [Start Here — Testing the Whole System](#0-start-here--testing-the-whole-system)
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
13. [Frontend E2E Tests](#frontend-e2e-tests-playwright)
14. [DevSecOps, Production, and AWS Verification](#devsecops-production-and-aws-verification)

---

## 0. Start Here — Testing the Whole System

This guide has two levels:

- Sections 1–24 are feature recipes for HTTP, CLI, TCP, UDP, WebSocket, gRPC,
  SSE, data import/export, and the React frontend.
- The final DevSecOps section is the system release ladder: source checks,
  production Compose, security, immutable images, EC2, rollback, backup, and
  monitoring.

Read [`CODE_FLOW.md`](CODE_FLOW.md) Section 0 first if you do not yet know which
component a test is exercising.

### 0.1 The validation ladder

Run the lowest useful layer first. A higher layer complements the lower layers;
it does not replace them.

| Layer | What a pass means | Where it runs |
|---|---|---|
| 1. Static/unit | Go behavior, types, lint, and frontend build are sound | Developer machine + CI |
| 2. Feature/integration | HTTP and each real-time protocol behave as intended | Developer machine |
| 3. Browser E2E | A real user journey crosses React and the API | Developer machine + CI |
| 4. Development Compose | All directly exposed learning services start together | Docker + CI smoke test |
| 5. Production-style Compose | Edge routing, private services, hardening, persistence, and optional raw mode work | Developer machine |
| 6. Security/operations | Dependencies, secrets, images, shell scripts, and operational contracts pass policy | CI; selected local checks |
| 7. Immutable release | Matching backend/frontend images exist for one full Git SHA | GHCR after green branch CI |
| 8. AWS runtime | The exact release works on the documented Sydney EC2 environment | Manual; not yet run |
| 9. Recovery/monitoring | EC2 rollback, backup/restore, metrics, logs, alarms, and failure rehearsal are proven | Manual; not yet run on AWS |

### 0.2 Fast feedback before a push

From the repository root:

```bash
go test -timeout 120s ./...
go vet ./...

cd frontend
npm ci
npx tsc --noEmit
npm run lint
npm run build
cd ..
```

The complete Go suite includes socket tests. If a restricted execution
environment blocks opening a loopback UDP socket, that is an environment denial,
not automatically an application failure; rerun in a normal local terminal or
CI and record the actual result.

### 0.3 Test-data and secret rules

- Use generated, disposable JWT secrets; never paste an AWS key, production
  token, or MFA code into a command, screenshot, issue, or committed file.
- The root development database is local test data. Do not use reset commands
  against an EC2 volume.
- Stop production-style Compose with `down`, never `down -v`; `-v` deletes the
  SQLite and backup volumes.
- Raw protocol ports are loopback-only in local production tests. On EC2 they
  require temporary owner IPv4 `/32` Security Group rules.
- Do not mark an AWS evidence row `PASS` from a local test or from a screenshot
  of a setup page. Observe the finished resource and the required behavior.

### 0.4 Command conventions

Most original feature recipes below use PowerShell. The system-wide DevSecOps
commands use Bash because the target server is Amazon Linux 2023. Run commands
from the repository root unless the example explicitly enters `frontend/`.

---

## 1. Prerequisites & Setup

### Build Everything

```bash
# Run from the cloned MangaHub repository root.
go mod download
go mod verify
go build ./cmd/api-server/
go build ./cmd/cli/
go build ./cmd/tcp-client/
go build ./cmd/tcp-server/
go build ./cmd/udp-server/
go build ./cmd/grpc-server/
go build ./cmd/db-tool/
```

### Fresh Database Reset

This is only for the local development database. Stop every local
process/container using it first. Rename the file so the reset remains
recoverable until you deliberately remove the backup.

```bash
mv ./data/mangahub.db ./data/mangahub.db.reset-backup
```

PowerShell equivalent:

```powershell
Move-Item .\data\mangahub.db .\data\mangahub.db.reset-backup
```

### Start Server

```bash
export JWT_SECRET="$(openssl rand -base64 48)"
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
  "data": { "status": "healthy" }
}
```

`/health` is intentionally minimal. In direct local development, detailed
diagnostics remain available at `/health/db`, `/health/cache`, `/health/tcp`,
`/health/udp`, `/health/ws`, and `/health/grpc`. The production edge must return
`404` for the corresponding `/api/health/*` paths so internal counts and service
details are not exposed publicly.

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
```

**Expected output:**
```
Importing library from library.json...

✓ Library import complete!
  Imported: 2 entries
  Skipped:  1 (already in library)
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

### SSE Browser Event Bridge (live notifications & activity for the SPA)
| Feature | Test Section | Status |
|---------|-------------|--------|
| internal/sse/hub.go (broadcast hub + Run loop) | -- | Done |
| GET /events/stream (token auth, same :8080 port) | S24.1 | Done |
| notification event published from /notify/broadcast | S24.1 | Done |
| progress event published from /users/progress | S24.1 | Done |
| /events exempt from rate limiter | S24.1 | Done |
| Browser: toast + notification bell (UDP path) | S24.2 | Done |
| Browser: live activity toast + feed refresh (TCP path) | S24.2 | Done |
| EventSource auto-reconnect after backend restart | S24.2 | Done |

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

## S17. GitHub Actions CI and Image Publication

### S17.1 Trigger CI
Push the current branch, or later open a pull request into `main`:

```bash
git push origin features/devsecops
```

Go to **github.com/anhhuynh1707/MangaHub → Actions**. Pushes to
`features/devsecops`, `develop`, and `main` run CI; pull requests into `main`
also run CI.

### S17.2 Expected Jobs
| Job | Required behavior |
|---|---|
| **Security gates** | Dependency audit, Gitleaks history scan, Trivy repository and production-image scans; PR dependency review where applicable |
| **Deployment script checks** | Bash syntax, monitoring JSON, ShellCheck, and health-metric publisher test |
| **Build & Test** | All backend binaries, focused race tests, complete Go suite, and `go vet` |
| **Frontend Build & Type Check** | Locked install, TypeScript, ESLint, npm high-severity audit, and build |
| **E2E Tests (Playwright)** | Real browser journey against an ephemeral backend; report uploaded |
| **Docker Build & Smoke Test** | Development smoke plus production base/raw/CloudWatch Compose rendering |
| **Publish immutable images to GHCR** | Runs only after all required gates on a push to `features/devsecops` or `main` |

Pull requests publish no image. A successful `features/devsecops` push publishes
only full-commit tags. A successful `main` push also updates the convenience
`latest` tags, but deployment never uses `latest`.

### S17.3 Pull Published Docker Image
Use one exact 40-character commit shared by backend and frontend:

```bash
FULL_SHA="$(git rev-parse HEAD)"
docker buildx imagetools inspect "ghcr.io/anhhuynh1707/mangahub:sha-${FULL_SHA}"
docker buildx imagetools inspect "ghcr.io/anhhuynh1707/mangahub-frontend:sha-${FULL_SHA}"
```

Both manifests must exist and include `linux/amd64` before that release is used
on the planned x86_64 EC2 instance. Do not run the backend image alone as the
production proof; the release contract is the complete Compose topology.

### S17.4 Diagnose an Image-Scan Failure

Do not bypass a newly failing Trivy gate with `exit-code: 0`, a severity
exclusion, or an ignore entry merely because the previous run was green.

1. Open **Security gates → Production image scan** and identify the failed
   backend, frontend, edge, or Redis step.
2. Reproduce with the same Trivy version, scanners, severity policy, and
   `ignore-unfixed` setting used by `.github/workflows/security.yml`.
3. Record the package, installed version, CVE, severity, status, and fixed
   version.
4. Replace or update the narrowest owning dependency. For a base-image package,
   resolve an official rebuilt image and pin its immutable digest.
5. Scan the replacement's `linux/amd64` manifest before editing the pin.
6. Update every runtime and explicit scan reference to the same digest, then
   rerun CI.

Remediation record from 2026-08-31:

| Item | Finding / resolution |
|---|---|
| Failed artifact | Frontend runtime image and the shared unprivileged Nginx edge base |
| Finding | `CVE-2026-14456`, HIGH, OpenSSL QUIC denial of service through unbounded memory growth |
| Vulnerable packages | `libcrypto3` and `libssl3` `3.5.7-r0` |
| Fixed packages | `3.5.8-r0` |
| Old image digest | `sha256:44e36330f74d4f3a1d4e222acca9e23b401fb87811a7597024502bb759c4dd49` |
| Replacement index | `sha256:45ce1e2e699234253d1def7baa96218a5d00b498d1ba0cbb1a17b6bdf73d1351` |
| Replacement amd64 manifest | `sha256:ee055adf39a3cc6c2b8fc5734342d42728fa5fcaa5e8798fd24e4117ac969b2a` |
| Replacement scan | Trivy 0.72.0: 0 HIGH/CRITICAL vulnerabilities; secret scanner enabled |

The tag remains human-readable, but the digest is the actual supply-chain pin.
The same replacement digest must appear in `frontend/Dockerfile`, production
Compose, and the explicit pinned-edge scan step.

Remediation record from 2026-09-24:

| Item | Finding / resolution |
|---|---|
| Failed artifact | Frontend runtime image built from the pinned unprivileged Nginx edge base |
| Findings | 11 HIGH Alpine findings across `libexpat` and `libuuid`, including denial-of-service, memory-corruption, and util-linux mount issues |
| Vulnerable packages | `libexpat` `2.8.3-r0`; `libuuid` `2.42.1-r0` on Alpine `3.24.1` |
| Fixed packages | `libexpat` `2.8.4-r0`; `libuuid` at least `2.42.3-r1` on Alpine `3.24.2` |
| Old image digest | `sha256:45ce1e2e699234253d1def7baa96218a5d00b498d1ba0cbb1a17b6bdf73d1351` |
| Replacement index | `sha256:4714e0b1b2577eaa1a6131d07c958b67f0eb68e6d0521e90c6e5287db8cf0bc5` |
| Replacement amd64 manifest | `sha256:9f1d635195267228edfdad0bbaacf390031785924f10f8449ff9c934ff765290` |
| Replacement scan | Trivy 0.72.0: 0 HIGH/CRITICAL vulnerabilities for the immutable multi-architecture digest |

The same run then reached the separately pinned Redis scan and found a second
base-image refresh requirement:

| Item | Finding / resolution |
|---|---|
| Failed artifact | Pinned Redis 7 Alpine production image |
| Finding | `CVE-2026-45447`, HIGH, OpenSSL heap use-after-free in `PKCS7_verify()` |
| Vulnerable packages | `libcrypto3` and `libssl3` `3.3.7-r0` on Alpine `3.21.7` |
| Fixed packages | `3.3.7-r1` on Alpine `3.21.8` |
| Old image digest | `sha256:ff02b58f971e7d7d156a1267e283fcbbeee91773b6aa36c49dac28ecfe28eadf` |
| Replacement index | `sha256:858f009f9709ce576febc734aa78b8f6d624b82571f9ddb6bda4377c833b3499` |
| Replacement amd64 manifest | `sha256:ca0acbb137c1dc3339c8b147a58fd6f42775d4599327b50e7b116c23de501af2` |
| Replacement scan | Trivy 0.72.0: 0 HIGH/CRITICAL vulnerabilities for the immutable multi-architecture digest |

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

## S24. SSE Browser Event Bridge (Live Notifications & Activity)

The SPA can't speak raw TCP/UDP, so the API server bridges UDP notifications and
TCP progress updates to the browser over **Server-Sent Events**. This is **not a
new port** — it's one extra route, `GET /events/stream`, on the existing API
port `:8080`. The TCP/UDP servers and the CLI are unaffected.

### S24.1 Verify the stream with curl (no browser needed)

```bash
# Save a token first (see §1 Setup): export ALICE_TOKEN=...

# Terminal A — open the stream and leave it running (it stays connected):
curl -N "http://localhost:8080/events/stream?token=$ALICE_TOKEN"
#   You should immediately get a 200; every ~25s a ": ping" keepalive comment appears.

# Terminal B — fire a notification (the UDP broadcast path):
curl -s -X POST http://localhost:8080/notify/broadcast \
  -H "Authorization: Bearer $ALICE_TOKEN" -H 'Content-Type: application/json' \
  -d '{"type":"new_chapter","manga_id":"one-piece","message":"Chapter 1100 is out!"}'

# → Terminal A prints (one line):
# data: {"type":"notification","data":{"type":"new_chapter","manga_id":"one-piece",
#        "message":"Chapter 1100 is out!","timestamp":...},"timestamp":...}

# Terminal B — bump a chapter (the TCP/gRPC progress path). The manga must be in
# your library first (POST /users/library), then:
curl -s -X PUT http://localhost:8080/users/progress \
  -H "Authorization: Bearer $ALICE_TOKEN" -H 'Content-Type: application/json' \
  -d '{"manga_id":"one-piece","current_chapter":42,"status":"reading"}'

# → Terminal A prints:
# data: {"type":"progress","data":{"chapter":42,"manga_id":"one-piece",
#        "user_id":"user-alice","username":"alice"},"timestamp":...}
```

**Auth checks (should both fail):**

```bash
curl -i "http://localhost:8080/events/stream"               # → 401 (missing token)
curl -i "http://localhost:8080/events/stream?token=garbage" # → 401 (invalid token)
```

> Note: `POST /notify/broadcast` may report `"sent to 0 clients"` — that count is
> the **UDP CLI** subscribers, which is independent of the SSE bridge. The browser
> still gets the event over SSE. To exercise the CLI side too, run
> `mangahub notify subscribe` in another terminal before broadcasting.

### S24.2 Test it in the browser (what to click and what you should see)

**Prerequisites:** backend on `:8080`, frontend running
(`cd frontend && npm run dev` → `:5173`, or the Docker frontend on `:3000`).

1. Open the app and **log in** (e.g. as `alice`). The navbar now shows a 🔔 **bell**
   icon next to your username.
2. **Notifications (UDP path).** In a terminal, fire the broadcast curl from
   S24.1 (with alice's token). In the browser you should immediately see:
   - a **toast** (top-right) with the message text, and
   - the **bell's red unread badge** increment. Click the bell → a dropdown lists
     the notification; opening it clears the unread count. "Clear" empties the list.
3. **Live activity (TCP path).** Open the app in **two browsers** (or a normal +
   an incognito window) logged in as **two different users** (alice and bob). As
   **bob**, go to the **Library** page and bump a chapter on any manga in his
   library. In **alice's** window you should see a "Reading activity" toast
   (`bob read a manga → ch. N`), and if alice is on the **Feed** page it refreshes
   live (the `['feed']` query is invalidated). You don't get toasted by your own
   progress updates.
4. **Auto-reconnect.** With the app open, restart the backend
   (`Ctrl+C` then `go run ./cmd/api-server/`). In the browser **DevTools →
   Network**, filter to `stream`: the `events/stream` request shows `pending`,
   drops when the server stops, and **re-establishes on its own** once the backend
   is back (EventSource reconnects automatically). Fire another broadcast to
   confirm events still arrive.

### S24.3 Troubleshooting

| Symptom | Likely cause / fix |
|---|---|
| No toast, no bell change | Not logged in (the stream only opens when authenticated), or the browser is pointed at the wrong API. Check `VITE_API_URL`. |
| `events/stream` is 401 | Token missing/expired — log out and back in. |
| Stream works in curl but not the browser | CORS: the API allows the SPA origin via `corsOrigins()`; make sure the frontend origin (`:5173` / `:3000`) is allowed. |
| Stream drops after ~30–60s behind a proxy | Proxy buffering — the handler sets `X-Accel-Buffering: no`; ensure your proxy honours it (nginx does). The 25s `: ping` keepalive also helps. |
| Progress event never fires | The manga must be in your library first (`POST /users/library`), and the chapter must be `> 0`. |

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

## DevSecOps, Production, and AWS Verification

This is the end-to-end release checklist for the AWS upgrade. Run it in order.
If one layer fails, diagnose that layer before moving upward.

### D1. Backend and frontend source gates

```bash
# Repository root
go mod verify
go test -timeout 120s ./...
go vet ./...

cd frontend
npm ci
npx tsc --noEmit
npm run lint
npm audit --audit-level=high
npm run build
cd ..
```

Expected: every command exits `0`. A committed generated API type file may be
used if the documented generation fallback is activated, but TypeScript and the
final Vite build must still pass.

### D2. Deployment configuration and script gates

```bash
bash -n deploy/scripts/*.sh deploy/tests/*.sh
jq empty \
  deploy/aws/MangaHubDemoCloudWatchPolicy.json \
  deploy/cloudwatch/amazon-cloudwatch-agent.json

docker run --rm \
  -v "$PWD:/workspace:ro" \
  -w /workspace \
  koalaman/shellcheck-alpine@sha256:9955be09ea7f0dbf7ae942ac1f2094355bb30d96fffba0ec09f5432207544002 \
  shellcheck --severity=warning deploy/scripts/*.sh deploy/tests/*.sh
```

The health-metric publisher contract needs Linux root semantics. CI runs:

```bash
sudo ./deploy/tests/publish-health-metrics.test.sh
```

Do not run the EC2 operator scripts (`configure-server.sh`, `deploy.sh`,
`restore.sh`, or `configure-monitoring.sh`) with `sudo` on macOS. They are
fail-closed Amazon Linux/server tools, not local installers.

### D3. Development Compose smoke test

This validates the simple learning topology in the repository root:

```bash
export JWT_SECRET="$(openssl rand -base64 48)"
docker compose config --quiet
docker compose up -d --build
curl --fail http://127.0.0.1:8080/health
docker compose ps
docker compose down
```

Expected: frontend `3000`, API `8080`, Redis `6379`, TCP `9090`, UDP `9091`, and
gRPC `9092` are directly published. `down` retains named volumes; do not add
`-v` when you are testing persistence.

### D4. Production-style base mode

Base mode is the configuration that should be proven before AWS. It builds from
local source but otherwise uses the EC2 topology.

```bash
export JWT_SECRET="$(openssl rand -base64 48)"
export HOST_HTTP_PORT=8088
export PUBLIC_ORIGIN=http://localhost:8088

docker compose --project-name mangahub-prod \
  -f deploy/docker/docker-compose.prod.yml \
  -f deploy/docker/docker-compose.prod.local.yml \
  config --quiet

docker compose --project-name mangahub-prod \
  -f deploy/docker/docker-compose.prod.yml \
  -f deploy/docker/docker-compose.prod.local.yml \
  up -d --build

./deploy/scripts/healthcheck.sh http://127.0.0.1:8088

docker compose --project-name mangahub-prod \
  -f deploy/docker/docker-compose.prod.yml \
  -f deploy/docker/docker-compose.prod.local.yml \
  ps
```

Expected:

- seven long-running services are running; `data-init` exited successfully;
- `/health` returns healthy JSON with public security headers;
- `/` returns the React app;
- `/api/health/db` and `/api/swagger/index.html` both return `404`;
- only edge port `8088` is published in base mode;
- API, Redis, frontend, TCP, UDP, gRPC, and SQLite are not direct host
  listeners.

Inspect hardening on a representative backend container:

```bash
API_ID="$(docker compose --project-name mangahub-prod \
  -f deploy/docker/docker-compose.prod.yml \
  -f deploy/docker/docker-compose.prod.local.yml \
  ps -q mangahub-api)"

docker inspect --format \
  'user={{.Config.User}} readonly={{.HostConfig.ReadonlyRootfs}} caps={{json .HostConfig.CapDrop}} security={{json .HostConfig.SecurityOpt}}' \
  "$API_ID"
```

Expected: user `10001:10001`, read-only root filesystem, all capabilities
dropped, and `no-new-privileges` enabled.

### D5. Optional raw-protocol mode

The local override binds the raw listeners to `127.0.0.1` by default:

```bash
docker compose --project-name mangahub-prod \
  -f deploy/docker/docker-compose.prod.yml \
  -f deploy/docker/docker-compose.prod.local.yml \
  -f deploy/docker/docker-compose.raw.yml \
  config --quiet

docker compose --project-name mangahub-prod \
  -f deploy/docker/docker-compose.prod.yml \
  -f deploy/docker/docker-compose.prod.local.yml \
  -f deploy/docker/docker-compose.raw.yml \
  up -d --build
```

Use Sections 7, S22, and S23 to exercise TCP, UDP ACK, and gRPC streaming. Then
rerun the HTTP health gate. The protocols are additional listeners; they do not
replace browser HTTP/WebSocket/SSE.

Return to base mode or stop cleanly without deleting volumes:

```bash
docker compose --project-name mangahub-prod \
  -f deploy/docker/docker-compose.prod.yml \
  -f deploy/docker/docker-compose.prod.local.yml \
  -f deploy/docker/docker-compose.raw.yml \
  down
```

### D6. CloudWatch configuration gate (local render only)

The `awslogs` runtime should not be started locally unless real scoped AWS
prerequisites exist. Rendering proves the override merges correctly without
making an AWS call. CI also rejects the unsupported `awslogs-create-stream`
override for the supported Amazon Linux Docker baseline: stream creation is
enabled by default, while `awslogs-create-group=false` still requires the
pre-created bounded group.

```bash
export MANGAHUB_AWS_REGION=ap-southeast-2
export MANGAHUB_CLOUDWATCH_LOG_GROUP=/mangahub/demo/containers

docker compose \
  -f deploy/docker/docker-compose.prod.yml \
  -f deploy/docker/docker-compose.cloudwatch.yml \
  config --quiet

docker compose \
  -f deploy/docker/docker-compose.prod.yml \
  -f deploy/docker/docker-compose.raw.yml \
  -f deploy/docker/docker-compose.cloudwatch.yml \
  config --quiet
```

### D7. CI and immutable GHCR candidate

After pushing `features/devsecops`, verify all jobs listed in S17 are green.
Then use the exact branch tip:

```bash
FULL_SHA="$(git rev-parse HEAD)"
test "$(printf '%s' "$FULL_SHA" | wc -c | tr -d ' ')" = 40

docker buildx imagetools inspect \
  "ghcr.io/anhhuynh1707/mangahub:sha-${FULL_SHA}"
docker buildx imagetools inspect \
  "ghcr.io/anhhuynh1707/mangahub-frontend:sha-${FULL_SHA}"
```

Both images must exist for the same SHA and include `linux/amd64`. Do not deploy
a short SHA, an unmatched image pair, or `latest`.

### D8. Manual AWS/EC2 gate — pending

The next cloud action remains Checkpoint A in
[`AWS_DEPLOYMENT.md`](AWS_DEPLOYMENT.md). Continue only in this order:

1. Root MFA, root billing-IAM access, daily console administrator MFA with zero
   access keys, USD 5 budget, and Sydney Region.
2. EC2 instance role for Session Manager.
3. Dedicated `10.20.0.0/16` learning VPC, public subnet, Internet Gateway, and
   explicit route table.
4. Security Group with public HTTP 80, no SSH, and no raw ports initially.
5. One approved Amazon Linux 2023 x86_64 instance with encrypted 8 GiB gp3 and
   IMDSv2 required.
6. Session Manager access and Docker installation.
7. Exact-SHA base deployment and public browser/API behavior.
8. Temporary owner `/32` raw rules plus `--with-raw`, followed by rule removal.
9. EC2 rollback, SQLite backup/restore, CloudWatch logs/metrics/alarms, and a
   controlled failure/recovery rehearsal.

Record only sanitized observations in [`AWS_EVIDENCE.md`](AWS_EVIDENCE.md).
Every AWS row remains `NOT RUN` until the corresponding finished state and
behavior are observed. Local green tests do not satisfy an AWS row.

### D9. Recovery and observability acceptance

Use the dedicated runbooks rather than improvising:

| Capability | Required acceptance evidence | Runbook |
|---|---|---|
| Release rollback | Previous full SHA and mode restored; health and demo data persist | [`ROLLBACK.md`](ROLLBACK.md) |
| SQLite backup | Online backup, integrity `ok`, checksum, retention, protected ownership | [`BACKUP.md`](BACKUP.md) |
| SQLite restore | Pre-restore point, controlled data reversal, same release mode, health pass | [`BACKUP.md`](BACKUP.md) |
| CloudWatch | Four metrics, seven bounded log streams, five alarms, alarm failure and recovery | [`MONITORING.md`](MONITORING.md) |

Image rollback and database restore solve different failures. Never substitute
one for the other, and never delete a volume to make a failed test appear clean.

### D10. Which AWS-upgrade files these tests cover

| Test group | Main artifacts exercised |
|---|---|
| Production runtime | `deploy/docker/*`, `deploy/nginx/nginx.conf`, backend/frontend Dockerfiles |
| Release safety | `configure-server.sh`, `deploy.sh`, `healthcheck.sh`, `rollback.sh` |
| Data recovery | `cmd/db-tool/`, `internal/dbops/`, `backup.sh`, `restore.sh` |
| Monitoring | CloudWatch policy/agent JSON, log override, publisher, systemd units |
| Supply chain | `ci.yml`, `security.yml`, Dependabot, Gitleaks, Docker ignore files |
| Proof boundary | AWS deployment, architecture, evidence, security, portfolio, and operations docs |

For the complete file-by-file explanation, see [`CODE_FLOW.md`](CODE_FLOW.md)
Section 22.

---

> **Note:** After resetting the database, all users must be re-registered.
> Passwords for test users: alice -> alice123, bob -> bob123
