package controller

import (
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func GetIPBlacklist(c *gin.Context) {
	entries, err := model.GetIPBlacklist()
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	common.ApiSuccess(c, entries)
}

type AddIPBlacklistRequest struct {
	IP     string `json:"ip" binding:"required"`
	Reason string `json:"reason"`
}

func AddIPBlacklist(c *gin.Context) {
	var request AddIPBlacklistRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	normalized, ok := common.NormalizeIPv4(request.IP)
	if !ok {
		common.ApiError(c, fmt.Errorf("invalid exact IPv4 address: %s", request.IP))
		return
	}
	if err := model.AddIPBlacklist(normalized, request.Reason, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "ip_blacklist.add", map[string]interface{}{"ip": normalized})
	common.ApiSuccess(c, nil)
}

func RemoveIPBlacklist(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if err := model.RemoveIPBlacklist(id); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "ip_blacklist.remove", map[string]interface{}{"id": id})
	common.ApiSuccess(c, nil)
}
