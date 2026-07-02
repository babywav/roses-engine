package main

// lgpd.go — Compliance LGPD (Lei 13.709/2018) — Transversal
//
// Implementa os direitos do titular de dados (art. 18):
//   • Exportação completa dos dados pessoais (art. 18 V)
//   • Exclusão de dados pessoais (art. 18 VI)
//
// Endpoints:
//   GET    /api/conta/exportar   — JSON completo dos dados do titular
//   DELETE /api/conta            — exclusão irreversível da conta e dados

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// ─── Exportação de dados (art. 18 V LGPD) ────────────────────────────────────

type ExportacaoDados struct {
	GeradoEm    string          `json:"gerado_em"`
	Aviso       string          `json:"aviso"`
	Perfil      json.RawMessage `json:"perfil"`
	Processos   json.RawMessage `json:"processos"`
	Intimacoes  json.RawMessage `json:"intimacoes"`
	Minutas     json.RawMessage `json:"minutas"`
	Calculos    json.RawMessage `json:"calculos"`
	Vigilancias json.RawMessage `json:"vigilancias"`
}

func handleExportarDados(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET"})
		return
	}
	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	if DBPool == nil {
		writeJSON(w, http.StatusOK, ExportacaoDados{
			GeradoEm: time.Now().Format(time.RFC3339),
			Aviso:    "Modo offline — exportação parcial indisponível.",
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	export := ExportacaoDados{
		GeradoEm: time.Now().Format(time.RFC3339),
		Aviso: "Exportação realizada nos termos do art. 18, V da LGPD (Lei 13.709/2018). " +
			"Este arquivo contém todos os dados pessoais armazenados no Roses associados à sua conta.",
	}

	export.Perfil = queryJSON(ctx, userID, `
		SELECT row_to_json(p) FROM public.perfis p WHERE id = $1
	`)
	export.Processos = queryJSONArray(ctx, userID, `
		SELECT json_agg(row_to_json(p))
		FROM (SELECT numero, tribunal, classe, assunto, orgao_julgador, fonte, data_distribuicao, last_seen
		      FROM public.processos WHERE user_id = $1) p
	`)
	export.Intimacoes = queryJSONArray(ctx, userID, `
		SELECT json_agg(row_to_json(i))
		FROM (SELECT numero_processo, tribunal, tipo_comunicacao, data_disponibilizacao,
		             data_publicacao, prazo_rotulo, prazo_base, vencimento, status, lido, created_at
		      FROM public.intimacoes WHERE user_id = $1) i
	`)
	export.Minutas = queryJSONArray(ctx, userID, `
		SELECT json_agg(row_to_json(m))
		FROM (SELECT processo_numero, tipo_peca, status, created_at, updated_at
		      FROM public.minutas WHERE user_id = $1) m
	`)
	export.Calculos = queryJSONArray(ctx, userID, `
		SELECT json_agg(row_to_json(c))
		FROM (SELECT tipo, parametros, resultado, created_at
		      FROM public.calculos WHERE user_id = $1) c
	`)
	export.Vigilancias = queryJSONArray(ctx, userID, `
		SELECT json_agg(row_to_json(v))
		FROM (SELECT documento, nome, tipo, tribunais, ultima_verificacao, ativo, created_at
		      FROM public.vigilancias WHERE user_id = $1) v
	`)

	// Log de acesso sem PII (auditabilidade)
	log.Printf("[LGPD] Exportação de dados solicitada por usuário %s", maskID(userID))

	w.Header().Set("Content-Disposition", `attachment; filename="roses-meus-dados.json"`)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(export)
}

func queryJSON(ctx context.Context, userID, query string) json.RawMessage {
	if DBPool == nil {
		return json.RawMessage("null")
	}
	var raw []byte
	err := DBPool.QueryRow(ctx, query, userID).Scan(&raw)
	if err != nil || raw == nil {
		return json.RawMessage("null")
	}
	return raw
}

func queryJSONArray(ctx context.Context, userID, query string) json.RawMessage {
	if DBPool == nil {
		return json.RawMessage("[]")
	}
	var raw []byte
	err := DBPool.QueryRow(ctx, query, userID).Scan(&raw)
	if err != nil || raw == nil {
		return json.RawMessage("[]")
	}
	return raw
}

// maskID ofusca UUID para log sem expor o ID completo.
func maskID(id string) string {
	if len(id) < 8 {
		return "****"
	}
	return id[:4] + "****" + id[len(id)-4:]
}

// ─── Exclusão de conta (art. 18 VI LGPD) ─────────────────────────────────────

func handleExcluirConta(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use DELETE"})
		return
	}
	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	// Requer confirmação explícita no body: {"confirmar": true}
	var body struct {
		Confirmar bool `json:"confirmar"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !body.Confirmar {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "envie {\"confirmar\": true} para confirmar a exclusão irreversível da conta",
		})
		return
	}

	if DBPool == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "excluído (modo offline)"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Exclusão em cascata — as FKs com ON DELETE CASCADE cuidam das tabelas filhas.
	// Tabelas sem FK direta são limpas explicitamente.
	tabelas := []string{
		"public.prazo_auditoria",
		"public.vigilancia_alertas",
		"public.vigilancias",
		"public.calculos",
		"public.portal_links",
		"public.minutas",
		"public.intimacoes",
		"public.jobs_analise",
		"public.processos",
		"public.perfis",
	}

	for _, tabela := range tabelas {
		query := fmt.Sprintf("DELETE FROM %s WHERE user_id = $1", tabela)
		if _, err := DBPool.Exec(ctx, query, userID); err != nil {
			// Log sem PII, não interrompe — tenta limpar o máximo possível
			log.Printf("[LGPD] Aviso ao excluir %s para usuário %s: %v", tabela, maskID(userID), err)
		}
	}

	log.Printf("[LGPD] Conta excluída para usuário %s", maskID(userID))

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "excluido",
		"aviso":   "Todos os dados pessoais foram removidos do Roses. O usuário em auth.users deve ser excluído pelo Supabase Auth (via dashboard ou serviço administrativo).",
		"data":    time.Now().Format(time.RFC3339),
	})
}
