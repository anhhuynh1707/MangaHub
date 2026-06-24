# MangaHub — Detailed Code Flow Documentation

---

## 1. HTTP REST API (15 pts)

### 1.1 Architecture Overview

```
Client (React SPA / CLI / curl)
  │
  ▼
cmd/api-server/  (package main — wiring + APIServer methods)
  ├── main.go         ← ~110 lines: config, build *APIServer, start, run
  ├── server.go       ← APIServer struct (holds every handler dependency)
  ├── routes.go       ← registerRoutes(): path → handler-method map + middleware
  ├── bootstrap.go    ← startTCP/UDP/GRPC, seedDatabase, env/CORS helpers
  └── health.go / sync.go / notify.go / data.go / chat.go  ← infra endpoints
  │
  ├── gin.Engine (gin.New)        ← HTTP framework
  │     │  Middleware chain (outer → inner):
  │     ├── gin.Recovery()              ← panic recovery
  │     ├── logger.RequestLogger()      ← structured per-request log + X-Request-ID
  │     ├── cors                        ← gin-contrib/cors
  │     ├── ratelimit.Middleware()      ← per-IP 100/min public, 300/min authed
  │     ├── auth.AuthMiddleware()       ← JWT validation (per route group)
  │     │
  │     ├── userHandler.*            ← internal/user/handler.go
  │     ├── mangaHandler.*           ← internal/manga/handler.go
  │     ├── reviewHandler.*          ← internal/review/handler.go
  │     ├── friendHandler.*          ← internal/friend/handler.go
  │     ├── sharedListHandler.*      ← internal/sharedlist/handler.go
  │     └── activityHandler.*        ← internal/activity/handler.go
  │
  ├── *Service  (business logic, return *utils.AppError)  ← internal/*/service.go
  ├── *Repository (raw SQL)          ← internal/*/repository.go
  ├── pkg/logger/logger.go           ← slog setup + RequestLogger middleware
  ├── pkg/ratelimit/ratelimit.go     ← token-bucket rate limiter
  ├── pkg/utils/errors.go            ← AppError + RespondError
  ├── pkg/cache/redis.go             ← Redis caching layer
  └── pkg/database/sqlite.go         ← SQLite schema + InitDB (WAL, busy_timeout)
```

Each domain follows the **Handler → Service → Repository** pattern:

| Layer | File | Responsibility |
|---|---|---|
| Handler | `internal/*/handler.go` | Parse HTTP request, call service, write response via `utils.RespondError` on error |
| Service | `internal/*/service.go` | Business rules, validation, cache; returns `*utils.AppError` (carries HTTP status) for known errors |
| Repository | `internal/*/repository.go` | Raw SQL queries against SQLite |

> **Error handling (R1-2):** services return typed `*AppError`s; handlers call
> `utils.RespondError(c, err)` which maps the error's `Code` to the HTTP status
> (409/404/401/403/400) — no more `err.Error() == "..."` string matching.

---

### 1.2 Server Bootstrap (`cmd/api-server/main.go`)

After the R1-1 refactor, `main()` is ~110 lines of wiring; handler bodies live in
methods on `*APIServer` across the sibling files.

```
main()
 ├─ loadEnvFile(".env")                         // bootstrap.go — read .env
 ├─ logger.Init()                               // pkg/logger — slog JSON/text + std-log bridge
 ├─ database.InitDB(dbPath)                     // pkg/database/sqlite.go (WAL + busy_timeout + FK)
 │    └─ createSchema(db)                       // Creates 7 tables + indexes
 ├─ cache.New(...)                              // pkg/cache/redis.go — graceful fallback
 │
 ├─ <repo> := <pkg>.NewRepository(db)           // user / manga / review / friend / sharedlist / activity
 ├─ <svc>  := <pkg>.NewService(<repo>)
 ├─ recommendation.NewService(db)
 ├─ <svc>.SetCache(redisCache)                  // manga / user / activity
 │
 ├─ s := &APIServer{                            // server.go — holds all deps + handlers
 │      Router: gin.New(), Database, Cache, Hub, …Service, …Handler, Port, … }
 ├─ s.Router.Use(gin.Recovery(), logger.RequestLogger())   // outer middleware
 │
 ├─ startTCP(s,…) / startUDP(s,…) / go s.Hub.Run() / startGRPC(s,…)   // bootstrap.go
 ├─ go seedDatabase(...)                        // BACKGROUND — API listens immediately
 │
 ├─ s.registerRoutes(corsOrigins())            // routes.go — CORS + RateLimit + all routes
 └─ s.Router.Run(":" + port)                    // Start HTTP listener
```

`NewHandler` signatures grew to support cross-cutting features, e.g.
`userPkg.NewHandler(userService, activityService, mangaService)` (logs library
activities) and `review.NewHandler(reviewService, activityService, mangaService)`.

---

### 1.2b Request Pipeline, Logging & Rate Limiting

Every request passes through this chain (registered in `main.go` + `routes.go`):

```
request
  → gin.Recovery()                 // turns panics into 500s
  → logger.RequestLogger()         // pkg/logger — assigns request_id, sets
  │                                //   X-Request-ID header; after the handler runs,
  │                                //   logs one structured line:
  │                                //   {request_id, method, path, status,
  │                                //    latency_ms, client_ip, user_id}
  → cors                           // gin-contrib/cors
  → ratelimit.Middleware(100, 300) // pkg/ratelimit — per-IP token bucket;
  │                                //   Authorization header → 300/min tier,
  │                                //   else 100/min; /health* and /swagger* exempt;
  │                                //   over limit → 429
  → auth.AuthMiddleware()          // on protected route groups only — validates JWT,
  │                                //   sets user_id + username in the gin context
  → handler method                 // logic; on error → utils.RespondError(c, err)
```

**Logging config:** `LOG_LEVEL` = debug|info|warn|error (default info);
`LOG_FORMAT` = text (readable, default for local dev) | json (set in
docker-compose for production). Log level is chosen by status: 2xx → INFO,
4xx → WARN, 5xx → ERROR. The std `log` package is bridged into slog, so legacy
`log.Printf` calls across all packages also emit structured output.

---

### 1.3 Authentication Flow

**Files:** `internal/auth/jwt.go`, `internal/user/handler.go`, `internal/user/service.go`

#### Registration: `POST /auth/register`

```
Client: POST /auth/register  {"username":"alice","password":"secret123"}
  │
  ▼
userHandler.Register()                          // internal/user/handler.go:23
  │ c.ShouldBindJSON(&req)                      // Parse RegisterRequest
  ▼
userService.Register(&req)                      // internal/user/service.go:47
  │ repo.FindByUsername(req.Username)            // Check duplicate
  │ bcrypt.GenerateFromPassword(password)        // Hash password
  │ generateUserID(username) → "user-alice"      // Slug-style ID
  │ repo.Create(user)                            // INSERT INTO users
  ▼
utils.CreatedResponse(c, "User registered", user)  // 201 JSON response
```

#### Login: `POST /auth/login`

```
Client: POST /auth/login  {"username":"alice","password":"secret123"}
  │
  ▼
userHandler.Login()                             // internal/user/handler.go:45
  ▼
userService.Login(&req)                         // internal/user/service.go:81
  │ repo.FindByUsername(req.Username)            // SELECT from users
  │ bcrypt.CompareHashAndPassword(hash, pass)    // Verify password
  │ auth.GenerateToken(user.ID, user.Username)   // internal/auth/jwt.go:29
  │   └─ jwt.NewWithClaims(HS256, claims)        // 24h expiry, issuer="mangahub"
  │   └─ token.SignedString(JWTSecret)
  ▼
Response: {"token":"eyJhbG...", "user":{...}}    // 200 OK
```

#### Auth Middleware (protects all authenticated routes)

```
Any authenticated request:
  Authorization: Bearer <jwt-token>
  │
  ▼
auth.AuthMiddleware()                           // internal/auth/jwt.go:68
  │ c.GetHeader("Authorization")                // Extract header
  │ strings.SplitN(header, " ", 2)              // Split "Bearer <token>"
  │ auth.ValidateToken(tokenString)             // jwt.ParseWithClaims
  │   └─ Verify HMAC-SHA256 signature
  │   └─ Check expiration
  │ c.Set("user_id", claims.UserID)             // Store in Gin context
  │ c.Set("username", claims.Username)
  │ c.Next()                                    // Continue to handler
  │
  ▼  (if invalid)
utils.UnauthorizedResponse(c, "Invalid token")  // 401, c.Abort()
```

**Helper functions used by all handlers:**
- `auth.GetUserIDFromContext(c)` → extracts `user_id` from Gin context
- `auth.GetUsernameFromContext(c)` → extracts `username` from Gin context

---

### 1.4 Manga CRUD Flow

**Files:** `internal/manga/handler.go`, `internal/manga/service.go`, `internal/manga/repository.go`

#### Search: `GET /manga` (Public)

```
GET /manga?search=one+piece&genre=action&page=1&limit=20
  │
  ▼
mangaHandler.Search()                           // handler.go:22
  │ c.ShouldBindQuery(&query)                   // Bind MangaSearchQuery
  ▼
mangaService.Search(query)                      // service.go:90
  │ cache.Get(MangaSearchKey(...))              // Try Redis first
  │   └─ HIT  → return cached result
  │   └─ MISS ↓
  │ repo.Search(query)                          // repository.go:78
  │   └─ Build dynamic WHERE clause
  │   └─ SELECT COUNT(*) for total
  │   └─ SELECT ... LIMIT ? OFFSET ?
  │   └─ json.Unmarshal genres JSON array
  │ cache.Set(key, result, 5min)                // Populate cache
  ▼
Response: {"manga":[...], "total":N, "page":1, "limit":20}
```

#### Get by ID: `GET /manga/:id` (Public)

```
GET /manga/one-piece
  │
  ▼
mangaHandler.GetByID()                          // handler.go:45
  ▼
mangaService.GetByID("one-piece")               // service.go:65
  │ cache.Get("manga:one-piece")                // Try Redis (10min TTL)
  │   └─ MISS → repo.FindByID("one-piece")     // repository.go:35
  │              └─ SELECT ... WHERE id = ?
  │   cache.Set("manga:one-piece", manga, 10min)
  ▼
Response: {"id":"one-piece","title":"One Piece",...}
```

#### Create: `POST /manga` (Auth required)

```
POST /manga  {"id":"my-manga","title":"My Manga",...}
  │
  ├─ auth.AuthMiddleware()                      // Validate JWT
  ▼
mangaHandler.Create()                           // handler.go:63
  ▼
mangaService.Create(&manga)                     // service.go:45
  │ repo.FindByID(manga.ID)                     // Check duplicate
  │ repo.Create(manga)                          // INSERT INTO manga
  │ cache.Set("manga:<id>", manga, 10min)       // Cache new entry
  │ invalidateMangaCaches()                     // Delete search/* and count keys
  ▼
Response: 201 Created
```

#### Update / Delete (Auth required)

```
PUT  /manga/:id → mangaHandler.Update() → service.Update() → repo.Update() + cache invalidation
DELETE /manga/:id → mangaHandler.Delete() → service.Delete() → repo.Delete() + cache invalidation
```

---

### 1.5 User Library & Progress Flow

**Files:** `internal/user/handler.go`, `internal/user/service.go`, `internal/user/repository.go`

#### Add to Library: `POST /users/library` (Auth)

```
POST /users/library  {"manga_id":"one-piece","status":"reading"}
  │
  ├─ AuthMiddleware → user_id from JWT
  ▼
userHandler.AddToLibrary()                      // handler.go:81
  │ auth.GetUserIDFromContext(c)
  ▼
userService.AddToLibrary(userID, &req)          // service.go:135
  │ repo.GetLibraryEntry(userID, mangaID)       // Check if already in library
  │ repo.AddToLibrary(progress)                 // INSERT INTO user_progress
  │ invalidateUserCaches(userID)                // Clear user:library:<id> key
  ▼
Response: 201 {"manga_id":"one-piece","status":"reading","current_chapter":0}
```

#### Get Library: `GET /users/library` (Auth)

```
userHandler.GetLibrary() → userService.GetLibrary(userID)
  │ cache.Get("user:library:<id>")              // Try Redis (5min TTL)
  │   └─ MISS → repo.GetUserReadingLists(userID)
  │              └─ repo.GetUserLibrary(userID)  // SELECT ... WHERE user_id=?
  │              └─ Categorize by status: reading/completed/plan_to_read
  │ cache.Set(key, userData, 5min)
  ▼
Response: {"reading":[...], "completed":[...], "plan_to_read":[...]}
```

#### Update Progress: `PUT /users/progress` (Auth + TCP broadcast)

```
PUT /users/progress  {"manga_id":"one-piece","current_chapter":50}
  │
  ├─ AuthMiddleware
  ▼
Inline handler in main.go (line 450)
  │ userHandler.UpdateProgress(c)               // handler.go:127
  │   └─ userService.UpdateProgress(userID, &req)
  │        └─ repo.GetLibraryEntry()            // Verify manga in library
  │        └─ repo.UpdateProgress()             // UPDATE user_progress SET ...
  │        └─ invalidateUserCaches(userID)
  │        └─ c.Set("progress_manga_id", ...)   // Store for TCP broadcast
  │
  │ if c.Writer.Status() == 200:                // After successful update
  │   Build models.ProgressUpdate
  │   server.TCPServer.SendProgressUpdate()     // Push to TCP broadcast channel
  │     └─ Broadcast channel → broadcastLoop → all connected TCP clients
  ▼
Response: 200 + TCP broadcast to all sync clients
```

---

### 1.6 Complete Route Table

#### Public Routes (No Auth)

| Method | Path | Handler | File |
|--------|------|---------|------|
| `POST` | `/auth/register` | `userHandler.Register` | `user/handler.go:23` |
| `POST` | `/auth/login` | `userHandler.Login` | `user/handler.go:45` |
| `GET` | `/manga` | `mangaHandler.Search` | `manga/handler.go:22` |
| `GET` | `/manga/:id` | `mangaHandler.GetByID` | `manga/handler.go:45` |
| `GET` | `/manga/:id/reviews` | `reviewHandler.GetReviews` | `review/handler.go:70` |
| `GET` | `/manga/:id/rating-stats` | `reviewHandler.GetRatingStats` | `review/handler.go:210` |
| `GET` | `/reading-lists/public` | `sharedListHandler.GetPublicLists` | `sharedlist/handler.go:148` |
| `GET` | `/reading-lists/:list_id` | `sharedListHandler.GetList` | `sharedlist/handler.go:197` |
| `GET` | `/health` | inline | `main.go:336` |
| `GET` | `/ws/chat` | `wsPkg.HandleWebSocket` | `websocket/client.go:26` |

#### Authenticated Routes (JWT Required)

| Method | Path | Handler |
|--------|------|---------|
| `GET` | `/auth/status` | `userHandler.AuthStatus` |
| `POST` | `/auth/logout` | `userHandler.Logout` |
| `PUT` | `/auth/change-password` | `userHandler.ChangePassword` |
| `POST` | `/manga` | `mangaHandler.Create` |
| `PUT` | `/manga/:id` | `mangaHandler.Update` |
| `DELETE` | `/manga/:id` | `mangaHandler.Delete` |
| `GET` | `/users/profile` | `userHandler.GetProfile` |
| `POST` | `/users/library` | `userHandler.AddToLibrary` |
| `GET` | `/users/library` | `userHandler.GetLibrary` |
| `DELETE` | `/users/library/:manga_id` | `userHandler.RemoveFromLibrary` |
| `PUT` | `/users/progress` | inline + `userHandler.UpdateProgress` |
| `POST` | `/manga/:id/reviews` | `reviewHandler.CreateReview` |
| `GET/PUT/DELETE` | `/reviews/:review_id` | `reviewHandler.Get/Update/Delete` |
| `POST` | `/reviews/:review_id/helpful` | `reviewHandler.MarkHelpful` |
| `GET` | `/users/reviews` | `reviewHandler.GetMyReviews` |
| `POST` | `/friends/add` | `friendHandler.AddFriend` |
| `POST` | `/friends/:id/accept` | `friendHandler.AcceptFriend` |
| `POST` | `/friends/:id/decline` | `friendHandler.DeclineFriend` |
| `DELETE` | `/friends/:id` | `friendHandler.RemoveFriend` |
| `GET` | `/users/friends` | `friendHandler.GetFriends` |
| `GET` | `/users/friends/pending` | `friendHandler.GetPendingRequests` |
| `POST` | `/reading-lists/create` | `sharedListHandler.CreateList` |
| `GET` | `/reading-lists/mine` | `sharedListHandler.GetMyLists` |
| `POST/DELETE` | `/reading-lists/:id/subscribe` | `Subscribe/Unsubscribe` |
| `POST` | `/reading-lists/:id/manga` | `sharedListHandler.AddMangaToList` |
| `POST/GET` | `/feed/activities` | `Post/GetActivityFeed` |
| `GET` | `/feed/timeline` | `activityHandler.GetTimelineView` |
| `POST` | `/notify/broadcast` | inline (main.go:591) |
| `GET` | `/sync/status` | inline (main.go:490) |
| `GET` | `/cache/stats` | inline (main.go:402) |
| `DELETE` | `/cache/flush` | inline (main.go:407) |

---

### 1.7 Database Schema

**File:** `pkg/database/sqlite.go:48` — 7 tables created on startup:

| Table | Primary Key | Foreign Keys |
|-------|------------|--------------|
| `users` | `id TEXT` | — |
| `manga` | `id TEXT` | — |
| `user_progress` | `(user_id, manga_id)` | → users, manga |
| `reviews` | `id TEXT` | → users, manga + UNIQUE(user_id, manga_id) |
| `friendships` | `(user_id, friend_id)` | → users × 2 + CHECK(user_id < friend_id) |
| `shared_reading_lists` | `id TEXT` | → users |
| `activities` | `id TEXT` | → users, manga |

---

### 1.8 Redis Caching Layer

**File:** `pkg/cache/redis.go`

| Key Pattern | TTL | Used By |
|-------------|-----|---------|
| `manga:<id>` | 10 min | `mangaService.GetByID/Create` |
| `manga:search:<params>` | 5 min | `mangaService.Search` |
| `manga:all` | 5 min | `mangaService.GetAll` |
| `manga:count` | 1 min | `mangaService.GetCount` |
| `user:library:<id>` | 5 min | `userService.GetLibrary` |
| `user:profile:<id>` | 10 min | `userService.GetProfile` |
| `feed:activities:<page>:<limit>` | 2 min | `activityService.GetAllActivities` |
| `feed:user:<id>:<page>:<limit>` | 2 min | `activityService.GetUserActivities` |

Cache invalidation happens on every write via `invalidate*Caches()` methods.

