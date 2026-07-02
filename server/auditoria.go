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

// ─── Estrutura de auditoria ──────────────────────────────────────────────────

type AuditoriaEntry struct {
	ID                   string    `json:"id"`
	UserID               string    `json:"user_id,omitempty"`
	IntimacaoID          *string   `json:"intimacao_id"`
	NumeroProcesso       string    `json:"numero_processo"`
	Fonte                string    `json:"fonte"`
	DataDisponibilizacao *string   `json:"data_disponibilizacao"`
	DataPublicacao       *string   `json:"data_publicacao"`
	Regra                string    `json:"regra"`
	BaseLegal            string    `json:"base_legal"`
	DiasUteis            int       `json:"dias_uteis"`
	Vencimento           *string   `json:"vencimento"`
	FeriadosConsiderados []string  `json:"feriados_considerados"`
	Divergencia          bool      `json:"divergencia"`
	DetalheDivergencia   string    `json:"detalhe_divergencia,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
}

// ─── Registro de auditoria ───────────────────────────────────────────────────

// RegistrarAuditoriaPrazo persiste uma entrada na trilha auditável.
// Chamado pelo SyncDJEN para cada intimação processada.
func RegistrarAuditoriaPrazo(
	ctx context.Context,
	userID string,
	intimacaoID string,
	numeroProcesso string,
	fonte string,
	dataDisp time.Time,
	dataPub time.Time,
	rule prazoRule,
	vencimento time.Time,
	uf string,
) {
	if DBPool == nil {
		return
	}

	feriados := coletarFeriados(dataPub, vencimento, uf)
	feriadosJSON, _ := json.Marshal(feriados)

	var intID *string
	if intimacaoID != "" {
		intID = &intimacaoID
	}

	_, err := DBPool.Exec(ctx, `
		INSERT INTO public.prazo_auditoria
			(user_id, intimacao_id, numero_processo, fonte,
			 data_disponibilizacao, data_publicacao,
			 regra, base_legal, dias_uteis, vencimento,
			 feriados_considerados, divergencia)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,false)
		ON CONFLICT DO NOTHING
	`,
		userID, intID, numeroProcesso, fonte,
		dataDisp, dataPub,
		rule.rotulo, rule.base, rule.dias, vencimento,
		string(feriadosJSON),
	)
	if err != nil {
		log.Printf("[Auditoria] Erro ao registrar trilha para %s: %v", numeroProcesso, err)
	}
}

// coletarFeriados lista as datas (DD/MM/YYYY) que foram puladas na contagem.
func coletarFeriados(inicio, fim time.Time, uf string) []string {
	var lista []string
	t := inicio
	for !t.After(fim) {
		if !ehDiaUtil(t, uf) && t.Weekday() != time.Saturday && t.Weekday() != time.Sunday {
			lista = append(lista, t.Format("02/01/2006"))
		}
		t = t.AddDate(0, 0, 1)
	}
	return lista
}

// ─── Cross-check DJEN × DataJud ──────────────────────────────────────────────

// CrossCheckPrazo compara a data de vencimento da intimação DJEN com a estimativa
// baseada na última movimentação do DataJud para o mesmo processo.
// Detecta divergências e movimentações órfãs (sem intimação correspondente).
func CrossCheckPrazo(
	ctx context.Context,
	userID string,
	numeroProcesso string,
	vencDJEN time.Time,
	intimacaoID string,
	uf string,
) {
	if DBPool == nil {
		return
	}

	processos := loadProcessos(userID)
	for _, p := range processos {
		if p.Numero != numeroProcesso {
			continue
		}

		// Calcula a estimativa DataJud/portal para o mesmo processo
		dataMovim, _, rule, found := gatilhoPrazoMaisRecente(p.Process)
		if !found {
			return
		}
		pubEstimado := adicionaDiasUteis(dataMovim, 1, uf)
		vencEstimado := adicionaDiasUteis(pubEstimado, rule.dias, uf)

		diffDias := diasUteisEntre(vencDJEN, vencEstimado, uf)
		if diffDias < 0 {
			diffDias = -diffDias
		}

		if diffDias > 0 {
			detalhe := fmt.Sprintf(
				"Vencimento DJEN: %s | Estimativa DataJud: %s | Diferença: %d dia(s) útil(eis)",
				vencDJEN.Format("02/01/2006"),
				vencEstimado.Format("02/01/2006"),
				diffDias,
			)
			log.Printf("[Auditoria DIVERGÊNCIA] Processo %s — %s", numeroProcesso, detalhe)

			_, err := DBPool.Exec(ctx, `
				UPDATE public.prazo_auditoria
				SET divergencia = true, detalhe_divergencia = $1
				WHERE intimacao_id = $2 AND user_id = $3
			`, detalhe, intimacaoID, userID)
			if err != nil {
				log.Printf("[Auditoria] Falha ao registrar divergência: %v", err)
			}
		}
		return
	}

	// Movimentação relevante encontrada no DataJud sem intimação DJEN correspondente
	log.Printf("[Auditoria ÓRFÃO] Processo %s tem movimentação sem intimação DJEN correspondente. Verificação manual recomendada.", numeroProcesso)
}

// ─── Handler HTTP ─────────────────────────────────────────────────────────────

// handleAuditoriaProcesso retorna a trilha auditável de um processo específico.
// GET /api/auditoria/{numero_processo}
func handleAuditoriaProcesso(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET"})
		return
	}
	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/auditoria/"), "/")
	numeroProcesso := parts[0]
	if numeroProcesso == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "número do processo ausente"})
		return
	}

	if DBPool == nil {
		writeJSON(w, http.StatusOK, []AuditoriaEntry{})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := DBPool.Query(ctx, `
		SELECT id, intimacao_id, numero_processo, fonte,
		       TO_CHAR(data_disponibilizacao, 'DD/MM/YYYY'),
		       TO_CHAR(data_publicacao, 'DD/MM/YYYY'),
		       regra, base_legal, dias_uteis,
		       TO_CHAR(vencimento, 'DD/MM/YYYY'),
		       feriados_considerados, divergencia, detalhe_divergencia, created_at
		FROM public.prazo_auditoria
		WHERE user_id = $1 AND numero_processo = $2
		ORDER BY created_at DESC
	`, userID, numeroProcesso)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var lista []AuditoriaEntry
	for rows.Next() {
		var e AuditoriaEntry
		var feriadosRaw []byte
		err := rows.Scan(
			&e.ID, &e.IntimacaoID, &e.NumeroProcesso, &e.Fonte,
			&e.DataDisponibilizacao, &e.DataPublicacao,
			&e.Regra, &e.BaseLegal, &e.DiasUteis,
			&e.Vencimento,
			&feriadosRaw, &e.Divergencia, &e.DetalheDivergencia, &e.CreatedAt,
		)
		if err != nil {
			continue
		}
		_ = json.Unmarshal(feriadosRaw, &e.FeriadosConsiderados)
		lista = append(lista, e)
	}
	if lista == nil {
		lista = []AuditoriaEntry{}
	}
	writeJSON(w, http.StatusOK, lista)
}

// handleAuditoriaDivergencias retorna todos os processos com divergência detectada.
// GET /api/auditoria/divergencias
func handleAuditoriaDivergencias(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusOK, []AuditoriaEntry{})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := DBPool.Query(ctx, `
		SELECT id, intimacao_id, numero_processo, fonte,
		       TO_CHAR(data_disponibilizacao, 'DD/MM/YYYY'),
		       TO_CHAR(data_publicacao, 'DD/MM/YYYY'),
		       regra, base_legal, dias_uteis,
		       TO_CHAR(vencimento, 'DD/MM/YYYY'),
		       feriados_considerados, divergencia, detalhe_divergencia, created_at
		FROM public.prazo_auditoria
		WHERE user_id = $1 AND divergencia = true
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var lista []AuditoriaEntry
	for rows.Next() {
		var e AuditoriaEntry
		var feriadosRaw []byte
		err := rows.Scan(
			&e.ID, &e.IntimacaoID, &e.NumeroProcesso, &e.Fonte,
			&e.DataDisponibilizacao, &e.DataPublicacao,
			&e.Regra, &e.BaseLegal, &e.DiasUteis,
			&e.Vencimento,
			&feriadosRaw, &e.Divergencia, &e.DetalheDivergencia, &e.CreatedAt,
		)
		if err != nil {
			continue
		}
		_ = json.Unmarshal(feriadosRaw, &e.FeriadosConsiderados)
		lista = append(lista, e)
	}
	if lista == nil {
		lista = []AuditoriaEntry{}
	}
	writeJSON(w, http.StatusOK, lista)
}
