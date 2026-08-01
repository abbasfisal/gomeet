package router

import (
	"meetroom/internal/handler"
	"meetroom/internal/middleware"

	"github.com/gin-gonic/gin"
)

func Setup(
	roomHandler *handler.RoomHandler,
	bookingHandler *handler.BookingHandler,
) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Logger())

	api := r.Group("/api")
	{
		rooms := api.Group("/rooms")
		{
			rooms.POST("", roomHandler.Create)
			rooms.GET("", roomHandler.List)
			rooms.GET("/:id", roomHandler.GetByID)
			rooms.GET("/:id/availability", roomHandler.GetAvailability)
		}

		bookings := api.Group("/bookings")
		{
			bookings.POST("", bookingHandler.Create)
			bookings.GET("", bookingHandler.List)
			bookings.DELETE("/:id", bookingHandler.Cancel)
		}
	}

	return r
}
