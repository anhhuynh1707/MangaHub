package manga

import (
	"mangahub/pkg/models"
	"mangahub/pkg/sanitize"
	"mangahub/pkg/utils"

	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests for manga operations.
type Handler struct {
	service *Service
}

// NewHandler creates a new manga handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Search returns a list of manga with search and filters.
//
// @Summary      Search manga
// @Description  Search and filter the manga catalogue. Supports keyword search, genre, status, and pagination.
// @Tags         manga
// @Produce      json
// @Param        search  query     string  false  "Keyword search (title / author)"
// @Param        genre   query     string  false  "Filter by genre (e.g. action, romance)"
// @Param        status  query     string  false  "Filter by status (ongoing / completed)"
// @Param        page    query     int     false  "Page number (default 1)"
// @Param        limit   query     int     false  "Results per page (default 20)"
// @Success      200     {object}  utils.APIResponse  "Manga list with pagination"
// @Failure      400     {object}  utils.APIResponse  "Invalid query parameters"
// @Router       /manga [get]
func (h *Handler) Search(c *gin.Context) {
	var query models.MangaSearchQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		utils.BadRequestResponse(c, "Invalid query parameters: "+err.Error())
		return
	}

	mangaList, total, err := h.service.Search(&query)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to search manga")
		return
	}

	utils.SuccessResponse(c, "Manga search results", gin.H{
		"manga": mangaList,
		"total": total,
		"page":  query.Page,
		"limit": query.Limit,
	})
}

// GetByID returns a single manga by ID.
//
// @Summary      Get manga by ID
// @Description  Retrieve a single manga entry by its slug ID
// @Tags         manga
// @Produce      json
// @Param        id   path      string  true  "Manga ID (slug)"
// @Success      200  {object}  utils.APIResponse  "Manga detail"
// @Failure      404  {object}  utils.APIResponse  "Manga not found"
// @Router       /manga/{id} [get]
func (h *Handler) GetByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.BadRequestResponse(c, "Manga ID is required")
		return
	}

	manga, err := h.service.GetByID(id)
	if err != nil {
		utils.NotFoundResponse(c, err.Error())
		return
	}

	utils.SuccessResponse(c, "Manga retrieved", manga)
}

// AdvancedSearch performs full-text search with multiple filters.
//
// @Summary      Advanced manga search
// @Description  Search manga with multi-genre OR filter, minimum rating, and sort options (popularity/rating/recent/title)
// @Tags         manga
// @Accept       json
// @Produce      json
// @Param        body  body      models.SearchFilters  true  "Search filters"
// @Success      200   {object}  utils.APIResponse     "Filtered manga list"
// @Failure      400   {object}  utils.APIResponse     "Invalid request"
// @Router       /manga/search [post]
func (h *Handler) AdvancedSearch(c *gin.Context) {
	var f models.SearchFilters
	if err := c.ShouldBindJSON(&f); err != nil {
		utils.BadRequestResponse(c, "Invalid request: "+err.Error())
		return
	}

	mangaList, total, err := h.service.SearchByFilters(&f)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Search failed")
		return
	}

	page := f.Page
	if page < 1 {
		page = 1
	}
	limit := f.Limit
	if limit < 1 {
		limit = 20
	}

	utils.SuccessResponse(c, "Advanced search results", gin.H{
		"manga":  mangaList,
		"total":  total,
		"page":   page,
		"limit":  limit,
		"pages":  (total + limit - 1) / limit,
		"filters": f,
	})
}

