package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"order-management/apperror"
	"order-management/auth"
	"order-management/model"
)

const actorKey = "actor"

func Authenticate(tokens *auth.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.SplitN(c.GetHeader("Authorization"), " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			fail(c, apperror.Unauthorized("missing or malformed authorization header"))
			return
		}
		actor, err := tokens.Parse(strings.TrimSpace(parts[1]))
		if err != nil {
			fail(c, apperror.Unauthorized("invalid or expired token"))
			return
		}
		c.Set(actorKey, *actor)
		c.Next()
	}
}

func RequireRoles(roles ...model.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := Actor(c)
		if !ok {
			fail(c, apperror.Unauthorized("authentication required"))
			return
		}
		for _, role := range roles {
			if actor.Role == role {
				c.Next()
				return
			}
		}
		fail(c, apperror.Forbidden("insufficient permissions"))
	}
}

func Actor(c *gin.Context) (model.Actor, bool) {
	v, ok := c.Get(actorKey)
	if !ok {
		return model.Actor{}, false
	}
	actor, ok := v.(model.Actor)
	return actor, ok
}
