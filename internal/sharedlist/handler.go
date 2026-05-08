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
	utils.BadRequestResponse(c, "Feature not yet implemented")
}

// UnsubscribeFromList handles DELETE /reading-lists/:list_id/subscribe
func (h *Handler) UnsubscribeFromList(c *gin.Context) {
	utils.BadRequestResponse(c, "Feature not yet implemented")
}

// GetSubscribedLists handles GET /reading-lists/subscribed
func (h *Handler) GetSubscribedLists(c *gin.Context) {
	utils.BadRequestResponse(c, "Feature not yet implemented")
}

// AddMangaToList handles POST /reading-lists/:list_id/manga
func (h *Handler) AddMangaToList(c *gin.Context) {
	utils.BadRequestResponse(c, "Feature not yet implemented")
}

// RemoveMangaFromList handles DELETE /reading-lists/:list_id/manga/:manga_id
func (h *Handler) RemoveMangaFromList(c *gin.Context) {
	utils.BadRequestResponse(c, "Feature not yet implemented")
}