// Create creates a new manga entry.
//
// @Summary      Create manga
// @Description  Add a new manga to the catalogue (requires authentication)
// @Tags         manga
// @Accept       json
// @Produce      json
// @Param        body  body      models.Manga       true  "Manga data"
// @Success      201   {object}  utils.APIResponse  "Manga created"
// @Failure      400   {object}  utils.APIResponse  "Invalid request or bad input"
// @Failure      401   {object}  utils.APIResponse  "Unauthorized"
// @Failure      409   {object}  utils.APIResponse  "Manga ID already exists"
// @Security     BearerAuth
// @Router       /manga [post]
func (h *Handler) Create(c *gin.Context) {
	var manga models.Manga
	if err := c.ShouldBindJSON(&manga); err != nil {
		utils.BadRequestResponse(c, "Invalid request: "+err.Error())
		return
	}

	if manga.ID == "" || manga.Title == "" {
		utils.BadRequestResponse(c, "Manga ID and title are required")
		return
	}

	cleanID, err := sanitize.ID(manga.ID)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid manga ID: "+err.Error())
		return
	}
	manga.ID = cleanID

	cleanTitle, err := sanitize.Text(manga.Title, sanitize.MaxMangaTitleLen)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid title: "+err.Error())
		return
	}
	manga.Title = cleanTitle

	cleanAuthor, err := sanitize.Text(manga.Author, sanitize.MaxMangaAuthorLen)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid author: "+err.Error())
		return
	}
	manga.Author = cleanAuthor

	cleanDesc, err := sanitize.Text(manga.Description, sanitize.MaxMangaDescLen)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid description: "+err.Error())
		return
	}
	manga.Description = cleanDesc

	if err := h.service.Create(&manga); err != nil {
		if err.Error() == "manga with this ID already exists" {
			utils.ConflictResponse(c, err.Error())
			return
		}
		utils.InternalServerErrorResponse(c, "Failed to create manga: "+err.Error())
		return
	}

	utils.CreatedResponse(c, "Manga created successfully", manga)
}

// Update updates an existing manga.
//
// @Summary      Update manga
// @Description  Update an existing manga entry by ID
// @Tags         manga
// @Accept       json
// @Produce      json
// @Param        id    path      string       true  "Manga ID (slug)"
// @Param        body  body      models.Manga true  "Updated manga fields"
// @Success      200   {object}  utils.APIResponse  "Manga updated"
// @Failure      400   {object}  utils.APIResponse  "Invalid request"
// @Failure      401   {object}  utils.APIResponse  "Unauthorized"
// @Failure      404   {object}  utils.APIResponse  "Manga not found"
// @Security     BearerAuth
// @Router       /manga/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	id := c.Param("id")

	var manga models.Manga
	if err := c.ShouldBindJSON(&manga); err != nil {
		utils.BadRequestResponse(c, "Invalid request: "+err.Error())
		return
	}
	manga.ID = id

	if manga.Title != "" {
		cleanTitle, err := sanitize.Text(manga.Title, sanitize.MaxMangaTitleLen)
		if err != nil {
			utils.BadRequestResponse(c, "Invalid title: "+err.Error())
			return
		}
		manga.Title = cleanTitle
	}

	if manga.Author != "" {
		cleanAuthor, err := sanitize.Text(manga.Author, sanitize.MaxMangaAuthorLen)
		if err != nil {
			utils.BadRequestResponse(c, "Invalid author: "+err.Error())
			return
		}
		manga.Author = cleanAuthor
	}

	if manga.Description != "" {
		cleanDesc, err := sanitize.Text(manga.Description, sanitize.MaxMangaDescLen)
		if err != nil {
			utils.BadRequestResponse(c, "Invalid description: "+err.Error())
			return
		}
		manga.Description = cleanDesc
	}

	if err := h.service.Update(&manga); err != nil {
		utils.NotFoundResponse(c, "Manga not found")
		return
	}

	utils.SuccessResponse(c, "Manga updated successfully", manga)
}

// Delete removes a manga.
//
// @Summary      Delete manga
// @Description  Remove a manga entry from the catalogue
// @Tags         manga
// @Produce      json
// @Param        id   path      string  true  "Manga ID (slug)"
// @Success      200  {object}  utils.APIResponse  "Manga deleted"
// @Failure      401  {object}  utils.APIResponse  "Unauthorized"
// @Failure      404  {object}  utils.APIResponse  "Manga not found"
// @Security     BearerAuth
// @Router       /manga/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.Delete(id); err != nil {
		utils.NotFoundResponse(c, "Manga not found")
		return
	}

	utils.SuccessResponse(c, "Manga deleted successfully", nil)
}
