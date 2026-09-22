package response

import "github.com/gin-gonic/gin"

type Pagination struct {
	Page       int64 `json:"page"`
	Limit      int64 `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"totalPages"`
}

type ListResponse[T any] struct {
	Success    bool       `json:"success"`
	Message    string     `json:"message"`
	Pagination Pagination `json:"pagination"`
	Data       []T        `json:"data"`
}

func Error(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{"success": false, "message": message})
}
