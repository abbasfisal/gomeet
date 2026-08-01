package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type APIResponse struct {
	Data  interface{} `json:"data"`
	Error *APIError   `json:"error,omitempty"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{Data: data})
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, APIResponse{Data: data})
}

func NotFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, APIResponse{Error: &APIError{Code: "NOT_FOUND", Message: msg}})
}

func BadRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, APIResponse{Error: &APIError{Code: "BAD_REQUEST", Message: msg}})
}

func Conflict(c *gin.Context, msg string) {
	c.JSON(http.StatusConflict, APIResponse{Error: &APIError{Code: "CONFLICT", Message: msg}})
}

func InternalError(c *gin.Context, msg string) {
	c.JSON(http.StatusInternalServerError, APIResponse{Error: &APIError{Code: "INTERNAL_ERROR", Message: msg}})
}
