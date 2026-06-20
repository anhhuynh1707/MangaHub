package main

import (
	"time"

	"mangahub/internal/auth"

	"github.com/gin-contrib/cors"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// registerRoutes wires every HTTP route to its handler method. Handlers contain
// the logic; this function only maps paths → methods and applies middleware.
func (s *APIServer) registerRoutes(allowedOrigins []string) {
	r := s.Router

	r.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// ── Health ──
	r.GET("/health", s.Health)
	r.GET("/health/db", s.HealthDB)
	r.GET("/health/cache", s.HealthCache)
	r.GET("/health/tcp", s.HealthTCP)
	r.GET("/health/udp", s.HealthUDP)
	r.GET("/health/ws", s.HealthWS)
	r.GET("/health/grpc", s.HealthGRPC)

	// ── Cache management (auth) ──
	cacheRoutes := r.Group("/cache", auth.AuthMiddleware())
	{
		cacheRoutes.GET("/stats", s.CacheStats)
		cacheRoutes.DELETE("/flush", s.CacheFlush)
	}

	// ── Auth ──
	r.POST("/auth/register", s.UserHandler.Register)
	r.POST("/auth/login", s.UserHandler.Login)
	authRoutes := r.Group("/auth", auth.AuthMiddleware())
	{
		authRoutes.GET("/status", s.UserHandler.AuthStatus)
		authRoutes.POST("/logout", s.UserHandler.Logout)
		authRoutes.PUT("/change-password", s.UserHandler.ChangePassword)
	}

	// ── Manga ──
	r.GET("/manga", s.MangaHandler.Search)
	r.POST("/manga/search", s.MangaHandler.AdvancedSearch)
	r.GET("/manga/:id", s.MangaHandler.GetByID)
	mangaAuth := r.Group("/manga", auth.AuthMiddleware())
	{
		mangaAuth.POST("", s.MangaHandler.Create)
		mangaAuth.PUT("/:id", s.MangaHandler.Update)
		mangaAuth.DELETE("/:id", s.MangaHandler.Delete)
	}

	// ── Users ──
	users := r.Group("/users", auth.AuthMiddleware())
	{
		users.GET("/profile", s.UserHandler.GetProfile)
		users.GET("/search", s.UserHandler.SearchUsers)
		users.POST("/library", s.UserHandler.AddToLibrary)
		users.GET("/library", s.UserHandler.GetLibrary)
		users.DELETE("/library/:manga_id", s.UserHandler.RemoveFromLibrary)
		users.GET("/recommendations", s.Recommendations)
		users.PUT("/progress", s.UpdateProgress)
	}

	// ── TCP sync (auth) ──
	syncRoutes := r.Group("/sync", auth.AuthMiddleware())
	{
		syncRoutes.GET("/status", s.SyncStatus)
		syncRoutes.GET("/conflicts", s.SyncConflicts)
		syncRoutes.GET("/strategy", s.SyncStrategy)
		syncRoutes.PUT("/strategy", s.SetSyncStrategy)
	}

	// ── UDP notifications (auth) ──
	notifyRoutes := r.Group("/notify", auth.AuthMiddleware())
	{
		notifyRoutes.POST("/broadcast", s.NotifyBroadcast)
		notifyRoutes.POST("/broadcast-ack", s.NotifyBroadcastACK)
		notifyRoutes.GET("/ack-stats", s.NotifyAckStats)
		notifyRoutes.GET("/status", s.NotifyStatus)
	}

	// ── Chat ──
	r.GET("/ws/chat", s.ChatWebSocket)
	r.GET("/chat/history", auth.AuthMiddleware(), s.ChatHistory)

	// ── Data collection / export (auth) ──
	dataRoutes := r.Group("/data", auth.AuthMiddleware())
	{
		dataRoutes.POST("/seed", s.DataSeed)
		dataRoutes.POST("/fetch-mangadex", s.DataFetchMangaDex)
		dataRoutes.GET("/export-json", s.DataExportJSON)
		dataRoutes.POST("/scrape-quotes", s.DataScrapeQuotes)
		dataRoutes.GET("/scraped-quotes", s.DataScrapedQuotes)
		dataRoutes.POST("/test-httpbin", s.DataTestHTTPBin)
		dataRoutes.POST("/export-files", s.DataExportFiles)
		dataRoutes.POST("/import-json", s.DataImportJSON)
		dataRoutes.GET("/export/library", s.DataExportLibrary)
		dataRoutes.GET("/export/progress", s.DataExportProgress)
		dataRoutes.GET("/export/manga", s.DataExportManga)
		dataRoutes.GET("/export/full", s.DataExportFull)
	}

	// ── Reviews ──
	mangaReviews := r.Group("/manga")
	{
		mangaReviews.GET("/:id/reviews", s.ReviewHandler.GetReviews)
		mangaReviews.GET("/:id/rating-stats", s.ReviewHandler.GetRatingStats)
		authMangaReviews := mangaReviews.Group("/:id/reviews", auth.AuthMiddleware())
		authMangaReviews.POST("", s.ReviewHandler.CreateReview)
	}
	reviewRoutes := r.Group("/reviews", auth.AuthMiddleware())
	{
		reviewRoutes.GET("/:review_id", s.ReviewHandler.GetReview)
		reviewRoutes.PUT("/:review_id", s.ReviewHandler.UpdateReview)
		reviewRoutes.DELETE("/:review_id", s.ReviewHandler.DeleteReview)
		reviewRoutes.POST("/:review_id/helpful", s.ReviewHandler.MarkHelpful)
	}
	r.Group("/users/reviews", auth.AuthMiddleware()).GET("", s.ReviewHandler.GetMyReviews)

	// ── Friends ──
	friendRoutes := r.Group("/friends", auth.AuthMiddleware())
	{
		friendRoutes.POST("/add", s.FriendHandler.AddFriend)
		friendRoutes.POST("/:friend_id/accept", s.FriendHandler.AcceptFriend)
		friendRoutes.POST("/:friend_id/decline", s.FriendHandler.DeclineFriend)
		friendRoutes.DELETE("/:friend_id", s.FriendHandler.RemoveFriend)
		friendRoutes.POST("/:friend_id/block", s.FriendHandler.BlockFriend)
		friendRoutes.GET("/:friend_id/info", s.FriendHandler.GetFriendInfo)
		friendRoutes.POST("/:friend_id/check", s.FriendHandler.CheckFriendship)
	}
	userFriends := r.Group("/users", auth.AuthMiddleware())
	{
		userFriends.GET("/friends", s.FriendHandler.GetFriends)
		userFriends.GET("/friends/pending", s.FriendHandler.GetPendingRequests)
		userFriends.GET("/friends/count", s.FriendHandler.GetFriendCount)
	}

	// ── Shared reading lists ──
	readingLists := r.Group("/reading-lists")
	{
		readingLists.GET("/public", s.SharedListHandler.GetPublicLists)
		readingLists.GET("/:list_id", s.SharedListHandler.GetList)
		authLists := readingLists.Group("", auth.AuthMiddleware())
		{
			authLists.POST("/create", s.SharedListHandler.CreateList)
			authLists.GET("/mine", s.SharedListHandler.GetMyLists)
			authLists.GET("/subscribed", s.SharedListHandler.GetSubscribedLists)
			authLists.PUT("/:list_id", s.SharedListHandler.UpdateList)
			authLists.DELETE("/:list_id", s.SharedListHandler.DeleteList)
			authLists.POST("/:list_id/subscribe", s.SharedListHandler.SubscribeToList)
			authLists.DELETE("/:list_id/subscribe", s.SharedListHandler.UnsubscribeFromList)
			authLists.POST("/:list_id/manga", s.SharedListHandler.AddMangaToList)
			authLists.DELETE("/:list_id/manga/:manga_id", s.SharedListHandler.RemoveMangaFromList)
		}
	}

	// ── Activity feed ──
	feedRoutes := r.Group("/feed", auth.AuthMiddleware())
	{
		feedRoutes.POST("/activities", s.ActivityHandler.PostActivity)
		feedRoutes.GET("/activities", s.ActivityHandler.GetActivityFeed)
		feedRoutes.GET("/timeline", s.ActivityHandler.GetTimelineView)
		feedRoutes.GET("/search", s.ActivityHandler.SearchActivities)
		feedRoutes.GET("/stats", s.ActivityHandler.GetActivityStats)
		feedRoutes.DELETE("/clear", s.ActivityHandler.ClearActivityFeed)
		feedRoutes.GET("/notifications", s.ActivityHandler.GetActivityNotifications)
		feedRoutes.GET("/stream", s.ActivityHandler.FollowActivityStream)
	}
	r.Group("/users", auth.AuthMiddleware()).GET("/:user_id/activities", s.ActivityHandler.GetUserActivities)

	// ── Swagger ──
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
