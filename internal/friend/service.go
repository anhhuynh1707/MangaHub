package friend

import (
	"mangahub/pkg/utils"
)

// Service handles business logic for friendships.
type Service struct {
	repo *Repository
}

// NewService creates a new friend service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// AddFriend sends a friend request to another user.
func (s *Service) AddFriend(userID, friendID string) error {
	if userID == friendID {
		return utils.ErrBadRequest("cannot add yourself as a friend")
	}

	return s.repo.AddFriend(userID, friendID)
}

// AcceptFriend accepts a pending friend request.
func (s *Service) AcceptFriend(userID, requesterID string) error {
	return s.repo.AcceptFriend(userID, requesterID)
}

// RemoveFriend removes a friend relationship.
func (s *Service) RemoveFriend(userID, friendID string) error {
	return s.repo.RemoveFriend(userID, friendID)
}

// BlockFriend blocks a user.
func (s *Service) BlockFriend(userID, blockedID string) error {
	if userID == blockedID {
		return utils.ErrBadRequest("cannot block yourself")
	}

	return s.repo.BlockFriend(userID, blockedID)
}

// GetFriends returns all friends for a user.
func (s *Service) GetFriends(userID string) ([]string, error) {
	return s.repo.GetFriends(userID)
}

// GetPendingRequests returns pending friend requests for a user.
func (s *Service) GetPendingRequests(userID string) ([]string, error) {
	return s.repo.GetPendingRequests(userID)
}

// IsFriend checks if two users are friends.
func (s *Service) IsFriend(userID, friendID string) (bool, error) {
	return s.repo.IsFriend(userID, friendID)
}

// IsBlocked checks if a user is blocked.
func (s *Service) IsBlocked(userID, blockedID string) (bool, error) {
	return s.repo.IsBlocked(userID, blockedID)
}

// GetFriendCount returns the number of friends a user has.
func (s *Service) GetFriendCount(userID string) (int, error) {
	friends, err := s.repo.GetFriends(userID)
	if err != nil {
		return 0, err
	}
	return len(friends), nil
}
