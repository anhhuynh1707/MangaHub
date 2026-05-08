package review

import (
	"strconv"

	"mangahub/internal/activity"
	"mangahub/internal/auth"
	"mangahub/internal/manga"
	"mangahub/pkg/utils"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service         *Service
	activityService *activity.Service
	mangaService    *manga.Service
}

func NewHandler(service *Service, activityService *activity.Service, mangaService *manga.Service) *Handler {
	return &Handler{service: service, activityService: activityService, mangaService: mangaService}
}

// CreateReview handles POST /manga/:id/reviews
func (h *Handler) CreateReview(c *gin.Context) {
	var req struct {
		Rating int    `json:"rating" binding:"required"`
		Text   string `json:"text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request body")
		return
	}

	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	mangaID := c.Param("id")

	// Validate rating
	if req.Rating < 1 || req.Rating > 10 {
		utils.BadRequestResponse(c, "Rating must be between 1 and 10")
		return
	}

	review, err := h.service.CreateReview(userID, mangaID, req.Rating, req.Text)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	username, _ := c.Get("username")
	mangaTitle := mangaID
	if h.mangaService != nil {
		if m, err := h.mangaService.GetByID(mangaID); err == nil && m != nil {
			mangaTitle = m.Title
		}
	}
	if h.activityService != nil && username != nil {
		h.activityService.LogReviewWritten(userID, username.(string), mangaID, mangaTitle, review.ID, req.Rating)
	}

	utils.SuccessResponse(c, "Review created successfully", review)
}

// GetReviews handles GET /manga/:id/reviews
func (h *Handler) GetReviews(c *gin.Context) {
	mangaID := c.Param("id")
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	offset := (page - 1) * limit
	reviews, total, err := h.service.GetReviewsByManga(mangaID, limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	stats, _ := h.service.GetMangaStats(mangaID)
	var avgRating float64
	if stats != nil {
		if val, ok := stats["avg_rating"].(float64); ok {
			avgRating = val
		}
	}

	utils.SuccessResponse(c, "Reviews retrieved successfully", gin.H{
		"reviews":        reviews,
		"total":          total,
		"page":           page,
		"limit":          limit,
		"pages":          (total + limit - 1) / limit,
		"average_rating": avgRating,
	})
}

// GetReview handles GET /reviews/:review_id
func (h *Handler) GetReview(c *gin.Context) {
	reviewID := c.Param("review_id")

	review, err := h.service.GetReviewByID(reviewID)
	if err != nil {
		utils.NotFoundResponse(c, "Review not found")
		return
	}

	utils.SuccessResponse(c, "Review retrieved", review)
}

// UpdateReview handles PUT /reviews/:review_id
func (h *Handler) UpdateReview(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	var req struct {
		Rating int    `json:"rating"`
		Text   string `json:"text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request body")
		return
	}

	reviewID := c.Param("review_id")

	// Verify ownership
	review, err := h.service.GetReviewByID(reviewID)
	if err != nil || review.UserID != userID {
		utils.ForbiddenResponse(c, "You can only edit your own reviews")
		return
	}

	// Validate rating
	if req.Rating < 1 || req.Rating > 10 {
		utils.BadRequestResponse(c, "Rating must be between 1 and 10")
		return
	}

	// Update review
	err = h.service.UpdateReview(userID, reviewID, &req.Rating, &req.Text)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to update review")
		return
	}

	utils.SuccessResponse(c, "Review updated successfully", gin.H{
		"review_id": reviewID,
		"rating":    req.Rating,
		"text":      req.Text,
	})
}

// DeleteReview handles DELETE /reviews/:review_id
func (h *Handler) DeleteReview(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	reviewID := c.Param("review_id")

	// Verify ownership
	review, err := h.service.GetReviewByID(reviewID)
	if err != nil || review.UserID != userID {
		utils.ForbiddenResponse(c, "You can only delete your own reviews")
		return
	}

	err = h.service.DeleteReview(userID, reviewID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to delete review")
		return
	}

	utils.SuccessResponse(c, "Review deleted successfully", nil)
}

// MarkHelpful handles POST /reviews/:review_id/helpful
func (h *Handler) MarkHelpful(c *gin.Context) {
	reviewID := c.Param("review_id")

	err := h.service.MarkHelpful(reviewID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to mark as helpful")
		return
	}

	review, _ := h.service.GetReviewByID(reviewID)
	utils.SuccessResponse(c, "Review marked as helpful", gin.H{"helpful": review.Helpful})
}

// GetRatingStats handles GET /manga/:id/rating-stats
func (h *Handler) GetRatingStats(c *gin.Context) {
	mangaID := c.Param("id")

	// Calculate stats from service
	stats, err := h.service.GetMangaStats(mangaID)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	utils.SuccessResponse(c, "Rating stats retrieved", stats)
}

// GetMyReviews handles GET /users/reviews
func (h *Handler) GetMyReviews(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	offset := (page - 1) * limit
	reviews, total, err := h.service.GetReviewsByUser(userID, limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	utils.SuccessResponse(c, "User reviews retrieved successfully", gin.H{
		"reviews": reviews,
		"total":   total,
		"page":    page,
		"limit":   limit,
		"pages":   (total + limit - 1) / limit,
	})
}
