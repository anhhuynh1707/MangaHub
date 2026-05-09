package activity

import (
	"fmt"

	"mangahub/pkg/models"
)

// Service handles business logic for activities.
type Service struct {
	repo *Repository
}

// NewService creates a new activity service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// LogMangaStarted logs when a user starts reading a manga.
func (s *Service) LogMangaStarted(userID, username, mangaID, mangaTitle string) (*models.Activity, error) {
	message := fmt.Sprintf("%s started reading %s", username, mangaTitle)
	return s.repo.CreateActivity(userID, "started_manga", mangaID, "", "", message)
}

// LogMangaCompleted logs when a user completes a manga.
func (s *Service) LogMangaCompleted(userID, username, mangaID, mangaTitle string) (*models.Activity, error) {
	message := fmt.Sprintf("%s completed %s", username, mangaTitle)
	return s.repo.CreateActivity(userID, "completed_manga", mangaID, "", "", message)
}

// LogReviewWritten logs when a user writes a review.
func (s *Service) LogReviewWritten(userID, username, mangaID, mangaTitle, reviewID string, rating int) (*models.Activity, error) {
	message := fmt.Sprintf("%s rated %s %d/10", username, mangaTitle, rating)
	return s.repo.CreateActivity(userID, "wrote_review", mangaID, reviewID, "", message)
}

// LogFriendAdded logs when a user adds a friend.
func (s *Service) LogFriendAdded(userID, username, friendID, friendName string) (*models.Activity, error) {
	message := fmt.Sprintf("%s added %s as a friend", username, friendName)
	return s.repo.CreateActivity(userID, "added_friend", "", "", friendID, message)
}

// LogSharedListCreated logs when a user creates a shared list.
func (s *Service) LogSharedListCreated(userID, username, listName string) (*models.Activity, error) {
	message := fmt.Sprintf("%s created a new reading list: %s", username, listName)
	return s.repo.CreateActivity(userID, "shared_list_created", "", "", "", message)
}

// LogUserPost logs a custom status update/post from a user.
func (s *Service) LogUserPost(userID, username, message string) (*models.Activity, error) {
	formattedMessage := fmt.Sprintf("%s posted: %s", username, message)
	return s.repo.CreateActivity(userID, "user_post", "", "", "", formattedMessage)
}

// GetUserActivities retrieves activities from a specific user.
func (s *Service) GetUserActivities(userID string, limit, offset int) ([]models.Activity, error) {
	return s.repo.GetUserActivities(userID, limit, offset)
}

// GetAllActivities retrieves all activities from all users globally.
func (s *Service) GetAllActivities(limit, offset int) ([]models.Activity, error) {
	return s.repo.GetAllActivities(limit, offset)
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

	return s.repo.DeleteActivity(activityID)
}
