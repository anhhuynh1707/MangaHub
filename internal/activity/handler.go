package activity

import (
	"net/http"
	"strconv"
	"strings"

	"mangahub/internal/auth"
	"mangahub/pkg/models"
	"mangahub/pkg/utils"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// PostActivity handles POST /feed/activities
//
// @Summary      Post a manual activity
// @Description  Publish a custom user activity message to the feed
// @Tags         feed
// @Accept       json
// @Produce      json
// @Param        body  body      object  true  "Activity payload — {message: string}"
// @Success      200   {object}  utils.APIResponse  "Activity posted"
// @Failure      400   {object}  utils.APIResponse  "Invalid request"
// @Failure      401   {object}  utils.APIResponse  "Unauthorized"
// @Security     BearerAuth
// @Router       /feed/activities [post]
func (h *Handler) PostActivity(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	username, err := auth.GetUsernameFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	var req models.CreateActivityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request: "+err.Error())
		return
	}

	activity, err := h.service.LogUserPost(userID, username, req.Message)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to post activity: "+err.Error())
		return
	}

	utils.SuccessResponse(c, "Activity posted successfully", activity)
}

// GetActivityFeed handles GET /feed/activities
//
// @Summary      Get activity feed
// @Description  Retrieve the global activity feed across all users with optional type filter and pagination
// @Tags         feed
// @Produce      json
// @Param        page   query     int     false  "Page number (default 1)"
// @Param        limit  query     int     false  "Results per page (default 20)"
// @Param        type   query     string  false  "Filter by activity type (e.g. manga_completed, review_written)"
// @Success      200    {object}  utils.APIResponse  "Activity feed"
// @Failure      401    {object}  utils.APIResponse  "Unauthorized"
// @Security     BearerAuth
// @Router       /feed/activities [get]
func (h *Handler) GetActivityFeed(c *gin.Context) {
	_, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")
	typeFilter := c.DefaultQuery("type", "")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit
	activities, err := h.service.GetAllActivities(limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	total := len(activities)

	// Filter by type if provided
	if typeFilter != "" {
		filtered := make([]models.Activity, 0)
		for _, activity := range activities {
			if activity.Type == typeFilter {
				filtered = append(filtered, activity)
			}
		}
		activities = filtered
	}

	utils.SuccessResponse(c, "Activity feed retrieved", gin.H{
		"activities": activities,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"pages":      (total + limit - 1) / limit,
	})
}

// GetUserActivities handles GET /users/:user_id/activities
//
// @Summary      Get activities for a user
// @Description  Retrieve the activity history for a specific user by their user ID
// @Tags         feed
// @Produce      json
// @Param        user_id  path      string  true   "User ID"
// @Param        page     query     int     false  "Page number (default 1)"
// @Param        limit    query     int     false  "Results per page (default 20)"
// @Param        type     query     string  false  "Filter by activity type"
// @Success      200      {object}  utils.APIResponse  "User activities"
// @Failure      500      {object}  utils.APIResponse  "Server error"
// @Security     BearerAuth
// @Router       /users/{user_id}/activities [get]
func (h *Handler) GetUserActivities(c *gin.Context) {
	userID := c.Param("user_id")

	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")
	typeFilter := c.DefaultQuery("type", "")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit
	activities, err := h.service.GetUserActivities(userID, limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	total := len(activities)

	// Filter by type if provided
	if typeFilter != "" {
		filtered := make([]models.Activity, 0)
		for _, activity := range activities {
			if activity.Type == typeFilter {
				filtered = append(filtered, activity)
			}
		}
		activities = filtered
	}

	utils.SuccessResponse(c, "User activities retrieved", gin.H{
		"activities": activities,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"pages":      (total + limit - 1) / limit,
	})
}

// GetActivityStats handles GET /feed/stats
//
// @Summary      Get activity statistics
// @Description  Return a count breakdown of your activities by type (manga_completed, review_written, etc.)
// @Tags         feed
// @Produce      json
// @Success      200  {object}  utils.APIResponse  "Activity stats by type"
// @Failure      401  {object}  utils.APIResponse  "Unauthorized"
// @Security     BearerAuth
// @Router       /feed/stats [get]
func (h *Handler) GetActivityStats(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	// Get stats by activity type
	activities, err := h.service.GetUserActivities(userID, 1000, 0)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	stats := make(map[string]int)
	for _, activity := range activities {
		stats[activity.Type]++
	}

	utils.SuccessResponse(c, "Activity stats retrieved", stats)
}

// SearchActivities handles GET /feed/search
func (h *Handler) SearchActivities(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	query := c.Query("q")
	if query == "" {
		utils.BadRequestResponse(c, "Search query is required")
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

	offset := (page - 1) * limit
	activities, err := h.service.GetUserActivities(userID, limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	// Filter by search query
	query = strings.ToLower(query)
	filtered := make([]models.Activity, 0)

	for _, activity := range activities {
		desc := strings.ToLower(activity.Message)
		if strings.Contains(desc, query) {
			filtered = append(filtered, activity)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   filtered,
		"total":  len(filtered),
		"query":  query,
	})
}

// GetTimelineView handles GET /feed/timeline
//
// @Summary      Get friends timeline
// @Description  Return a combined chronological feed of activities from the authenticated user and their friends
// @Tags         feed
// @Produce      json
// @Param        page   query     int  false  "Page number (default 1)"
// @Param        limit  query     int  false  "Results per page (default 20)"
// @Success      200    {object}  utils.APIResponse  "Timeline feed"
// @Failure      401    {object}  utils.APIResponse  "Unauthorized"
// @Security     BearerAuth
// @Router       /feed/timeline [get]
func (h *Handler) GetTimelineView(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "50")
	timeRange := c.DefaultQuery("range", "all_time")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}

	offset := (page - 1) * limit
	activities, err := h.service.GetFriendsActivityFeed(userID, limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	total := len(activities)

	// Group by date for timeline view
	timeline := make(map[string][]models.Activity)

	for _, activity := range activities {
		// Extract date from timestamp
		date := activity.CreatedAt.Format("2006-01-02")
		timeline[date] = append(timeline[date], activity)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"data":       timeline,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"time_range": timeRange,
	})
}

// ClearActivityFeed handles DELETE /feed/clear
func (h *Handler) ClearActivityFeed(c *gin.Context) {
	_, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	// This would delete or mark all activities for the user as read
	// Implementation depends on whether you want to delete or mark as read
	utils.SuccessResponse(c, "Activity feed cleared", nil)
}


// FollowActivityStream handles WebSocket connection for real-time activities (optional)
func (h *Handler) FollowActivityStream(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	// WebSocket implementation would go here
	// For now, return a simple message
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Activity stream ready for WebSocket connection",
		"user_id": userID,
	})
}

// GetActivityNotifications handles GET /notifications/activity
func (h *Handler) GetActivityNotifications(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	unreadOnly := c.DefaultQuery("unread_only", "true") == "true"
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

	offset := (page - 1) * limit
	activities, err := h.service.GetUserActivities(userID, limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "success",
		"data":        activities,
		"total":       len(activities),
		"page":        page,
		"limit":       limit,
		"unread_only": unreadOnly,
	})
}
