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

type RoomHandler struct {
	svc *service.RoomService
}

func NewRoomHandler(svc *service.RoomService) *RoomHandler {
	return &RoomHandler{svc: svc}
}

func (h *RoomHandler) Create(c *gin.Context) {
	var req service.CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	room, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrConflict):
			response.Conflict(c, "room name already exists")
		case errors.Is(err, repository.ErrInternal):
			slog.Error("create room failed", "error", err)
			response.InternalError(c, "internal server error")
		default:
			response.BadRequest(c, err.Error())
		}
		return
	}
	response.Created(c, room)
}

func (h *RoomHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid room id")
		return
	}

	room, err := h.svc.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.NotFound(c, "room not found")
			return
		}
		slog.Error("get room failed", "error", err)
		response.InternalError(c, "internal server error")
		return
	}
	response.Success(c, room)
}

func (h *RoomHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	rooms, total, err := h.svc.List(c.Request.Context(), page, pageSize)
	if err != nil {
		slog.Error("list rooms failed", "error", err)
		response.InternalError(c, "internal server error")
		return
	}

	response.Success(c, gin.H{
		"rooms":     rooms,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *RoomHandler) GetAvailability(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid room id")
		return
	}

	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		response.BadRequest(c, "from and to query parameters are required")
		return
	}

	from, err := strconv.ParseInt(fromStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid from (use unix timestamp)")
		return
	}
	to, err := strconv.ParseInt(toStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid to (use unix timestamp)")
		return
	}

	fromTime := time.Unix(from, 0).UTC()
	toTime := time.Unix(to, 0).UTC()

	if toTime.Before(fromTime) || toTime.Equal(fromTime) {
		response.BadRequest(c, "to must be after from")
		return
	}

	slots, err := h.svc.GetAvailableSlots(c.Request.Context(), uint(id), fromTime, toTime)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.NotFound(c, "room not found")
			return
		}
		slog.Error("get available slots failed", "error", err)
		response.InternalError(c, "internal server error")
		return
	}

	response.Success(c, gin.H{"available_slots": slots})
}
