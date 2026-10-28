package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"order-management/model"
	"order-management/service"
)

type AuthController struct {
	svc service.AuthService
}

func NewAuthController(svc service.AuthService) *AuthController {
	return &AuthController{svc: svc}
}

func (ac *AuthController) Register(c *gin.Context) {
	var req model.RegisterRequest
	if !bindJSON(c, &req) {
		return
	}
	user, err := ac.svc.Register(c.Request.Context(), req)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, user)
}

func (ac *AuthController) Login(c *gin.Context) {
	var req model.LoginRequest
	if !bindJSON(c, &req) {
		return
	}
	token, err := ac.svc.Login(c.Request.Context(), req)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, token)
}
