package management

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
)

func (h *Handler) GetOverview(c *gin.Context) {
	c.JSON(http.StatusOK, h.buildOverviewPayload(c.Request.Context()))
}

func (h *Handler) buildOverviewPayload(ctx context.Context) gin.H {
	payload := gin.H{
		"accounts":        h.listCodexAccountEntries(),
		"usage":           usage.StatisticsSnapshot{},
		"failed_requests": 0,
		"replenishment":   nil,
	}

	if h != nil && h.usageStats != nil {
		snapshot := h.usageStats.Snapshot()
		payload["usage"] = snapshot
		payload["failed_requests"] = snapshot.FailureCount
	}

	if status, err := h.lookupReplenishmentStatus(ctx); err == nil {
		payload["replenishment"] = status
	} else {
		payload["replenishment_error"] = err.Error()
	}

	return payload
}
