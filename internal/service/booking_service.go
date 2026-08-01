package service

import (
	"context"
	"errors"
	"fmt"
	"meetroom/internal/cache"
	"meetroom/internal/model"
	"meetroom/internal/repository"
	"time"

	"gorm.io/gorm"
)

type BookingService struct {
	bookingRepo *repository.BookingRepository
	roomRepo    *repository.RoomRepository
	availRepo   *repository.AvailabilityRepository
	cache       *cache.Cache
}

func NewBookingService(
	bookingRepo *repository.BookingRepository,
	roomRepo *repository.RoomRepository,
	availRepo *repository.AvailabilityRepository,
	cache *cache.Cache,
) *BookingService {
	return &BookingService{
		bookingRepo: bookingRepo,
		roomRepo:    roomRepo,
		availRepo:   availRepo,
		cache:       cache,
	}
}

type CreateBookingRequest struct {
	RoomID    uint      `json:"room_id" binding:"required"`
	StartTime time.Time `json:"start_time" binding:"required"`
	EndTime   time.Time `json:"end_time" binding:"required"`
}

func (s *BookingService) Create(ctx context.Context, req CreateBookingRequest) (*model.Booking, error) {
	if !req.EndTime.After(req.StartTime) {
		return nil, fmt.Errorf("end_time must be after start_time")
	}

	_, err := s.roomRepo.FindByID(req.RoomID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, repository.AsInternal(err)
	}

	avails, err := s.availRepo.FindByRoomID(req.RoomID)
	if err != nil {
		return nil, repository.AsInternal(err)
	}

	ok, err := isWithinAvailability(req.StartTime, req.EndTime, avails)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("booking is outside room availability")
	}

	booking := &model.Booking{
		RoomID:    req.RoomID,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
	}

	if err := s.bookingRepo.CreateWithOverlapCheck(booking); err != nil {
		if errors.Is(err, repository.ErrOverlap) {
			return nil, repository.ErrOverlap
		}
		return nil, err
	}

	s.cache.DeleteByPattern(ctx, cache.RoomAvailabilityPattern(req.RoomID))

	return booking, nil
}

func (s *BookingService) List(ctx context.Context, roomID *uint, from, to *time.Time, page, pageSize int) ([]model.Booking, int64, error) {
	return s.bookingRepo.List(roomID, from, to, page, pageSize)
}

func (s *BookingService) Cancel(ctx context.Context, id uint) error {
	booking, err := s.bookingRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return repository.ErrNotFound
		}
		return repository.AsInternal(err)
	}

	if err := s.bookingRepo.Delete(id); err != nil {
		return repository.AsInternal(err)
	}

	s.cache.DeleteByPattern(ctx, cache.RoomAvailabilityPattern(booking.RoomID))
	return nil
}