---

### 1.9 Response Format

**File:** `pkg/utils/response.go` — All endpoints use a consistent JSON envelope:

```json
{
  "status": "success",
  "message": "Human-readable message",
  "data": { ... }
}
```

---

## 2. TCP Progress Sync (13 pts)

### 2.1 Architecture Overview

```
┌─────────────────────────────────────────────────────┐
│  cmd/api-server/main.go  (embedded mode)            │
│    OR                                               │
│  cmd/tcp-server/main.go  (standalone mode)          │
│                                                     │
│  ┌───────────────────────────────────────────────┐  │
│  │  tcp.ProgressSyncServer   (server.go)         │  │
│  │    ├── net.Listener (port 9090)               │  │
│  │    ├── Connections map[string]net.Conn         │  │
│  │    ├── Broadcast chan ProgressUpdate           │  │
│  │    ├── Persister (ProgressPersister interface) │  │
│  │    └── ConflictResolver (conflict.go)         │  │
│  └───────────────────────────────────────────────┘  │
│         ▲                                           │
│         │  TCP (newline-delimited JSON)              │
│         ▼                                           │
│  ┌──────────────┐  ┌──────────────┐                 │
│  │  CLI client   │  │  CLI client   │  ...           │
│  │  (cmd/cli/    │  │  (tcp-client/ │                │
│  │   sync.go)    │  │   main.go)    │                │
│  └──────────────┘  └──────────────┘                 │
└─────────────────────────────────────────────────────┘
```

**Key files:**

| File | Purpose |
|------|---------|
| `internal/tcp/server.go` | `ProgressSyncServer` struct, Start/Stop, broadcast loop |
| `internal/tcp/handler.go` | Per-connection message handling (auth, progress, status, strategy) |
| `internal/tcp/protocol.go` | `TCPMessage` struct, encode/decode, message factories |
| `internal/tcp/conflict.go` | `ConflictResolver` with 3 strategies |
| `internal/tcp/client.go` | `ProgressSyncClient` for remote-mode API server |
| `cmd/tcp-server/main.go` | Standalone TCP server entry point |
| `cmd/tcp-client/main.go` | Interactive test client |
| `cmd/cli/sync.go` | CLI commands for TCP sync |

---

### 2.2 Protocol Specification

**Wire format:** Newline-delimited JSON (`\n` terminated)

**File:** `internal/tcp/protocol.go`

```go
type TCPMessage struct {
    Type           string  // Message type identifier
    UserID         string  // User who sent/is referenced
    Username       string  // Human-readable name
    MangaID        string  // Manga slug ID
    Chapter        int     // Chapter number
    Message        string  // Human-readable text
    Token          string  // JWT token (auth only)
    Timestamp      int64   // Unix timestamp
    ConnectedUsers int     // For status responses
    DeviceID       string  // Client device/session ID
    Strategy       string  // Conflict resolution strategy
}
```

#### Client → Server Messages

| Type | Fields | Purpose |
|------|--------|---------|
| `auth` | `token` | Authenticate with JWT token |
| `connect` | `token` or `user_id` | Legacy connect (redirects to auth) |
| `progress` | `manga_id`, `chapter`, `device_id` | Report reading progress |
| `set_strategy` | `strategy` | Change conflict resolution strategy |
| `get_strategy` | — | Query current strategy |
| `get_conflicts` | — | Query conflict resolution log |
| `status` | — | Request server status |
| `ping` | — | Keepalive ping |
| `disconnect` | — | Graceful disconnect |

#### Server → Client Messages

| Type | Fields | Purpose |
|------|--------|---------|
| `welcome` | `message` | Sent on initial connection |
| `auth` | `user_id`, `username`, `message` | Auth success confirmation |
| `broadcast` | `user_id`, `manga_id`, `chapter` | Progress update from any user |
| `user_joined` | `user_id`, `username` | Someone connected to sync |
| `user_left` | `user_id`, `username` | Someone disconnected |
| `conflict` | `manga_id`, `chapter`, `strategy`, `message` | Conflict detected/resolved |
| `strategy_info` | `strategy`, `message` | Current strategy response |
| `conflicts_info` | `strategy`, `message` (JSON), `chapter` (count) | Conflict log response |
| `status` | `message`, `connected_users` | Server status response |
| `pong` | `message` | Keepalive response |
| `error` | `message` | Error notification |

---

### 2.3 Server Lifecycle

#### Embedded Mode (api-server, `main.go:158-169`)

```
cmd/api-server/main.go
  │
  │ if enableTCPServer == "true":
  │   tcpServer := tcp.NewProgressSyncServer("9090")    // server.go:33
  │   tcpServer.Persister = userService                 // Wire DB persistence
  │   server.TCPServer = tcpServer
  │   go tcpServer.Start()                              // Run in goroutine
  │
  │ else:
  │   tcpClient := tcp.NewProgressSyncClient(tcpPort)   // client.go:21
  │   server.TCPClient = tcpClient                      // Use remote server
```

#### Standalone Mode (`cmd/tcp-server/main.go`)

```
main()
  │ database.InitDB(dbPath)                   // Initialize SQLite
  │ userPkg.NewRepository(db)
  │ userPkg.NewService(userRepo)
  │
  │ server := tcp.NewProgressSyncServer(port)
  │ server.Persister = userService            // Enable DB persistence
  │ server.Start()                            // Blocks (not in goroutine)
```

#### Server Start (`internal/tcp/server.go:43`)

```
ProgressSyncServer.Start()
  │ net.Listen("tcp", ":9090")                // Bind TCP port
  │ s.startTime = time.Now()
  │
  │ go s.broadcastLoop()                      // Start broadcast goroutine
  │
  │ for {                                     // Accept loop (blocks)
  │   conn := listener.Accept()               // Wait for new connection
  │   go s.handleConnection(conn)             // One goroutine per client
  │ }
```

---

### 2.4 Connection Handling (`internal/tcp/handler.go:17`)

```
handleConnection(conn)
  │
  │ sendMessage(conn, NewWelcomeMessage())    // "Connected to MangaHub TCP..."
  │
  │ scanner := bufio.NewScanner(conn)         // Read newline-delimited JSON
  │ for scanner.Scan() {                      // Loop until disconnect/error
  │   msg := DecodeMessage(line)              // protocol.go:65
  │   │
  │   switch msg.Type:
  │     "auth"          → handleAuth()
  │     "connect"       → handleAuth() (if token present)
  │     "progress"      → handleProgress()
  │     "set_strategy"  → handleSetStrategy()
  │     "get_strategy"  → handleGetStrategy()
  │     "get_conflicts" → handleGetConflicts()
  │     "ping"          → send {type:"pong"}
  │     "status"        → handleStatusRequest()
  │     "disconnect"    → goto cleanup
  │     default         → send error
  │ }
  │
  │ cleanup:
  │   delete(s.Connections, userID)            // Remove from map
  │   broadcastToOthers("user_left", ...)      // Notify other clients
  │   conn.Close()
```

---

### 2.5 Authentication Flow (`handler.go:121`)

```
Client sends: {"type":"auth","token":"eyJhbG..."}\n
  │
  ▼
handleAuth(conn, msg, remoteAddr)
  │ auth.ValidateToken(msg.Token)             // internal/auth/jwt.go:45
  │   └─ Parse JWT, verify HMAC-SHA256, check expiry
  │   └─ Returns claims.UserID, claims.Username
  │
  │ s.mu.Lock()
  │ if oldConn exists for this user:           // Handle reconnection
  │   sendMessage(oldConn, "disconnect: Replaced by new connection")
  │   oldConn.Close()
  │ s.Connections[claims.UserID] = conn        // Register new connection
  │ s.mu.Unlock()
  │
  │ sendMessage(conn, {type:"auth", user_id, username})
  │
  │ broadcastToOthers(userID, {type:"user_joined", username})
  │
  │ return userID, username                    // Used by subsequent messages
```

---

### 2.6 Progress Update with Conflict Resolution

```
Client sends: {"type":"progress","manga_id":"one-piece","chapter":50,"device_id":"cli-alice"}\n
  │
  ▼
handleProgress(conn, msg, userID, username)     // handler.go:169
  │
  │ if userID == "": send error "authenticate first"
  │ if manga_id == "" || chapter <= 0: send error
  │
  ▼
s.ConflictResolver.Resolve(userID, mangaID, chapter, deviceID)   // conflict.go:72
  │
  │ key = "user-alice:one-piece"
  │ existing = recentUpdates[key]
  │
  │ if !found:                                 // First update for this pair
  │   recentUpdates[key] = {chapter, deviceID}
  │   return {Accepted:true, FinalChapter:50}
  │
  │ if existing.Chapter == chapter:            // Same chapter = no conflict
  │   return {Accepted:true, FinalChapter:50}
  │
  │ // ─── CONFLICT DETECTED ───
  │ switch strategy:
  │
  │   "last_write_wins" (default):
  │     Accept incoming chapter as-is
  │     return {Accepted:true, FinalChapter: incoming}
  │
  │   "merge":
  │     Pick higher chapter (max of existing vs incoming)
  │     return {Accepted:true, FinalChapter: max(existing, incoming)}
  │
  │   "user_choice":
  │     Reject incoming, keep existing
  │     return {Accepted:false, FinalChapter: existing}
  │
  │ Append to conflictLog (capped at 100 entries)
  │ Update recentUpdates[key] if accepted
  │
  ▼ Back in handleProgress():
  │
  │ if !result.Accepted:
  │   sendMessage(conn, {type:"conflict", message:"awaiting user choice"})
  │   return
  │
  │ // ─── PERSIST TO DATABASE ───
  │ if s.Persister != nil:
  │   s.Persister.UpdateProgress(userID, &UpdateProgressRequest{
  │     MangaID: mangaID, CurrentChapter: finalChapter, Status: "reading"
  │   })
  │   └─ userService.UpdateProgress()          // internal/user/service.go:206
  │       └─ repo.UpdateProgress()             // UPDATE user_progress SET ...
  │
  │ if result.Conflict != nil:                 // Auto-resolved conflict
  │   sendMessage(conn, {type:"conflict", message:"auto-resolved"})
  │
  │ // ─── BROADCAST TO ALL ───
  │ s.Broadcast <- ProgressUpdate{userID, mangaID, finalChapter, timestamp}
```

---

### 2.7 Broadcast Loop (`server.go:96`)

```
broadcastLoop()                                // Runs as dedicated goroutine
  │
  │ for update := range s.Broadcast:           // Blocks until message arrives
  │   msg := NewBroadcastMessage(userID, mangaID, chapter)  // protocol.go:108
  │     └─ {type:"broadcast", user_id, manga_id, chapter, message, timestamp}
  │
  │   s.mu.RLock()
  │   for _, conn := range s.Connections:       // Send to ALL connected clients
  │     SendMessage(conn, msg)                  // protocol.go:74
  │       └─ json.Marshal(msg) + '\n'
  │       └─ conn.SetWriteDeadline(5s)
  │       └─ conn.Write(data)
  │   s.mu.RUnlock()
```

**Also:** `broadcastToOthers(excludeUserID, msg)` — same logic but skips the sender (used for join/leave notifications).

---

### 2.8 Strategy & Conflict Management

#### Set Strategy (`handler.go:281`)

```
Client: {"type":"set_strategy","strategy":"merge"}\n
  │
  ▼
handleSetStrategy()
  │ Validate: must be "last_write_wins", "merge", or "user_choice"
  │ s.ConflictResolver.SetStrategy(strategy)   // conflict.go:221
  │ sendMessage(conn, {type:"status", strategy, message:"Strategy changed"})
```

#### Get Strategy (`handler.go:316`)

```
Client: {"type":"get_strategy"}\n → Server: {type:"strategy_info", strategy:"merge"}
```

#### Get Conflicts (`handler.go:332`)

```
Client: {"type":"get_conflicts"}\n
  ▼
Server: {type:"conflicts_info", strategy, message: "<JSON array>", chapter: <count>}
  └─ message field contains JSON-encoded []ProgressConflict
```

---

### 2.9 HTTP API Integration (Sync Endpoints)

These HTTP endpoints in `main.go` query the TCP server state directly:

| Method | Path | What it does | main.go line |
|--------|------|-------------|-------------|
| `GET` | `/sync/status` | Returns connected users, uptime | 490 |
| `GET` | `/sync/conflicts` | Returns conflict log + strategy | 525 |
| `GET` | `/sync/strategy` | Returns current strategy | 545 |
| `PUT` | `/sync/strategy` | Changes strategy at runtime | 558 |

```
GET /sync/status
  ▼
if server.TCPServer != nil:
  connectedUsers = server.TCPServer.GetConnectedUsers()  // server.go:126
  uptime = server.TCPServer.GetUptime()                  // server.go:145
else if server.TCPClient != nil:
  status = server.TCPClient.RequestStatus()              // client.go:63
```

---

### 2.10 Concurrency Model

```
                    ┌─────────────────────────┐
                    │  Accept Loop (main)      │
                    │  listener.Accept()       │
                    └────────┬────────────────┘
                             │ go handleConnection(conn)
               ┌─────────────┼──────────────┐
               ▼             ▼              ▼
     ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
     │ Client 1     │ │ Client 2     │ │ Client 3     │
     │ goroutine    │ │ goroutine    │ │ goroutine    │
     │ (read loop)  │ │ (read loop)  │ │ (read loop)  │
     └──────┬───────┘ └──────┬───────┘ └──────┬───────┘
            │                │                │
            └─── Broadcast ──┴──── chan ──────┘
                             │
                    ┌────────▼────────────────┐
                    │  broadcastLoop()        │
                    │  (dedicated goroutine)  │
                    │  reads from chan,        │
                    │  writes to ALL conns     │
                    └─────────────────────────┘

Shared state protection:
  s.Connections map ← guarded by s.mu (sync.RWMutex)
  ConflictResolver  ← guarded by cr.mu (sync.RWMutex)
  Broadcast channel ← buffered (100), non-blocking send with fallback
```

---

### 2.11 ProgressPersister Interface

```go
// internal/tcp/server.go:15
type ProgressPersister interface {
    UpdateProgress(userID string, req *models.UpdateProgressRequest) (*models.UserProgress, error)
}
```

This is satisfied by `userPkg.Service` — decouples TCP package from user package (avoids circular imports). Wired in both `cmd/api-server/main.go:160` and `cmd/tcp-server/main.go:36`.

---

### 2.12 Test Client (`cmd/tcp-client/main.go`)

```
Usage: tcp-client <username> <jwt-token>

main()
  │ net.DialTimeout("tcp", "localhost:9090", 5s)
  │
  │ go func() {                                // Reader goroutine
  │   scanner := bufio.NewScanner(conn)
  │   for scanner.Scan():
  │     json.Unmarshal → switch msg.Type:
  │       "welcome"     → print 🟢
  │       "auth"        → print ✅
  │       "broadcast"   → print 📢
  │       "user_joined" → print 👋
  │       "user_left"   → print 👋
  │       "error"       → print ❌
  │       "pong"        → print 🏓
  │ }()
  │
  │ sendMessage(conn, {type:"auth", token})    // Authenticate
  │
  │ // Interactive stdin loop:
  │   "progress <manga_id> <chapter>" → send progress message
  │   "ping"                          → send ping
  │   "quit"                          → send disconnect, exit
```

---

## 3. UDP Notifications (18 pts)

### 3.1 Architecture Overview

```
┌────────────────────────────────────────────────────────────┐
│  cmd/api-server/main.go  (embedded mode)                   │
│    OR                                                      │
│  cmd/udp-server/main.go  (standalone mode)                 │
│                                                            │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  udp.NotificationServer   (server.go)                │  │
│  │    ├── net.UDPConn (port 9091)                       │  │
│  │    ├── Clients []net.UDPAddr  (registered listeners) │  │
│  │    └── mu sync.RWMutex                               │  │
│  └──────────────────────────────────────────────────────┘  │
│         ▲                                                  │
│         │  UDP datagrams (JSON + \n)                        │
│         ▼                                                  │
│  ┌──────────────┐  ┌──────────────┐                        │
│  │  CLI client   │  │  CLI client   │  ...                  │
│  │  (cmd/cli/    │  │  (cmd/cli/    │                       │
│  │   notify.go)  │  │   notify.go)  │                       │
│  └──────────────┘  └──────────────┘                        │
│                                                            │
│  Also receives from:                                       │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  HTTP API (POST /notify/broadcast)                   │  │
│  │    → UDPServer.BroadcastNotification() (local mode)  │  │
│  │    → UDPClient.SendNotification()      (remote mode) │  │
│  └──────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────┘
```

**Key files:**

| File | Purpose |
|------|---------|
| `internal/udp/server.go` | `NotificationServer` struct, Start/Stop, message handling, client registration |
| `internal/udp/notifier.go` | `BroadcastNotification`, `NotifyNewChapter`, `NotifySystem` helpers |
| `internal/udp/client.go` | `NotificationClient` for remote-mode API → standalone UDP server |
| `cmd/udp-server/main.go` | Standalone UDP server entry point |
| `cmd/cli/notify.go` | CLI commands (subscribe, unsubscribe, test, send) |

---

### 3.2 Notification Message Format

**File:** `internal/udp/server.go:22`

```go
type Notification struct {
    Type      string `json:"type"`      // "new_chapter", "system", "manga_update",
                                        // "register", "unregister", "register_ack", "test", "error"
    MangaID   string `json:"manga_id,omitempty"`
    Title     string `json:"title,omitempty"`
    Message   string `json:"message"`
    Timestamp int64  `json:"timestamp,omitempty"`
}
```

#### Client → Server Messages

| Type | Purpose |
|------|---------|
| `register` | Subscribe to notifications |
| `unregister` | Unsubscribe from notifications |
| `test` | Echo test — server responds directly |

#### Server → Client Messages

| Type | Purpose |
|------|---------|
| `register_ack` | Confirms registration/unregistration |
| `new_chapter` | A new chapter was released |
| `manga_update` | Manga metadata was updated |
| `system` | System-wide announcement |
| `test` | Echo test response |
| `error` | Invalid message notification |

#### API → Server Broadcast Wrapper

```json
{
  "type": "api_broadcast",
  "notification": { "type": "new_chapter", "manga_id": "...", "message": "..." }
}
```

---

### 3.3 Server Lifecycle

#### Embedded Mode (`cmd/api-server/main.go:185-203`)

```
if enableUDPServer == "true":
  udpServer := udp.NewNotificationServer("9091")   // server.go:31
  server.UDPServer = udpServer
  go udpServer.Start()                              // Run in goroutine

else:
  udpClient := udp.NewNotificationClient(udpPort)   // client.go:17
  server.UDPClient = udpClient                       // Remote mode
```

