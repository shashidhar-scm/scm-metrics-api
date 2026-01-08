package handlers

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
)

var errInvalidPagination = errors.New("invalid pagination parameters")

type paginationParams struct {
	page          int
	pageSize      int
	limit         int
	offset        int
	includeTotals bool
}

type paginationPayload struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

func parsePaginationParams(r *http.Request, defaultPageSize, maxPageSize int) (paginationParams, error) {
	q := r.URL.Query()

	page := 1
	if raw := q.Get("page"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			return paginationParams{}, errInvalidPagination
		}
		page = v
	}

	pageSize := defaultPageSize
	if raw := q.Get("page_size"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			return paginationParams{}, errInvalidPagination
		}
		pageSize = v
	}

	if maxPageSize > 0 && pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	includeTotals := false
	switch strings.ToLower(q.Get("include_totals")) {
	case "1", "true", "yes":
		includeTotals = true
	}

	offset := (page - 1) * pageSize
	return paginationParams{
		page:          page,
		pageSize:      pageSize,
		limit:         pageSize,
		offset:        offset,
		includeTotals: includeTotals,
	}, nil
}

func writePaginatedResponse(w http.ResponseWriter, status int, data interface{}, page, pageSize, total int, includeTotals bool) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	payload := map[string]interface{}{
		"data":       data,
		"pagination": buildPaginationPayload(page, pageSize, total, includeTotals),
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func buildPaginationPayload(page, pageSize, total int, includeTotals bool) paginationPayload {
	totalPages := 0
	if includeTotals && pageSize > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(pageSize)))
	} else if !includeTotals {
		total = -1
		totalPages = -1
	}
	return paginationPayload{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}
}
