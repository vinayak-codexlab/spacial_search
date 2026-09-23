package response

import "github.com/gin-gonic/gin"

type SearchMeta struct {
	Count            int    `json:"count"`
	ResolutionUsed   string `json:"resolution_used"`
	Zoom             int    `json:"zoom"`
	Ring             int    `json:"ring"`
	TargetHexesCount int    `json:"target_hexes_count"`
}

type ListResponse[T any] struct {
	Success bool       `json:"success"`
	Message string     `json:"message"`
	Meta    SearchMeta `json:"meta"`
	Data    []T        `json:"data"`
}

func Error(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{"success": false, "message": message})
}