#### Standalone Mode (`cmd/udp-server/main.go`)

```
main()
  │ server := udp.NewNotificationServer(port)
  │
  │ // Graceful shutdown on Ctrl+C
  │ signal.Notify(sigChan, SIGINT, SIGTERM)
  │ go func() { <-sigChan; server.Stop(); os.Exit(0) }()
  │
  │ server.Start()                                    // Blocks
```

#### Server Start (`internal/udp/server.go:39`)

```
NotificationServer.Start()
  │ net.ResolveUDPAddr("udp", ":9091")
  │ net.ListenUDP("udp", addr)                    // Bind UDP port
  │ s.conn = conn
  │
  │ buf := make([]byte, 4096)                     // Read buffer
  │ for {
  │   n, clientAddr, _ := conn.ReadFromUDP(buf)   // Block until datagram arrives
  │   go s.handleMessage(buf[:n], clientAddr)      // Handle in goroutine
  │ }
```

---

### 3.4 Message Handling (`server.go:75`)

```
handleMessage(data, clientAddr)
  │
  │ json.Unmarshal(data, &broadcastMsg)            // Try generic map first
  │
  │ // Check if this is an API broadcast
  │ if broadcastMsg["type"] == "api_broadcast":
  │   Extract broadcastMsg["notification"]
  │   json.Marshal → json.Unmarshal into Notification
  │   s.BroadcastNotification(notif)               // notifier.go:12
  │   return
  │
  │ // Otherwise: regular client message
  │ json.Unmarshal(data, &msg)                     // Parse as Notification
  │
  │ switch msg.Type:
  │
  │   "register":
  │     s.registerClient(clientAddr)               // server.go:162
  │       └─ Check if already registered (IP:Port match)
  │       └─ s.Clients = append(s.Clients, *addr)
  │     s.sendTo(clientAddr, {type:"register_ack",
  │       message:"Registered. You are client #N."})
  │
  │   "unregister":
  │     s.unregisterClient(clientAddr)             // server.go:177
  │       └─ Find and remove from s.Clients slice
  │     s.sendTo(clientAddr, {type:"register_ack",
  │       message:"Unregistered."})
  │
  │   "test":
  │     s.sendTo(clientAddr, {type:"test",
  │       message:"UDP notification system is working!"})
  │
  │   default:
  │     s.sendTo(clientAddr, {type:"error",
  │       message:"Unknown type. Valid: register, unregister, test"})
```

---

### 3.5 Broadcast Mechanism (`internal/udp/notifier.go`)

#### BroadcastNotification (`notifier.go:12`)

```
BroadcastNotification(notif)                    // Fire-and-forget to ALL clients
  │
  │ if notif.Timestamp == 0:
  │   notif.Timestamp = time.Now().Unix()
  │
  │ s.mu.RLock()
  │ clients := copy of s.Clients                // Snapshot under lock
  │ s.mu.RUnlock()
  │
  │ if len(clients) == 0:
  │   log "No clients registered, notification dropped"
  │   return 0
  │
  │ for _, addr := range clients:
  │   s.sendTo(addr, notif)                     // server.go:209
  │     └─ json.Marshal(notif) + '\n'
  │     └─ s.conn.WriteToUDP(data, addr)        // UDP datagram send
  │   sent++
  │
  │ return sent                                 // Number of clients notified
```

#### Convenience Methods (`notifier.go:41-66`)

```
NotifyNewChapter(mangaID, title, chapter)
  └─ BroadcastNotification({type:"new_chapter", manga_id, title,
       message:"New chapter! <title> — Chapter <N> released!"})

NotifyMangaUpdate(mangaID, title, updateMsg)
  └─ BroadcastNotification({type:"manga_update", manga_id, title, message})

NotifySystem(message)
  └─ BroadcastNotification({type:"system", message})
```

---

### 3.6 sendTo — Single Client Send (`server.go:209`)

```
sendTo(addr, notif)
  │ data, _ := json.Marshal(notif)
  │ data = append(data, '\n')                   // Newline delimiter
  │ s.conn.WriteToUDP(data, addr)               // Fire-and-forget, no ACK
```

> **Key design:** UDP is connectionless. No acknowledgement, no retransmission.
> If a client is offline or the packet is lost, the notification is simply dropped.

---

### 3.7 Client Management

#### Data Structure

```go
type NotificationServer struct {
    Port    string
    Clients []net.UDPAddr    // Slice of registered client addresses
    mu      sync.RWMutex     // Protects Clients slice
    conn    *net.UDPConn     // Server's bound UDP connection
}
```

#### registerClient (`server.go:162`)

```
registerClient(addr)
  │ s.mu.Lock()
  │ for _, client := range s.Clients:
  │   if client.IP.Equal(addr.IP) && client.Port == addr.Port:
  │     return                                  // Already registered, skip
  │ s.Clients = append(s.Clients, *addr)        // Add new client
  │ s.mu.Unlock()
```

#### unregisterClient (`server.go:177`)

```
unregisterClient(addr)
  │ s.mu.Lock()
  │ for i, client := range s.Clients:
  │   if client.IP.Equal(addr.IP) && client.Port == addr.Port:
  │     s.Clients = append(s.Clients[:i], s.Clients[i+1:]...)  // Remove
  │     break
  │ s.mu.Unlock()
```

#### Query Methods

```
GetClientCount() int        // len(s.Clients) under RLock
GetClients() []string       // Returns string representations of all client addrs
```

---

### 3.8 HTTP API Integration

#### POST `/notify/broadcast` (Auth required, `main.go:591`)

```
POST /notify/broadcast
  {"type":"new_chapter","manga_id":"one-piece","message":"Chapter 1100!"}
  │
  ├─ AuthMiddleware
  ▼
Inline handler (main.go:591)
  │ c.ShouldBindJSON(&req)
  │ Build udp.Notification{Type, MangaID, Message, Timestamp}
  │
  │ if server.UDPClient != nil:                 // Remote mode
  │   server.UDPClient.SendNotification(notif)  // client.go:31
  │     └─ Wrap in {"type":"api_broadcast","notification":{...}}
  │     └─ net.DialUDP → conn.Write(data) → conn.Close()
  │
  │ else if server.UDPServer != nil:            // Local mode
  │   sent = server.UDPServer.BroadcastNotification(notif)
  │
  ▼
Response: {"type":"new_chapter","sent_count":N,"message":"..."}
```

#### GET `/notify/status` (Auth required, `main.go:630`)

```
GET /notify/status
  ▼
if server.UDPServer != nil:
  clientCount = server.UDPServer.GetClientCount()
  clients = server.UDPServer.GetClients()

Response: {"server":"localhost:9091","client_count":N,"clients":["addr1","addr2"]}
```

---

### 3.9 NotificationClient — Remote Mode (`internal/udp/client.go`)

Used when `ENABLE_UDP_SERVER=false` (API connects to standalone UDP server):

```go
type NotificationClient struct {
    ServerAddr string
}
```

#### SendNotification (`client.go:31`)

```
SendNotification(notif)
  │ broadcastMsg := {
  │   "type":         "api_broadcast",
  │   "notification": notif
  │ }
  │ data := json.Marshal(broadcastMsg) + '\n'
  │
  │ addr := net.ResolveUDPAddr("udp", serverAddr)
  │ conn := net.DialUDP("udp", nil, addr)      // Create ephemeral connection
  │ conn.SetWriteDeadline(2s)
  │ conn.Write(data)                            // Send single datagram
  │ conn.Close()                                // Immediately close
```

> Each notification creates a new UDP "connection" (just a local socket with a target).
> This is fine for UDP since it's connectionless — no handshake overhead.

---

### 3.10 CLI Integration (`cmd/cli/notify.go`)

```
mangahub notify subscribe    → Send {"type":"register"} to UDP server
                               Start listener goroutine to receive notifications
mangahub notify unsubscribe  → Send {"type":"unregister"} to UDP server
mangahub notify test         → Send {"type":"test"}, wait for echo response
mangahub notify send         → HTTP POST /notify/broadcast (requires auth)
```

The CLI subscriber flow:

```
CLI: mangahub notify subscribe
  │
  │ net.ResolveUDPAddr("udp", "localhost:9091")
  │ net.DialUDP("udp", nil, serverAddr)
  │
  │ Send: {"type":"register"}\n
  │ ← Receive: {"type":"register_ack","message":"Registered..."}
  │
  │ // Blocking listen loop:
  │ for {
  │   conn.ReadFromUDP(buf)
  │   json.Unmarshal → display notification
  │     "new_chapter"  → 📚 New chapter!
  │     "manga_update" → 📝 Manga updated
  │     "system"       → 📢 System message
  │     "test"         → ✅ Test received
  │ }
```

---

### 3.11 Health Check Integration (`main.go:284`)

```
GET /health/udp
  ▼
if server.UDPServer == nil:
  return {"status":"disabled","mode":"external"}

return {
  "status":       "healthy",
  "mode":         "internal",
  "port":         "9091",
  "client_count": server.UDPServer.GetClientCount(),
  "clients":      server.UDPServer.GetClients()
}
```

---

## 4. WebSocket Chat & Room Management (10 pts)

### 4.1 Architecture Overview

```
Browser / CLI client
  │
  │  GET /ws/chat?token=<jwt>&room=<room>
  │
  ▼  HTTP → WebSocket Upgrade (gorilla/websocket)
┌───────────────────────────────────────────────────────────────┐
│  internal/websocket/client.go                                 │
│    HandleWebSocket(hub, w, r)                                 │
│      ├── auth.ValidateToken(token)       ← JWT from ?token=   │
│      ├── upgrader.Upgrade(w, r, nil)     ← HTTP → WS upgrade │
│      ├── hub.Register <- client                               │
│      ├── go writePump(client)            ← dedicated writer   │
│      └── readPump(hub, client)           ← blocks (reader)    │
└───────────────────────────────────────────────────────────────┘
         ▲                     │
         │ client.Send chan    │ hub.Broadcast chan
         │                     ▼
┌───────────────────────────────────────────────────────────────┐
│  internal/websocket/hub.go                                    │
│    ChatHub.Run()  ← single central event loop goroutine       │
│      select:                                                  │
│        Register   → add client, send join notification        │
│        Unregister → remove client, send leave notification    │
│        Broadcast  → store in history, fan out to room clients │
└───────────────────────────────────────────────────────────────┘
```

**Key files:**

| File | Purpose |
|------|---------|
| `internal/websocket/hub.go` | `ChatHub` struct, `Run()` event loop, room management, history |
| `internal/websocket/client.go` | `HandleWebSocket`, `readPump`, `writePump`, slash command handling |
| `cmd/api-server/main.go` | Hub initialization, route registration, chat history endpoint |

---

### 4.2 Core Data Structures (`hub.go`)

```go
type ChatClient struct {
    Conn     *ws.Conn           // gorilla/websocket connection
    Username string
    UserID   string
    Room     string             // Room this client is in (e.g., "general", "one-piece")
    Send     chan ChatMessage   // Buffered channel (256) — outgoing messages
}

type ChatHub struct {
    Clients    map[*ChatClient]bool   // Set of ALL active clients (all rooms)
    Broadcast  chan ChatMessage        // Incoming messages to broadcast (256 buffer)
    Register   chan *ChatClient        // Client registration requests
    Unregister chan *ChatClient        // Client unregistration requests
    mu         sync.RWMutex
    History    map[string][]ChatMessage // Per-room message history
    maxHistory int                      // 50 messages per room
}

type ChatMessage struct {
    Type      string   `json:"type"`       // "message","system","pm","join","leave","users","history","error"
    UserID    string   `json:"user_id"`
    Username  string   `json:"username"`
    Message   string   `json:"message"`
    Recipient string   `json:"recipient"`  // For /pm
    Room      string   `json:"room"`       // Room scope
    Users     []string `json:"users"`      // For /users response
    Timestamp int64    `json:"timestamp"`
}
```

---

### 4.3 Hub Initialization & Route Registration

```
cmd/api-server/main.go:

main()
  │ chatHub := wsPkg.NewChatHub()               // hub.go:56
  │   └─ Initializes Clients map, 3 channels, History map, maxHistory=50
  │ go chatHub.Run()                             // Start event loop goroutine
  │ server.ChatHub = chatHub
  │
  │ // Register HTTP route (public — auth via query param)
  │ r.GET("/ws/chat", func(c *gin.Context) {
  │   wsPkg.HandleWebSocket(chatHub, c.Writer, c.Request)  // client.go:26
  │ })
  │
  │ // Chat history endpoint (authenticated)
  │ r.GET("/chat/history", auth.AuthMiddleware(), func(c *gin.Context) {
  │   room := c.DefaultQuery("room", "general")
  │   limit := c.DefaultQuery("limit", "50")
  │   history := chatHub.GetHistory(room, limitInt)
  │   return {"room": room, "messages": history, "count": len(history)}
  │ })
```

---

### 4.4 Client Connection Lifecycle

```
Client: ws://localhost:8080/ws/chat?token=eyJhbG...&room=one-piece
  │
  ▼
HandleWebSocket(hub, w, r)                      // client.go:26
  │
  │ token := r.URL.Query().Get("token")         // Extract JWT from URL
  │ if token == "": return 401 "Missing token"
  │
  │ claims := auth.ValidateToken(token)          // internal/auth/jwt.go:45
  │ if err: return 401 "Invalid or expired token"
  │
  │ room := r.URL.Query().Get("room")
  │ if room == "": room = "general"             // Default room
  │
  │ conn := upgrader.Upgrade(w, r, nil)         // HTTP → WebSocket
  │   └─ ReadBufferSize: 1024, WriteBufferSize: 1024
  │   └─ CheckOrigin: allow all (development)
  │
  │ client := &ChatClient{
  │   Conn: conn, Username: claims.Username,
  │   UserID: claims.UserID, Room: room,
  │   Send: make(chan ChatMessage, 256)
  │ }
  │
  │ hub.Register <- client                      // Send to hub event loop
  │
  │ client.Send <- ChatMessage{                 // Welcome message
  │   Type: "system",
  │   Message: "Welcome to MangaHub Chat, alice!",
  │   Room: room,
  │   Users: hub.GetOnlineUsers(room)           // Include user list
  │ }
  │
  │ go writePump(client)                        // Start writer goroutine
  │ readPump(hub, client)                       // Blocks here until disconnect
```

---

### 4.5 The Hub Event Loop (`hub.go:71 — Run()`)

**This is the single goroutine that coordinates all state changes:**

```
Run()
  for {
    select {

    // ─── CLIENT JOINS ───
    case client := <-h.Register:
      │ h.Clients[client] = true                // Add to global map
      │
      │ // Notify others in SAME room
      │ for c in Clients where c.Room == client.Room && c != client:
      │   c.Send <- {type:"join", message:"alice joined the chat", room:"one-piece"}
      │
      │ // Send room history to new client
      │ for msg in History[client.Room]:
      │   client.Send <- msg

    // ─── CLIENT LEAVES ───
    case client := <-h.Unregister:
      │ delete(h.Clients, client)
      │ close(client.Send)                      // Signals writePump to exit
      │
      │ // Notify remaining clients in SAME room
      │ for c in Clients where c.Room == client.Room:
      │   c.Send <- {type:"leave", message:"alice left the chat", room:"one-piece"}

    // ─── MESSAGE BROADCAST ───
    case msg := <-h.Broadcast:
      │ // Store in per-room history (message type only)
      │ if msg.Type == "message":
      │   History[msg.Room] = append(History[msg.Room], msg)
      │   if len > 50: trim to last 50          // Sliding window
      │
      │ // Fan out to all clients in SAME room
      │ for c in Clients where c.Room == msg.Room:
      │   select {
      │     case c.Send <- msg:                 // Non-blocking send
      │     default:                            // Skip if buffer full
      │   }
    }
  }
```

> **Room isolation**: Every operation filters by `c.Room == msg.Room`. Clients in different rooms never see each other's messages, joins, or leaves.

---

### 4.6 Read Pump — Per-Client Reader (`client.go:84`)

```
readPump(hub, client)                           // Runs in handler goroutine
  │
  │ defer: hub.Unregister <- client; conn.Close()  // Cleanup on exit
  │
  │ conn.SetReadDeadline(5 minutes)             // Keepalive timeout
  │ conn.SetPongHandler(func() {                // Reset on pong
  │   conn.SetReadDeadline(now + 5min)
  │ })
  │
  │ for {
  │   msg := conn.ReadJSON(&ChatMessage)        // Block until message
  │   if err: break                             // Disconnected → cleanup
  │
  │   msg.Username = client.Username            // Stamp sender info
  │   msg.UserID = client.UserID
  │   msg.Timestamp = time.Now().Unix()
  │
  │   handleClientMessage(hub, client, &msg)    // Process message/command
  │ }
```

---

### 4.7 Write Pump — Per-Client Writer (`client.go:122`)

```
writePump(client)                               // Runs as dedicated goroutine
  │
  │ ticker := time.NewTicker(25 seconds)        // Ping interval
  │ defer: ticker.Stop(); conn.Close()
  │
  │ for {
  │   select {
  │     case msg, ok := <-client.Send:
  │       if !ok:                               // Channel closed = unregistered
  │         conn.WriteMessage(CloseMessage)
  │         return
  │       conn.SetWriteDeadline(10s)
  │       conn.WriteJSON(msg)                   // Send to WebSocket
  │
  │     case <-ticker.C:                        // Every 25s
  │       conn.SetWriteDeadline(10s)
  │       conn.WriteMessage(PingMessage)        // Send WebSocket ping frame
  │   }
  │ }
```

> **Why separate read/write goroutines?** gorilla/websocket does NOT support concurrent writes. The write pump is the ONLY goroutine that writes to the connection, preventing data races.

---

### 4.8 Slash Commands (`client.go:155 — handleClientMessage`)

```
handleClientMessage(hub, client, msg)
  │
  │ text = strings.TrimSpace(msg.Message)
  │
  │ if !strings.HasPrefix(text, "/"):
  │   // Regular message → broadcast to room
  │   msg.Type = "message"
  │   msg.Room = client.Room
  │   hub.Broadcast <- *msg                     // Into the hub event loop
  │   return
  │
  │ // ─── SLASH COMMANDS ───
  │ switch parts[0]:
```

| Command | Response | Details |
|---------|----------|---------|
| `/help` | `{type:"system"}` | Lists all available commands |
| `/users` | `{type:"users", users:[...]}` | Online users in current room via `hub.GetOnlineUsers(room)` |
| `/quit` | `{type:"system", message:"Goodbye!"}` | Sends goodbye, closes connection after 100ms flush |
| `/pm <user> <msg>` | `{type:"pm"}` to recipient + sender | Via `hub.SendPrivateMessage()` — cross-room delivery |
| `/history` | Multiple `{type:"message"}` | Last 20 messages via `hub.GetHistory(room, 20)` |
| `/status` | `{type:"system"}` | Shows username, userID, room, user count |

