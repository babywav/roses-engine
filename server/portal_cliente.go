package main

// portal_cliente.go — Portal público do cliente (Diferencial 9.5)
//
// O advogado gera um link com token para o cliente acompanhar
// o processo em linguagem leiga, sem precisar de login.
//
// Endpoints:
//   POST   /api/portal/links           — gera link para um processo
//   GET    /api/portal/links           — lista links do advogado
//   DELETE /api/portal/links/{id}      — revoga link
//   GET    /api/portal/{token}         — rota pública (sem JWT) para o cliente

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ─── Handlers do advogado (requerem JWT) ──────────────────────────────────────

func handlePortalLinks(w http.ResponseWriter, r *http.Request) {
	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/portal/links")
	path = strings.TrimPrefix(path, "/")

	switch {
	case r.Method == http.MethodGet && path == "":
		listarPortalLinks(w, userID)
	case r.Method == http.MethodPost && path == "":
		criarPortalLink(w, r, userID)
	case r.Method == http.MethodDelete && path != "":
		revogarPortalLink(w, userID, path)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "método não suportado"})
	}
}

type criarLinkReq struct {
	NumeroProcesso string `json:"numero_processo"`
	NomeCliente    string `json:"nome_cliente"`
	DiasValidade   int    `json:"dias_validade"` // 0 = sem expiração
}

func criarPortalLink(w http.ResponseWriter, r *http.Request, userID string) {
	var req criarLinkReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON inválido"})
		return
	}
	if strings.TrimSpace(req.NumeroProcesso) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "numero_processo é obrigatório"})
		return
	}

	if DBPool == nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"token": "mock-token-abc123",
			"url":   "/portal/mock-token-abc123",
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var expiraEm *time.Time
	if req.DiasValidade > 0 {
		t := time.Now().AddDate(0, 0, req.DiasValidade)
		expiraEm = &t
	}

	var linkID, token string
	err := DBPool.QueryRow(ctx, `
		INSERT INTO public.portal_links (user_id, numero_processo, nome_cliente, expira_em)
		VALUES ($1, $2, $3, $4)
		RETURNING id, token
	`, userID, req.NumeroProcesso, req.NomeCliente, expiraEm).Scan(&linkID, &token)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"id":    linkID,
		"token": token,
		"url":   fmt.Sprintf("/api/portal/%s", token),
	})
}

