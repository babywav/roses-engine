package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestWithAuth(t *testing.T) {
	// Configura segredo temporário para o teste
	secret := "my-temporary-jwt-secret-key-12345"
	os.Setenv("SUPABASE_JWT_SECRET", secret)
	defer os.Unsetenv("SUPABASE_JWT_SECRET")

	// Handler simples de teste que valida se o userID foi injetado com sucesso no contexto
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := GetUserIDFromContext(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(userID))
	})

	authHandler := withAuth(testHandler)

	// Caso 1: Header Authorization Ausente -> Espera 401 Unauthorized
	req, _ := http.NewRequest("GET", "/api/test", nil)
	rr := httptest.NewRecorder()
	authHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Esperava Status 401, obteve %d", rr.Code)
	}

	// Caso 2: JWT Válido -> Espera 200 OK e o userID correto decodificado do claim 'sub'
	claims := jwt.MapClaims{
		"sub": "user-123-abc-test",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(secret))

	req, _ = http.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr = httptest.NewRecorder()
	authHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Esperava Status 200, obteve %d. Resposta: %s", rr.Code, rr.Body.String())
	}
	if rr.Body.String() != "user-123-abc-test" {
		t.Errorf("Esperava userID 'user-123-abc-test', obteve '%s'", rr.Body.String())
	}

	// Caso 3: JWT Assinado com chave incorreta -> Espera 401 Unauthorized
	badToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	badTokenStr, _ := badToken.SignedString([]byte("wrong-secret-key-xyz"))

	req, _ = http.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+badTokenStr)
	rr = httptest.NewRecorder()
	authHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Esperava Status 401 para assinatura inválida, obteve %d", rr.Code)
	}
}
