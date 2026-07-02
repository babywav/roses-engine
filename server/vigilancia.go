package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// ─── Estruturas ───────────────────────────────────────────────────────────────

type Vigilancia struct {
	ID                 string    `json:"id"`
	UserID             string    `json:"user_id,omitempty"`
	Documento          string    `json:"documento"`        // CPF ou CNPJ
	Nome               string    `json:"nome"`
	Tipo               string    `json:"tipo"`             // cliente | adversario | devedor | parte
	Tribunais          []string  `json:"tribunais"`        // vazio = todos
	UltimaVerificacao  *string   `json:"ultima_verificacao"`
	Ativo              bool      `json:"ativo"`
	CreatedAt          time.Time `json:"created_at"`
}

type VigilanciaAlerta struct {
	ID             string    `json:"id"`
	VigilanciaID   string    `json:"vigilancia_id"`
	TipoAlerta     string    `json:"tipo_alerta"`
	NumeroProcesso *string   `json:"numero_processo"`
	Descricao      string    `json:"descricao"`
	Lido           bool      `json:"lido"`
	CreatedAt      time.Time `json:"created_at"`
}

// ─── CRUD de Vigílias ─────────────────────────────────────────────────────────

// handleVigilancias roteador principal: /api/vigilancias e /api/vigilancias/{id}
func handleVigilancias(w http.ResponseWriter, r *http.Request) {
	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/vigilancias")
	path = strings.TrimPrefix(path, "/")

	switch {
	case path == "" || path == "/":
		switch r.Method {
		case http.MethodGet:
			listarVigilancias(w, userID)
		case http.MethodPost:
			criarVigilancia(w, r, userID)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET ou POST"})
		}
	case strings.HasPrefix(path, "alertas"):
		listarAlertasVigilancia(w, userID)
	default:
		// /api/vigilancias/{id}
		vigID := path
		switch r.Method {
		case http.MethodDelete:
			deletarVigilancia(w, userID, vigID)
		case http.MethodPatch:
			atualizarVigilancia(w, r, userID, vigID)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use DELETE ou PATCH"})
		}
	}
}

