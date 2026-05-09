package activity

import (
	"fmt"
	"log"

	"mangahub/pkg/cache"
	"mangahub/pkg/models"
)

// Service handles business logic for activities.
type Service struct {
	repo  *Repository
	cache *cache.RedisCache
}

// NewService creates a new activity service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// SetCache sets the Redis cache instance for this service.
func (s *Service) SetCache(c *cache.RedisCache) {
	s.cache = c
}

// invalidateActivityCaches removes all activity feed caches.
func (s *Service) invalidateActivityCaches() {
	if s.cache == nil {
		return
	}
	if err := s.cache.DeletePattern(cache.PrefixActivityFeed + "*"); err != nil {
		log.Printf("cache: failed to invalidate activity feed keys: %v", err)
	}
	if err := s.cache.DeletePattern(cache.PrefixActivityUser + "*"); err != nil {
		log.Printf("cache: failed to invalidate activity user keys: %v", err)
	}
}

// LogMangaStarted logs when a user starts reading a manga.
func (s *Service) LogMangaStarted(userID, username, mangaID, mangaTitle string) (*models.Activity, error) {
	message := fmt.Sprintf("%s started reading %s", username, mangaTitle)
	activity, err := s.repo.CreateActivity(userID, "started_manga", mangaID, "", "", message)
	if err == nil {
		s.invalidateActivityCaches()
	}
	return activity, err
}

// LogMangaCompleted logs when a user completes a manga.
func (s *Service) LogMangaCompleted(userID, username, mangaID, mangaTitle string) (*models.Activity, error) {
	message := fmt.Sprintf("%s completed %s", username, mangaTitle)
	activity, err := s.repo.CreateActivity(userID, "completed_manga", mangaID, "", "", message)
	if err == nil {
		s.invalidateActivityCaches()
	}
	return activity, err
}

// LogReviewWritten logs when a user writes a review.
func (s *Service) LogReviewWritten(userID, username, mangaID, mangaTitle, reviewID string, rating int) (*models.Activity, error) {
	message := fmt.Sprintf("%s rated %s %d/10", username, mangaTitle, rating)
	activity, err := s.repo.CreateActivity(userID, "wrote_review", mangaID, reviewID, "", message)
	if err == nil {
		s.invalidateActivityCaches()
	}
	return activity, err
}

// LogFriendAdded logs when a user adds a friend.
func (s *Service) LogFriendAdded(userID, username, friendID, friendName string) (*models.Activity, error) {
	message := fmt.Sprintf("%s added %s as a friend", username, friendName)
	activity, err := s.repo.CreateActivity(userID, "added_friend", "", "", friendID, message)
	if err == nil {
		s.invalidateActivityCaches()
	}
	return activity, err
}

// LogSharedListCreated logs when a user creates a shared list.
func (s *Service) LogSharedListCreated(userID, username, listName string) (*models.Activity, error) {
	message := fmt.Sprintf("%s created a new reading list: %s", username, listName)
	activity, err := s.repo.CreateActivity(userID, "shared_list_created", "", "", "", message)
	if err == nil {
		s.invalidateActivityCaches()
	}
	return activity, err
}

// LogUserPost logs a custom status update/post from a user.
func (s *Service) LogUserPost(userID, username, message string) (*models.Activity, error) {
	formattedMessage := fmt.Sprintf("%s posted: %s", username, message)
	activity, err := s.repo.CreateActivity(userID, "user_post", "", "", "", formattedMessage)
	if err == nil {
		s.invalidateActivityCaches()
	}
	return activity, err
}

// GetUserActivities retrieves activities from a specific user.
func (s *Service) GetUserActivities(userID string, limit, offset int) ([]models.Activity, error) {
	// Try cache first
	if s.cache != nil {
		var cached []models.Activity
		if s.cache.Get(cache.ActivityUserKey(userID, offset/limit+1, limit), &cached) {
			return cached, nil
		}
	}

	activities, err := s.repo.GetUserActivities(userID, limit, offset)
	if err != nil {
		return nil, err
	}

	// Populate cache
	if s.cache != nil {
		_ = s.cache.Set(cache.ActivityUserKey(userID, offset/limit+1, limit), activities, cache.ActivityFeedTTL)
	}
	return activities, nil
}

// GetAllActivities retrieves all activities from all users globally.
func (s *Service) GetAllActivities(limit, offset int) ([]models.Activity, error) {
	// Try cache first
	if s.cache != nil {
		var cached []models.Activity
		cacheKey := cache.ActivityFeedKey(offset/limit+1, limit, "")
		if s.cache.Get(cacheKey, &cached) {
			return cached, nil
		}
	}

	activities, err := s.repo.GetAllActivities(limit, offset)
	if err != nil {
		return nil, err
	}

	// Populate cache
	if s.cache != nil {
		cacheKey := cache.ActivityFeedKey(offset/limit+1, limit, "")
		_ = s.cache.Set(cacheKey, activities, cache.ActivityFeedTTL)
	}
	return activities, nil
}

// GetFriendsActivityFeed retrieves the activity feed for a user's friends.
func (s *Service) GetFriendsActivityFeed(userID string, limit, offset int) ([]models.Activity, error) {
	return s.repo.GetFriendsActivities(userID, limit, offset)
}

// GetActivityByID retrieves a specific activity.
func (s *Service) GetActivityByID(activityID string) (*models.Activity, error) {
	return s.repo.GetActivityByID(activityID)
}

// DeleteActivity removes an activity (typically admin only).
func (s *Service) DeleteActivity(userID, activityID string) error {
	// Get activity to verify ownership
	activity, err := s.repo.GetActivityByID(activityID)
	if err != nil {
		return err
	}

	if activity.UserID != userID {
		return fmt.Errorf("not authorized to delete this activity")
	}

	if err := s.repo.DeleteActivity(activityID); err != nil {
		return err
	}
	s.invalidateActivityCaches()
	return nil
}

