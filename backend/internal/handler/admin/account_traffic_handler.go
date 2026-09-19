package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type AccountTrafficHandler struct {
	admin   service.AdminService
	traffic *service.AccountTrafficService
}

func NewAccountTrafficHandler(admin service.AdminService, traffic *service.AccountTrafficService) *AccountTrafficHandler {
	return &AccountTrafficHandler{admin: admin, traffic: traffic}
}
func (h *AccountTrafficHandler) Get(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	a, err := h.admin.GetAccount(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	policy, err := service.ParseAccountTrafficPolicy(a.Extra)
	if response.ErrorFrom(c, err) {
		return
	}
	state, err := h.traffic.State(c.Request.Context(), a)
	// Configuration remains editable during a temporary telemetry outage.
	if err != nil {
		response.Success(c, gin.H{"policy": policy, "state": nil, "state_available": false, "hard_limit": a.Concurrency})
		return
	}
	response.Success(c, gin.H{"policy": policy, "state": state, "state_available": true, "hard_limit": a.Concurrency})
}
