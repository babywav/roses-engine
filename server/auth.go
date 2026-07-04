package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v2"
	"github.com/golang-jwt/jwt/v5"
)

// Tipo para a chave do context
type contextKey string

const userIDKey contextKey = "userID"

var jwks *keyfunc.JWKS

// initCognitoJWKS inicializa o gerenciador de chaves públicas (JWKS) do Cognito
func initCognitoJWKS() {
	region := os.Getenv("AWS_REGION")
	userPoolID := os.Getenv("COGNITO_USER_POOL_ID")

	if region == "" || userPoolID == "" {
		log.Println("[Auth] Aviso: AWS_REGION ou COGNITO_USER_POOL_ID não definidos. Autenticação via Cognito desabilitada.")
		return
	}

	jwksURL := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s/.well-known/jwks.json", region, userPoolID)
	
	options := keyfunc.Options{
		RefreshErrorHandler: func(err error) {
			log.Printf("[Auth] Erro ao atualizar JWKS: %v", err)
		},
		RefreshInterval:   time.Hour,
		RefreshRateLimit:  time.Minute * 5,
		RefreshTimeout:    time.Second * 10,
		RefreshUnknownKID: true,
	}

	var err error
	jwks, err = keyfunc.Get(jwksURL, options)
	if err != nil {
		log.Fatalf("[Auth] Falha ao carregar JWKS do Cognito: %v", err)
	}
	log.Println("[Auth] JWKS do Cognito carregado com sucesso.")
}

// GetUserIDFromContext extrai o userID (sub) injetado pelo middleware withAuth
func GetUserIDFromContext(ctx context.Context) (string, error) {
	val := ctx.Value(userIDKey)
	if val == nil {
		return "", fmt.Errorf("usuário não autenticado no contexto")
	}
	userID, ok := val.(string)
	if !ok || userID == "" {
		return "", fmt.Errorf("formato inválido para userID no contexto")
	}
	return userID, nil
}

// ─── Middleware de autenticação ─────────────────────────────────────────────
// withAuth valida o JWT do AWS Cognito e injeta o user_id no request context.
func withAuth(h http.HandlerFunc) http.HandlerFunc {
	origins := getenv("ROSES_CORS_ORIGINS", "*")

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origins)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key, Authorization, apikey")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Bypass de desenvolvimento: ROSES_DISABLE_AUTH=1
		if os.Getenv("ROSES_DISABLE_AUTH") == "1" || jwks == nil {
			ctx := context.WithValue(r.Context(), userIDKey, "00000000-0000-0000-0000-000000000000")
			h(w, r.WithContext(ctx))
			return
		}

		authHeader := r.Header.Get("Authorization")
		tokenStr := ""
		if parts := strings.SplitN(authHeader, " ", 2); len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			tokenStr = strings.TrimSpace(parts[1])
		}
		if tokenStr == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Header Authorization ausente ou inválido"})
			return
		}

		// Valida o JWT usando as chaves públicas do Cognito (JWKS)
		token, err := jwt.Parse(tokenStr, jwks.Keyfunc)
		if err != nil || !token.Valid {
			log.Printf("[Auth] Token inválido: %v", err)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Token JWT inválido ou expirado"})
			return
		}

		var userID string
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			if sub, _ := claims["sub"].(string); sub != "" {
				userID = sub
			}
		}

		if userID == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Token não contém subject (sub)"})
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, userID)
		h(w, r.WithContext(ctx))
	}
}
