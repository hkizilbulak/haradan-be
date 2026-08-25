package catalog

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

var (
	dynamicMu   sync.RWMutex
	dynamicData json.RawMessage
)

// RegisterDynamicRoutes registers the dynamic catalog synchronization routes.
func (h *Handler) RegisterDynamicRoutes(rg gin.IRouter) {
	rg.GET("/v1/catalog/dynamic", func(c *gin.Context) {
		dynamicMu.RLock()
		defer dynamicMu.RUnlock()
		if len(dynamicData) == 0 {
			c.JSON(http.StatusOK, gin.H{"categories": []any{}, "categoryProperties": []any{}})
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", dynamicData)
	})

	rg.POST("/v1/catalog/dynamic", func(c *gin.Context) {
		raw, err := c.GetRawData()
		if err != nil || len(raw) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid catalog data"})
			return
		}
		dynamicMu.Lock()
		dynamicData = make([]byte, len(raw))
		copy(dynamicData, raw)
		dynamicMu.Unlock()

		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	rg.PUT("/v1/catalog/dynamic", func(c *gin.Context) {
		raw, err := c.GetRawData()
		if err != nil || len(raw) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid catalog data"})
			return
		}
		dynamicMu.Lock()
		dynamicData = make([]byte, len(raw))
		copy(dynamicData, raw)
		dynamicMu.Unlock()

		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}
