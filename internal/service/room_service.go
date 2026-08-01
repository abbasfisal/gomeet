package service

import (
	"context"
	"errors"
	"fmt"
	"meetroom/internal/cache"
	"meetroom/internal/model"
	"meetroom/internal/repository"
	"sort"
	"time"

	"gorm.io/gorm"
)

type RoomService struct {
	roomRepo    *repository.RoomRepository
	availRepo   *repository.AvailabilityRepository
	bookingRepo *repository.BookingRepository
	cache       *cache.Cache
}

func NewRoomService(
	roomRepo *repository.RoomRepository,
	availRepo *repository.AvailabilityRepository,
	bookingRepo *repository.BookingRepository,
	cache *cache.Cache,
) *RoomService {
	return &RoomService{
		roomRepo:    roomRepo,
		availRepo:   availRepo,
		bookingRepo: bookingRepo,
		cache:       cache,
	}
}

func (s *RoomService) Create(ctx context.Context, req CreateRoomRequest) (*model.Room, error) {
	if err := validateAvailabilities(req.Availabilities); err != nil {
		return nil, err
	}

	room := &model.Room{
		Name:        req.Name,
		Capacity:    req.Capacity,
		Location:    req.Location,
		Description: req.Description,
	}

	avails := make([]model.Availability, len(req.Availabilities))
	for i, a := range req.Availabilities {
		avails[i] = model.Availability{
			DayOfWeek:    a.DayOfWeek,
			SpecificDate: a.SpecificDate,
			StartTime:    a.StartTime,
			EndTime:      a.EndTime,
		}
	}

	if err := s.roomRepo.CreateWithAvailabilities(room, avails); err != nil {
		if errors.Is(repository.TranslateDBError(err), repository.ErrConflict) {
			return nil, repository.ErrConflict
		}
		return nil, repository.AsInternal(err)
	}
	room.Availabilities = avails

	s.cache.DeleteByPattern(ctx, cache.CachePrefixRooms)
	s.cache.DeleteByPattern(ctx, cache.CachePrefixRoom)

	return room, nil
}

func (s *RoomService) GetByID(ctx context.Context, id uint) (*model.Room, error) {
	cacheKey := cache.RoomKey(id)
	var room model.Room
	if err := s.cache.Get(ctx, cacheKey, &room); err == nil {
		return &room, nil
	}

	roomPtr, err := s.roomRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, repository.AsInternal(err)
	}

	s.cache.Set(ctx, cacheKey, roomPtr, cache.TTLRoomDetail)
	return roomPtr, nil
}

func (s *RoomService) List(ctx context.Context, page, pageSize int) ([]model.Room, int64, error) {
	cacheKey := cache.RoomListKey(page, pageSize)
	type cachedData struct {
		Rooms []model.Room `json:"rooms"`
		Total int64        `json:"total"`
	}
	var cached cachedData
	if err := s.cache.Get(ctx, cacheKey, &cached); err == nil {
		return cached.Rooms, cached.Total, nil
	}

	rooms, total, err := s.roomRepo.List(page, pageSize)
	if err != nil {
		return nil, 0, repository.AsInternal(err)
	}

	s.cache.Set(ctx, cacheKey, cachedData{Rooms: rooms, Total: total}, cache.TTLRoomList)
	return rooms, total, nil
}

func (s *RoomService) GetAvailableSlots(ctx context.Context, roomID uint, from, to time.Time) ([]TimeSlot, error) {
	cacheKey := cache.RoomAvailabilityKey(roomID, from.Format(time.RFC3339), to.Format(time.RFC3339))
	var slots []TimeSlot
	if err := s.cache.Get(ctx, cacheKey, &slots); err == nil {
		return slots, nil
	}

	_, err := s.roomRepo.FindByID(roomID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, repository.AsInternal(err)
	}

	avails, err := s.availRepo.FindByRoomID(roomID)
	if err != nil {
		return nil, repository.AsInternal(err)
	}

	bookings, _, err := s.bookingRepo.List(&roomID, &from, &to, 1, 10000)
	if err != nil {
		return nil, repository.AsInternal(err)
	}

	slots = calculateFreeSlots(from, to, avails, bookings)

	s.cache.Set(ctx, cacheKey, slots, cache.TTLRoomAvail)
	return slots, nil
}

