package repository

import (
	"meetroom/internal/model"

	"gorm.io/gorm"
)

type AvailabilityRepository struct {
	db *gorm.DB
}

func NewAvailabilityRepository(db *gorm.DB) *AvailabilityRepository {
	return &AvailabilityRepository{db: db}
}

func (r *AvailabilityRepository) CreateBatch(avails []model.Availability) error {
	if len(avails) == 0 {
		return nil
	}
	return r.db.Create(&avails).Error
}

func (r *AvailabilityRepository) FindByRoomID(roomID uint) ([]model.Availability, error) {
	var avails []model.Availability
	err := r.db.Where("room_id = ?", roomID).Find(&avails).Error
	return avails, err
}
