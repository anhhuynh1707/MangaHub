package sharedlist

import (
	"strconv"

	"mangahub/internal/activity"
	"mangahub/internal/auth"
	"mangahub/pkg/models"
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

// CreateList handles POST /reading-lists/create
func (h *Handler) CreateList(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	var req struct {
		Title       string   `json:"title"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		MangaList   []string `json:"manga_list"`
		MangaIDs    []string `json:"manga_ids"`
		IsPublic    bool     `json:"is_public"`
		SharedWith  []string `json:"shared_with"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request body")
		return
	}

	title := req.Title
	if title == "" {
		title = req.Name
	}
	if title == "" {
		utils.BadRequestResponse(c, "title or name is required")
		return
	}

	mangaList := req.MangaList
	if len(mangaList) == 0 {
		mangaList = req.MangaIDs
	}
	if len(mangaList) == 0 {
		utils.BadRequestResponse(c, "manga_list or manga_ids is required")
		return
	}

	createdList, err := h.service.CreateList(userID, title, req.Description, req.IsPublic, mangaList, req.SharedWith)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	username, _ := c.Get("username")
	if h.activityService != nil && username != nil {
		h.activityService.LogSharedListCreated(userID, username.(string), title)
	}

	utils.SuccessResponse(c, "Reading list created successfully", createdList)
}

// GetMyLists handles GET /reading-lists/mine
func (h *Handler) GetMyLists(c *gin.Context) {
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

	lists, err := h.service.GetListsByOwner(userID)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	// Manual pagination
	total := len(lists)
	offset := (page - 1) * limit
	end := offset + limit
	if end > total {
		end = total
	}

	pagedLists := lists
	if offset < total {
		pagedLists = lists[offset:end]
	} else {
		pagedLists = []models.SharedReadingList{}
	}

	var formattedLists []map[string]interface{}
	for _, list := range pagedLists {
		formattedLists = append(formattedLists, map[string]interface{}{
			"id":          list.ID,
			"owner_id":    list.OwnerID,
			"owner_name":  list.OwnerName,
			"title":       list.Title,
			"name":        list.Title,
			"description": list.Description,
			"is_public":   list.IsPublic,
			"manga_list":  list.MangaList,
			"manga_ids":   list.MangaList,
			"shared_with": list.SharedWith,
			"created_at":  list.CreatedAt,
			"updated_at":  list.UpdatedAt,
		})
	}

	utils.SuccessResponse(c, "My reading lists retrieved", gin.H{
		"lists": formattedLists,
		"total": total,
		"page":  page,
		"limit": limit,
		"pages": (total + limit - 1) / limit,
	})
}

// GetPublicLists handles GET /reading-lists/public
func (h *Handler) GetPublicLists(c *gin.Context) {
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
	lists, total, err := h.service.GetPublicLists(limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	var formattedLists []map[string]interface{}
	for _, list := range lists {
		formattedLists = append(formattedLists, map[string]interface{}{
			"id":          list.ID,
			"owner_id":    list.OwnerID,
			"owner_name":  list.OwnerName,
			"title":       list.Title,
			"name":        list.Title,
			"description": list.Description,
			"is_public":   list.IsPublic,
			"manga_list":  list.MangaList,
			"manga_ids":   list.MangaList,
			"shared_with": list.SharedWith,
			"created_at":  list.CreatedAt,
			"updated_at":  list.UpdatedAt,
		})
	}

	utils.SuccessResponse(c, "Public reading lists retrieved", gin.H{
		"lists": formattedLists,
		"total": total,
		"page":  page,
		"limit": limit,
		"pages": (total + limit - 1) / limit,
	})
}

// GetList handles GET /reading-lists/:list_id
func (h *Handler) GetList(c *gin.Context) {
	listID := c.Param("list_id")

	list, err := h.service.GetListByID(listID)
	if err != nil {
		utils.NotFoundResponse(c, "Reading list not found")
		return
	}

	// Check if list is public or user has access
	userID, _ := auth.GetUserIDFromContext(c)
	if !list.IsPublic && list.OwnerID != userID {
		// Check if user has been granted access
		hasAccess, _ := h.service.CanAccessList(userID, listID)
		if !hasAccess {
			utils.ForbiddenResponse(c, "You don't have access to this list")
			return
		}
	}

	utils.SuccessResponse(c, "Reading list retrieved", list)
}

// UpdateList handles PUT /reading-lists/:list_id
func (h *Handler) UpdateList(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	listID := c.Param("list_id")

	var req struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		MangaList   []string `json:"manga_list"`
		IsPublic    bool     `json:"is_public"`
		SharedWith  []string `json:"shared_with"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request body")
		return
	}

	// Verify ownership
	list, err := h.service.GetListByID(listID)
	if err != nil || list.OwnerID != userID {
		utils.ForbiddenResponse(c, "You can only edit your own lists")
		return
	}

	// Update list using service method
	err = h.service.UpdateList(userID, listID, req.Title, req.Description, req.IsPublic, req.MangaList, req.SharedWith)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	// Fetch updated list
	updatedList, _ := h.service.GetListByID(listID)

	utils.SuccessResponse(c, "Reading list updated successfully", updatedList)
}

// DeleteList handles DELETE /reading-lists/:list_id
func (h *Handler) DeleteList(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	listID := c.Param("list_id")

	// Verify ownership
	list, err := h.service.GetListByID(listID)
	if err != nil || list.OwnerID != userID {
		utils.ForbiddenResponse(c, "You can only delete your own lists")
		return
	}

	err = h.service.DeleteList(userID, listID)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	utils.SuccessResponse(c, "Reading list deleted successfully", nil)
}

// SubscribeToList handles POST /reading-lists/:list_id/subscribe
func (h *Handler) SubscribeToList(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	listID := c.Param("list_id")

	// Get the list
	list, err := h.service.GetListByID(listID)
	if err != nil {
		utils.NotFoundResponse(c, "Reading list not found")
		return
	}

	// Cannot subscribe to your own list
	if list.OwnerID == userID {
		utils.BadRequestResponse(c, "You cannot subscribe to your own list")
		return
	}

	// Check if already subscribed
	for _, sub := range list.SharedWith {
		if sub == userID {
			utils.BadRequestResponse(c, "You are already subscribed to this list")
			return
		}
	}

	// Add user to shared_with (subscribers)
	list.SharedWith = append(list.SharedWith, userID)
	err = h.service.UpdateList(list.OwnerID, listID, list.Title, list.Description, list.IsPublic, list.MangaList, list.SharedWith)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to subscribe: "+err.Error())
		return
	}

	// Log activity
	username, _ := c.Get("username")
	if h.activityService != nil && username != nil {
		h.activityService.LogSharedListCreated(userID, username.(string), "Subscribed to: "+list.Title)
	}

	utils.SuccessResponse(c, "Subscribed to reading list successfully", gin.H{
		"list_id": listID,
		"title":   list.Title,
	})
}

// UnsubscribeFromList handles DELETE /reading-lists/:list_id/subscribe
func (h *Handler) UnsubscribeFromList(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	listID := c.Param("list_id")

	// Get the list
	list, err := h.service.GetListByID(listID)
	if err != nil {
		utils.NotFoundResponse(c, "Reading list not found")
		return
	}

	// Find and remove user from shared_with
	found := false
	newSharedWith := make([]string, 0, len(list.SharedWith))
	for _, sub := range list.SharedWith {
		if sub == userID {
			found = true
		} else {
			newSharedWith = append(newSharedWith, sub)
		}
	}

	if !found {
		utils.BadRequestResponse(c, "You are not subscribed to this list")
		return
	}

	err = h.service.UpdateList(list.OwnerID, listID, list.Title, list.Description, list.IsPublic, list.MangaList, newSharedWith)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to unsubscribe: "+err.Error())
		return
	}

	utils.SuccessResponse(c, "Unsubscribed from reading list successfully", gin.H{
		"list_id": listID,
		"title":   list.Title,
	})
}

// GetSubscribedLists handles GET /reading-lists/subscribed
func (h *Handler) GetSubscribedLists(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	// Get all public lists + lists shared with this user
	allLists, _, err := h.service.GetPublicLists(1000, 0)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to fetch lists: "+err.Error())
		return
	}

	// Filter to only lists where user is in shared_with (subscribed)
	var subscribedLists []map[string]interface{}
	for _, list := range allLists {
		for _, sub := range list.SharedWith {
			if sub == userID {
				subscribedLists = append(subscribedLists, map[string]interface{}{
					"id":          list.ID,
					"owner_id":    list.OwnerID,
					"owner_name":  list.OwnerName,
					"title":       list.Title,
					"description": list.Description,
					"is_public":   list.IsPublic,
					"manga_list":  list.MangaList,
					"manga_ids":   list.MangaList,
					"created_at":  list.CreatedAt,
					"updated_at":  list.UpdatedAt,
				})
				break
			}
		}
	}

	utils.SuccessResponse(c, "Subscribed reading lists retrieved", gin.H{
		"lists": subscribedLists,
		"total": len(subscribedLists),
	})
}

// AddMangaToList handles POST /reading-lists/:list_id/manga
func (h *Handler) AddMangaToList(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	listID := c.Param("list_id")

	var req struct {
		MangaID string `json:"manga_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request body: manga_id is required")
		return
	}

	// Verify ownership
	list, err := h.service.GetListByID(listID)
	if err != nil {
		utils.NotFoundResponse(c, "Reading list not found")
		return
	}
	if list.OwnerID != userID {
		utils.ForbiddenResponse(c, "You can only add manga to your own lists")
		return
	}

	// Check if manga already exists in the list
	for _, m := range list.MangaList {
		if m == req.MangaID {
			utils.BadRequestResponse(c, "Manga is already in this list")
			return
		}
	}

	// Add manga to list
	list.MangaList = append(list.MangaList, req.MangaID)
	err = h.service.UpdateList(userID, listID, list.Title, list.Description, list.IsPublic, list.MangaList, list.SharedWith)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to add manga: "+err.Error())
		return
	}

	utils.SuccessResponse(c, "Manga added to reading list successfully", gin.H{
		"list_id":    listID,
		"manga_id":   req.MangaID,
		"manga_list": list.MangaList,
	})
}

// RemoveMangaFromList handles DELETE /reading-lists/:list_id/manga/:manga_id
func (h *Handler) RemoveMangaFromList(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	listID := c.Param("list_id")
	mangaID := c.Param("manga_id")

	// Verify ownership
	list, err := h.service.GetListByID(listID)
	if err != nil {
		utils.NotFoundResponse(c, "Reading list not found")
		return
	}
	if list.OwnerID != userID {
		utils.ForbiddenResponse(c, "You can only remove manga from your own lists")
		return
	}

	// Find and remove the manga
	found := false
	newMangaList := make([]string, 0, len(list.MangaList))
	for _, m := range list.MangaList {
		if m == mangaID {
			found = true
		} else {
			newMangaList = append(newMangaList, m)
		}
	}

	if !found {
		utils.NotFoundResponse(c, "Manga not found in this list")
		return
	}

	err = h.service.UpdateList(userID, listID, list.Title, list.Description, list.IsPublic, newMangaList, list.SharedWith)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to remove manga: "+err.Error())
		return
	}

	utils.SuccessResponse(c, "Manga removed from reading list successfully", gin.H{
		"list_id":    listID,
		"manga_id":   mangaID,
		"manga_list": newMangaList,
	})
}