// calculateFreeSlots subtracts existing bookings from the room's availability
// windows for each day in [from, to]. Windows on the same day are merged first
// (overlapping or back-to-back windows become one range) so gaps between
// separate windows are never reported as free.
func calculateFreeSlots(from, to time.Time, avails []model.Availability, bookings []model.Booking) []TimeSlot {
	var availableWindows []TimeSlot

	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		dayStart := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
		dayEnd := dayStart.AddDate(0, 0, 1)

		var intervals []TimeSlot
		for _, a := range avails {
			if !availabilityMatchesDay(a, d) {
				continue
			}

			startParts, err := parseTime(a.StartTime)
			if err != nil {
				continue
			}
			endParts, err := parseTime(a.EndTime)
			if err != nil {
				continue
			}
			slotStart := time.Date(d.Year(), d.Month(), d.Day(), startParts.h, startParts.m, 0, 0, d.Location())
			slotEnd := time.Date(d.Year(), d.Month(), d.Day(), endParts.h, endParts.m, 0, 0, d.Location())

			if slotEnd.Before(slotStart) || slotEnd.Equal(slotStart) {
				continue
			}
			intervals = append(intervals, TimeSlot{Start: slotStart, End: slotEnd})
		}

		if len(intervals) == 0 {
			continue
		}

		dayBookings := filterBookingsForDay(bookings, dayStart, dayEnd)
		for _, interval := range mergeIntervals(intervals) {
			availableWindows = append(availableWindows, subtractBookings(interval.Start, interval.End, dayBookings)...)
		}
	}

	return availableWindows
}

