package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/learna/learna-api/internal/dto/request"
	"github.com/learna/learna-api/internal/dto/response"
	"github.com/learna/learna-api/internal/middleware"
	"github.com/learna/learna-api/internal/services"
	"github.com/learna/learna-api/internal/utils"
)

// AuthHandler serves /api/v1/auth and the self-service /api/v1/me routes.
type AuthHandler struct {
	auth *services.AuthService
}

// Signup registers a learner and returns a session — feature A1.
//
// POST /api/v1/auth/signup
func (h *AuthHandler) Signup(c *gin.Context) {
	var req request.Signup
	if err := utils.BindJSON(c, &req); err != nil {
		utils.Fail(c, err)
		return
	}

	result, err := h.auth.Signup(c.Request.Context(), req, clientInfo(c))
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.Created(c, result)
}

// Login exchanges credentials for a token pair — feature A2.
//
// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req request.Login
	if err := utils.BindJSON(c, &req); err != nil {
		utils.Fail(c, err)
		return
	}

	result, err := h.auth.Login(c.Request.Context(), req, clientInfo(c))
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, result)
}

// Refresh rotates a refresh token into a new pair — feature A3.
//
// POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req request.RefreshToken
	if err := utils.BindJSON(c, &req); err != nil {
		utils.Fail(c, err)
		return
	}

	tokens, err := h.auth.Refresh(c.Request.Context(), req.RefreshToken, clientInfo(c))
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, tokens)
}

// Logout revokes a refresh token — feature A4.
//
// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	var req request.RefreshToken
	if err := utils.BindJSON(c, &req); err != nil {
		utils.Fail(c, err)
		return
	}

	if err := h.auth.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, response.Message{Message: "Signed out."})
}

// ForgotPassword issues a reset token — feature A5.
//
// POST /api/v1/auth/forgot-password
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req request.ForgotPassword
	if err := utils.BindJSON(c, &req); err != nil {
		utils.Fail(c, err)
		return
	}

	result, err := h.auth.ForgotPassword(c.Request.Context(), req)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, result)
}

// ResetPassword consumes a reset token — feature A6.
//
// POST /api/v1/auth/reset-password
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req request.ResetPassword
	if err := utils.BindJSON(c, &req); err != nil {
		utils.Fail(c, err)
		return
	}

	if err := h.auth.ResetPassword(c.Request.Context(), req); err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, response.Message{Message: "Password updated. Please sign in again."})
}

// Me returns the caller's profile — feature P1.
//
// GET /api/v1/me
func (h *AuthHandler) Me(c *gin.Context) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		utils.Fail(c, err)
		return
	}

	user, err := h.auth.Me(c.Request.Context(), userID)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, user)
}

// UpdateMe applies a partial profile update — feature P2.
//
// PATCH /api/v1/me
func (h *AuthHandler) UpdateMe(c *gin.Context) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		utils.Fail(c, err)
		return
	}

	var req request.UpdateProfile
	if err := utils.BindJSON(c, &req); err != nil {
		utils.Fail(c, err)
		return
	}

	user, err := h.auth.UpdateProfile(c.Request.Context(), userID, req)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, user)
}

// ChangePassword updates the caller's password — feature P3.
//
// PATCH /api/v1/me/password
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		utils.Fail(c, err)
		return
	}

	var req request.ChangePassword
	if err := utils.BindJSON(c, &req); err != nil {
		utils.Fail(c, err)
		return
	}

	if err := h.auth.ChangePassword(c.Request.Context(), userID, req); err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, response.Message{Message: "Password updated. Other sessions have been signed out."})
}

// clientInfo captures where a session was created from, for the refresh-token
// record.
func clientInfo(c *gin.Context) services.ClientInfo {
	return services.ClientInfo{
		UserAgent: c.Request.UserAgent(),
		IPAddress: c.ClientIP(),
	}
}
