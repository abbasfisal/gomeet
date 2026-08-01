package handler

import (
	"errors"
	"log/slog"
	"meetroom/internal/repository"
	"meetroom/internal/service"
	"meetroom/pkg/response"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type BookingHandler struct {
	svc *service.BookingService
}

func NewBookingHandler(svc *service.BookingService) *BookingHandler {
	return &BookingHandler{svc: svc}
}

func (h *BookingHandler) Create(c *gin.Context) {
	var req service.CreateBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	booking, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			response.NotFound(c, "room not found")
		case errors.Is(err, repository.ErrOverlap):
			response.Conflict(c, "booking overlaps with an existing booking")
		case errors.Is(err, repository.ErrInternal):
			slog.Error("create booking failed", "error", err)
			response.InternalError(c, "internal server error")
		default:
			response.BadRequest(c, err.Error())
		}
		return
	}
	response.Created(c, booking)
}

func (h *BookingHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	var roomID *uint
	if rid := c.Query("room_id"); rid != "" {
		id, err := strconv.ParseUint(rid, 10, 64)
		if err != nil {
			response.BadRequest(c, "invalid room_id")
			return
		}
		uid := uint(id)
		roomID = &uid
	}

	var from, to *time.Time
	if f := c.Query("from"); f != "" {
		ft, err := strconv.ParseInt(f, 10, 64)
		if err != nil {
			response.BadRequest(c, "invalid from (use unix timestamp)")
			return
		}
		t := time.Unix(ft, 0).UTC()
		from = &t
	}
	if t := c.Query("to"); t != "" {
		tt, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			response.BadRequest(c, "invalid to (use unix timestamp)")
			return
		}
		tm := time.Unix(tt, 0).UTC()
		to = &tm
	}

	bookings, total, err := h.svc.List(c.Request.Context(), roomID, from, to, page, pageSize)
	if err != nil {
		slog.Error("list bookings failed", "error", err)
		response.InternalError(c, "internal server error")
		return
	}

	response.Success(c, gin.H{
		"bookings":  bookings,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *BookingHandler) Cancel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid booking id")
		return
	}

	if err := h.svc.Cancel(c.Request.Context(), uint(id)); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.NotFound(c, "booking not found")
			return
		}
		slog.Error("cancel booking failed", "error", err)
		response.InternalError(c, "internal server error")
		return
	}

	response.Success(c, gin.H{"deleted": true})
}