// isWithinAvailability reports whether [start, end] is fully covered by the
// room's availability. A booking may span multiple adjacent windows on the same
// day (they are merged), but any part of a booking that falls in a gap between
// windows is rejected.
func isWithinAvailability(start, end time.Time, avails []model.Availability) (bool, error) {
	if end.Before(start) || end.Equal(start) {
		return false, nil
	}

	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		dayStart := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
		dayEnd := dayStart.AddDate(0, 0, 1)

		bookingDayStart := maxTime(start, dayStart)
		bookingDayEnd := minTime(end, dayEnd)

		if !bookingDayStart.Before(bookingDayEnd) {
			continue
		}

		var intervals []TimeSlot
		for _, a := range avails {
			if !availabilityMatchesDay(a, d) {
				continue
			}

			startParts, err := parseTime(a.StartTime)
			if err != nil {
				continue
			}
			endParts, err := parseTime(a.EndTime)
			if err != nil {
				continue
			}
			slotStart := time.Date(d.Year(), d.Month(), d.Day(), startParts.h, startParts.m, 0, 0, d.Location())
			slotEnd := time.Date(d.Year(), d.Month(), d.Day(), endParts.h, endParts.m, 0, 0, d.Location())

			if slotEnd.Before(slotStart) || slotEnd.Equal(slotStart) {
				continue
			}
			intervals = append(intervals, TimeSlot{Start: slotStart, End: slotEnd})
		}

		found := false
		for _, interval := range mergeIntervals(intervals) {
			if !bookingDayStart.Before(interval.Start) && !bookingDayEnd.After(interval.End) {
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}
	return true, nil
}

// availabilityMatchesDay reports whether an availability rule applies to day d.
func availabilityMatchesDay(a model.Availability, d time.Time) bool {
	if a.SpecificDate != nil {
		availDate, err := time.Parse("2006-01-02", *a.SpecificDate)
		if err != nil {
			return false
		}
		return isSameDay(d, availDate)
	}
	if a.DayOfWeek != nil {
		return int(d.Weekday()) == *a.DayOfWeek
	}
	return false
}

// mergeIntervals merges overlapping or back-to-back intervals into a minimal
// set of disjoint ranges. Back-to-back intervals (next.Start == last.End) merge
// into one continuous range.
func mergeIntervals(intervals []TimeSlot) []TimeSlot {
	if len(intervals) == 0 {
		return nil
	}
	sort.Slice(intervals, func(i, j int) bool {
		return intervals[i].Start.Before(intervals[j].Start)
	})

	merged := []TimeSlot{intervals[0]}
	for _, iv := range intervals[1:] {
		last := &merged[len(merged)-1]
		if iv.Start.After(last.End) {
			merged = append(merged, iv)
		} else if iv.End.After(last.End) {
			last.End = iv.End
		}
	}
	return merged
}

type TimeSlot struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type timeParts struct {
	h, m int
}

// parseTime parses an "HH:MM" time of day and rejects invalid values instead of
// silently normalizing them (e.g. time.Date would turn "25:99" into 02:39).
func parseTime(s string) (timeParts, error) {
	parsed, err := time.Parse("15:04", s)
	if err != nil {
		return timeParts{}, err
	}
	return timeParts{h: parsed.Hour(), m: parsed.Minute()}, nil
}

func isSameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

func filterBookingsForDay(bookings []model.Booking, dayStart, dayEnd time.Time) []model.Booking {
	var filtered []model.Booking
	for _, b := range bookings {
		if b.StartTime.Before(dayEnd) && b.EndTime.After(dayStart) {
			filtered = append(filtered, b)
		}
	}
	return filtered
}

func subtractBookings(availStart, availEnd time.Time, bookings []model.Booking) []TimeSlot {
	sort.Slice(bookings, func(i, j int) bool {
		return bookings[i].StartTime.Before(bookings[j].StartTime)
	})

	var slots []TimeSlot
	currentStart := availStart

	for _, b := range bookings {
		overlapStart := maxTime(b.StartTime, availStart)
		overlapEnd := minTime(b.EndTime, availEnd)

		if overlapStart.Before(overlapEnd) {
			if currentStart.Before(overlapStart) {
				slots = append(slots, TimeSlot{Start: currentStart, End: overlapStart})
			}
			currentStart = maxTime(currentStart, overlapEnd)
		}
	}

	if currentStart.Before(availEnd) {
		slots = append(slots, TimeSlot{Start: currentStart, End: availEnd})
	}

	return slots
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

type CreateRoomRequest struct {
	Name           string                  `json:"name" binding:"required"`
	Capacity       int                     `json:"capacity" binding:"required"`
	Location       string                  `json:"location"`
	Description    string                  `json:"description"`
	Availabilities []CreateAvailabilityReq `json:"availabilities"`
}

type CreateAvailabilityReq struct {
	DayOfWeek    *int    `json:"day_of_week"`
	SpecificDate *string `json:"specific_date"`
	StartTime    string  `json:"start_time" binding:"required"`
	EndTime      string  `json:"end_time" binding:"required"`
}

func validateAvailabilities(avails []CreateAvailabilityReq) error {
	for i, a := range avails {
		if a.DayOfWeek == nil && a.SpecificDate == nil {
			return fmt.Errorf("each availability must have day_of_week or specific_date")
		}
		if a.DayOfWeek != nil && (*a.DayOfWeek < 0 || *a.DayOfWeek > 6) {
			return fmt.Errorf("day_of_week must be between 0 (Sunday) and 6 (Saturday)")
		}
		if err := validateAvailabilityTimes(a.StartTime, a.EndTime); err != nil {
			return fmt.Errorf("availability %d: %w", i, err)
		}
	}
	return nil
}

func validateAvailabilityTimes(startTime, endTime string) error {
	start, err := time.Parse("15:04", startTime)
	if err != nil {
		return fmt.Errorf("invalid start_time %q (use HH:MM)", startTime)
	}
	end, err := time.Parse("15:04", endTime)
	if err != nil {
		return fmt.Errorf("invalid end_time %q (use HH:MM)", endTime)
	}
	if !end.After(start) {
		return fmt.Errorf("end_time must be after start_time")
	}
	return nil
}
