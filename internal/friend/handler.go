package friend

import (
	"strconv"

	"mangahub/internal/activity"
	"mangahub/internal/auth"
	"mangahub/pkg/utils"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service         *Service
	activityService *activity.Service
}

func NewHandler(service *Service, activityService *activity.Service) *Handler {
	return &Handler{service: service, activityService: activityService}
}

// AddFriend handles POST /friends/add
func (h *Handler) AddFriend(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	var req struct {
		FriendID string `json:"friend_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request body")
		return
	}

	if req.FriendID == userID {
		utils.BadRequestResponse(c, "Cannot add yourself as a friend")
		return
	}

	err = h.service.AddFriend(userID, req.FriendID)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	utils.SuccessResponse(c, "Friend request sent successfully", nil)
}

// AcceptFriend handles POST /friends/:friend_id/accept
func (h *Handler) AcceptFriend(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	friendID := c.Param("friend_id")

	err = h.service.AcceptFriend(userID, friendID)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	username, _ := c.Get("username")
	if h.activityService != nil && username != nil {
		// Log activity that user added a friend
		h.activityService.LogFriendAdded(userID, username.(string), friendID, friendID)
		
		// Optional: also log it for the friend since friendship is mutual
		h.activityService.LogFriendAdded(friendID, friendID, userID, username.(string))
	}

	utils.SuccessResponse(c, "Friend request accepted", nil)
}

// DeclineFriend handles POST /friends/:friend_id/decline
func (h *Handler) DeclineFriend(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	friendID := c.Param("friend_id")

	// Remove friend (decline is same as removing pending request)
	err = h.service.RemoveFriend(userID, friendID)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	utils.SuccessResponse(c, "Friend request declined", nil)
}

// RemoveFriend handles DELETE /friends/:friend_id
func (h *Handler) RemoveFriend(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	friendID := c.Param("friend_id")

	err = h.service.RemoveFriend(userID, friendID)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	utils.SuccessResponse(c, "Friend removed successfully", nil)
}

// BlockFriend handles POST /friends/:user_id/block
func (h *Handler) BlockFriend(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	blockID := c.Param("user_id")

	if blockID == userID {
		utils.BadRequestResponse(c, "Cannot block yourself")
		return
	}

	err = h.service.BlockFriend(userID, blockID)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	utils.SuccessResponse(c, "User blocked successfully", nil)
}

// GetFriends handles GET /users/friends
func (h *Handler) GetFriends(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	friends, err := h.service.GetFriends(userID)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	// Apply pagination manually
	start := (page - 1) * limit
	end := start + limit
	if start > len(friends) {
		start = len(friends)
	}
	if end > len(friends) {
		end = len(friends)
	}

	// Map []string to the struct expected by the CLI
	var formattedFriends []map[string]string
	for _, id := range friends[start:end] {
		formattedFriends = append(formattedFriends, map[string]string{
			"user_id":  id,
			"username": id, // Since we only have ID, use ID as username for now
			"status":   "accepted",
		})
	}

	utils.SuccessResponse(c, "Friends retrieved successfully", gin.H{
		"friends": formattedFriends,
		"total":   len(friends),
		"page":    page,
		"limit":   limit,
		"pages":   (len(friends) + limit - 1) / limit,
	})
}

// GetPendingRequests handles GET /users/friends/pending
func (h *Handler) GetPendingRequests(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// Get pending friend requests for this user
	pending, err := h.service.GetPendingRequests(userID)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	// Apply pagination manually
	start := (page - 1) * limit
	end := start + limit
	if start > len(pending) {
		start = len(pending)
	}
	if end > len(pending) {
		end = len(pending)
	}

	utils.SuccessResponse(c, "Pending requests retrieved successfully", gin.H{
		"pending_requests": pending[start:end],
		"count":            len(pending),
		"page":             page,
		"limit":            limit,
		"pages":            (len(pending) + limit - 1) / limit,
	})
}

// GetFriendCount handles GET /users/friends/count
func (h *Handler) GetFriendCount(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	count, err := h.service.GetFriendCount(userID)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	utils.SuccessResponse(c, "Friend count retrieved", gin.H{"count": count})
}

// GetFriendInfo handles GET /friends/:friend_id/info
func (h *Handler) GetFriendInfo(c *gin.Context) {
	friendID := c.Param("friend_id")

	// Return friend information
	isFriend, err := h.service.IsFriend(friendID, friendID)
	if err != nil || !isFriend {
		utils.NotFoundResponse(c, "Friend not found")
		return
	}

	utils.SuccessResponse(c, "Friend info retrieved", gin.H{
		"friend_id": friendID,
		"status":    "accepted",
	})
}

// CheckFriendship helper
func (h *Handler) CheckFriendship(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	var req struct {
		FriendID string `json:"friend_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request body")
		return
	}

	isFriend, err := h.service.IsFriend(userID, req.FriendID)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	utils.SuccessResponse(c, "Friendship status retrieved", gin.H{
		"is_friend": isFriend,
	})
}
