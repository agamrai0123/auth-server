package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func routes(r *gin.Engine, s *authServer) {
	service := r.Group("auth-server")
	api := service.Group("/v1")
	v1 := api.Group("/oauth")
	v1.POST("/token", s.tokenHandler)
	v1.POST("/validate", s.validateHandler)
	v1.POST("/revoke", s.revokeHandler)
	// v1.POST("/revoke-all", s.revokeAllHandler)
	v1.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
}