---

### 4.9 Private Messaging (`hub.go:217`)

```
/pm bob Hello there!
  │
  ▼
handleClientMessage → case "/pm":
  │ target = "bob"
  │ pmMsg = "Hello there!"
  │
  │ if target == self: send error "Can't PM yourself!"
  │
  │ found = hub.SendPrivateMessage("alice", "bob", "Hello there!")
  │   └─ hub.go:217
  │   └─ Iterate ALL h.Clients (any room)
  │   └─ Find c.Username == "bob"
  │   └─ c.Send <- {type:"pm", username:"alice", recipient:"bob",
  │                  message:"Hello there!", room:"pm"}
  │   └─ return true
  │
  │ if found: send confirmation to alice (sender echo)
  │ else: send error "User 'bob' is not online"
```

> **Cross-room**: Private messages are NOT room-scoped. A user in "general" can PM a user in "one-piece".

---

### 4.10 Room Management

#### Room Creation

Rooms are created **implicitly** when a client connects with `?room=<name>`:

```
ws://localhost:8080/ws/chat?token=xxx&room=one-piece    ← creates "one-piece" room
ws://localhost:8080/ws/chat?token=xxx&room=naruto        ← creates "naruto" room
ws://localhost:8080/ws/chat?token=xxx                     ← joins "general" (default)
```

No explicit room creation API — rooms exist as long as clients are in them.

#### Room Isolation

All hub operations filter by `c.Room == targetRoom`:

| Operation | Scope |
|-----------|-------|
| `Register` (join notification) | Same room only |
| `Unregister` (leave notification) | Same room only |
| `Broadcast` (message) | Same room only |
| `GetOnlineUsers(room)` | Filtered by room |
| `GetClientCount(room)` | Filtered by room |
| `GetHistory(room, limit)` | Per-room history map |
| `SendPrivateMessage` | **Cross-room** (all clients) |

#### Room History

```go
History map[string][]ChatMessage   // key = room name

// On broadcast (type "message" only):
History["one-piece"] = append(History["one-piece"], msg)
if len > 50: trim to last 50 entries             // Sliding window

// On new client join:
for msg in History[client.Room]:
  client.Send <- msg                              // Replay history
```

---

### 4.11 Concurrency Model

```
                     ┌──────────────────────────┐
                     │  ChatHub.Run()            │
                     │  (single goroutine)       │
                     │  select:                  │
                     │    Register / Unregister  │
                     │    Broadcast              │
                     └──────┬──────┬────────────┘
                            │      │
            ┌───────────────┘      └───────────────┐
            ▼                                      ▼
  ┌───────────────────┐                  ┌───────────────────┐
  │  Client A          │                  │  Client B          │
  │  ┌──────────────┐ │                  │  ┌──────────────┐ │
  │  │ readPump()   │ │                  │  │ readPump()   │ │
  │  │ (goroutine 1)│ │                  │  │ (goroutine 1)│ │
  │  └──────┬───────┘ │                  │  └──────┬───────┘ │
  │         │ msg      │                  │         │ msg      │
  │  ┌──────▼───────┐ │                  │  ┌──────▼───────┐ │
  │  │ writePump()  │ │                  │  │ writePump()  │ │
  │  │ (goroutine 2)│ │                  │  │ (goroutine 2)│ │
  │  └──────────────┘ │                  │  └──────────────┘ │
  └───────────────────┘                  └───────────────────┘

Per client: 2 goroutines (read + write)
Hub: 1 goroutine (event loop)
Total for N clients: 2N + 1 goroutines

Channels prevent data races:
  hub.Register   ← readPump sends on connect
  hub.Unregister ← readPump sends on disconnect
  hub.Broadcast  ← readPump sends on message
  client.Send    ← hub writes, writePump reads
```

---

### 4.12 Keepalive Mechanism

```
writePump: every 25s → conn.WriteMessage(PingMessage)
                          │
                          ▼ (WebSocket protocol)
                        Client replies with Pong frame (automatic)
                          │
                          ▼
readPump: PongHandler → conn.SetReadDeadline(now + 5min)

If no pong received within 5 minutes:
  ReadJSON fails → readPump exits → hub.Unregister <- client
```

---

### 4.13 HTTP API for Chat (`main.go`)

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| `GET` | `/ws/chat?token=&room=` | Query param JWT | WebSocket upgrade endpoint |
| `GET` | `/chat/history?room=&limit=` | Bearer JWT | Retrieve room history via REST |
| `GET` | `/health/ws` | None | WebSocket health check |

#### Chat History Endpoint (`main.go:656`)

```
GET /chat/history?room=one-piece&limit=20
  │
  ├─ AuthMiddleware
  ▼
Inline handler
  │ room = c.DefaultQuery("room", "general")
  │ limit = c.DefaultQuery("limit", "50")
  │ history = chatHub.GetHistory(room, limit)    // hub.go:241
  │   └─ Returns copy of last N messages from History[room]
  ▼
Response: {"room":"one-piece","messages":[...],"count":N}
```

#### Health Check (`main.go:388`)

```
GET /health/ws
  ▼
if server.ChatHub == nil:
  return {"status":"disabled"}

rooms := count unique rooms from all clients
return {
  "status":        "healthy",
  "total_clients": len(Clients),
  "rooms":         roomDetails,       // [{name, clients, history_size}]
}
```

---

## 5. gRPC Service (7 pts)

### 5.1 Architecture Overview

```
┌───────────────────────────────────────────────────────────────┐
│  cmd/api-server/main.go  (embedded mode)                      │
│    OR                                                         │
│  cmd/grpc-server/main.go (standalone mode)                    │
│                                                               │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │  grpc.MangaServer   (internal/grpc/server.go)           │  │
│  │    ├── GetManga(ctx, req)     → mangaService.GetByID()  │  │
│  │    ├── SearchManga(ctx, req)  → mangaService.Search()   │  │
│  │    └── UpdateProgress(ctx,req)→ userService.Update...() │  │
│  │                                                         │  │
│  │  AuthInterceptor  (JWT validation)                      │  │
│  │    └── auth.ValidateToken()                             │  │
│  └─────────────────────────────────────────────────────────┘  │
│         ▲                                                     │
│         │  gRPC / HTTP/2 (protobuf, port 9092)                │
│         ▼                                                     │
│  ┌──────────────┐                                             │
│  │  gRPC Client  │  (internal/grpc/client.go)                 │
│  │  MangaClient  │  OR any gRPC client (grpcurl, etc.)        │
│  └──────────────┘                                             │
└───────────────────────────────────────────────────────────────┘
```

**Key files:**

| File | Purpose |
|------|---------|
| `proto/manga.proto` | Protobuf service + message definitions |
| `internal/grpc/pb/` | Generated Go code from protoc |
| `internal/grpc/server.go` | `MangaServer` implementation, `AuthInterceptor`, `StartGRPCServer` |
| `internal/grpc/client.go` | `MangaClient` wrapper for consuming the gRPC service |
| `cmd/grpc-server/main.go` | Standalone gRPC server entry point |

---

### 5.2 Protobuf Definition (`proto/manga.proto`)

```protobuf
syntax = "proto3";
package mangahub;
option go_package = "mangahub/internal/grpc/pb";

service MangaService {
    rpc GetManga(GetMangaRequest)     returns (MangaResponse);
    rpc SearchManga(SearchRequest)    returns (SearchResponse);
    rpc UpdateProgress(ProgressRequest) returns (ProgressResponse);
}
```

#### Messages

| Message | Fields |
|---------|--------|
| `GetMangaRequest` | `string manga_id` |
| `MangaResponse` | `id`, `title`, `author`, `genres[]`, `status`, `total_chapters`, `description` |
| `SearchRequest` | `string query`, `string genre`, `int32 limit` |
| `SearchResponse` | `MangaResponse[] results`, `int32 total` |
| `ProgressRequest` | `string user_id`, `string manga_id`, `int32 chapter` |
| `ProgressResponse` | `bool success`, `string message` |

---

### 5.3 Server Implementation (`internal/grpc/server.go`)

#### MangaServer Struct

```go
type MangaServer struct {
    pb.UnimplementedMangaServiceServer       // Forward compatibility
    mangaService *mangaPkg.Service           // Reuses HTTP API's manga service
    userService  *userPkg.Service            // Reuses HTTP API's user service
}
```

> **Key design**: The gRPC server delegates ALL business logic to the same `mangaService` and `userService` used by the HTTP API. No duplicated logic.

---

### 5.4 RPC Method Flows

#### GetManga (`server.go:39`)

```
gRPC Client: GetManga({manga_id: "one-piece"})
  │
  ├─ AuthInterceptor (validates JWT from metadata)
  ▼
MangaServer.GetManga(ctx, req)
  │ if req.MangaId == "": return InvalidArgument
  │
  │ manga := s.mangaService.GetByID("one-piece")   // internal/manga/service.go:65
  │   └─ cache.Get("manga:one-piece")               // Try Redis first
  │   └─ repo.FindByID("one-piece")                 // SELECT ... WHERE id = ?
  │
  │ return mangaToProto(manga)                       // Convert to protobuf
  │   └─ &pb.MangaResponse{
  │        Id: m.ID, Title: m.Title, Author: m.Author,
  │        Genres: m.Genres, Status: m.Status,
  │        TotalChapters: int32(m.TotalChapters),
  │        Description: m.Description,
  │      }
```

#### SearchManga (`server.go:53`)

```
gRPC Client: SearchManga({query: "one", genre: "action", limit: 10})
  │
  ├─ AuthInterceptor
  ▼
MangaServer.SearchManga(ctx, req)
  │ limit = req.Limit (default 20, max 100)
  │
  │ query := &models.MangaSearchQuery{
  │   Search: req.Query, Genre: req.Genre,
  │   Page: 1, Limit: limit,
  │ }
  │
  │ mangaList, total := s.mangaService.Search(query)  // service.go:90
  │   └─ Redis cache check → SQLite query → cache populate
  │
  │ for each manga: results = append(results, mangaToProto(&manga))
  │
  │ return &pb.SearchResponse{Results: results, Total: int32(total)}
```

#### UpdateProgress (`server.go:86`)

```
gRPC Client: UpdateProgress({user_id: "user-alice", manga_id: "one-piece", chapter: 50})
  │
  ├─ AuthInterceptor
  ▼
MangaServer.UpdateProgress(ctx, req)
  │ Validate: user_id required, manga_id required, chapter >= 0
  │
  │ progressReq := &models.UpdateProgressRequest{
  │   MangaID: req.MangaId,
  │   CurrentChapter: int(req.Chapter),
  │   Status: "reading",
  │ }
  │
  │ s.userService.UpdateProgress(req.UserId, progressReq)  // user/service.go:206
  │   └─ repo.GetLibraryEntry()           // Verify manga in library
  │   └─ repo.UpdateProgress()            // UPDATE user_progress SET ...
  │   └─ invalidateUserCaches(userID)     // Clear Redis cache
  │
  │ if err:
  │   return {success: false, message: "failed: ..."}
  │ return {success: true, message: "Updated one-piece to chapter 50"}
```

---

### 5.5 JWT Auth Interceptor (`server.go:131`)

```
AuthInterceptor(ctx, req, info, handler)        // grpc.UnaryServerInterceptor
  │
  │ md := metadata.FromIncomingContext(ctx)      // Extract gRPC metadata
  │ if !ok: return Unauthenticated "missing metadata"
  │
  │ authHeader := md["authorization"]            // Get authorization header
  │ if missing: return Unauthenticated "missing authorization header"
  │
  │ tokenString := authHeader[0]
  │ if HasPrefix("Bearer "): trim prefix         // Support "Bearer <token>" format
  │
  │ claims := auth.ValidateToken(tokenString)    // internal/auth/jwt.go:45
  │ if err: return Unauthenticated "invalid token"
  │
  │ // Inject user info into context
  │ ctx = context.WithValue(ctx, "user_id", claims.UserID)
  │ ctx = context.WithValue(ctx, "username", claims.Username)
  │
  │ return handler(ctx, req)                     // Continue to actual RPC method
```

> **Same JWT**: Uses the exact same `auth.ValidateToken()` function as HTTP and TCP. One token works across all protocols.

---

### 5.6 Server Lifecycle

#### Embedded Mode (`cmd/api-server/main.go:216-225`)

```
if enableGRPCServer == "true":
  go grpcPkg.StartGRPCServer(grpcPort, mangaService, userService)
```

#### Standalone Mode (`cmd/grpc-server/main.go`)

```
main()
  │ database.InitDB(dbPath)
  │ mangaRepo := mangaPkg.NewRepository(db)
  │ mangaService := mangaPkg.NewService(mangaRepo)
  │ userRepo := userPkg.NewRepository(db)
  │ userService := userPkg.NewService(userRepo)
  │
  │ grpcPkg.StartGRPCServer(port, mangaService, userService)  // Blocks
```

#### StartGRPCServer (`server.go:162`)

```
StartGRPCServer(port, mangaService, userService)
  │ lis := net.Listen("tcp", ":9092")           // gRPC uses TCP transport
  │
  │ grpcServer := grpc.NewServer(
  │   grpc.UnaryInterceptor(AuthInterceptor),   // JWT middleware for ALL RPCs
  │ )
  │
  │ mangaServer := NewMangaServer(mangaService, userService)  // server.go:31
  │ pb.RegisterMangaServiceServer(grpcServer, mangaServer)    // Register service
  │
  │ log.Printf("🔌 gRPC MangaService listening on :%s", port)
  │ grpcServer.Serve(lis)                        // Blocks, serves requests
```

---

### 5.7 gRPC Client (`internal/grpc/client.go`)

```go
type MangaClient struct {
    conn   *grpc.ClientConn
    client pb.MangaServiceClient    // Generated gRPC client interface
    token  string                    // JWT token for auth
}
```

#### Connection (`client.go:22`)

```
NewMangaClient(addr, token)
  │ ctx := context.WithTimeout(5s)
  │ conn := grpc.DialContext(ctx, addr,
  │   grpc.WithTransportCredentials(insecure.NewCredentials()),  // No TLS (dev)
  │   grpc.WithBlock(),                                          // Wait for connection
  │ )
  │ return &MangaClient{conn, pb.NewMangaServiceClient(conn), token}
```

#### Auth Header Injection (`client.go:38`)

```
withAuth(ctx) context.Context
  │ if c.token == "": return ctx                 // No auth
  │ return metadata.AppendToOutgoingContext(ctx,
  │   "authorization", "Bearer " + c.token)      // Inject into gRPC metadata
```

#### Client Methods

```
GetManga(mangaID)
  │ ctx := withAuth(context.WithTimeout(5s))
  │ return c.client.GetManga(ctx, &pb.GetMangaRequest{MangaId: mangaID})

SearchManga(query, genre, limit)
  │ ctx := withAuth(context.WithTimeout(5s))
  │ return c.client.SearchManga(ctx, &pb.SearchRequest{Query, Genre, Limit})

UpdateProgress(userID, mangaID, chapter)
  │ ctx := withAuth(context.WithTimeout(5s))
  │ return c.client.UpdateProgress(ctx, &pb.ProgressRequest{UserId, MangaId, Chapter})

Close()
  │ c.conn.Close()
```

---

### 5.8 Health Check (`main.go:393`)

```
GET /health/grpc
  ▼
if ENABLE_GRPC_SERVER != "true":
  return {"status":"disabled","mode":"external","port":"9092"}

return {
  "status":  "healthy",
  "mode":    "internal",
  "port":    grpcPort,
  "methods": ["GetManga","SearchManga","UpdateProgress"]
}
```

---

### 5.9 Service Reuse Pattern

```
┌──────────────────────┐     ┌──────────────────────┐
│   HTTP API (Gin)     │     │   gRPC Server         │
│   mangaHandler.      │     │   MangaServer.        │
│     GetByID()        │     │     GetManga()         │
│   userHandler.       │     │   MangaServer.         │
│     UpdateProgress() │     │     UpdateProgress()   │
└──────────┬───────────┘     └──────────┬─────────────┘
           │                            │
           └────────────┬───────────────┘
                        ▼
              ┌──────────────────┐
              │  manga.Service   │  ← SHARED instance
              │  user.Service    │  ← SHARED instance
              └────────┬─────────┘
                       ▼
              ┌──────────────────┐
              │  manga.Repository│  ← Same SQLite DB
              │  user.Repository │
              └──────────────────┘
```

Both the HTTP API and gRPC server use the **same service instances**, which means:
- Same Redis cache hit/miss behavior
- Same validation rules
- Same database queries
- Changes via gRPC are immediately visible via HTTP and vice versa

---

*All 5 core feature code flows are now documented.*

---

## 6. User Reviews & Ratings

### 6.1 Architecture

**Files:**

| File | Purpose |
|------|---------|
| `internal/review/handler.go` | HTTP handlers for review CRUD + rating stats |
| `internal/review/service.go` | Business logic, validation, ownership checks |
| `internal/review/repository.go` | SQL queries against `reviews` table |

**Dependencies injected via handler constructor:**

```go
type Handler struct {
    service         *Service           // review business logic
    activityService *activity.Service  // logs "wrote_review" activity
    mangaService    *manga.Service     // resolves manga title for activity messages
}
```

---

### 6.2 Database Schema

```sql
CREATE TABLE reviews (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id),
    manga_id   TEXT NOT NULL REFERENCES manga(id),
    rating     INTEGER NOT NULL CHECK(rating >= 1 AND rating <= 10),
    text       TEXT,
    helpful    INTEGER DEFAULT 0,
    created_at DATETIME,
    updated_at DATETIME,
    UNIQUE(user_id, manga_id)    -- One review per user per manga
);
```

---

### 6.3 Create Review: `POST /manga/:id/reviews` (Auth)

```
POST /manga/one-piece/reviews  {"rating":8,"text":"Amazing story!"}
  │
  ├─ AuthMiddleware → user_id, username
  ▼
reviewHandler.CreateReview()                     // handler.go:25
  │ c.ShouldBindJSON(&req)                       // {rating, text}
  │ Validate: rating must be 1–10
  │ mangaID = c.Param("id")                      // "one-piece"
  ▼
reviewService.CreateReview(userID, mangaID, 8, "Amazing story!")  // service.go:20
  │ repo.GetReviewByUserAndManga(userID, mangaID)  // Check duplicate
  │   └─ SELECT ... WHERE user_id=? AND manga_id=?
  │   └─ Returns nil if no existing review
  │ if existing != nil: return error "user already reviewed this manga"
  │
  │ repo.CreateReview(userID, mangaID, 8, "Amazing story!")  // repository.go:23
  │   └─ id = "rev_<nanoseconds>_user-alice_one-piece"
  │   └─ INSERT INTO reviews (id, user_id, manga_id, rating, text, ...)
  │   └─ Return &Review{ID, UserID, MangaID, Rating, Text, Helpful:0}
  ▼
Back in handler:
  │ mangaTitle = mangaService.GetByID("one-piece").Title   // Resolve title
  │ activityService.LogReviewWritten(                      // Log to activity feed
  │   userID, username, mangaID, mangaTitle, review.ID, 8
  │ )
  ▼
Response: 200 {"id":"rev_...","user_id":"user-alice","rating":8,...}
```

