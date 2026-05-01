package user

import (
	"mangahub/internal/auth"
	"mangahub/pkg/models"
	"mangahub/pkg/utils"

	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests for user operations.
type Handler struct {
	service *Service
}

// NewHandler creates a new user handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Register handles user registration.
// POST /auth/register
func (h *Handler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request: "+err.Error())
		return
	}

	user, err := h.service.Register(&req)
	if err != nil {
		if err.Error() == "username already taken" {
			utils.ConflictResponse(c, err.Error())
			return
		}
		utils.InternalServerErrorResponse(c, "Registration failed: "+err.Error())
		return
	}

	utils.CreatedResponse(c, "User registered successfully", user)
}

// Login handles user authentication.
// POST /auth/login
func (h *Handler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request: "+err.Error())
		return
	}

	resp, err := h.service.Login(&req)
	if err != nil {
		utils.UnauthorizedResponse(c, err.Error())
		return
	}

	utils.SuccessResponse(c, "Login successful", resp)
}

// GetProfile returns the authenticated user's profile.
// GET /users/profile
func (h *Handler) GetProfile(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unable to identify user")
		return
	}

	user, err := h.service.GetProfile(userID)
	if err != nil {
		utils.NotFoundResponse(c, err.Error())
		return
	}

	utils.SuccessResponse(c, "Profile retrieved", user)
}

// AddToLibrary adds a manga to the user's library.
// POST /users/library
func (h *Handler) AddToLibrary(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unable to identify user")
		return
	}

	var req models.AddToLibraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request: "+err.Error())
		return
	}

	entry, err := h.service.AddToLibrary(userID, &req)
	if err != nil {
		if err.Error() == "manga already in library" {
			utils.ConflictResponse(c, err.Error())
			return
		}
		utils.InternalServerErrorResponse(c, "Failed to add to library: "+err.Error())
		return
	}

	utils.CreatedResponse(c, "Manga added to library", entry)
}

// GetLibrary returns the user's library with categorized reading lists.
// GET /users/library
func (h *Handler) GetLibrary(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unable to identify user")
		return
	}

	userData, err := h.service.GetLibrary(userID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to retrieve library")
		return
	}

	utils.SuccessResponse(c, "Library retrieved", userData)
}

// UpdateProgress updates reading progress for a manga.
// PUT /users/progress
func (h *Handler) UpdateProgress(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unable to identify user")
		return
	}

	var req models.UpdateProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request: "+err.Error())
		return
	}

	progress, err := h.service.UpdateProgress(userID, &req)
	if err != nil {
		if err.Error() == "manga not found in library" {
			utils.NotFoundResponse(c, err.Error())
			return
		}
		utils.InternalServerErrorResponse(c, "Failed to update progress")
		return
	}

	// Store in context for TCP broadcast integration
	c.Set("progress_manga_id", req.MangaID)
	c.Set("progress_chapter", req.CurrentChapter)

	utils.SuccessResponse(c, "Progress updated", progress)
}

// RemoveFromLibrary removes a manga from the user's library.
// DELETE /users/library/:manga_id
func (h *Handler) RemoveFromLibrary(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unable to identify user")
		return
	}

	mangaID := c.Param("manga_id")
	if mangaID == "" {
		utils.BadRequestResponse(c, "Manga ID is required")
		return
	}

	if err := h.service.RemoveFromLibrary(userID, mangaID); err != nil {
		utils.NotFoundResponse(c, "Manga not found in library")
		return
	}

	utils.SuccessResponse(c, "Manga removed from library", nil)
}

// ChangePassword handles password changes for authenticated users.
// PUT /auth/change-password
func (h *Handler) ChangePassword(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unable to identify user")
		return
	}

	var req models.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request: "+err.Error())
		return
	}

	if err := h.service.ChangePassword(userID, &req); err != nil {
		if err.Error() == "incorrect current password" {
			utils.UnauthorizedResponse(c, err.Error())
			return
		}
		utils.InternalServerErrorResponse(c, "Failed to change password")
		return
	}

	utils.SuccessResponse(c, "Password changed successfully", nil)
}

// AuthStatus returns the current authentication status.
// GET /auth/status
func (h *Handler) AuthStatus(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Not authenticated")
		return
	}

	username, _ := c.Get("username")

	user, err := h.service.GetProfile(userID)
	if err != nil {
		utils.UnauthorizedResponse(c, "User not found")
		return
	}

	utils.SuccessResponse(c, "Authenticated", gin.H{
		"user_id":    userID,
		"username":   username,
		"created_at": user.CreatedAt,
	})
}

// Logout is a no-op for stateless JWT auth.
// The client should discard the token.
// POST /auth/logout
func (h *Handler) Logout(c *gin.Context) {
	utils.SuccessResponse(c, "Logged out successfully. Discard your token.", nil)
}

