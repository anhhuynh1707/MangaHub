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
// GET /manga
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
// GET /manga/:id
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

// Create creates a new manga entry.
// POST /manga
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
// PUT /manga/:id
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
// DELETE /manga/:id
func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.Delete(id); err != nil {
		utils.NotFoundResponse(c, "Manga not found")
		return
	}

	utils.SuccessResponse(c, "Manga deleted successfully", nil)
}