func listarVigilancias(w http.ResponseWriter, userID string) {
	if DBPool == nil {
		writeJSON(w, http.StatusOK, []Vigilancia{})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := DBPool.Query(ctx, `
		SELECT id, documento, nome, tipo, tribunais,
		       TO_CHAR(ultima_verificacao, 'DD/MM/YYYY HH24:MI'), ativo, created_at
		FROM public.vigilancias
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var lista []Vigilancia
	for rows.Next() {
		var v Vigilancia
		err := rows.Scan(&v.ID, &v.Documento, &v.Nome, &v.Tipo,
			&v.Tribunais, &v.UltimaVerificacao, &v.Ativo, &v.CreatedAt)
		if err == nil {
			lista = append(lista, v)
		}
	}
	if lista == nil {
		lista = []Vigilancia{}
	}
	writeJSON(w, http.StatusOK, lista)
}

type criarVigilanciaReq struct {
	Documento string   `json:"documento"` // CPF ou CNPJ
	Nome      string   `json:"nome"`
	Tipo      string   `json:"tipo"`
	Tribunais []string `json:"tribunais"`
}

func criarVigilancia(w http.ResponseWriter, r *http.Request, userID string) {
	var req criarVigilanciaReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON inválido"})
		return
	}
	doc := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, req.Documento)
	if len(doc) != 11 && len(doc) != 14 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "documento deve ser CPF (11 dígitos) ou CNPJ (14 dígitos)"})
		return
	}
	if strings.TrimSpace(req.Nome) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "nome é obrigatório"})
		return
	}
	if req.Tipo == "" {
		req.Tipo = "parte"
	}
	if req.Tribunais == nil {
		req.Tribunais = []string{}
	}

	if DBPool == nil {
		writeJSON(w, http.StatusOK, map[string]string{"id": "mock-vig-id", "status": "criado (offline)"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tribunaisJSON, _ := json.Marshal(req.Tribunais)

	var vigID string
	err := DBPool.QueryRow(ctx, `
		INSERT INTO public.vigilancias (user_id, documento, nome, tipo, tribunais)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, documento)
		DO UPDATE SET nome = EXCLUDED.nome, tipo = EXCLUDED.tipo, tribunais = EXCLUDED.tribunais, ativo = true, updated_at = now()
		RETURNING id
	`, userID, doc, req.Nome, req.Tipo, string(tribunaisJSON)).Scan(&vigID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Executa verificação inicial em background
	go RunVigilanciaCheck(vigID, userID, doc, req.Nome, req.Tipo, req.Tribunais)

	writeJSON(w, http.StatusOK, map[string]string{"id": vigID, "status": "criado"})
}

func deletarVigilancia(w http.ResponseWriter, userID, vigID string) {
	if DBPool == nil {
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := DBPool.Exec(ctx, `
		UPDATE public.vigilancias SET ativo = false, updated_at = now()
		WHERE id = $1 AND user_id = $2
	`, vigID, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

type atualizarVigilanciaReq struct {
	Nome      string   `json:"nome"`
	Tribunais []string `json:"tribunais"`
	Ativo     *bool    `json:"ativo"`
}

func atualizarVigilancia(w http.ResponseWriter, r *http.Request, userID, vigID string) {
	var req atualizarVigilanciaReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON inválido"})
		return
	}
	if DBPool == nil {
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if req.Nome != "" {
		_, _ = DBPool.Exec(ctx, `UPDATE public.vigilancias SET nome = $1, updated_at = now() WHERE id = $2 AND user_id = $3`, req.Nome, vigID, userID)
	}
	if req.Tribunais != nil {
		tj, _ := json.Marshal(req.Tribunais)
		_, _ = DBPool.Exec(ctx, `UPDATE public.vigilancias SET tribunais = $1, updated_at = now() WHERE id = $2 AND user_id = $3`, string(tj), vigID, userID)
	}
	if req.Ativo != nil {
		_, _ = DBPool.Exec(ctx, `UPDATE public.vigilancias SET ativo = $1, updated_at = now() WHERE id = $2 AND user_id = $3`, *req.Ativo, vigID, userID)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func listarAlertasVigilancia(w http.ResponseWriter, userID string) {
	if DBPool == nil {
		writeJSON(w, http.StatusOK, []VigilanciaAlerta{})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := DBPool.Query(ctx, `
		SELECT id, vigilancia_id, tipo_alerta, numero_processo, descricao, lido, created_at
		FROM public.vigilancia_alertas
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 100
	`, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var lista []VigilanciaAlerta
	for rows.Next() {
		var a VigilanciaAlerta
		_ = rows.Scan(&a.ID, &a.VigilanciaID, &a.TipoAlerta, &a.NumeroProcesso, &a.Descricao, &a.Lido, &a.CreatedAt)
		lista = append(lista, a)
	}
	if lista == nil {
		lista = []VigilanciaAlerta{}
	}
	writeJSON(w, http.StatusOK, lista)
}

// ─── Worker de Vigilância ─────────────────────────────────────────────────────

// RunVigilanciaCheck consulta processos do CPF/CNPJ e compara com o snapshot anterior.
// Notifica apenas o que é novo.
func RunVigilanciaCheck(vigID, userID, documento, nome, tipo string, tribunais []string) {
	log.Printf("[Vigília] Iniciando verificação para %s (%s)...", nome, documento)

	uf := ""
	if len(tribunais) == 1 && len(tribunais[0]) == 2 {
		uf = tribunais[0]
	}

	// Busca por nome (porta existente)
	res := portalConsulta("nome", nome, uf, 60)
	if res.Status != "OK" {
		log.Printf("[Vigília] Nenhum resultado para %s: %s", nome, res.Status)
		return
	}

	// Snapshot atual: lista de números de processo
	atual := make(map[string]bool)
	for _, p := range res.Processos {
		atual[p.Numero] = true
	}

	if DBPool == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Carrega snapshot anterior do banco
	var snapJSON []byte
	_ = DBPool.QueryRow(ctx, `SELECT snapshot_anterior FROM public.vigilancias WHERE id = $1`, vigID).Scan(&snapJSON)

	var anterior []string
	_ = json.Unmarshal(snapJSON, &anterior)
	anteriorSet := make(map[string]bool, len(anterior))
	for _, n := range anterior {
		anteriorSet[n] = true
	}

	// Detecta novidades
	novos := 0
	for _, p := range res.Processos {
		if anteriorSet[p.Numero] {
			continue
		}
		tipoAlerta := "novo_processo"
		classeLC := strings.ToLower(p.Classe)
		if strings.Contains(classeLC, "execução") || strings.Contains(classeLC, "execucao") || strings.Contains(classeLC, "penhora") {
			tipoAlerta = "nova_execucao"
		}
		descricao := fmt.Sprintf("Novo processo detectado para %s: %s (%s) — %s", nome, p.Numero, p.Tribunal, p.Classe)
		_, err := DBPool.Exec(ctx, `
			INSERT INTO public.vigilancia_alertas (vigilancia_id, user_id, tipo_alerta, numero_processo, descricao)
			VALUES ($1, $2, $3, $4, $5)
		`, vigID, userID, tipoAlerta, p.Numero, descricao)
		if err != nil {
			log.Printf("[Vigília] Erro ao salvar alerta: %v", err)
		} else {
			novos++
			// Notifica via alertas multi-canal (Fase 3)
			_ = GlobalDispatcher.Push.SendPush(userID,
				fmt.Sprintf("Roses — Novo processo: %s", nome),
				descricao,
			)
		}
	}

	// Atualiza snapshot e data de verificação
	novoSnap, _ := json.Marshal(func() []string {
		keys := make([]string, 0, len(atual))
		for k := range atual {
			keys = append(keys, k)
		}
		return keys
	}())
	_, _ = DBPool.Exec(ctx, `
		UPDATE public.vigilancias
		SET snapshot_anterior = $1, ultima_verificacao = now(), updated_at = now()
		WHERE id = $2
	`, string(novoSnap), vigID)

	log.Printf("[Vigília] Verificação concluída para %s: %d novo(s) processo(s).", nome, novos)
}

// StartVigilanciaWorker dispara o worker de vigília diariamente.
func StartVigilanciaWorker() {
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		// Primeira rodada: espera 5 min para o servidor estabilizar
		time.Sleep(5 * time.Minute)
		runAllVigilancias()
		for range ticker.C {
			runAllVigilancias()
		}
	}()
}

func runAllVigilancias() {
	if DBPool == nil {
		return
	}
	log.Println("[Vigília Worker] Iniciando ciclo diário de vigilância...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := DBPool.Query(ctx, `
		SELECT id, user_id, documento, nome, tipo, tribunais
		FROM public.vigilancias
		WHERE ativo = true
	`)
	if err != nil {
		log.Printf("[Vigília Worker] Erro ao buscar vigílias: %v", err)
		return
	}
	defer rows.Close()

	type vigRow struct {
		ID        string
		UserID    string
		Documento string
		Nome      string
		Tipo      string
		Tribunais []string
	}

	var vigs []vigRow
	for rows.Next() {
		var v vigRow
		var tjJSON []byte
		_ = rows.Scan(&v.ID, &v.UserID, &v.Documento, &v.Nome, &v.Tipo, &tjJSON)
		_ = json.Unmarshal(tjJSON, &v.Tribunais)
		vigs = append(vigs, v)
	}

	for _, v := range vigs {
		go RunVigilanciaCheck(v.ID, v.UserID, v.Documento, v.Nome, v.Tipo, v.Tribunais)
		time.Sleep(2 * time.Second) // rate limiting
	}
}
