package main

import (
	"fmt"
	"net/http/httptest"
	"github.com/gin-gonic/gin"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.POST("/api/v1/me/adverts/:advertId/media", func(c *gin.Context) {
		fmt.Println("FullPath:", c.FullPath())
	})
	req := httptest.NewRequest("POST", "/api/v1/me/adverts/150/media", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
}