---

### 6.4 Get Reviews for Manga: `GET /manga/:id/reviews` (Public)

```
GET /manga/one-piece/reviews?page=1&limit=10
  │
  ▼
reviewHandler.GetReviews()                       // handler.go:70
  │ Parse page/limit with defaults (page=1, limit=10, max=100)
  │ offset = (page - 1) * limit
  ▼
reviewService.GetReviewsByManga("one-piece", 10, 0)  // service.go:44
  └─ repo.GetReviewsByManga()                    // repository.go:89
       └─ SELECT COUNT(*) FROM reviews WHERE manga_id=?    // total
       └─ SELECT ... FROM reviews WHERE manga_id=?
            ORDER BY created_at DESC LIMIT 10 OFFSET 0     // paginated

reviewService.GetMangaStats("one-piece")         // service.go:94
  └─ repo.GetAverageRating("one-piece")          // repository.go:220
       └─ SELECT AVG(rating), COUNT(*) FROM reviews WHERE manga_id=?
  ▼
Response: {
  "reviews": [...],
  "total": 5,
  "page": 1,
  "limit": 10,
  "pages": 1,
  "average_rating": 7.6
}
```

---

### 6.5 Get Single Review: `GET /reviews/:review_id` (Auth)

```
reviewHandler.GetReview()                        // handler.go:111
  └─ service.GetReviewByID(reviewID)
       └─ repo.GetReviewByID()                   // repository.go:49
            └─ SELECT ... FROM reviews WHERE id=?
```

---

### 6.6 Update Review: `PUT /reviews/:review_id` (Auth + ownership)

```
PUT /reviews/rev_123  {"rating":9,"text":"Updated review"}
  │
  ├─ AuthMiddleware → user_id
  ▼
reviewHandler.UpdateReview()                     // handler.go:124
  │ Validate: rating 1–10
  │
  │ // ─── OWNERSHIP CHECK ───
  │ review = service.GetReviewByID(reviewID)
  │ if review.UserID != userID:
  │   return 403 "You can only edit your own reviews"
  │
  │ service.UpdateReview(userID, reviewID, &9, &"Updated review")  // service.go:54
  │   └─ SECOND ownership check in service layer
  │   └─ repo.UpdateReview(reviewID, &9, &text)  // repository.go:161
  │        └─ if rating != nil:
  │             UPDATE reviews SET rating=9, updated_at=now WHERE id=?
  │        └─ if text != nil:
  │             UPDATE reviews SET text=?, updated_at=now WHERE id=?
  ▼
Response: 200 {"review_id":"rev_123","rating":9,"text":"Updated review"}
```

> **Double ownership check**: Both handler (line 144) and service (line 61) verify `review.UserID == userID` — defense in depth.

---

### 6.7 Delete Review: `DELETE /reviews/:review_id` (Auth + ownership)

```
DELETE /reviews/rev_123
  │
  ├─ AuthMiddleware → user_id
  ▼
reviewHandler.DeleteReview()                     // handler.go:170
  │ review = service.GetReviewByID(reviewID)
  │ if review.UserID != userID: return 403
  │
  │ service.DeleteReview(userID, reviewID)        // service.go:74
  │   └─ SECOND ownership check
  │   └─ repo.DeleteReview(reviewID)             // repository.go:188
  │        └─ DELETE FROM reviews WHERE id=?
  │        └─ Check rowsAffected > 0
  ▼
Response: 200 "Review deleted successfully"
```

---

### 6.8 Mark Helpful: `POST /reviews/:review_id/helpful` (Auth)

```
POST /reviews/rev_123/helpful
  │
  ├─ AuthMiddleware
  ▼
reviewHandler.MarkHelpful()                      // handler.go:196
  └─ service.MarkHelpful(reviewID)               // service.go:89
       └─ repo.MarkHelpful(reviewID)             // repository.go:207
            └─ UPDATE reviews SET helpful = helpful + 1 WHERE id=?
  │
  │ review = service.GetReviewByID(reviewID)     // Fetch updated count
  ▼
Response: 200 {"helpful": 3}
```

> **No vote tracking**: Any authenticated user can increment the counter multiple times. No per-user vote deduplication.

---

### 6.9 Rating Stats: `GET /manga/:id/rating-stats` (Public)

```
GET /manga/one-piece/rating-stats
  │
  ▼
reviewHandler.GetRatingStats()                   // handler.go:210
  └─ service.GetMangaStats("one-piece")          // service.go:94
       └─ repo.GetAverageRating("one-piece")     // repository.go:220
            └─ SELECT AVG(rating), COUNT(*) FROM reviews WHERE manga_id=?
            └─ Handle NullFloat64 (returns 0 if no reviews)
  ▼
Response: {"manga_id":"one-piece","avg_rating":7.6,"review_count":5}
```

---

### 6.10 My Reviews: `GET /users/reviews` (Auth)

```
GET /users/reviews?page=1&limit=10
  │
  ├─ AuthMiddleware → user_id
  ▼
reviewHandler.GetMyReviews()                     // handler.go:224
  └─ service.GetReviewsByUser(userID, 10, 0)     // service.go:49
       └─ repo.GetReviewsByUser()                // repository.go:125
            └─ SELECT COUNT(*) FROM reviews WHERE user_id=?
            └─ SELECT ... FROM reviews WHERE user_id=?
                 ORDER BY created_at DESC LIMIT ? OFFSET ?
  ▼
Response: {"reviews":[...],"total":3,"page":1,"limit":10,"pages":1}
```

---

### 6.11 Route Table

| Method | Path | Auth | Handler |
|--------|------|------|---------|
| `POST` | `/manga/:id/reviews` | ✅ | `CreateReview` |
| `GET` | `/manga/:id/reviews` | ❌ | `GetReviews` |
| `GET` | `/manga/:id/rating-stats` | ❌ | `GetRatingStats` |
| `GET` | `/reviews/:review_id` | ✅ | `GetReview` |
| `PUT` | `/reviews/:review_id` | ✅ | `UpdateReview` |
| `DELETE` | `/reviews/:review_id` | ✅ | `DeleteReview` |
| `POST` | `/reviews/:review_id/helpful` | ✅ | `MarkHelpful` |
| `GET` | `/users/reviews` | ✅ | `GetMyReviews` |

---

### 6.12 Activity Feed Integration

When a review is created, the handler logs an activity:

```
activityService.LogReviewWritten(userID, username, mangaID, mangaTitle, reviewID, rating)
  └─ repo.CreateActivity(userID, "wrote_review", mangaID, reviewID, "",
       "alice rated One Piece 8/10: Amazing story!")
       └─ INSERT INTO activities (id, user_id, type, manga_id, review_id, message, ...)
```

This activity then appears in:
- `GET /feed/activities` — global feed
- `GET /feed/timeline` — friends' timeline
- `GET /users/:user_id/activities` — user-specific feed

---

*Sections for Friend System, Shared Reading Lists, Activity Feed, and other features will follow.*

---

## 7. Friend System

### 7.1 Architecture

**Files:**

| File | Purpose |
|------|---------|
| `internal/friend/handler.go` | HTTP handlers for friend CRUD (add, accept, decline, remove, block, list) |
| `internal/friend/service.go` | Business logic, self-friend prevention |
| `internal/friend/repository.go` | SQL queries with canonical ID ordering |

**Dependencies:**

```go
type Handler struct {
    service         *Service           // friend business logic
    activityService *activity.Service  // logs "added_friend" activity
}
```

---

### 7.2 Database Schema & Canonical Ordering

```sql
CREATE TABLE friendships (
    user_id    TEXT NOT NULL REFERENCES users(id),
    friend_id  TEXT NOT NULL REFERENCES users(id),
    status     TEXT NOT NULL DEFAULT 'pending',  -- 'pending', 'accepted', 'blocked'
    created_at DATETIME,
    PRIMARY KEY (user_id, friend_id),
    CHECK(user_id < friend_id)                  -- Canonical ordering constraint
);
```

> **Key design**: The `CHECK(user_id < friend_id)` constraint ensures every friendship is stored exactly once. Before any query, the repository **swaps IDs** so the smaller one is always `user_id`:

```go
if userID > friendID {
    userID, friendID = friendID, userID   // Enforce canonical order
}
```

This eliminates duplicate rows — Alice↔Bob is stored as `(alice, bob)` regardless of who initiated.

---

### 7.3 Friendship Lifecycle

```
                ┌──────────┐
                │  (none)  │
                └────┬─────┘
                     │ POST /friends/add
                     ▼
              ┌──────────────┐
              │   pending    │
              └──┬───┬───┬──┘
                 │   │   │
    Accept       │   │   │  Decline/Remove
    ┌────────────┘   │   └────────────┐
    ▼                │                ▼
┌──────────┐   Block │          ┌──────────┐
│ accepted │         │          │ (deleted) │
└──────────┘         ▼          └──────────┘
                ┌──────────┐
                │ blocked  │
                └──────────┘
```

---

### 7.4 Send Friend Request: `POST /friends/add` (Auth)

```
POST /friends/add  {"friend_id":"user-bob"}
  │
  ├─ AuthMiddleware → user_id = "user-alice"
  ▼
friendHandler.AddFriend()                        // handler.go:23
  │ if friend_id == user_id: return 400 "Cannot add yourself"
  ▼
friendService.AddFriend("user-alice", "user-bob")  // service.go:18
  │ if userID == friendID: return error (double check)
  ▼
repo.AddFriend("user-alice", "user-bob")         // repository.go:21
  │
  │ // ─── CANONICAL ORDERING ───
  │ if "user-alice" > "user-bob": swap            // Ensure user_id < friend_id
  │
  │ // Check existing friendship
  │ SELECT status FROM friendships WHERE user_id=? AND friend_id=?
  │
  │ if found:
  │   if status == "accepted": return "already friends"
  │   if status == "pending":
  │     UPDATE friendships SET status='accepted'   // Auto-accept mutual request
  │     return nil
  │
  │ if not found:
  │   INSERT INTO friendships (user_id, friend_id, status, created_at)
  │     VALUES (?, ?, 'pending', now)
  ▼
Response: 200 "Friend request sent successfully"
```

---

### 7.5 Accept Request: `POST /friends/:friend_id/accept` (Auth)

```
POST /friends/user-alice/accept
  │
  ├─ AuthMiddleware → user_id = "user-bob"
  ▼
friendHandler.AcceptFriend()                     // handler.go:54
  ▼
repo.AcceptFriend("user-bob", "user-alice")      // repository.go:64
  │ Swap if needed for canonical order
  │ UPDATE friendships SET status='accepted'
  │   WHERE user_id=? AND friend_id=? AND status='pending'
  │ if rowsAffected == 0: return "no pending friend request found"
  ▼
Activity feed logging (MUTUAL):
  │ activityService.LogFriendAdded(user-bob, "bob", user-alice, "user-alice")
  │ activityService.LogFriendAdded(user-alice, "user-alice", user-bob, "bob")
  │   └─ Both users get an activity: "bob became friends with user-alice"
  ▼
Response: 200 "Friend request accepted"
```

---

### 7.6 Decline Request: `POST /friends/:friend_id/decline` (Auth)

```
friendHandler.DeclineFriend()                    // handler.go:82
  └─ service.RemoveFriend(userID, friendID)      // Same as removal
       └─ repo.RemoveFriend()                    // repository.go:87
            └─ Swap IDs for canonical order
            └─ DELETE FROM friendships WHERE user_id=? AND friend_id=?
            └─ if rowsAffected == 0: return "friendship not found"
```

> Declining is implemented as a `DELETE` — the pending row is simply removed.

---

### 7.7 Remove Friend: `DELETE /friends/:friend_id` (Auth)

```
friendHandler.RemoveFriend()                     // handler.go:102
  └─ service.RemoveFriend(userID, friendID)       // service.go:32
       └─ repo.RemoveFriend()                    // repository.go:87
            └─ Swap for canonical order
            └─ DELETE FROM friendships WHERE user_id=? AND friend_id=?
```

---

### 7.8 Block User: `POST /friends/:user_id/block` (Auth)

```
POST /friends/user-bob/block
  │
  ├─ AuthMiddleware → user_id = "user-alice"
  ▼
friendHandler.BlockFriend()                      // handler.go:121
  │ if blockID == userID: return 400 "Cannot block yourself"
  ▼
repo.BlockFriend("user-alice", "user-bob")       // repository.go:110
  │ Swap for canonical order
  │ INSERT INTO friendships (user_id, friend_id, status, created_at)
  │   VALUES (?, ?, 'blocked', now)
  │   ON CONFLICT(user_id, friend_id)
  │     DO UPDATE SET status = 'blocked'          // Overwrite any existing status
  ▼
Response: 200 "User blocked successfully"
```

> **Upsert pattern**: Blocking works whether or not a friendship row exists. If they're already friends, it overwrites to `blocked`.

---

### 7.9 Get Friends: `GET /users/friends` (Auth)

```
GET /users/friends?page=1&limit=20
  │
  ├─ AuthMiddleware → user_id
  ▼
friendHandler.GetFriends()                       // handler.go:145
  ▼
repo.GetFriends(userID)                          // repository.go:126
  │ SELECT CASE
  │   WHEN user_id = ? THEN friend_id            // If I'm stored as user_id,
  │   ELSE user_id                                //   return the other person
  │ END as friend_id
  │ FROM friendships
  │ WHERE (user_id = ? OR friend_id = ?)          // Match either column
  │   AND status = 'accepted'
  │
  │ Returns: []string{"user-bob", "user-charlie"}
  ▼
Handler applies manual pagination:
  │ start = (page-1) * limit
  │ end = min(start + limit, len(friends))
  │ Formats as: [{user_id, username, status:"accepted"}, ...]
  ▼
Response: {"friends":[...],"total":2,"page":1,"limit":20,"pages":1}
```

> **Bidirectional query**: The `CASE WHEN` pattern returns the *other* user's ID regardless of which column the current user appears in, thanks to canonical ordering.

---

### 7.10 Get Pending Requests: `GET /users/friends/pending` (Auth)

```
GET /users/friends/pending?page=1&limit=20
  │
  ├─ AuthMiddleware → user_id
  ▼
repo.GetPendingRequests(userID)                  // repository.go:155
  │ SELECT user_id FROM friendships
  │   WHERE friend_id = ? AND status = 'pending'
  │
  │ Returns: []string of requester IDs
  ▼
Handler applies manual pagination → Response with count
```

> **Direction matters here**: Only queries where the current user is `friend_id` (the *recipient*), since the canonical ordering means the sender's request sets `user_id` as the smaller ID.

---

### 7.11 Friend Count: `GET /users/friends/count` (Auth)

```
friendHandler.GetFriendCount()                   // handler.go:248
  └─ service.GetFriendCount(userID)              // service.go:66
       └─ friends = repo.GetFriends(userID)      // Reuses GetFriends
       └─ return len(friends)
  ▼
Response: {"count": 5}
```

---

### 7.12 Helper Methods

#### IsFriend (`repository.go:180`)

```
IsFriend(userID, friendID) → bool
  │ Swap for canonical order
  │ SELECT status FROM friendships
  │   WHERE user_id=? AND friend_id=? AND status='accepted'
  │ if ErrNoRows: return false
  │ return true
```

#### IsBlocked (`repository.go:203`)

```
IsBlocked(userID, blockedID) → bool
  │ Swap for canonical order
  │ SELECT status FROM friendships
  │   WHERE user_id=? AND friend_id=? AND status='blocked'
  │ if ErrNoRows: return false
  │ return true
```

---

### 7.13 Route Table

| Method | Path | Handler |
|--------|------|---------|
| `POST` | `/friends/add` | `AddFriend` |
| `POST` | `/friends/:friend_id/accept` | `AcceptFriend` |
| `POST` | `/friends/:friend_id/decline` | `DeclineFriend` |
| `DELETE` | `/friends/:friend_id` | `RemoveFriend` |
| `POST` | `/friends/:user_id/block` | `BlockFriend` |
| `GET` | `/users/friends` | `GetFriends` |
| `GET` | `/users/friends/pending` | `GetPendingRequests` |
| `GET` | `/users/friends/count` | `GetFriendCount` |

All routes require JWT authentication.

---

### 7.14 Activity Feed Integration

When a friend request is **accepted**, the handler logs activities for **both users**:

```
activityService.LogFriendAdded(userID, username, friendID, friendID)
activityService.LogFriendAdded(friendID, friendID, userID, username)
  └─ Each creates: INSERT INTO activities
       (type="added_friend", user_id, friend_id, message)
```

This means both users see the friendship in their activity feeds.

---

*Sections for Shared Reading Lists, Activity Feed, and other features will follow.*

---

## 8. Reading Lists Sharing

### 8.1 Architecture

**Files:**

| File | Purpose |
|------|---------|
| `internal/sharedlist/handler.go` | HTTP handlers for list CRUD, subscribe/unsubscribe, manga management |
| `internal/sharedlist/service.go` | Business logic, ownership verification, access control |
| `internal/sharedlist/repository.go` | SQL queries with JSON serialization for arrays |

**Dependencies:**

```go
type Handler struct {
    service         *Service           // shared list business logic
    activityService *activity.Service  // logs "shared_list_created" activity
}
```

---

### 8.2 Database Schema

```sql
CREATE TABLE shared_reading_lists (
    id          TEXT PRIMARY KEY,
    owner_id    TEXT NOT NULL REFERENCES users(id),
    title       TEXT NOT NULL,
    description TEXT,
    is_public   BOOLEAN DEFAULT 0,
    manga_list  TEXT,           -- JSON array: ["one-piece","naruto"]
    shared_with TEXT,           -- JSON array: ["user-bob","user-charlie"]
    created_at  DATETIME,
    updated_at  DATETIME
);
```

> **JSON-in-SQLite**: `manga_list` and `shared_with` are stored as JSON strings. The repository serializes with `json.Marshal` on write and `json.Unmarshal` on read.

---

### 8.3 Create List: `POST /reading-lists/create` (Auth)

