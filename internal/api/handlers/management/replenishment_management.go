package management

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/replenishment"
)

type ReplenishmentStatus struct {
	Configured         bool                    `json:"configured"`
	ServiceReady       bool                    `json:"serviceReady"`
	Enabled            bool                    `json:"enabled"`
	ServiceURL         string                  `json:"serviceUrl,omitempty"`
	TargetAccountCount int                     `json:"targetAccountCount"`
	CheckInterval      int                     `json:"checkIntervalSeconds"`
	Pool               replenishment.PoolSnapshot `json:"pool"`
	Deficit            int                     `json:"deficit"`
	CurrentJob         *replenishment.Job      `json:"currentJob,omitempty"`
	FailureCount       int                     `json:"failureCount"`
	NextAttemptAt      *time.Time              `json:"nextAttemptAt,omitempty"`
}

func (h *Handler) GetReplenishmentStatus(c *gin.Context) {
	status, err := h.lookupReplenishmentStatus(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"replenishment": status})
}

func (h *Handler) RunReplenishment(c *gin.Context) {
	if h == nil || h.replenishmentRun == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "replenishment is unavailable"})
		return
	}

	status, err := h.replenishmentRun(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error":         err.Error(),
			"replenishment": status,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":         "ok",
		"replenishment": status,
	})
}

func (h *Handler) lookupReplenishmentStatus(ctx context.Context) (ReplenishmentStatus, error) {
	if h == nil || h.replenishmentStatus == nil {
		return ReplenishmentStatus{}, fmt.Errorf("replenishment is unavailable")
	}
	return h.replenishmentStatus(ctx)
}
