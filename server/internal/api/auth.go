package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/axonize/server/internal/auth"
)

type signupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token  string   `json:"token"`
	User   authUser `json:"user"`
	APIKey string   `json:"api_key,omitempty"`
}

type authUser struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	TenantID string `json:"tenant_id"`
}

type meResponse struct {
	User         authUser   `json:"user"`
	Tenant       meTenant   `json:"tenant"`
	APIKeyPrefix string     `json:"api_key_prefix"`
}

type meTenant struct {
	Name string `json:"name"`
	Plan string `json:"plan"`
}

// registerAuthRoutes adds public auth endpoints to the mux.
func registerAuthRoutes(mux *http.ServeMux, pool *pgxpool.Pool, jwtSecret string) {
	mux.HandleFunc("POST /api/v1/auth/signup", handleSignup(pool, jwtSecret))
	mux.HandleFunc("POST /api/v1/auth/login", handleLogin(pool, jwtSecret))
	mux.HandleFunc("GET /api/v1/auth/me", handleMe(pool, jwtSecret))
	mux.HandleFunc("POST /api/v1/auth/logout", handleLogout())
}

func handleSignup(pool *pgxpool.Pool, jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req signupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		req.Email = strings.TrimSpace(strings.ToLower(req.Email))
		if req.Email == "" || req.Password == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password are required"})
			return
		}
		if len(req.Password) < 8 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password must be at least 8 characters"})
			return
		}

		// Check email uniqueness
		var exists bool
		pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, req.Email).Scan(&exists)
		if exists {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "email already registered"})
			return
		}

		// Hash password
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		// Create tenant
		tenantID := generateTenantID()
		now := time.Now()
		_, err = pool.Exec(r.Context(), `
			INSERT INTO tenants (tenant_id, name, plan, status, created_at, updated_at)
			VALUES ($1, $2, 'free', 'active', $3, $3)
		`, tenantID, req.Email, now)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("create tenant: %v", err)})
			return
		}

		// Create user
		userID := generateUserID()
		_, err = pool.Exec(r.Context(), `
			INSERT INTO users (id, email, password_hash, tenant_id, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'active', $5, $5)
		`, userID, req.Email, hash, tenantID, now)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("create user: %v", err)})
			return
		}

		// Create API key
		rawKey := generateAPIKey()
		keyHash := hashKey(rawKey)
		keyPrefix := rawKey[:12]
		_, err = pool.Exec(r.Context(), `
			INSERT INTO api_keys (key_hash, key_prefix, tenant_id, name, scopes, status, created_at)
			VALUES ($1, $2, $3, 'default', 'ingest,read', 'active', $4)
		`, keyHash, keyPrefix, tenantID, now)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("create api key: %v", err)})
			return
		}

		// Generate JWT
		token, err := auth.GenerateJWT(userID, tenantID, req.Email, jwtSecret)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		writeJSON(w, http.StatusCreated, authResponse{
			Token:  token,
			User:   authUser{ID: userID, Email: req.Email, TenantID: tenantID},
			APIKey: rawKey,
		})
	}
}

func handleLogin(pool *pgxpool.Pool, jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		req.Email = strings.TrimSpace(strings.ToLower(req.Email))
		if req.Email == "" || req.Password == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password are required"})
			return
		}

		// Lookup user
		var userID, passwordHash, tenantID, status string
		err := pool.QueryRow(r.Context(), `
			SELECT id, password_hash, tenant_id, status FROM users WHERE email = $1
		`, req.Email).Scan(&userID, &passwordHash, &tenantID, &status)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		if status != "active" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "account is " + status})
			return
		}

		// Verify password
		if !auth.CheckPassword(passwordHash, req.Password) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
			return
		}

		// Get API key for response
		var apiKey string
		err = pool.QueryRow(r.Context(), `
			SELECT key_prefix FROM api_keys
			WHERE tenant_id = $1 AND status = 'active'
			ORDER BY created_at ASC LIMIT 1
		`, tenantID).Scan(&apiKey)
		if err != nil {
			apiKey = ""
		}

		// Generate JWT
		token, err := auth.GenerateJWT(userID, tenantID, req.Email, jwtSecret)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		writeJSON(w, http.StatusOK, authResponse{
			Token: token,
			User:  authUser{ID: userID, Email: req.Email, TenantID: tenantID},
		})
	}
}

func handleMe(pool *pgxpool.Pool, jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract and validate JWT
		tokenStr := extractBearerToken(r)
		if tokenStr == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing token"})
			return
		}

		claims, err := auth.ValidateJWT(tokenStr, jwtSecret)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
			return
		}

		// Get tenant info
		var tenantName, tenantPlan string
		err = pool.QueryRow(r.Context(), `
			SELECT name, plan FROM tenants WHERE tenant_id = $1
		`, claims.TenantID).Scan(&tenantName, &tenantPlan)
		if err != nil {
			tenantName = claims.TenantID
			tenantPlan = "free"
		}

		// Get API key prefix
		var apiKeyPrefix string
		err = pool.QueryRow(r.Context(), `
			SELECT key_prefix FROM api_keys
			WHERE tenant_id = $1 AND status = 'active'
			ORDER BY created_at ASC LIMIT 1
		`, claims.TenantID).Scan(&apiKeyPrefix)
		if err != nil {
			apiKeyPrefix = ""
		}

		writeJSON(w, http.StatusOK, meResponse{
			User:         authUser{ID: claims.UserID, Email: claims.Email, TenantID: claims.TenantID},
			Tenant:       meTenant{Name: tenantName, Plan: tenantPlan},
			APIKeyPrefix: apiKeyPrefix,
		})
	}
}

func handleLogout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func extractBearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return h[7:]
	}
	return ""
}

func generateUserID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return "usr_" + hex.EncodeToString(b)
}
