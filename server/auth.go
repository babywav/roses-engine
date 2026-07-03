package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const userIDKey contextKey = "userID"

// GetUserIDFromContext recupera o userID injetado no contexto pelo middleware.
func GetUserIDFromContext(ctx context.Context) (string, error) {
	val := ctx.Value(userIDKey)
	if val == nil {
		return "", errors.New("user ID não encontrado no contexto")
	}
	s, ok := val.(string)
	if !ok {
		return "", errors.New("user ID inválido no contexto")
	}
	return s, nil
}

// ─── Introspecção de token via Supabase Auth ────────────────────────────────
// Valida o access_token chamando GET /auth/v1/user. Não exige o JWT secret,
// funciona com a anon/publishable key. Cache curto para evitar round-trips.

type introspectEntry struct {
	userID string
	exp    time.Time
}

var (
	introspectCache = map[string]introspectEntry{}
	introspectMu    sync.Mutex
	authHTTPClient  = &http.Client{Timeout: 8 * time.Second}
)

func introspectSupabase(token string) (string, error) {
	introspectMu.Lock()
	if e, ok := introspectCache[token]; ok && time.Now().Before(e.exp) {
		introspectMu.Unlock()
		return e.userID, nil
	}
	introspectMu.Unlock()

	base := strings.TrimRight(os.Getenv("SUPABASE_URL"), "/")
	anon := getenv("SUPABASE_ANON_KEY", os.Getenv("SUPABASE_PUBLISHABLE_KEY"))
	if base == "" || anon == "" {
		return "", errors.New("SUPABASE_URL/SUPABASE_ANON_KEY não configurados")
	}

	req, err := http.NewRequest(http.MethodGet, base+"/auth/v1/user", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("apikey", anon)

	resp, err := authHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("supabase auth retornou status %d", resp.StatusCode)
	}

	var u struct {
		ID string `json:"id"`
	}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &u); err != nil || u.ID == "" {
		return "", errors.New("resposta inválida do Supabase auth")
	}

	introspectMu.Lock()
	introspectCache[token] = introspectEntry{userID: u.ID, exp: time.Now().Add(5 * time.Minute)}
	introspectMu.Unlock()
	return u.ID, nil
}

// ─── Middleware de autenticação ─────────────────────────────────────────────
// withAuth valida o JWT do Supabase e injeta o user_id no request context.
// Ordem: bypass de dev → HS256 local (se SUPABASE_JWT_SECRET) → introspecção.
func withAuth(h http.HandlerFunc) http.HandlerFunc {
	origins := getenv("ROSES_CORS_ORIGINS", "*")

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origins)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key, Authorization, apikey")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Bypass de desenvolvimento: ROSES_DISABLE_AUTH=1, ou nenhum meio de
		// validar configurado (sem SUPABASE_URL e sem JWT secret).
		if os.Getenv("ROSES_DISABLE_AUTH") == "1" ||
			(os.Getenv("SUPABASE_URL") == "" && os.Getenv("SUPABASE_JWT_SECRET") == "") {
			ctx := context.WithValue(r.Context(), userIDKey, "00000000-0000-0000-0000-000000000000")
			h(w, r.WithContext(ctx))
			return
		}

		authHeader := r.Header.Get("Authorization")
		token := ""
		if parts := strings.SplitN(authHeader, " ", 2); len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			token = strings.TrimSpace(parts[1])
		}
		if token == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Header Authorization ausente ou inválido"})
			return
		}

		var userID string

		// Caminho rápido: verificação HS256 local, se o segredo estiver definido.
		if secret := os.Getenv("SUPABASE_JWT_SECRET"); secret != "" {
			tok, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("método de assinatura inesperado: %v", t.Header["alg"])
				}
				return []byte(secret), nil
			})
			if err == nil && tok.Valid {
				if claims, ok := tok.Claims.(jwt.MapClaims); ok {
					if sub, _ := claims["sub"].(string); sub != "" {
						userID = sub
					}
				}
			}
		}

		// Caminho padrão: introspecção via Supabase Auth.
		if userID == "" {
			id, err := introspectSupabase(token)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Token inválido ou expirado"})
				return
			}
			userID = id
		}

		ctx := context.WithValue(r.Context(), userIDKey, userID)
		h(w, r.WithContext(ctx))
	}
}
