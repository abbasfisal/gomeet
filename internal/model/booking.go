package model

import "time"

type Booking struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	RoomID    uint      `gorm:"not null;index:idx_booking_room_id" json:"room_id"`
	StartTime time.Time `gorm:"not null;index:idx_booking_start_time" json:"start_time"`
	EndTime   time.Time `gorm:"not null" json:"end_time"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
