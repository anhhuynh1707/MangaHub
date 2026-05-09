package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// Default TTLs for different data types.
const (
	MangaDetailTTL  = 10 * time.Minute  // Individual manga lookups
	MangaListTTL    = 5 * time.Minute   // Search results / manga lists
	UserLibraryTTL  = 5 * time.Minute   // User library data
	UserProfileTTL  = 10 * time.Minute  // User profile data
	ActivityFeedTTL = 2 * time.Minute   // Activity feed (changes frequently)
	MangaCountTTL   = 1 * time.Minute   // Manga count (used in health check)
)

// Key prefixes for organized cache namespacing.
const (
	PrefixManga        = "manga:"
	PrefixMangaSearch  = "manga:search:"
	PrefixMangaCount   = "manga:count"
	PrefixMangaAll     = "manga:all"
	PrefixUserLibrary  = "user:library:"
	PrefixUserProfile  = "user:profile:"
	PrefixActivityFeed = "feed:activities:"
	PrefixActivityUser = "feed:user:"
)

// RedisCache wraps a Redis client with helper methods for caching.
type RedisCache struct {
	client *redis.Client
	ctx    context.Context
}

// New creates a new RedisCache. If the connection fails, caching will be
// disabled gracefully — the application still works, just without caching.
func New(addr, password string, db int) *RedisCache {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		PoolSize:     10,
	})

	ctx := context.Background()

	// Ping to verify connection
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("⚠️  Redis connection failed (%s): %v — caching disabled", addr, err)
		return &RedisCache{client: nil, ctx: ctx}
	}

	log.Printf("✅ Redis connected at %s (db=%d)", addr, db)
	return &RedisCache{client: client, ctx: ctx}
}

// IsAvailable returns true if the Redis client is connected and usable.
func (c *RedisCache) IsAvailable() bool {
	return c.client != nil
}

// Client returns the underlying Redis client (for advanced operations).
func (c *RedisCache) Client() *redis.Client {
	return c.client
}

// --- Generic helpers ---

// Set serializes value as JSON and stores it in Redis with the given TTL.
func (c *RedisCache) Set(key string, value interface{}, ttl time.Duration) error {
	if c.client == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache marshal error: %w", err)
	}
	return c.client.Set(c.ctx, key, data, ttl).Err()
}

// Get retrieves a cached value and unmarshals it into dest.
// Returns true if the cache hit was successful, false on miss or error.
func (c *RedisCache) Get(key string, dest interface{}) bool {
	if c.client == nil {
		return false
	}
	data, err := c.client.Get(c.ctx, key).Bytes()
	if err != nil {
		return false // cache miss or error — both are non-fatal
	}
	if err := json.Unmarshal(data, dest); err != nil {
		log.Printf("cache unmarshal error for key %s: %v", key, err)
		return false
	}
	return true
}

// Delete removes a specific key from the cache.
func (c *RedisCache) Delete(key string) error {
	if c.client == nil {
		return nil
	}
	return c.client.Del(c.ctx, key).Err()
}

// DeletePattern removes all keys matching a glob pattern (e.g. "manga:search:*").
func (c *RedisCache) DeletePattern(pattern string) error {
	if c.client == nil {
		return nil
	}

	iter := c.client.Scan(c.ctx, 0, pattern, 100).Iterator()
	var keys []string
	for iter.Next(c.ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("cache scan error: %w", err)
	}
	if len(keys) > 0 {
		return c.client.Del(c.ctx, keys...).Err()
	}
	return nil
}

// Flush clears the entire Redis database. Use with caution.
func (c *RedisCache) Flush() error {
	if c.client == nil {
		return nil
	}
	return c.client.FlushDB(c.ctx).Err()
}

// Close gracefully shuts down the Redis connection.
func (c *RedisCache) Close() error {
	if c.client == nil {
		return nil
	}
	return c.client.Close()
}

// --- Domain-specific key builders ---

// MangaKey returns the cache key for a single manga by ID.
func MangaKey(id string) string {
	return PrefixManga + id
}

// MangaSearchKey returns the cache key for a search query result set.
func MangaSearchKey(search, genre, status string, page, limit int) string {
	return fmt.Sprintf("%s%s:%s:%s:%d:%d", PrefixMangaSearch, search, genre, status, page, limit)
}

// UserLibraryKey returns the cache key for a user's library.
func UserLibraryKey(userID string) string {
	return PrefixUserLibrary + userID
}

// UserProfileKey returns the cache key for a user's profile.
func UserProfileKey(userID string) string {
	return PrefixUserProfile + userID
}

// ActivityFeedKey returns the cache key for the global activity feed.
func ActivityFeedKey(page, limit int, typeFilter string) string {
	return fmt.Sprintf("%s%d:%d:%s", PrefixActivityFeed, page, limit, typeFilter)
}

// ActivityUserKey returns the cache key for a specific user's activities.
func ActivityUserKey(userID string, page, limit int) string {
	return fmt.Sprintf("%s%s:%d:%d", PrefixActivityUser, userID, page, limit)
}

// Stats returns basic cache statistics for the health endpoint.
func (c *RedisCache) Stats() map[string]interface{} {
	if c.client == nil {
		return map[string]interface{}{
			"status": "disabled",
		}
	}

	info, err := c.client.Info(c.ctx, "stats", "memory", "keyspace").Result()
	if err != nil {
		return map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		}
	}

	dbSize, _ := c.client.DBSize(c.ctx).Result()

	return map[string]interface{}{
		"status":    "connected",
		"keys":      dbSize,
		"info_raw":  info[:min(len(info), 500)], // Truncate for readability
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
