package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

var errInvalidPagination = errors.New("invalid pagination parameters")

type paginationParams struct {
	page     int
	pageSize int
	limit    int
	offset   int
}

type paginationPayload struct {
	Page     int  `json:"page"`
	PageSize int  `json:"page_size"`
	HasMore  bool `json:"has_more"`
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

	offset := (page - 1) * pageSize
	return paginationParams{
		page:     page,
		pageSize: pageSize,
		limit:    pageSize,
		offset:   offset,
	}, nil
}

func writePaginatedResponse(w http.ResponseWriter, status int, data interface{}, page, pageSize int, hasMore bool) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	payload := map[string]interface{}{
		"data":       data,
		"pagination": buildPaginationPayload(page, pageSize, hasMore),
	}
	_ = json.NewEncoder(w).Encode(payload)
}


func buildPaginationPayload(page, pageSize int, hasMore bool) paginationPayload {
	return paginationPayload{
		Page:     page,
		PageSize: pageSize,
		HasMore:  hasMore,
	}
}
