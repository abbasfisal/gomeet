package model

import "time"

type Room struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	Name           string         `gorm:"size:255;not null;uniqueIndex" json:"name"`
	Capacity       int            `gorm:"not null" json:"capacity"`
	Location       string         `gorm:"size:500" json:"location,omitempty"`
	Description    string         `gorm:"size:1000" json:"description,omitempty"`
	Availabilities []Availability `gorm:"foreignKey:RoomID;constraint:OnDelete:CASCADE" json:"availabilities,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}
