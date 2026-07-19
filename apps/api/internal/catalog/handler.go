package catalog

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/platform/response"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) CreateCategory(c *gin.Context) {
	var input CreateCategoryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	category, err := h.service.CreateCategory(c.Request.Context(), input)
	if errors.Is(err, ErrSlugTaken) {
		response.Fail(c, http.StatusConflict, "SLUG_TAKEN", "slug already taken")
		return
	}
	if err != nil {
		if isValidationError(err) {
			response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		_ = c.Error(err)
		return
	}
	response.Created(c, category)
}

func (h *Handler) UpdateCategory(c *gin.Context) {
	var input UpdateCategoryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	category, err := h.service.UpdateCategory(c.Request.Context(), c.Param("id"), input)
	if errors.Is(err, ErrCategoryNotFound) {
		response.Fail(c, http.StatusNotFound, "NOT_FOUND", "category not found")
		return
	}
	if errors.Is(err, ErrSlugTaken) {
		response.Fail(c, http.StatusConflict, "SLUG_TAKEN", "slug already taken")
		return
	}
	if err != nil {
		if isValidationError(err) {
			response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		_ = c.Error(err)
		return
	}
	response.OK(c, category)
}

func (h *Handler) GetCategory(c *gin.Context) {
	category, err := h.service.GetCategory(c.Request.Context(), c.Param("id"))
	if errors.Is(err, ErrCategoryNotFound) {
		response.Fail(c, http.StatusNotFound, "NOT_FOUND", "category not found")
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, category)
}

func (h *Handler) ListCategories(c *gin.Context) {
	categories, err := h.service.ListCategories(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, categories)
}

func (h *Handler) DeleteCategory(c *gin.Context) {
	if err := h.service.DeleteCategory(c.Request.Context(), c.Param("id")); errors.Is(err, ErrCategoryNotFound) {
		response.Fail(c, http.StatusNotFound, "NOT_FOUND", "category not found")
		return
	} else if err != nil {
		_ = c.Error(err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) CreateProduct(c *gin.Context) {
	var input CreateProductInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	product, err := h.service.CreateProduct(c.Request.Context(), input)
	if errors.Is(err, ErrSlugTaken) {
		response.Fail(c, http.StatusConflict, "SLUG_TAKEN", "slug already taken")
		return
	}
	if err != nil {
		if isValidationError(err) {
			response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		_ = c.Error(err)
		return
	}
	response.Created(c, product)
}

func (h *Handler) UpdateProduct(c *gin.Context) {
	var input UpdateProductInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	product, err := h.service.UpdateProduct(c.Request.Context(), c.Param("id"), input)
	if errors.Is(err, ErrProductNotFound) {
		response.Fail(c, http.StatusNotFound, "NOT_FOUND", "product not found")
		return
	}
	if errors.Is(err, ErrSlugTaken) {
		response.Fail(c, http.StatusConflict, "SLUG_TAKEN", "slug already taken")
		return
	}
	if err != nil {
		if isValidationError(err) {
			response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		_ = c.Error(err)
		return
	}
	response.OK(c, product)
}

func (h *Handler) GetProduct(c *gin.Context) {
	product, err := h.service.GetProduct(c.Request.Context(), c.Param("id"))
	if errors.Is(err, ErrProductNotFound) {
		response.Fail(c, http.StatusNotFound, "NOT_FOUND", "product not found")
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, product)
}

func (h *Handler) ListProducts(c *gin.Context) {
	filter := ProductListFilter{
		Status:     strings.TrimSpace(c.Query("status")),
		CategoryID: strings.TrimSpace(c.Query("category_id")),
		Query:      strings.TrimSpace(c.Query("q")),
		Limit:      queryInt(c, "limit", 50),
		Offset:     queryInt(c, "offset", 0),
	}
	products, err := h.service.ListProducts(c.Request.Context(), filter)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, products)
}

func (h *Handler) DeleteProduct(c *gin.Context) {
	if err := h.service.DeleteProduct(c.Request.Context(), c.Param("id")); errors.Is(err, ErrProductNotFound) {
		response.Fail(c, http.StatusNotFound, "NOT_FOUND", "product not found")
		return
	} else if err != nil {
		_ = c.Error(err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) CreateVariant(c *gin.Context) {
	var input CreateVariantInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	input.ProductID = c.Param("id")
	variant, err := h.service.CreateVariant(c.Request.Context(), input)

	if errors.Is(err, ErrSKUTaken) {
		response.Fail(c, http.StatusConflict, "SKU_TAKEN", "sku already taken")
		return
	}
	if err != nil {
		if isValidationError(err) {
			response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		_ = c.Error(err)
		return
	}
	response.Created(c, variant)
}

func (h *Handler) UpdateVariant(c *gin.Context) {
	var input UpdateVariantInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	variant, err := h.service.UpdateVariant(c.Request.Context(), c.Param("variant_id"), input)
	if errors.Is(err, ErrVariantNotFound) {

		response.Fail(c, http.StatusNotFound, "NOT_FOUND", "variant not found")
		return
	}
	if errors.Is(err, ErrSKUTaken) {
		response.Fail(c, http.StatusConflict, "SKU_TAKEN", "sku already taken")
		return
	}
	if err != nil {
		if isValidationError(err) {
			response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		_ = c.Error(err)
		return
	}
	response.OK(c, variant)
}

func (h *Handler) GetVariant(c *gin.Context) {
	variant, err := h.service.GetVariant(c.Request.Context(), c.Param("variant_id"))

	if errors.Is(err, ErrVariantNotFound) {
		response.Fail(c, http.StatusNotFound, "NOT_FOUND", "variant not found")
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, variant)
}

func (h *Handler) ListVariantsByProduct(c *gin.Context) {
	variants, err := h.service.ListVariantsByProduct(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, variants)
}

func (h *Handler) DeleteVariant(c *gin.Context) {
	if err := h.service.DeleteVariant(c.Request.Context(), c.Param("variant_id")); errors.Is(err, ErrVariantNotFound) {

		response.Fail(c, http.StatusNotFound, "NOT_FOUND", "variant not found")
		return
	} else if err != nil {
		_ = c.Error(err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) UpdateStock(c *gin.Context) {
	var input UpdateStockInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	variant, err := h.service.UpdateStock(c.Request.Context(), c.Param("id"), input)
	if errors.Is(err, ErrVariantNotFound) {
		response.Fail(c, http.StatusNotFound, "NOT_FOUND", "variant not found")
		return
	}
	if err != nil {
		if isValidationError(err) {
			response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		_ = c.Error(err)
		return
	}
	response.OK(c, variant)
}

func (h *Handler) Search(c *gin.Context) {
	products, err := h.service.SearchActive(c.Request.Context(), c.Query("q"), queryInt(c, "limit", 50), queryInt(c, "offset", 0))
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, products)
}

func (h *Handler) GetByID(c *gin.Context) {
	product, err := h.service.GetActiveByID(c.Request.Context(), c.Param("id"))
	if errors.Is(err, ErrProductNotFound) {
		response.Fail(c, http.StatusNotFound, "NOT_FOUND", "product not found")
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, product)
}

func (h *Handler) GetBySlug(c *gin.Context) {
	product, err := h.service.GetActiveBySlug(c.Request.Context(), c.Param("slug"))
	if errors.Is(err, ErrProductNotFound) {
		response.Fail(c, http.StatusNotFound, "NOT_FOUND", "product not found")
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, product)
}

func queryInt(c *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(c.DefaultQuery(key, strconv.Itoa(fallback)))
	if err != nil || value < 0 {
		return fallback
	}
	return value
}

func isValidationError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "required") ||
		strings.Contains(message, "must be") ||
		strings.Contains(message, "status must be")
}