```
POST /reading-lists/create
  {"title":"My Favorites","description":"Top picks","manga_list":["one-piece","naruto"],"is_public":true}
  │
  ├─ AuthMiddleware → user_id, username
  ▼
sharedListHandler.CreateList()                   // handler.go:24
  │ c.ShouldBindJSON(&req)
  │ title = req.Title || req.Name               // Dual field support
  │ mangaList = req.MangaList || req.MangaIDs    // Dual field support
  │ Validate: title required, manga_list required
  ▼
service.CreateList(userID, title, desc, isPublic, mangaList, sharedWith)  // service.go:20
  │ Validate: title != "", len(mangaList) > 0
  ▼
repo.CreateList(...)                             // repository.go:24
  │ id = "list_<nanoseconds>"
  │ mangaJSON = json.Marshal(["one-piece","naruto"])
  │ sharedJSON = json.Marshal(sharedWith)
  │ INSERT INTO shared_reading_lists
  │   (id, owner_id, title, description, is_public, manga_list, shared_with, ...)
  ▼
Activity: activityService.LogSharedListCreated(userID, username, title)
  ▼
Response: 200 {id, owner_id, title, manga_list, is_public, ...}
```

---

### 8.4 Get My Lists: `GET /reading-lists/mine` (Auth)

```
GET /reading-lists/mine?page=1&limit=20
  │
  ├─ AuthMiddleware → user_id
  ▼
repo.GetListsByOwner(userID)                     // repository.go:80
  │ SELECT ... FROM shared_reading_lists WHERE owner_id=?
  │ ORDER BY created_at DESC
  │ json.Unmarshal manga_list and shared_with per row
  ▼
Handler applies manual pagination + formats response:
  │ Each list includes both "title"/"name" and "manga_list"/"manga_ids"
  │   (dual fields for backward compatibility)
  ▼
Response: {"lists":[...],"total":N,"page":1,"limit":20,"pages":N}
```

---

### 8.5 Get Public Lists: `GET /reading-lists/public` (Public)

```
GET /reading-lists/public?page=1&limit=20
  ▼
repo.GetPublicLists(limit, offset)               // repository.go:114
  │ SELECT COUNT(*) FROM shared_reading_lists WHERE is_public=1
  │ SELECT ... FROM shared_reading_lists WHERE is_public=1
  │   ORDER BY created_at DESC LIMIT ? OFFSET ?
  ▼
Response: {"lists":[...],"total":N,"page":1,"limit":20}
```

---

### 8.6 Get List by ID: `GET /reading-lists/:list_id` (Public with access check)

```
GET /reading-lists/list_123
  ▼
sharedListHandler.GetList()                      // handler.go:197
  │ list = service.GetListByID(listID)
  │
  │ // ─── ACCESS CONTROL ───
  │ if !list.IsPublic && list.OwnerID != userID:
  │   hasAccess = service.CanAccessList(userID, listID)  // service.go:88
  │     └─ Check: owner? → true
  │     └─ Check: is_public? → true
  │     └─ Check: userID in shared_with[]? → true/false
  │   if !hasAccess: return 403 "You don't have access"
  ▼
Response: full list object
```

---

### 8.7 Update List: `PUT /reading-lists/:list_id` (Auth + ownership)

```
PUT /reading-lists/list_123
  {"title":"Updated","description":"New desc","manga_list":["naruto"],"is_public":false}
  │
  ├─ AuthMiddleware → user_id
  ▼
handler.UpdateList()                             // handler.go:221
  │ list = service.GetListByID(listID)
  │ if list.OwnerID != userID: return 403
  ▼
service.UpdateList(userID, listID, ...)          // service.go:48
  │ SECOND ownership check
  │ Merge non-empty fields into existing list
  ▼
repo.UpdateList(listID, title, desc, isPublic, mangaList, sharedWith)  // repository.go:155
  │ mangaJSON = json.Marshal(mangaList)
  │ sharedJSON = json.Marshal(sharedWith)
  │ UPDATE shared_reading_lists SET
  │   title=?, description=?, is_public=?, manga_list=?, shared_with=?, updated_at=?
  │   WHERE id=?
  ▼
Response: updated list object (re-fetched after update)
```

---

### 8.8 Delete List: `DELETE /reading-lists/:list_id` (Auth + ownership)

```
handler.DeleteList()                             // handler.go:264
  │ Ownership check (handler + service)
  └─ repo.DeleteList(listID)                     // repository.go:173
       └─ DELETE FROM shared_reading_lists WHERE id=?
       └─ Check rowsAffected > 0
```

---

### 8.9 Subscribe: `POST /reading-lists/:list_id/subscribe` (Auth)

```
POST /reading-lists/list_123/subscribe
  │
  ├─ AuthMiddleware → user_id
  ▼
handler.SubscribeToList()                        // handler.go:290
  │ list = service.GetListByID(listID)
  │
  │ if list.OwnerID == userID:
  │   return 400 "Cannot subscribe to your own list"
  │
  │ for sub in list.SharedWith:
  │   if sub == userID: return 400 "Already subscribed"
  │
  │ // ─── APPEND USER TO shared_with ───
  │ list.SharedWith = append(list.SharedWith, userID)
  │ service.UpdateList(list.OwnerID, listID, ..., list.SharedWith)
  │   └─ json.Marshal(newSharedWith) → UPDATE ... SET shared_with=?
  │
  │ activityService.LogSharedListCreated(userID, username, "Subscribed to: "+title)
  ▼
Response: 200 {"list_id":"list_123","title":"My Favorites"}
```

> **Implementation**: Subscribe appends the user's ID to the JSON `shared_with` array and updates the whole row.

---

### 8.10 Unsubscribe: `DELETE /reading-lists/:list_id/subscribe` (Auth)

```
DELETE /reading-lists/list_123/subscribe
  │
  ├─ AuthMiddleware → user_id
  ▼
handler.UnsubscribeFromList()                    // handler.go:341
  │ list = service.GetListByID(listID)
  │
  │ // ─── REMOVE USER FROM shared_with ───
  │ newSharedWith = []
  │ for sub in list.SharedWith:
  │   if sub == userID: found = true; skip
  │   else: newSharedWith = append(newSharedWith, sub)
  │
  │ if !found: return 400 "You are not subscribed"
  │
  │ service.UpdateList(list.OwnerID, listID, ..., newSharedWith)
  ▼
Response: 200 {"list_id":"list_123","title":"My Favorites"}
```

---

### 8.11 Get Subscribed Lists: `GET /reading-lists/subscribed` (Auth)

```
GET /reading-lists/subscribed
  │
  ├─ AuthMiddleware → user_id
  ▼
handler.GetSubscribedLists()                     // handler.go:386
  │ allLists = service.GetPublicLists(1000, 0)   // Fetch all public lists
  │
  │ // Filter: only lists where userID is in shared_with[]
  │ for list in allLists:
  │   for sub in list.SharedWith:
  │     if sub == userID: subscribedLists.append(list)
  ▼
Response: {"lists":[...],"total":N}
```

---

### 8.12 Add Manga: `POST /reading-lists/:list_id/manga` (Auth + ownership)

```
POST /reading-lists/list_123/manga  {"manga_id":"bleach"}
  │
  ├─ AuthMiddleware → user_id
  ▼
handler.AddMangaToList()                         // handler.go:429
  │ list = service.GetListByID(listID)
  │ if list.OwnerID != userID: return 403
  │
  │ // Check duplicate
  │ for m in list.MangaList:
  │   if m == "bleach": return 400 "Already in list"
  │
  │ list.MangaList = append(list.MangaList, "bleach")
  │ service.UpdateList(userID, listID, ..., list.MangaList, ...)
  ▼
Response: {"list_id":"list_123","manga_id":"bleach","manga_list":["one-piece","naruto","bleach"]}
```

---

### 8.13 Remove Manga: `DELETE /reading-lists/:list_id/manga/:manga_id` (Auth + ownership)

```
DELETE /reading-lists/list_123/manga/naruto
  │
  ├─ AuthMiddleware → user_id
  ▼
handler.RemoveMangaFromList()                    // handler.go:481
  │ list = service.GetListByID(listID)
  │ if list.OwnerID != userID: return 403
  │
  │ newMangaList = filter out "naruto" from list.MangaList
  │ if not found: return 404 "Manga not found in this list"
  │
  │ service.UpdateList(userID, listID, ..., newMangaList, ...)
  ▼
Response: {"list_id":"list_123","manga_id":"naruto","manga_list":["one-piece","bleach"]}
```

---

### 8.14 Route Table

| Method | Path | Auth | Handler |
|--------|------|------|---------|
| `POST` | `/reading-lists/create` | ✅ | `CreateList` |
| `GET` | `/reading-lists/mine` | ✅ | `GetMyLists` |
| `GET` | `/reading-lists/public` | ❌ | `GetPublicLists` |
| `GET` | `/reading-lists/:list_id` | ❌* | `GetList` |
| `PUT` | `/reading-lists/:list_id` | ✅ | `UpdateList` |
| `DELETE` | `/reading-lists/:list_id` | ✅ | `DeleteList` |
| `POST` | `/reading-lists/:list_id/subscribe` | ✅ | `SubscribeToList` |
| `DELETE` | `/reading-lists/:list_id/subscribe` | ✅ | `UnsubscribeFromList` |
| `GET` | `/reading-lists/subscribed` | ✅ | `GetSubscribedLists` |
| `POST` | `/reading-lists/:list_id/manga` | ✅ | `AddMangaToList` |
| `DELETE` | `/reading-lists/:list_id/manga/:manga_id` | ✅ | `RemoveMangaFromList` |

\* Public lists visible without auth; private lists require owner/subscriber check.

---

### 8.15 Access Control Model

```
CanAccessList(userID, listID) → bool

  1. Owner?      list.OwnerID == userID          → true
  2. Public?     list.IsPublic == true            → true
  3. Subscriber? userID in list.SharedWith[]       → true
  4. Otherwise                                    → false (403)
```

---

*Sections for Activity Feed, Data Export/Import, and other features will follow.*

---

## 9. Activity Feed

### 9.1 Architecture

```
┌──────────────────────────────────────────────────────────┐
│  Event Sources (Write Side)                               │
│    reviewHandler  → activityService.LogReviewWritten()    │
│    friendHandler  → activityService.LogFriendAdded()      │
│    sharedList     → activityService.LogSharedListCreated()│
│    userHandler    → activityService.LogMangaStarted()     │
│    userHandler    → activityService.LogMangaCompleted()   │
│    activityHandler→ activityService.LogUserPost()         │
└──────────────────────┬────────────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────────┐
│  activity.Service                                         │
│    Log*() → repo.CreateActivity() → INSERT INTO activities│
│           → invalidateActivityCaches()                    │
└──────────────────────┬────────────────────────────────────┘
                       │
          ┌────────────┴────────────┐
          ▼                         ▼
   ┌─────────────┐          ┌──────────────┐
   │  SQLite DB   │          │  Redis Cache  │
   │  activities  │          │  act:feed:*   │
   │   table      │          │  act:user:*   │
   └─────────────┘          └──────────────┘
```

**Files:**

| File | Purpose |
|------|---------|
| `internal/activity/handler.go` | HTTP handlers for feed retrieval, search, stats, timeline |
| `internal/activity/service.go` | 6 typed log methods, Redis cache-aside, feed retrieval |
| `internal/activity/repository.go` | SQL queries for CRUD and friends-activity JOIN |

---

### 9.2 Database Schema

```sql
CREATE TABLE activities (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id),
    type       TEXT NOT NULL,    -- 'started_manga','completed_manga','wrote_review',
                                 --  'added_friend','shared_list_created','user_post'
    manga_id   TEXT,             -- nullable, for manga-related activities
    review_id  TEXT,             -- nullable, for review activities
    friend_id  TEXT,             -- nullable, for friend activities
    message    TEXT NOT NULL,    -- Human-readable description
    created_at DATETIME
);
```

---

### 9.3 Activity Types & Log Methods (`service.go`)

Each method follows the same pattern: format message → `repo.CreateActivity()` → `invalidateActivityCaches()`.

| Method | Type | Trigger | Message Format |
|--------|------|---------|----------------|
| `LogMangaStarted` | `started_manga` | User adds manga to library | `"alice started reading One Piece"` |
| `LogMangaCompleted` | `completed_manga` | User sets status=completed | `"alice completed Naruto"` |
| `LogReviewWritten` | `wrote_review` | Review created | `"alice rated One Piece 8/10"` |
| `LogFriendAdded` | `added_friend` | Friend request accepted | `"alice added bob as a friend"` |
| `LogSharedListCreated` | `shared_list_created` | Reading list created/subscribed | `"alice created a new reading list: Favorites"` |
| `LogUserPost` | `user_post` | Manual post via API | `"alice posted: Great chapter today!"` |

#### Create Activity Flow

```
LogReviewWritten(userID, "alice", "one-piece", "One Piece", "rev_123", 8)
  │ message = "alice rated One Piece 8/10"
  ▼
repo.CreateActivity(userID, "wrote_review", "one-piece", "rev_123", "", message)
  │ id = "act_<nanoseconds>"
  │ INSERT INTO activities (id, user_id, type, manga_id, review_id, friend_id, message, created_at)
  │   VALUES (?, ?, ?, ?, ?, ?, ?, now)
  │   └─ manga_id, review_id, friend_id are nullable (stored as NULL if empty)
  ▼
invalidateActivityCaches()
  │ cache.DeletePattern("act:feed:*")    // Invalidate all global feed pages
  │ cache.DeletePattern("act:user:*")    // Invalidate all user-specific pages
```

---

### 9.4 Redis Caching

```go
type Service struct {
    repo  *Repository
    cache *cache.RedisCache       // Optional (nil if Redis unavailable)
}
```

#### Cache Keys

| Pattern | Example | TTL |
|---------|---------|-----|
| `act:feed:<page>:<limit>:<type>` | `act:feed:1:20:` | `ActivityFeedTTL` |
| `act:user:<userID>:<page>:<limit>` | `act:user:user-alice:1:20` | `ActivityFeedTTL` |

#### Read Flow (Cache-Aside)

```
GetAllActivities(limit=20, offset=0)
  │ if cache != nil:
  │   key = "act:feed:1:20:"
  │   if cache.Get(key, &cached): return cached     // HIT
  │
  │ activities = repo.GetAllActivities(20, 0)       // MISS → SQLite
  │   └─ SELECT ... FROM activities ORDER BY created_at DESC LIMIT 20 OFFSET 0
  │
  │ if cache != nil:
  │   cache.Set(key, activities, ActivityFeedTTL)    // Populate cache
  │
  │ return activities
```

#### Invalidation

Every `Log*()` method calls `invalidateActivityCaches()` which does pattern-based deletion:

```
invalidateActivityCaches()
  │ cache.DeletePattern("act:feed:*")    // Wipe ALL global feed pages
  │ cache.DeletePattern("act:user:*")    // Wipe ALL user feed pages
```

> **Trade-off**: Aggressive invalidation ensures freshness at the cost of cache hit rate. Every new activity wipes the entire feed cache.

---

### 9.5 Global Feed: `GET /feed/activities` (Auth)

```
GET /feed/activities?page=1&limit=20&type=wrote_review
  │
  ├─ AuthMiddleware
  ▼
handler.GetActivityFeed()                        // handler.go:53
  │ Parse page/limit (default 20, max 100)
  │ offset = (page-1) * limit
  ▼
service.GetAllActivities(20, 0)                  // service.go:123
  │ Cache check → repo.GetAllActivities()         // repository.go:101
  │   └─ SELECT ... FROM activities ORDER BY created_at DESC LIMIT ? OFFSET ?
  ▼
In-memory type filter (if ?type= provided):
  │ for activity in activities:
  │   if activity.Type == "wrote_review": keep
  ▼
Response: {"activities":[...],"total":N,"page":1,"limit":20,"pages":N}
```

---

### 9.6 User Activities: `GET /users/:user_id/activities` (Auth)

```
GET /users/user-alice/activities?page=1&limit=20&type=started_manga
  ▼
handler.GetUserActivities()                      // handler.go:104
  │ userID = c.Param("user_id")
  ▼
service.GetUserActivities(userID, 20, 0)         // service.go:101
  │ Cache check → repo.GetUserActivities()        // repository.go:60
  │   └─ SELECT ... FROM activities WHERE user_id=?
  │        ORDER BY created_at DESC LIMIT ? OFFSET ?
  ▼
In-memory type filter + pagination → Response
```

---

### 9.7 Friends Timeline: `GET /feed/timeline` (Auth)

```
GET /feed/timeline?page=1&limit=50&range=all_time
  │
  ├─ AuthMiddleware → user_id
  ▼
handler.GetTimelineView()                        // handler.go:227
  ▼
service.GetFriendsActivityFeed(userID, 50, 0)    // service.go:147
  └─ repo.GetFriendsActivities(userID, 50, 0)    // repository.go:143
       │
       │ // ─── STEP 1: Find friends ───
       │ SELECT CASE WHEN user_id=? THEN friend_id ELSE user_id END
       │ FROM friendships
       │ WHERE (user_id=? OR friend_id=?) AND status='accepted'
       │ → friendIDs = ["user-bob", "user-charlie"]
       │
       │ if len(friendIDs) == 0: return []     // No friends → empty feed
       │
       │ // ─── STEP 2: Get their activities ───
       │ SELECT ... FROM activities
       │ WHERE user_id IN (?,?)                 // Dynamic IN clause
       │ ORDER BY created_at DESC LIMIT 50 OFFSET 0
  ▼
Group by date for timeline view:
  │ timeline = map[string][]Activity
  │ for activity in activities:
  │   date = activity.CreatedAt.Format("2006-01-02")   // "2026-05-10"
  │   timeline[date] = append(timeline[date], activity)
  ▼
Response: {
  "data": {
    "2026-05-10": [{...}, {...}],
    "2026-05-09": [{...}]
  },
  "total": 3,
  "time_range": "all_time"
}
```

---

### 9.8 Search Activities: `GET /feed/search` (Auth)

```
GET /feed/search?q=one+piece&page=1&limit=20
  │
  ├─ AuthMiddleware → user_id
  ▼
handler.SearchActivities()                       // handler.go:174
  │ query = "q" param (required)
  │
  │ activities = service.GetUserActivities(userID, limit, offset)
  │
  │ // ─── IN-MEMORY SEARCH ───
  │ query = strings.ToLower("one piece")
  │ for activity in activities:
  │   if strings.Contains(lower(activity.Message), query):
  │     filtered.append(activity)
  ▼
Response: {"data": filtered, "total": len(filtered), "query":"one piece"}
```

> **In-memory filter**: Search is not SQL-based. It fetches the user's activities and filters by substring match on `Message`. This works for small datasets but won't scale.

---

