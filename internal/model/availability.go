package model

import "time"

type Availability struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	RoomID       uint      `gorm:"not null;index:idx_avail_room_id" json:"room_id"`
	DayOfWeek    *int      `gorm:"type:smallint" json:"day_of_week,omitempty"`
	SpecificDate *string   `gorm:"type:date" json:"specific_date,omitempty"`
	StartTime    string    `gorm:"type:time without time zone;not null" json:"start_time"`
	EndTime      string    `gorm:"type:time without time zone;not null" json:"end_time"`
	CreatedAt    time.Time `json:"created_at"`
}
