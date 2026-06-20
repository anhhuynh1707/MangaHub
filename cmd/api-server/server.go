package main

import (
	"database/sql"

	"mangahub/internal/activity"
	"mangahub/internal/friend"
	grpcServer "mangahub/internal/grpc"
	mangaPkg "mangahub/internal/manga"
	"mangahub/internal/recommendation"
	"mangahub/internal/review"
	"mangahub/internal/sharedlist"
	"mangahub/internal/tcp"
	"mangahub/internal/udp"
	userPkg "mangahub/internal/user"
	wsPkg "mangahub/internal/websocket"
	"mangahub/pkg/cache"

	"github.com/gin-gonic/gin"
)

// APIServer is the core server structure: it holds every dependency the HTTP
// handlers need so route handlers can be methods on *APIServer instead of inline
// closures in main(). Wiring lives in main(); handler methods live in the
// health.go / sync.go / notify.go / data.go / chat.go / routes.go files.
type APIServer struct {
	Router   *gin.Engine
	Database *sql.DB

	// Real-time / streaming services
	TCPServer       *tcp.ProgressSyncServer
	TCPClient       *tcp.ProgressSyncClient
	UDPServer       *udp.NotificationServer
	UDPClient       *udp.NotificationClient
	GRPCMangaServer *grpcServer.MangaServer
	Hub             *wsPkg.ChatHub
	UseClients      bool

	// Cache + business services
	Cache          *cache.RedisCache
	MangaService   *mangaPkg.Service
	UserService    *userPkg.Service
	RecService     *recommendation.Service
	MangaDexClient *mangaPkg.MangaDexClient

	// Domain handlers
	UserHandler       *userPkg.Handler
	MangaHandler      *mangaPkg.Handler
	ReviewHandler     *review.Handler
	FriendHandler     *friend.Handler
	SharedListHandler *sharedlist.Handler
	ActivityHandler   *activity.Handler

	// Config
	Port       string
	TCPPort    string
	UDPPort    string
	GRPCPort   string
	EnableGRPC bool
}