### 9.9 Activity Stats: `GET /feed/stats` (Auth)

```
GET /feed/stats
  │
  ├─ AuthMiddleware → user_id
  ▼
handler.GetActivityStats()                       // handler.go:151
  │ activities = service.GetUserActivities(userID, 1000, 0)  // Fetch ALL
  │
  │ stats = map[string]int{}
  │ for activity in activities:
  │   stats[activity.Type]++                     // Count by type
  ▼
Response: {
  "started_manga": 5,
  "completed_manga": 2,
  "wrote_review": 3,
  "added_friend": 4,
  "user_post": 1
}
```

---

### 9.10 Post Activity: `POST /feed/activities` (Auth)

```
POST /feed/activities  {"message":"Just finished an amazing chapter!"}
  │
  ├─ AuthMiddleware → user_id, username
  ▼
handler.PostActivity()                           // handler.go:24
  │ c.ShouldBindJSON(&models.CreateActivityRequest)
  ▼
service.LogUserPost(userID, "alice", "Just finished an amazing chapter!")
  │ message = "alice posted: Just finished an amazing chapter!"
  │ repo.CreateActivity(userID, "user_post", "", "", "", message)
  │ invalidateActivityCaches()
  ▼
Response: 200 {id, user_id, type:"user_post", message, created_at}
```

---

### 9.11 Notifications: `GET /feed/notifications` (Auth)

```
handler.GetActivityNotifications()               // handler.go:308
  │ unreadOnly = c.DefaultQuery("unread_only", "true")
  │ activities = service.GetUserActivities(userID, limit, offset)
  ▼
Response: {"data": activities, "total": N, "unread_only": true}
```

> **Note**: `unread_only` is accepted but not implemented — all activities are returned regardless. No read/unread tracking exists in the schema.

---

### 9.12 Route Table

| Method | Path | Auth | Handler |
|--------|------|------|---------|
| `POST` | `/feed/activities` | ✅ | `PostActivity` |
| `GET` | `/feed/activities` | ✅ | `GetActivityFeed` |
| `GET` | `/feed/timeline` | ✅ | `GetTimelineView` |
| `GET` | `/feed/search` | ✅ | `SearchActivities` |
| `GET` | `/feed/stats` | ✅ | `GetActivityStats` |
| `DELETE` | `/feed/clear` | ✅ | `ClearActivityFeed` |
| `GET` | `/feed/notifications` | ✅ | `GetActivityNotifications` |
| `GET` | `/feed/stream` | ✅ | `FollowActivityStream` |
| `GET` | `/users/:user_id/activities` | ✅ | `GetUserActivities` |

---

### 9.13 Integration Points

The activity service is injected as a dependency into other feature handlers:

```
reviewHandler    := review.NewHandler(reviewService, activityService, mangaService)
friendHandler    := friend.NewHandler(friendService, activityService)
sharedListHandler := sharedlist.NewHandler(sharedListService, activityService)
```

Each handler calls `activityService.Log*()` at the appropriate point in its business logic:

| Feature | When | Log Method |
|---------|------|------------|
| Reviews | After creating a review | `LogReviewWritten` |
| Friends | After accepting a friend request (mutual) | `LogFriendAdded` × 2 |
| Shared Lists | After creating a list or subscribing | `LogSharedListCreated` |
| User Library | After adding manga / completing | `LogMangaStarted` / `LogMangaCompleted` |
| Activity Feed | Direct user post | `LogUserPost` |

---

*Sections for Data Export/Import, Health Checks, and Docker Compose will follow.*

---

## 10. System Health Checks

### 10.1 Architecture

Unlike business features which are organized into `internal/` packages with handler/service/repository layers, the health checks are implemented directly in the API server's entrypoint (`cmd/api-server/main.go`). This is because the health checks need direct access to the underlying infrastructure clients (DB pool, Redis pool, internal server instances) that are instantiated during startup.

**File:** `cmd/api-server/main.go`

---

### 10.2 Diagnostic Helpers

The server defines 6 closure functions to diagnose each infrastructure component:

1. **`checkDatabase()`**:
   - Executes `db.Ping()`
   - Measures latency
   - Executes `COUNT(*)` on `manga` and `users` tables
   - Returns `{status: "healthy"|"unhealthy", latency, manga_count, user_count}`
2. **`checkRedis()`**:
   - Checks `redisCache.IsAvailable()`
   - Returns full Redis stats (hits, misses, keys) if enabled, else `{status: "disabled"}`
3. **`checkTCP()`**:
   - If internal `server.TCPServer` is running, queries `GetConnectedUsers()` and `GetUptime()`
   - Also exposes the current conflict resolution `strategy`
4. **`checkUDP()`**:
   - If internal `server.UDPServer` is running, queries `GetClientCount()`
5. **`checkWebSocket()`**:
   - Queries `chatHub.GetClientCount("general")` and `GetOnlineUsers("general")`
6. **`checkGRPC()`**:
   - Performs a lightweight "liveness probe" by executing `net.DialTimeout("tcp", "localhost:"+grpcPort, 2s)`
   - Returns `{status: "healthy", latency}` on success

---

### 10.3 Comprehensive Health: `GET /health` (Public)

This endpoint aggregates the results from all 6 diagnostic helpers.

```
GET /health
  │
  ▼
func (c *gin.Context)                            // main.go:336
  │ dbHealth = checkDatabase()
  │ redisHealth = checkRedis()
  │ tcpHealth = checkTCP()
  │ udpHealth = checkUDP()
  │ wsHealth = checkWebSocket()
  │ grpcHealth = checkGRPC()
  │
  │ overallStatus = "healthy"
  │ if dbHealth["status"] == "unhealthy": overallStatus = "degraded"
  ▼
Response: 200 {
  "status": "success",
  "message": "MangaHub API is running",
  "data": {
    "status": "healthy",
    "manga_count": 142,
    "services": {
      "api": {"status": "healthy", "port": "8080"},
      "database": {...},
      "cache": {...},
      "tcp": {...},
      "udp": {...},
      "websocket": {...},
      "grpc": {...}
    }
  }
}
```

---

### 10.4 Granular Health Routes (Public)

For targeted monitoring (e.g., Docker `HEALTHCHECK` instructions or Kubernetes liveness/readiness probes), the system provides granular endpoints that isolate specific components:

| Route | Helper Invoked | Primary Use Case |
|-------|----------------|------------------|
| `GET /health/db` | `checkDatabase()` | Validate SQLite persistence |
| `GET /health/cache` | `checkRedis()` | Validate Redis connectivity |
| `GET /health/tcp` | `checkTCP()` | Monitor sync server load |
| `GET /health/udp` | `checkUDP()` | Monitor notification delivery |
| `GET /health/ws` | `checkWebSocket()` | Monitor chat room occupancy |
| `GET /health/grpc` | `checkGRPC()` | Validate gRPC RPC availability |

All health routes are completely unauthenticated (`Public`), making them suitable for external pinging tools like UptimeRobot or container orchestrators.

---

## 11. Container Orchestration (Docker Compose)

### 11.1 Architecture

MangaHub is deployed as a suite of microservices using `docker-compose.yml`. Despite the fragmented execution models (HTTP, TCP, UDP, gRPC), the system uses a single unified `Dockerfile` and a shared database volume to simplify deployment while maintaining network isolation.

### 11.2 Multi-Stage Build (`Dockerfile`)

```dockerfile
# ─── STAGE 1: BUILDER ───
FROM golang:1.25.6 AS builder
# Requires Debian-based golang image for GCC/CGO (SQLite requires C bindings)
ENV GOFLAGS=-tags=sqlite_fts5
COPY . .
# Builds all 5 entrypoints into binaries
RUN go build -o /app/bin/api-server ./cmd/api-server
RUN go build -o /app/bin/udp-server ./cmd/udp-server
RUN go build -o /app/bin/tcp-server ./cmd/tcp-server
RUN go build -o /app/bin/grpc-server ./cmd/grpc-server
RUN go build -o /app/bin/mangahub ./cmd/cli

# ─── STAGE 2: RUNNER ───
FROM debian:12-slim
# Copies all 5 binaries from the builder stage
COPY --from=builder /app/bin/* /usr/local/bin/
CMD ["api-server"] # Default execution
```

**Key Takeaways:**
- **CGO Dependency**: Because the system relies on `go-sqlite3` which uses C bindings, we use `golang:1.25.6` (Debian-based, GCC included) rather than Alpine.
- **Unified Image**: A single Docker image contains all binaries (`api-server`, `tcp-server`, `udp-server`, `grpc-server`, `mangahub`).

---

### 11.3 Microservices Topology (`docker-compose.yml`)

The compose file defines 5 independent services that communicate internally:

| Service Name | Command | Exposed Port | Internal Dependency | Role |
|--------------|---------|--------------|---------------------|------|
| `redis` | `redis-server` | `6379` | None | Distributed caching |
| `mangahub-api` | `api-server` | `8080` | `redis`, tcp, udp, grpc | HTTP API & WebSocket Chat |
| `mangahub-tcp` | `tcp-server` | `9090` | Shared SQLite volume | Sync progress & strategies |
| `mangahub-udp` | `udp-server` | `9091/udp` | Shared SQLite volume | Broadcast notifications |
| `mangahub-grpc`| `grpc-server` | `9092` | Shared SQLite volume | Remote Procedure Calls |

---

### 11.4 Volume Management & Data Consistency

**The SQLite Concurrency Model:**

```yaml
volumes:
  mangahub-data:
    driver: local

services:
  mangahub-api:  { volumes: ["mangahub-data:/app/data"] }
  mangahub-tcp:  { volumes: ["mangahub-data:/app/data"] }
  mangahub-udp:  { volumes: ["mangahub-data:/app/data"] }
  mangahub-grpc: { volumes: ["mangahub-data:/app/data"] }
```

Because SQLite is a file-based database, deploying it in a microservice environment requires **sharing the database file** (`mangahub.db`) across all containers via the `mangahub-data` volume.

**How it prevents locks:**
- `go-sqlite3` is configured with `_journal_mode=WAL` (Write-Ahead Logging) during connection.
- WAL mode allows simultaneous readers and writers, preventing `database is locked` errors when `mangahub-api` and `mangahub-tcp` try to write concurrently.

---

### 11.5 Startup Sequence & Healthchecks

`docker-compose.yml` ensures that the API server waits for Redis to become healthy before attempting connections:

```yaml
# redis service
healthcheck:
  test: ["CMD", "redis-cli", "ping"]
  interval: 10s

# mangahub-api service
depends_on:
  redis:
    condition: service_healthy
  mangahub-tcp:
    condition: service_started
```

1. **Redis** boots and runs its `healthcheck`.
2. **TCP/UDP/gRPC** servers boot (`service_started`).
3. **API Server** boots only when Redis is marked `healthy` and the other servers are started.
4. If an internal server is missing, the API disables proxy routes by inspecting the `TCP_PORT`, `UDP_PORT`, and `GRPC_PORT` environment variables.

---

*This concludes the core architecture sections.*

---

## 12. Data Export & Import

### 12.1 Architecture

Unlike many systems where data export is handled entirely server-side (e.g., an API that streams a ZIP file), MangaHub's Export/Import functionality is driven client-side by the **MangaHub CLI** (`cmd/cli/export.go` and `cmd/cli/dataimport.go`). 

The CLI orchestrates the export process by making standard API requests to fetch user data, transforming the JSON responses into the desired format (CSV, JSON, or Tar.GZ), and saving them locally. The import process reverses this, parsing local files and firing multiple API `POST`/`PUT` requests to reconstruct the data.

**Files:**
- `cmd/cli/export.go`
- `cmd/cli/dataimport.go`

---

### 12.2 Export Flows (`mangahub export`)

#### `mangahub export library` / `mangahub export progress`
```
CLI User: mangahub export library --format csv --output lib.csv
  │
  ├─ requireAuth() → Gets token
  ▼
API Call: GET /users/library                     // export.go:62
  │ (Returns standard JSON payload with reading/completed/plan_to_read lists)
  ▼
CLI Data Transformation:                         // export.go:85
  │ json.Unmarshal(resp.Data, &library)
  │ Flatten reading, completed, and plan_to_read arrays into one list
  │ (If "progress" command: filter out entries where CurrentChapter == 0)
  ▼
CLI File Writer:                                 // export.go:106
  │ If --format=csv: writeLibraryCSV(output, allEntries)
  │   └─ Columns: "manga_id", "current_chapter", "status", "updated_at"
  │ If --format=json: writeJSON(output, exportData)
  ▼
Saved to lib.csv
```

#### `mangahub export all` (Full Archive)
Creates a `tar.gz` backup of all user data.

```
CLI User: mangahub export all --output backup.tar.gz
  │
  ├─ API Call: GET /users/library
  ├─ API Call: GET /users/reviews
  ├─ API Call: GET /users/friends
  ▼
CLI Archive Builder:                             // export.go:238
  │ Create backup.tar.gz → gzip.Writer → tar.Writer
  │
  │ Add library.json (Raw API response)
  │ Add reviews.json (Raw API response)
  │ Add friends.json (Raw API response)
  │ Add progress.csv (Flattened library data converted to CSV format)
  │ Add metadata.json (Export timestamp, username, CLI version, file count)
  ▼
Saved to backup.tar.gz
```

---

### 12.3 Import Flows (`mangahub import`)

The import commands read local files, detect the format (JSON/CSV), and sequentially execute API requests to ingest the data.

#### `mangahub import library`
```
CLI User: mangahub import library --file lib.csv
  │
  ▼
CLI Parser:                                      // dataimport.go:66
  │ detectFormat() → "csv"
  │ Parse CSV → loop rows → Extract: manga_id (col 0), status (col 2)
  ▼
CLI Ingestion Loop:                              // dataimport.go:110
  │ for entry in entries:
  │   body = {"manga_id": entry.MangaID, "status": entry.Status}
  │   API Call: POST /users/library
  │   (If 400 Duplicate, increments "Skipped" counter)
  ▼
Finished: "Imported 45 entries, Skipped 2"
```

#### `mangahub import progress`
```
CLI User: mangahub import progress --file prog.csv
  │
  ▼
CLI Parser:
  │ Extract: manga_id (col 0), current_chapter (col 1), status (col 2)
  ▼
CLI Ingestion Loop:                              // dataimport.go:203
  │ for entry in entries:
  │   body = {"manga_id":..., "current_chapter":..., "status":...}
  │   API Call: PUT /users/progress
  │   (If 400 Not Found in Library, increments "Failed" counter)
  ▼
Finished: "Updated 12 entries"
```

#### `mangahub import manga`
Allows mass-ingestion of catalog data.
```
CLI User: mangahub import manga --file catalog.json
  │
  ▼
CLI Parser:
  │ Unmarshals JSON array into []mangaImport structs
  ▼
CLI Ingestion Loop:                              // dataimport.go:314
  │ for manga in mangaList:
  │   API Call: POST /manga (Requires Admin Token usually, depending on API rules)
  ▼
Finished
```

---

### 12.4 Design Trade-offs

1. **Client-side processing**: By shifting the CSV/Tar.gz generation to the CLI, the API server remains lightweight and avoids expensive file compression operations in memory. 
2. **Reusing existing routes**: The export tool simply consumes the existing `GET /users/library` endpoints rather than requiring dedicated `GET /export` routes on the server.
3. **N+1 Import Problem**: The import loop fires a distinct `POST` request for *every single item* in the CSV. While this ensures existing validation logic runs correctly, it can be slow for massive libraries (e.g., 500+ manga) due to network overhead. A batch import API route `POST /users/library/batch` would be the optimal future enhancement.

---

## 13. Input Sanitization (`pkg/sanitize`)

### 13.1 Package Overview

```
pkg/sanitize/sanitize.go
  │
  ├── Text(s string, maxLen int) (string, error)
  │     Trims whitespace, rejects '<' or '>' characters, enforces max byte length.
  │     Used for: manga title, author, description, review text.
  │
  ├── ID(s string) (string, error)
  │     Trims whitespace, allows only [a-zA-Z0-9\-_], enforces max 100 chars.
  │     Used for: manga ID (URL slug) on POST /manga.
  │
  ├── Username(s string) (string, error)
  │     Same character rules as ID. Max 50 chars.
  │     Used for: registration validation beyond binding tags.
  │
  └── ChatMessage(s string) (string, error)
        Trims whitespace, enforces max 500 chars. Allows '<'/'>' (React escapes on render).
        Used for: WebSocket chat messages before broadcast.
```

### 13.2 Call Sites

| Handler | Function Called | Field |
|---------|----------------|-------|
| `manga.Create` | `sanitize.ID` + `sanitize.Text` | id, title, author, description |
| `manga.Update` | `sanitize.Text` | title, author, description (if present) |
| `review.CreateReview` | `sanitize.Text` | review text |
| `review.UpdateReview` | `sanitize.Text` | review text |
| `websocket.handleClientMessage` | `sanitize.ChatMessage` | chat message before hub broadcast |

### 13.3 Error Response on Rejection

```
POST /manga  {"title": "<script>alert(1)</script>"}
  │
  ▼
manga.Create()
  │ sanitize.Text(manga.Title, 200)
  │   └─ strings.ContainsAny("<>") == true → return error
  ▼
utils.BadRequestResponse(c, "Invalid title: input must not contain < or > characters")
HTTP 400
```

---

## 14. OpenAPI / Swagger Documentation

### 14.1 Overview

```
cmd/api-server/docs_info.go   ← @title, @version, @host, @securityDefinitions
internal/*/handler.go         ← @Summary, @Tags, @Param, @Success, @Security, @Router
docs/                         ← generated by: swag init -g cmd/api-server/main.go -o docs
  ├── docs.go                 ← Go source file, imported as _ "mangahub/docs"
  ├── swagger.json            ← OpenAPI 2.0 JSON spec
  └── swagger.yaml            ← OpenAPI 2.0 YAML spec
```

### 14.2 Route Registration

```
cmd/api-server/main.go (import block):
  _ "mangahub/docs"                       // side-effect: registers swagger spec on init
  ginSwagger "github.com/swaggo/gin-swagger"
  swaggerFiles "github.com/swaggo/files"

Route:
  r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
  // Serves Swagger UI at http://localhost:8080/swagger/index.html
```

### 14.3 Annotation Coverage (43 endpoints / 34 paths)

| Tag | Endpoints |
|-----|-----------|
| auth | register, login, logout, status, change-password |
| manga | search, get, create, update, delete |
| users | profile, library (CRUD), progress |
| reviews | create, get, update, delete, helpful, rating-stats, my-reviews |
| friends | add, accept, decline, remove, list, pending |
| reading-lists | create, mine, public, subscribed, get, subscribe, unsubscribe, add/remove manga |
| feed | post, get, timeline, stats, user-activities |

