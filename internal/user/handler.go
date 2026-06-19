package user

import (
	"mangahub/internal/activity"
	"mangahub/internal/auth"
	"mangahub/internal/manga"
	"mangahub/pkg/models"
	"mangahub/pkg/utils"

	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests for user operations.
type Handler struct {
	service         *Service
	activityService *activity.Service
	mangaService    *manga.Service
}

// NewHandler creates a new user handler. activityService and mangaService are
// optional (may be nil) and used to log library activities to the feed.
func NewHandler(service *Service, activityService *activity.Service, mangaService *manga.Service) *Handler {
	return &Handler{service: service, activityService: activityService, mangaService: mangaService}
}

// logLibraryActivity logs a started/completed feed activity when a library
// entry transitions into the "reading" or "completed" state. It is a no-op
// when the status did not actually change (e.g. a plain chapter update), so
// the feed isn't spammed on every progress tick.
func (h *Handler) logLibraryActivity(c *gin.Context, userID, mangaID, oldStatus, newStatus string) {
	if h.activityService == nil || oldStatus == newStatus {
		return
	}
	username, ok := c.Get("username")
	if !ok || username == nil {
		return
	}

	title := mangaID
	if h.mangaService != nil {
		if m, err := h.mangaService.GetByID(mangaID); err == nil && m != nil {
			title = m.Title
		}
	}

	switch newStatus {
	case "reading":
		h.activityService.LogMangaStarted(userID, username.(string), mangaID, title)
	case "completed":
		h.activityService.LogMangaCompleted(userID, username.(string), mangaID, title)
	}
}

// Register handles user registration.
//
// @Summary      Register a new user
// @Description  Create a new account with a unique username and password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      models.RegisterRequest  true  "Registration payload"
// @Success      201   {object}  utils.APIResponse       "User registered successfully"
// @Failure      400   {object}  utils.APIResponse       "Invalid request body"
// @Failure      409   {object}  utils.APIResponse       "Username already taken"
// @Router       /auth/register [post]
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

	// Issue a token so the client is logged in immediately after registering.
	// The response shape ({token, user}) matches the login response.
	token, err := auth.GenerateToken(user.ID, user.Username)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to generate token")
		return
	}

	utils.CreatedResponse(c, "User registered successfully", models.LoginResponse{Token: token, User: *user})
}

// Login handles user authentication.
//
// @Summary      Login
// @Description  Authenticate with username and password, returns a JWT token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      models.LoginRequest   true  "Login payload"
// @Success      200   {object}  utils.APIResponse     "Login successful, token in data"
// @Failure      400   {object}  utils.APIResponse     "Invalid request body"
// @Failure      401   {object}  utils.APIResponse     "Wrong credentials"
// @Router       /auth/login [post]
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
//
// @Summary      Get current user profile
// @Description  Returns the authenticated user's profile information
// @Tags         users
// @Produce      json
// @Success      200  {object}  utils.APIResponse  "Profile data"
// @Failure      401  {object}  utils.APIResponse  "Unauthorized"
// @Failure      404  {object}  utils.APIResponse  "User not found"
// @Security     BearerAuth
// @Router       /users/profile [get]
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

// SearchUsers handles GET /users/search?q=<query> — username autocomplete for
// finding friends. Excludes the requesting user from results.
func (h *Handler) SearchUsers(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unable to identify user")
		return
	}

	users, err := h.service.SearchUsers(c.Query("q"), userID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Search failed")
		return
	}

	utils.SuccessResponse(c, "Users found", users)
}

