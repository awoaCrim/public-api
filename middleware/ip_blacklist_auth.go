package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

func checkBlacklistedIPRequest(c *gin.Context, userID int, role int) (bool, error) {
	if userID <= 0 || role >= common.RoleRootUser || !model.IsIPBlacklisted(c.ClientIP()) {
		return false, nil
	}
	return true, model.BanUserByBlacklistedIP(userID, c.ClientIP())
}

func abortBlacklistedIPSessionRequest(c *gin.Context, userID int, role int, openAICompatible bool) bool {
	blocked, err := checkBlacklistedIPRequest(c, userID, role)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"code":    "IP_BLACKLIST_CHECK_FAILED",
			"message": common.TranslateMessage(c, i18n.MsgDatabaseError),
		})
		return true
	}
	if !blocked {
		return false
	}
	if !openAICompatible {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"success": false,
			"code":    "IP_BLACKLISTED",
			"message": model.BlacklistedIPBanMessage,
		})
		return true
	}
	abortWithOpenAiMessage(c, http.StatusForbidden, model.BlacklistedIPBanMessage, types.ErrorCodeAccessDenied)
	return true
}
