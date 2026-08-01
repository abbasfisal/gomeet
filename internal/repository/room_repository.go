package repository

import (
	"meetroom/internal/model"

	"gorm.io/gorm"
)

type RoomRepository struct {
	db *gorm.DB
}

func NewRoomRepository(db *gorm.DB) *RoomRepository {
	return &RoomRepository{db: db}
}

// CreateWithAvailabilities persists the room and its availability windows in a
// single transaction so a failure on either side leaves no partial state.
func (r *RoomRepository) CreateWithAvailabilities(room *model.Room, avails []model.Availability) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(room).Error; err != nil {
			return err
		}
		if len(avails) > 0 {
			for i := range avails {
				avails[i].RoomID = room.ID
			}
			if err := tx.Create(&avails).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *RoomRepository) FindByID(id uint) (*model.Room, error) {
	var room model.Room
	err := r.db.Preload("Availabilities").First(&room, id).Error
	if err != nil {
		return nil, err
	}
	return &room, nil
}

func (r *RoomRepository) List(page, pageSize int) ([]model.Room, int64, error) {
	var rooms []model.Room
	var total int64

	r.db.Model(&model.Room{}).Count(&total)

	offset := (page - 1) * pageSize
	err := r.db.Preload("Availabilities").Offset(offset).Limit(pageSize).Find(&rooms).Error
	if err != nil {
		return nil, 0, err
	}
	return rooms, total, nil
}