func listarPortalLinks(w http.ResponseWriter, userID string) {
	if DBPool == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := DBPool.Query(ctx, `
		SELECT id, numero_processo, nome_cliente, token, ativo, acessos,
		       TO_CHAR(ultimo_acesso,'DD/MM/YYYY HH24:MI'),
		       TO_CHAR(expira_em,'DD/MM/YYYY'),
		       TO_CHAR(created_at,'DD/MM/YYYY')
		FROM public.portal_links
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type linkItem struct {
		ID             string  `json:"id"`
		NumeroProcesso string  `json:"numero_processo"`
		NomeCliente    *string `json:"nome_cliente"`
		Token          string  `json:"token"`
		URL            string  `json:"url"`
		Ativo          bool    `json:"ativo"`
		Acessos        int     `json:"acessos"`
		UltimoAcesso   *string `json:"ultimo_acesso"`
		ExpiraEm       *string `json:"expira_em"`
		CreatedAt      string  `json:"created_at"`
	}
	var lista []linkItem
	for rows.Next() {
		var it linkItem
		if err := rows.Scan(&it.ID, &it.NumeroProcesso, &it.NomeCliente, &it.Token,
			&it.Ativo, &it.Acessos, &it.UltimoAcesso, &it.ExpiraEm, &it.CreatedAt); err == nil {
			it.URL = fmt.Sprintf("/api/portal/%s", it.Token)
			lista = append(lista, it)
		}
	}
	if lista == nil {
		out := make([]any, 0)
		writeJSON(w, http.StatusOK, out)
		return
	}
	out := make([]any, len(lista))
	for i, v := range lista {
		out[i] = v
	}
	writeJSON(w, http.StatusOK, out)
}

func revogarPortalLink(w http.ResponseWriter, userID, linkID string) {
	if DBPool == nil {
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := DBPool.Exec(ctx, `
		UPDATE public.portal_links SET ativo = false WHERE id = $1 AND user_id = $2
	`, linkID, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// ─── Rota pública do cliente (sem JWT) ───────────────────────────────────────

// PortalView é o que o cliente enxerga — linguagem leiga, sem dados sensíveis.
type PortalView struct {
	NumeroProcesso  string           `json:"numero_processo"`
	NomeCliente     string           `json:"nome_cliente,omitempty"`
	Tribunal        string           `json:"tribunal,omitempty"`
	Classe          string           `json:"classe,omitempty"`
	Resumo          string           `json:"resumo"`
	Situacao        string           `json:"situacao"`
	UltimaMovimento string           `json:"ultima_movimentacao,omitempty"`
	DataUltimaMov   string           `json:"data_ultima_movimentacao,omitempty"`
	ProximoPasso    string           `json:"proximo_passo"`
	Avisos          []string         `json:"avisos"`
}

func handlePortalPublico(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET"})
		return
	}

	token := strings.TrimPrefix(r.URL.Path, "/api/portal/")
	if token == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token ausente"})
		return
	}

	if DBPool == nil {
		writeJSON(w, http.StatusOK, PortalView{
			NumeroProcesso: "0000000-00.0000.0.00.0000",
			Resumo:         "Modo offline — dados indisponíveis.",
			Situacao:       "em_andamento",
			ProximoPasso:   "Aguarde atualização do seu advogado.",
			Avisos:         []string{},
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var linkID, userID, numeroProcesso string
	var nomeCliente *string
	var ativo bool
	var expiraEm *time.Time

	err := DBPool.QueryRow(ctx, `
		SELECT id, user_id, numero_processo, nome_cliente, ativo, expira_em
		FROM public.portal_links WHERE token = $1
	`, token).Scan(&linkID, &userID, &numeroProcesso, &nomeCliente, &ativo, &expiraEm)

	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "link não encontrado ou expirado"})
		return
	}
	if !ativo {
		writeJSON(w, http.StatusGone, map[string]string{"error": "este link foi revogado pelo advogado"})
		return
	}
	if expiraEm != nil && time.Now().After(*expiraEm) {
		writeJSON(w, http.StatusGone, map[string]string{"error": "link expirado"})
		return
	}

	M.PortalAcessos.Add(1)

	// Incrementa contador de acessos (não bloqueia)
	go func() {
		ctxUp, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = DBPool.Exec(ctxUp, `
			UPDATE public.portal_links
			SET acessos = acessos + 1, ultimo_acesso = now()
			WHERE id = $1
		`, linkID)
	}()

	// Busca o processo na carteira do advogado
	processos := loadProcessos(userID)
	var view PortalView
	view.Avisos = []string{}

	nomeClienteStr := ""
	if nomeCliente != nil {
		nomeClienteStr = *nomeCliente
	}
	view.NomeCliente = nomeClienteStr
	view.NumeroProcesso = numeroProcesso

	for _, p := range processos {
		if p.Numero != numeroProcesso {
			continue
		}
		view.Tribunal = p.Tribunal
		view.Classe = p.Classe

		// Situação em linguagem leiga
		view.Situacao = situacaoLeiga(p.Classe, p.Movimentacoes)
		view.Resumo = resumoLeigo(p)

		// Última movimentação (sem jargão)
		if len(p.Movimentacoes) > 0 {
			ult := p.Movimentacoes[len(p.Movimentacoes)-1]
			view.UltimaMovimento = simplificarMovimento(ult.Descricao)
			view.DataUltimaMov = ult.Data
		}

		view.ProximoPasso = proximoPassoLeigo(p.Movimentacoes)
		break
	}

	if view.Situacao == "" {
		view.Situacao = "em_andamento"
		view.Resumo = "Seu processo está em andamento. Seu advogado acompanha cada etapa."
		view.ProximoPasso = "Aguarde as próximas movimentações."
	}

	writeJSON(w, http.StatusOK, view)
}

// ─── Helpers de linguagem leiga ───────────────────────────────────────────────

func situacaoLeiga(classe string, movs []Movement) string {
	if len(movs) == 0 {
		return "aguardando_inicio"
	}
	ult := strings.ToLower(movs[len(movs)-1].Descricao)
	switch {
	case strings.Contains(ult, "sentença") || strings.Contains(ult, "sentenca"):
		return "sentenca_proferida"
	case strings.Contains(ult, "acordão") || strings.Contains(ult, "acordao"):
		return "julgado_em_segunda_instancia"
	case strings.Contains(ult, "arquiv") || strings.Contains(ult, "extinç") || strings.Contains(ult, "extincao"):
		return "encerrado"
	case strings.Contains(ult, "audiência") || strings.Contains(ult, "audiencia"):
		return "aguardando_audiencia"
	case strings.Contains(ult, "penhora") || strings.Contains(ult, "execução") || strings.Contains(ult, "execucao"):
		return "em_execucao"
	default:
		return "em_andamento"
	}
}

var movSimplificado = map[string]string{
	"citação":     "A outra parte foi informada do processo",
	"citacao":     "A outra parte foi informada do processo",
	"contestação": "A outra parte apresentou sua defesa",
	"contestacao": "A outra parte apresentou sua defesa",
	"sentença":    "O juiz proferiu uma decisão",
	"sentenca":    "O juiz proferiu uma decisão",
	"audiência":   "Foi realizada uma audiência",
	"audiencia":   "Foi realizada uma audiência",
	"penhora":     "Bens foram bloqueados para garantir o pagamento",
	"recurso":     "Foi apresentado um recurso contra a decisão",
	"arquivamento": "O processo foi encerrado",
	"acordo":      "As partes chegaram a um acordo",
}

func simplificarMovimento(descricao string) string {
	lower := strings.ToLower(descricao)
	for kw, leigo := range movSimplificado {
		if strings.Contains(lower, kw) {
			return leigo
		}
	}
	// Trunca descrição técnica
	if len(descricao) > 80 {
		return "Atualização registrada no processo"
	}
	return descricao
}

func resumoLeigo(p StoredProcess) string {
	return fmt.Sprintf(
		"Processo %s no %s. Classe: %s. Seu advogado está acompanhando todas as movimentações.",
		p.Numero, p.Tribunal, p.Classe,
	)
}

func proximoPassoLeigo(movs []Movement) string {
	if len(movs) == 0 {
		return "Aguardando o início das movimentações processuais."
	}
	ult := strings.ToLower(movs[len(movs)-1].Descricao)
	switch {
	case strings.Contains(ult, "sentença") || strings.Contains(ult, "sentenca"):
		return "O juiz já decidiu. Seu advogado avaliará se há necessidade de recurso."
	case strings.Contains(ult, "citação") || strings.Contains(ult, "citacao"):
		return "A outra parte tem prazo para responder. Seu advogado aguarda."
	case strings.Contains(ult, "contestação") || strings.Contains(ult, "contestacao"):
		return "Seu advogado analisará a resposta da outra parte e preparará a próxima peça."
	case strings.Contains(ult, "audiência") || strings.Contains(ult, "audiencia"):
		return "Aguarde o resultado da audiência realizada."
	case strings.Contains(ult, "penhora"):
		return "Os bens bloqueados garantem o pagamento. Seu advogado acompanha a execução."
	default:
		return "Seu advogado está acompanhando o andamento. Qualquer novidade importante, você será informado."
	}
}