// AddToLibrary adds a manga to the user's library.
//
// @Summary      Add manga to library
// @Description  Add a manga entry to the authenticated user's reading library
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        body  body      models.AddToLibraryRequest  true  "Library entry"
// @Success      201   {object}  utils.APIResponse            "Manga added"
// @Failure      400   {object}  utils.APIResponse            "Invalid request"
// @Failure      401   {object}  utils.APIResponse            "Unauthorized"
// @Failure      409   {object}  utils.APIResponse            "Already in library"
// @Security     BearerAuth
// @Router       /users/library [post]
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

	// New entry — log started/completed based on the status it was added with.
	h.logLibraryActivity(c, userID, req.MangaID, "", entry.Status)

	utils.CreatedResponse(c, "Manga added to library", entry)
}

// GetLibrary returns the user's library with categorized reading lists.
//
// @Summary      Get user library
// @Description  Returns the user's manga library categorised by status (reading / completed / plan_to_read)
// @Tags         users
// @Produce      json
// @Success      200  {object}  utils.APIResponse  "Library data"
// @Failure      401  {object}  utils.APIResponse  "Unauthorized"
// @Security     BearerAuth
// @Router       /users/library [get]
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
//
// @Summary      Update reading progress
// @Description  Update the current chapter and status for a manga in the user's library. Also triggers a TCP broadcast to connected sync clients.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        body  body      models.UpdateProgressRequest  true  "Progress update"
// @Success      200   {object}  utils.APIResponse              "Progress updated"
// @Failure      400   {object}  utils.APIResponse              "Invalid request"
// @Failure      401   {object}  utils.APIResponse              "Unauthorized"
// @Failure      404   {object}  utils.APIResponse              "Manga not in library"
// @Security     BearerAuth
// @Router       /users/progress [put]
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

	// Capture the previous status so we can detect a real transition below.
	oldStatus := ""
	if prev, _ := h.service.GetProgressEntry(userID, req.MangaID); prev != nil {
		oldStatus = prev.Status
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

	// Log to the feed only when the status actually transitions (e.g. a user
	// moves a manga to reading, or finishes it) — not on every chapter tick.
	h.logLibraryActivity(c, userID, req.MangaID, oldStatus, progress.Status)

	// Store in context for TCP broadcast integration
	c.Set("progress_manga_id", req.MangaID)
	c.Set("progress_chapter", req.CurrentChapter)

	utils.SuccessResponse(c, "Progress updated", progress)
}

// RemoveFromLibrary removes a manga from the user's library.
//
// @Summary      Remove manga from library
// @Description  Delete a manga entry from the authenticated user's library
// @Tags         users
// @Produce      json
// @Param        manga_id  path      string  true  "Manga ID"
// @Success      200       {object}  utils.APIResponse  "Removed successfully"
// @Failure      401       {object}  utils.APIResponse  "Unauthorized"
// @Failure      404       {object}  utils.APIResponse  "Not in library"
// @Security     BearerAuth
// @Router       /users/library/{manga_id} [delete]
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
//
// @Summary      Change password
// @Description  Change the current user's password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      models.ChangePasswordRequest  true  "Password change payload"
// @Success      200   {object}  utils.APIResponse              "Password changed"
// @Failure      400   {object}  utils.APIResponse              "Invalid request"
// @Failure      401   {object}  utils.APIResponse              "Wrong current password"
// @Security     BearerAuth
// @Router       /auth/change-password [put]
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
//
// @Summary      Check auth status
// @Description  Verify whether the current JWT token is valid and return user info
// @Tags         auth
// @Produce      json
// @Success      200  {object}  utils.APIResponse  "Authenticated"
// @Failure      401  {object}  utils.APIResponse  "Unauthorized"
// @Security     BearerAuth
// @Router       /auth/status [get]
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
//
// @Summary      Logout
// @Description  Stateless logout — invalidates nothing server-side. The client must discard the JWT token.
// @Tags         auth
// @Produce      json
// @Success      200  {object}  utils.APIResponse  "Logged out"
// @Security     BearerAuth
// @Router       /auth/logout [post]
func (h *Handler) Logout(c *gin.Context) {
	utils.SuccessResponse(c, "Logged out successfully. Discard your token.", nil)
}

