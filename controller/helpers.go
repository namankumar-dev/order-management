package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"order-management/apperror"
	"order-management/middleware"
	"order-management/model"
	"order-management/validation"
)

func fail(c *gin.Context, err error) {
	_ = c.Error(err)
}

func bindJSON(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		fail(c, validation.Translate(err))
		return false
	}
	return true
}

func bindQuery(c *gin.Context, dst any) bool {
	if err := c.ShouldBindQuery(dst); err != nil {
		fail(c, validation.Translate(err))
		return false
	}
	return true
}

func actorOf(c *gin.Context) (model.Actor, bool) {
	actor, ok := middleware.Actor(c)
	if !ok {
		fail(c, apperror.Unauthorized("authentication required"))
	}
	return actor, ok
}

func parseID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		fail(c, apperror.BadRequest("id must be a positive integer"))
		return 0, false
	}
	return uint(id), true
}
