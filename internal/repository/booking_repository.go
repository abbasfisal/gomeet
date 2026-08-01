package repository

import (
	"meetroom/internal/model"
	"time"

	"gorm.io/gorm"
)

type BookingRepository struct {
	db *gorm.DB
}

func NewBookingRepository(db *gorm.DB) *BookingRepository {
	return &BookingRepository{db: db}
}

func (r *BookingRepository) FindByID(id uint) (*model.Booking, error) {
	var booking model.Booking
	err := r.db.First(&booking, id).Error
	if err != nil {
		return nil, err
	}
	return &booking, nil
}

func (r *BookingRepository) List(roomID *uint, from, to *time.Time, page, pageSize int) ([]model.Booking, int64, error) {
	var bookings []model.Booking
	var total int64

	query := r.db.Model(&model.Booking{})
	if roomID != nil {
		query = query.Where("room_id = ?", *roomID)
	}
	if from != nil {
		query = query.Where("end_time > ?", *from)
	}
	if to != nil {
		query = query.Where("start_time < ?", *to)
	}

	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("start_time asc").Find(&bookings).Error
	if err != nil {
		return nil, 0, err
	}
	return bookings, total, nil
}

// CreateWithOverlapCheck inserts a booking atomically. The in-transaction count
// check provides fast rejection of obvious overlaps; under a concurrent race
// the PostgreSQL exclusion constraint (no_overlap_bookings) is the final
// authority, and its violation is translated to ErrOverlap.
func (r *BookingRepository) CreateWithOverlapCheck(booking *model.Booking) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		err := tx.Model(&model.Booking{}).
			Where("room_id = ?", booking.RoomID).
			Where("start_time < ?", booking.EndTime).
			Where("end_time > ?", booking.StartTime).
			Count(&count).Error
		if err != nil {
			return AsInternal(err)
		}
		if count > 0 {
			return ErrOverlap
		}
		return TranslateDBError(tx.Create(booking).Error)
	})
}

func (r *BookingRepository) Delete(id uint) error {
	return r.db.Delete(&model.Booking{}, id).Error
}
