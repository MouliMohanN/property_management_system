package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/MouliMohanN/property_management_system/be/services/core/internal/domain/user"
	"github.com/MouliMohanN/property_management_system/be/services/core/internal/transport/http/middleware"
	"github.com/MouliMohanN/property_management_system/be/services/core/internal/usecase/auth"
)

// validate is a package-level validator. It is safe for concurrent use.
var validate = validator.New()

// AuthHandler groups all authentication-related HTTP handlers.
type AuthHandler struct {
	registerUC *auth.RegisterUseCase
	loginUC    *auth.LoginUseCase
	refreshUC  *auth.RefreshUseCase
	logoutUC   *auth.LogoutUseCase
	getMeUC    *auth.GetMeUseCase
	log        zerolog.Logger
}

// NewAuthHandler constructs an AuthHandler with all required use cases.
func NewAuthHandler(
	registerUC *auth.RegisterUseCase,
	loginUC *auth.LoginUseCase,
	refreshUC *auth.RefreshUseCase,
	logoutUC *auth.LogoutUseCase,
	getMeUC *auth.GetMeUseCase,
	log zerolog.Logger,
) *AuthHandler {
	return &AuthHandler{
		registerUC: registerUC,
		loginUC:    loginUC,
		refreshUC:  refreshUC,
		logoutUC:   logoutUC,
		getMeUC:    getMeUC,
		log:        log,
	}
}

// ── Register ─────────────────────────────────────────────────────────────────

type registerRequest struct {
	Email       string  `json:"email"        validate:"required,email"`
	Password    string  `json:"password"     validate:"required,min=8"`
	FirstName   string  `json:"first_name"   validate:"required"`
	LastName    string  `json:"last_name"    validate:"required"`
	Role        string  `json:"role"         validate:"required"`
	PhoneNumber *string `json:"phone_number"`
}

type registerUserResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Register handles POST /api/v1/auth/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	out, err := h.registerUC.Execute(r.Context(), auth.RegisterInput{
		Email:       req.Email,
		Password:    req.Password,
		FirstName:   req.FirstName,
		LastName:    req.LastName,
		Role:        user.Role(req.Role),
		PhoneNumber: req.PhoneNumber,
	})
	if err != nil {
		h.mapError(w, err)
		return
	}

	u := out.User
	respondJSON(w, http.StatusCreated, envelope{
		"data": registerUserResponse{
			ID:        u.ID,
			Email:     u.Email,
			FirstName: u.FirstName,
			LastName:  u.LastName,
			Role:      u.Role.String(),
			Status:    string(u.Status),
			CreatedAt: u.CreatedAt,
		},
	})
}

// ── Login ─────────────────────────────────────────────────────────────────────

type loginRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type loginUserPayload struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
	Role  string    `json:"role"`
}

type loginResponse struct {
	AccessToken  string           `json:"access_token"`
	RefreshToken string           `json:"refresh_token"`
	User         loginUserPayload `json:"user"`
}

// Login handles POST /api/v1/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	out, err := h.loginUC.Execute(r.Context(), auth.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		h.mapError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, envelope{
		"data": loginResponse{
			AccessToken:  out.AccessToken,
			RefreshToken: out.RefreshToken,
			User: loginUserPayload{
				ID:    out.User.ID,
				Email: out.User.Email,
				Role:  out.User.Role.String(),
			},
		},
	})
}

// ── Refresh ───────────────────────────────────────────────────────────────────

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type refreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Refresh handles POST /api/v1/auth/refresh.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	out, err := h.refreshUC.Execute(r.Context(), auth.RefreshInput{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		h.mapError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, envelope{
		"data": refreshResponse{
			AccessToken:  out.AccessToken,
			RefreshToken: out.RefreshToken,
		},
	})
}

// ── Logout ────────────────────────────────────────────────────────────────────

type logoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// Logout handles POST /api/v1/auth/logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req logoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	if err := h.logoutUC.Execute(r.Context(), auth.LogoutInput{
		RefreshToken: req.RefreshToken,
	}); err != nil {
		h.mapError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ── GetMe ─────────────────────────────────────────────────────────────────────

type meResponse struct {
	ID          uuid.UUID  `json:"id"`
	Email       string     `json:"email"`
	PhoneNumber *string    `json:"phone_number,omitempty"`
	FirstName   string     `json:"first_name"`
	LastName    string     `json:"last_name"`
	Role        string     `json:"role"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Me handles GET /api/v1/auth/me.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == uuid.Nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}

	out, err := h.getMeUC.Execute(r.Context(), auth.GetMeInput{UserID: userID})
	if err != nil {
		h.mapError(w, err)
		return
	}

	u := out.User
	respondJSON(w, http.StatusOK, envelope{
		"data": meResponse{
			ID:          u.ID,
			Email:       u.Email,
			PhoneNumber: u.PhoneNumber,
			FirstName:   u.FirstName,
			LastName:    u.LastName,
			Role:        u.Role.String(),
			Status:      string(u.Status),
			CreatedAt:   u.CreatedAt,
			UpdatedAt:   u.UpdatedAt,
		},
	})
}

// ── Error mapping ─────────────────────────────────────────────────────────────

// mapError translates domain errors to HTTP responses. Any error not explicitly
// mapped is treated as an internal server error — the raw error is logged but
// never exposed to the client to prevent information leakage.
func (h *AuthHandler) mapError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, user.ErrNotFound):
		respondError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
	case errors.Is(err, user.ErrEmailTaken):
		respondError(w, http.StatusConflict, "EMAIL_TAKEN", "email already in use")
	case errors.Is(err, user.ErrInvalidCredentials):
		respondError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid email or password")
	case errors.Is(err, user.ErrAccountInactive):
		respondError(w, http.StatusForbidden, "ACCOUNT_INACTIVE", "account is not active")
	case errors.Is(err, auth.ErrTokenNotFound):
		respondError(w, http.StatusUnauthorized, "INVALID_TOKEN", "token not found or expired")
	default:
		h.log.Error().Err(err).Msg("unhandled error in auth handler")
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "an unexpected error occurred")
	}
}