> **Note:** Inline anonymous handlers in `main.go` (health, cache, sync, notify, data)
> are not annotatable by swag and are intentionally omitted. These are operational/admin
> endpoints, not user-facing API.

### 14.4 Regenerating Docs

```bash
# After editing any @Router annotation:
swag init -g cmd/api-server/main.go -o docs
```

---

## 15. CI/CD Pipeline (GitHub Actions)

### 15.1 Workflow File

```
.github/workflows/ci.yml
```

### 15.2 Jobs and Trigger Logic

```
Push to main / PR to main
        │
        ▼
Job 1: build-and-test  (ubuntu-latest)
  ├── actions/checkout@v4
  ├── actions/setup-go@v5  (reads version from go.mod)
  ├── go mod download + go mod verify
  ├── go build ./cmd/api-server/
  ├── go build ./cmd/tcp-server/
  ├── go build ./cmd/udp-server/
  ├── go build ./cmd/grpc-server/
  ├── go build ./cmd/cli/
  ├── go test -race -v ./internal/auth/...
  ├── go test -race -v ./internal/tcp/...
  ├── go test -race -v ./pkg/sanitize/...
  └── go vet ./...
        │
        ▼ (needs: build-and-test)
Job 2: docker  (ubuntu-latest)
  ├── docker compose build
  ├── docker compose up -d
  ├── poll localhost:8080/health (30 retries × 3s)
  ├── verify JSON response
  ├── docker compose logs --tail=50  (always)
  └── docker compose down -v         (always)
        │
        ▼ (needs: docker — push to main only)
Job 3: publish  (ubuntu-latest)
  ├── docker/login-action@v3  → ghcr.io (GITHUB_TOKEN)
  ├── docker/metadata-action@v5  → tags: latest + sha-<commit>
  ├── docker/setup-buildx-action@v3
  └── docker/build-push-action@v6
        push: ghcr.io/anhhuynh1707/mangahub:latest
        push: ghcr.io/anhhuynh1707/mangahub:sha-<commit>
```

### 15.3 Test Files

| File | Tests | What They Cover |
|------|-------|----------------|
| `internal/auth/jwt_test.go` | 8 | GenerateToken, ValidateToken (valid / expired / tampered / wrong secret) |
| `internal/tcp/protocol_test.go` | 9 | Encode/decode roundtrip, factory functions, omitempty |
| `internal/tcp/conflict_test.go` | 9 | All 3 strategies, concurrent safety, conflict log |
| `pkg/sanitize/sanitize_test.go` | 26 | Text, ID, Username, ChatMessage |

---

## 16. UDP Delivery Confirmation (ACK System)

### 16.1 Architecture

```
┌─────────────────────────────────────────────────────┐
│  NotificationServer (server.go)                     │
│    ├── ackTracker *AckTracker    (ack.go)            │
│    │     ├── pending map[notifID]*pendingDelivery    │
│    │     └── history []*DeliveryRecord (last 50)    │
│    │                                                 │
│    ├── BroadcastNotification()  ← fire-and-forget   │
│    └── BroadcastWithACK()       ← reliable delivery │
└─────────────────────────────────────────────────────┘
```

### 16.2 BroadcastWithACK Flow

```
POST /notify/broadcast-ack
  │
  ▼
BroadcastWithACK(notif)                         // notifier.go
  │
  │ 1. snapshot s.Clients (thread-safe copy)
  │ 2. ackTracker.track(msg, addrStrs)          // generate notif_id, open ackCh
  │ 3. notif.NotificationID = id
  │ 4. for each addr: s.sendTo(addr, notif)     // sends {"type":"...","notification_id":"notif-xxx"}
  │ 5. ackTracker.finalise(id, pd, 3s)          // blocks up to 3 seconds
  │      │
  │      │  Meanwhile, clients send back:
  │      │  {"type":"ack","notification_id":"notif-xxx"}
  │      │
  │      │  handleMessage() → case "ack":
  │      │    ackTracker.RecordACK(notifID, addr) → pd.ackCh <- addr
  │      │
  │      └─ after 3s timeout: build DeliveryRecord
  │           AckedBy  = addrs that sent ACK
  │           Unacked  = addrs that did not
  │           AckRate  = len(AckedBy) / len(SentTo)
  ▼
Return *DeliveryRecord to HTTP handler → JSON response
```

### 16.3 New Message Type

| Direction | Type | Fields | Purpose |
|-----------|------|--------|---------|
| Server → Client | any notification | + `notification_id` | Identifies this specific send |
| Client → Server | `"ack"` | `notification_id` | Confirms receipt |

### 16.4 New HTTP Endpoints

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| `POST` | `/notify/broadcast-ack` | Bearer | Broadcast and wait for ACKs (3s) |
| `GET` | `/notify/ack-stats` | Bearer | View last 50 delivery records |

---

## 17. gRPC Server-side Streaming

### 17.1 New Proto Definitions

```protobuf
// proto/manga.proto — new additions

service MangaService {
    // ... existing unary RPCs ...
    rpc StreamSearch(SearchRequest)        returns (stream MangaResponse);
    rpc WatchMangaUpdates(WatchRequest)    returns (stream MangaEvent);
}

message WatchRequest {
    string manga_id = 1;  // filter by manga; empty = all manga
    string user_id  = 2;
}

message MangaEvent {
    string event_type = 1;  // "connected","progress_updated","manga_updated"
    string manga_id   = 2;
    string user_id    = 3;
    string message    = 4;
    int64  timestamp  = 5;
    int32  chapter    = 6;
}
```

### 17.2 MangaEventHub

```
internal/grpc/event_hub.go

MangaEventHub
  ├── subscribers map[string]chan *pb.MangaEvent   (protected by sync.RWMutex)
  │
  ├── Subscribe(id)   → creates buffered chan (32), registers it
  ├── Unsubscribe(id) → closes chan, removes from map
  └── Publish(event)  → fans out to ALL subscriber channels (non-blocking, drops if full)

Convenience methods (called from HTTP layer):
  ├── PublishProgressUpdate(userID, mangaID, chapter)
  └── PublishMangaUpdated(mangaID, title)
```

### 17.3 StreamSearch Flow

```
gRPC Client: StreamSearch({query:"one", genre:"action", limit:5})
  │
  ├─ StreamAuthInterceptor → validate JWT from metadata
  ▼
MangaServer.StreamSearch(req, stream)
  │ mangaService.Search(query)           // single DB call, returns list
  │
  │ for each manga in result:
  │   stream.Send(mangaToProto(&manga))  // one message per manga
  │
  └─ return nil  (stream closed server-side)

Client receives: 5 separate MangaResponse messages
```

### 17.4 WatchMangaUpdates Flow

```
gRPC Client: WatchMangaUpdates({manga_id:"one-piece", user_id:"user-alice"})
  │
  ├─ StreamAuthInterceptor → validate JWT
  ▼
MangaServer.WatchMangaUpdates(req, stream)
  │
  │ subID = "watch-<timestamp>"
  │ ch = EventHub.Subscribe(subID)        // register for events
  │ defer EventHub.Unsubscribe(subID)     // clean up on exit
  │
  │ stream.Send({event_type:"connected"}) // initial confirmation
  │
  │ for {
  │   select {
  │     case <-ctx.Done():    return nil  // client disconnected
  │     case event := <-ch:
  │       if req.MangaId != "" && event.MangaId != req.MangaId:
  │         continue                      // filter by manga_id
  │       stream.Send(event)
  │   }
  │ }

                    Meanwhile (HTTP layer):
                    PUT /users/progress
                      │
                      ▼
                    GRPCMangaServer.EventHub.PublishProgressUpdate(userID, mangaID, chapter)
                      │
                      └─ EventHub.Publish({event_type:"progress_updated", ...})
                              │
                              └─ fans out to all subscriber channels
                                   └─ WatchMangaUpdates stream.Send() fires
```

### 17.5 Auth Interceptors

| Interceptor | Type | Handles |
|-------------|------|---------|
| `AuthInterceptor` | `UnaryServerInterceptor` | GetManga, SearchManga, UpdateProgress |
| `StreamAuthInterceptor` | `StreamServerInterceptor` | StreamSearch, WatchMangaUpdates |

Both validate the same `Authorization: Bearer <JWT>` metadata header using `auth.ValidateToken`.

### 17.6 Regenerating pb Files

```bash
# Run from project root after editing proto/manga.proto:
PATH="$PATH:$HOME/go/bin" protoc \
  --go_out=. --go_opt=module=mangahub \
  --go-grpc_out=. --go-grpc_opt=module=mangahub \
  proto/manga.proto
```

---

## 18. Advanced Search & Filtering

### 18.1 SearchFilters Struct (spec-required)

```go
// pkg/models/models.go
type SearchFilters struct {
    Search    string   // keyword (title, author, description)
    Genres    []string // multi-genre OR filter
    Status    string   // "ongoing" | "completed"
    YearRange [2]int   // mapped to chapter range (proxy — no year column)
    MinRating float64  // minimum average review rating (0 = no filter)
    SortBy    string   // "title" | "popularity" | "rating" | "recent"
    Page      int
    Limit     int
}
```

### 18.2 Extended MangaSearchQuery

```go
type MangaSearchQuery struct {
    Search    string   // existing — keyword search
    Genre     string   // existing — single genre (legacy)
    Genres    []string // NEW — multi-genre OR filter
    Status    string   // existing
    MinRating float64  // NEW — minimum avg review rating
    SortBy    string   // NEW — sort order
    Page      int
    Limit     int
}
```

### 18.3 Repository Search — SQL Generation

```
Search(query)
  │
  │ 1. Full-text WHERE clause:
  │    "(LOWER(title) LIKE ? OR LOWER(author) LIKE ? OR LOWER(description) LIKE ?)"
  │
  │ 2. Multi-genre: for each genre in allGenres:
  │    "LOWER(genres) LIKE ?"       ← AND-combined (each genre must match)
  │
  │ 3. Status: "status = ?"
  │
  │ 4. Rating join (when MinRating > 0 OR SortBy == "rating"):
  │    SELECT m.*, COALESCE(AVG(r.rating), 0) AS avg_rating
  │    FROM manga m
  │    LEFT JOIN reviews r ON r.manga_id = m.id
  │    WHERE ...
  │    GROUP BY m.id
  │    HAVING avg_rating >= ?
  │
  │ 5. ORDER BY switch:
  │    "popularity" → ORDER BY total_chapters DESC
  │    "rating"     → ORDER BY avg_rating DESC
  │    "recent"     → ORDER BY rowid DESC
  │    default      → ORDER BY title ASC
  │
  └─ LIMIT ? OFFSET ?
```

### 18.4 SearchByFilters — Adapter

```
SearchByFilters(f *SearchFilters)
  │ converts SearchFilters → MangaSearchQuery
  └─ calls Search(q)                    // reuses all existing logic
```

### 18.5 New Endpoint and CLI

| Layer | What |
|-------|------|
| Handler | `manga.AdvancedSearch` — POST body → `SearchFilters` → service |
| Service | `SearchByFilters(f)` → delegates to repo |
| Route | `POST /manga/search` (public — no auth required) |
| CLI | `mangahub manga advanced [query] --genres a,b --sort rating --min-rating 8` |

---

## 19. Recommendation System

### 19.1 Spec Structs

```go
// pkg/models/models.go
type UserProfile struct {
    UserID         string
    ReadManga      []string            // all manga IDs in library
    CompletedManga []string            // status=completed only
    Ratings        map[string]int      // manga_id -> review rating
    GenreScores    map[string]float64  // genre -> accumulated weight
}

// internal/recommendation/engine.go
type RecommendationEngine struct {
    UserSimilarity  map[string]float64   // other_user_id -> Jaccard score
    MangaSimilarity map[string][]string  // manga_id -> top-10 similar manga IDs
    UserProfiles    map[string]UserProfile
}
```

### 19.2 Algorithm Overview

```
Recommendation pipeline for userID:

1. LoadProfiles(allProgress, allRatings, allManga)
   │ Build UserProfile for every user:
   │   ReadManga      ← from user_progress table
   │   CompletedManga ← status = "completed"
   │   Ratings        ← from reviews table
   │   GenreScores    ← accumulated from genres of read manga
   │                     (completed manga weighted 2x)

2. ComputeUserSimilarity(targetUserID)
   │ for each other user:
   │   similarity = Jaccard(targetUser.ReadManga, otherUser.ReadManga)
   │   Jaccard(A, B) = |A ∩ B| / |A ∪ B|

3. ComputeMangaSimilarity()
   │ co[mangaA][mangaB] = count of users who read BOTH
   │ for each manga: sort co-read manga by count → top 10

4. Recommend(targetUserID, limit)
   │
   │ ── Collaborative filtering ──────────────────────────
   │ for each similar user (sorted by Jaccard score desc):
   │   for each manga they completed → score += similarity × 2.0
   │   for each manga they read     → score += similarity × 1.0
   │   (skip if target already read it)
   │
   │ ── Content-based filtering ──────────────────────────
   │ for each manga targetUser has completed:
   │   for each similar manga (MangaSimilarity[manga]):
   │     score += 1.5  (if not already read)
   │
   │ → sort by score desc → take top N
   └─ return []ScoredManga{MangaID, Score, Reason}
```

### 19.3 Service Flow

```
GET /users/recommendations?limit=10
  │
  ├─ auth.AuthMiddleware() → userID
  ▼
recommendation.Service.GetRecommendations(userID, limit)
  │
  │ loadAllProgress()  → SELECT FROM user_progress
  │ loadAllRatings()   → SELECT FROM reviews
  │ loadAllManga()     → SELECT FROM manga
  │
  │ engine.LoadProfiles(...)
  │ engine.ComputeUserSimilarity(userID)
  │ engine.ComputeMangaSimilarity()
  │ scored := engine.Recommend(userID, limit)
  │
  │ enrich each ScoredManga with full Manga data
  │ build ProfileStats (total read, completed, top genres, similar user count)
  ▼
RecommendationResult {
    UserID: "user-alice",
    Recommendations: [{MangaID, Score, Reason, Manga: {...}}],
    ProfileStats: {TotalRead, TotalCompleted, TopGenres, SimilarUsers}
}
```

### 19.4 CLI Flow

```
mangahub manga recommend --limit 5
  │
  │ requireAuth() → load token from config
  │ GET /users/recommendations?limit=5
  ▼
Display:
  📚 Your Reading Profile: Read: 5 | Completed: 2 | Similar users: 1
  🌟 Top 5 Recommendations:
    1. Attack on Titan  score: 1.75  reason: similar to naruto
    2. Demon Slayer     score: 1.20  reason: collaborative
    ...
```

---

## 20. CLI Updates

### 20.1 New CLI Commands Summary

| Command | Subcommand | HTTP / gRPC Call | New |
|---------|-----------|-----------------|-----|
| `manga` | `advanced` | `POST /manga/search` (SearchFilters body) | ✅ |
| `manga` | `recommend` | `GET /users/recommendations` | ✅ |
| `notify` | `send-ack` | `POST /notify/broadcast-ack` | ✅ |
| `notify` | `ack-stats` | `GET /notify/ack-stats` | ✅ |
| `grpc manga` | `stream` | gRPC `StreamSearch` (server streaming) | ✅ |
| `grpc` | `watch` | gRPC `WatchMangaUpdates` (server streaming) | ✅ |

### 20.2 manga advanced — Code Flow

```
mangahub manga advanced "naruto" --genres action --sort rating --min-rating 7
  │
  │ parseFlag → build SearchFilters body
  │ apiRequest("POST", "/manga/search", body, token)
  ▼
Server: manga.AdvancedSearch → SearchByFilters → repository.Search
  │ WHERE (title/author/desc LIKE ?) AND (genres LIKE ?)
  │ LEFT JOIN reviews → HAVING avg_rating >= 7
  │ ORDER BY avg_rating DESC
  ▼
CLI: printTable(results) + pagination hint
```

### 20.3 grpc manga stream — Code Flow

```
mangahub grpc manga stream --query "one" --limit 5
  │
  │ grpcClient.NewMangaClient(localhost:9092, token)
  │ client.StreamSearch("one", "", 5, onResult)
  │   └─ pb.MangaService.StreamSearch (server-side streaming RPC)
  │
  │ Server: for each manga in search results:
  │   stream.Send(mangaToProto(&manga))   ← one message at a time
  │
  │ Client recv loop: for each message:
  │   onResult(msg) → print formatted row
  │   count++
  │
  └─ io.EOF → return nil → print "Stream complete"
```

### 20.4 grpc watch — Code Flow

```
mangahub grpc watch [--manga-id one-piece]
  │
  │ context.WithCancel → ctx
  │ signal.Notify(interrupt) → go func { <-interrupt; cancel() }
  │ client.WatchMangaUpdates(ctx, mangaID, userID, onEvent)
  │   └─ pb.MangaService.WatchMangaUpdates (long-lived server stream)
  │
  │ Server: for { select {
  │   case <-ctx.Done(): return nil
  │   case event := <-eventHub.ch:
  │     if filter matches: stream.Send(event)
  │ }}
  │
  │ Client recv loop: for each event:
  │   onEvent(event) → print formatted line
  │   (blocks until Ctrl+C or server closes)
  │
  └─ ctx.Done() → cancel → io.EOF → clean exit

When user runs: mangahub progress update --manga-id one-piece --chapter 1096
  HTTP PUT /users/progress
    → userService.UpdateProgress
    → GRPCMangaServer.EventHub.PublishProgressUpdate("user-alice","one-piece",1096)
    → EventHub.Publish(event) → fans out to all watch subscribers
    → grpc watch Terminal prints: [10:30:15] 📖 PROGRESS manga=one-piece ch=1096
```

### 20.5 notify send-ack — Code Flow

```
mangahub notify send-ack --type new_chapter --manga-id one-piece --message "Ch 1121!"
  │
  │ apiRequest("POST", "/notify/broadcast-ack", body, token)
  │
  │ Server: BroadcastWithACK(notif)
  │   1. ackTracker.track(msg, addrs) → notif_id
  │   2. sendTo each client: notif + notification_id
  │   3. ackTracker.finalise(id, pd, 3s) ← blocks 3 seconds
  │      └─ drains ackCh for ACK messages from clients
  │   4. build DeliveryRecord{AckedBy, Unacked, AckRate}
  │
  └─ CLI prints delivery report (sent, ACK'd, unacked, rate)

If a client is running mangahub notify subscribe:
  Client receives: {"type":"new_chapter","notification_id":"notif-xxx",...}
  Client can ACK:  echo '{"type":"ack","notification_id":"notif-xxx"}' | nc -u localhost 9091
  Server records ACK → AckRate = 100%
```

---
**End of Documentation.**
