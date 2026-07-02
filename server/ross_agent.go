package main

// ross_agent.go — Ross agêntico (Diferencial 9.4)
//
// Detecta intenções de carteira na mensagem do usuário, executa a ferramenta
// Go correspondente e injeta o resultado como contexto antes de chamar a IA.
// Não depende de função de tool-calling nativa do modelo: funciona com qualquer
// provider (OpenRouter free, Gemini, etc.).
//
// Ferramentas disponíveis:
//   • listar_prazos          — prazos oficiais e estimados do usuário
//   • listar_minutas         — minutas geradas automaticamente
//   • buscar_divergencias    — processos com divergência DJEN × DataJud
//   • criar_vigilancia       — cadastra CPF/CNPJ para monitoramento
//   • listar_vigilancias     — vigílias ativas do usuário
//   • listar_alertas_vigia   — alertas de novos processos detectados

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ─── Detecção de intenção ─────────────────────────────────────────────────────

type agentTool string

const (
	toolListarPrazos        agentTool = "listar_prazos"
	toolListarMinutas       agentTool = "listar_minutas"
	toolBuscarDivergencias  agentTool = "buscar_divergencias"
	toolCriarVigilancia     agentTool = "criar_vigilancia"
	toolListarVigilancias   agentTool = "listar_vigilancias"
	toolListarAlertasVigia  agentTool = "listar_alertas_vigia"
	toolNenhuma             agentTool = ""
)

type toolIntent struct {
	Tool agentTool
	Args map[string]string // parâmetros opcionais extraídos do texto
}

// detectIntent analisa o texto da última mensagem do usuário para inferir
// qual ferramenta da carteira deve ser chamada (se alguma).
func detectIntent(mensagem string) toolIntent {
	m := strings.ToLower(mensagem)

	// Vigilância — criar
	if containsAny(m, "vigiar", "monitorar cpf", "monitorar cnpj", "criar vigilância",
		"criar vigilancia", "adicionar vigilância", "adicionar vigilancia",
		"watch", "acompanhar cpf", "acompanhar cnpj") {
		doc := extractDocumento(mensagem)
		nome := extractNomeVigilancia(mensagem)
		return toolIntent{Tool: toolCriarVigilancia, Args: map[string]string{"documento": doc, "nome": nome}}
	}

	// Vigilância — listar alertas
	if containsAny(m, "alerta", "alertas da vigília", "alertas de vigilância", "novo processo detectado",
		"novidades da vigilância", "o que mudou") {
		return toolIntent{Tool: toolListarAlertasVigia}
	}

	// Vigilância — listar
	if containsAny(m, "vigilância", "vigilancias", "quem estou monitorando",
		"partes monitoradas", "minhas vigilâncias") {
		return toolIntent{Tool: toolListarVigilancias}
	}

	// Divergências
	if containsAny(m, "divergência", "divergencias", "conflito de prazo",
		"prazo errado", "prazo diferente", "inconsistência de prazo") {
		return toolIntent{Tool: toolBuscarDivergencias}
	}

	// Minutas
	if containsAny(m, "minuta", "minutas", "rascunho", "rascunhos", "peça gerada",
		"petição sugerida", "contestação sugerida", "réplica sugerida") {
		return toolIntent{Tool: toolListarMinutas}
	}

	// Prazos — keywords mais abrangentes
	if containsAny(m, "prazo", "prazos", "vence", "vencimento", "vencendo",
		"urgente", "hoje", "agenda", "carteira", "semana", "mês",
		"o que tenho", "o que vence", "minhas intimações", "intimação",
		"perder prazo", "não perder") {
		return toolIntent{Tool: toolListarPrazos}
	}

	return toolIntent{Tool: toolNenhuma}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// extractDocumento tenta extrair CPF (11 dígitos) ou CNPJ (14 dígitos) do texto.
func extractDocumento(s string) string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return ' '
	}, s)
	for _, token := range strings.Fields(digits) {
		t := strings.ReplaceAll(token, " ", "")
		if len(t) == 11 || len(t) == 14 {
			return t
		}
	}
	return ""
}

// extractNomeVigilancia tenta extrair o nome após "de" ou "para" no texto.
func extractNomeVigilancia(s string) string {
	lower := strings.ToLower(s)
	for _, prefix := range []string{"nome ", "de ", "para ", "chamado ", "chamada "} {
		if idx := strings.Index(lower, prefix); idx >= 0 {
			after := strings.TrimSpace(s[idx+len(prefix):])
			// Pega até vírgula, ponto ou parêntese
			end := strings.IndexAny(after, ",.;:()")
			if end > 0 {
				after = after[:end]
			}
			after = strings.TrimSpace(after)
			if len(after) >= 3 {
				return after
			}
		}
	}
	return "Parte monitorada"
}

// ─── Execução das ferramentas ─────────────────────────────────────────────────

// ExecuteAgentTool executa a ferramenta detectada e retorna um bloco de contexto
// pronto para ser injetado no system prompt da chamada à IA.
func ExecuteAgentTool(intent toolIntent, userID string) string {
	switch intent.Tool {

	case toolListarPrazos:
		return executarListarPrazos(userID)

	case toolListarMinutas:
		return executarListarMinutas(userID)

	case toolBuscarDivergencias:
		return executarBuscarDivergencias(userID)

	case toolCriarVigilancia:
		return executarCriarVigilancia(userID, intent.Args)

	case toolListarVigilancias:
		return executarListarVigilancias(userID)

	case toolListarAlertasVigia:
		return executarListarAlertasVigia(userID)
	}
	return ""
}

func executarListarPrazos(userID string) string {
	prazos := computePrazos(userID)
	if len(prazos) == 0 {
		return "[FERRAMENTA listar_prazos] Nenhum prazo encontrado na carteira do usuário."
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[FERRAMENTA listar_prazos] %d prazo(s) encontrado(s). Data de referência: %s\n\n",
		len(prazos), time.Now().Format("02/01/2006")))
	for _, p := range prazos {
		sb.WriteString(fmt.Sprintf("• Processo %s | %s | %s | Vence: %s | Restam: %d dia(s) útil(eis) | Status: %s | Fonte: %s\n",
			p.Numero, p.Tribunal, p.Rotulo, p.Vencimento, p.DiasRestantes, p.Status, p.TipoContagem))
	}
	sb.WriteString("\nAo responder, cite os dados acima literalmente. Não invente prazos adicionais.")
	return sb.String()
}

func executarListarMinutas(userID string) string {
	if DBPool == nil {
		return "[FERRAMENTA listar_minutas] Banco de dados offline — minutas indisponíveis."
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := DBPool.Query(ctx, `
		SELECT processo_numero, tipo_peca, status, TO_CHAR(created_at, 'DD/MM/YYYY')
		FROM public.minutas
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 20
	`, userID)
	if err != nil {
		return fmt.Sprintf("[FERRAMENTA listar_minutas] Erro ao consultar banco: %v", err)
	}
	defer rows.Close()

	var sb strings.Builder
	count := 0
	for rows.Next() {
		var proc, tipo, status, data string
		if err := rows.Scan(&proc, &tipo, &status, &data); err == nil {
			sb.WriteString(fmt.Sprintf("• %s | %s | Status: %s | Criada em: %s\n", proc, tipo, status, data))
			count++
		}
	}
	if count == 0 {
		return "[FERRAMENTA listar_minutas] Nenhuma minuta gerada ainda."
	}
	return fmt.Sprintf("[FERRAMENTA listar_minutas] %d minuta(s):\n\n%s\nAo responder, cite os dados acima literalmente.", count, sb.String())
}

func executarBuscarDivergencias(userID string) string {
	if DBPool == nil {
		return "[FERRAMENTA buscar_divergencias] Banco de dados offline."
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := DBPool.Query(ctx, `
		SELECT numero_processo, regra, TO_CHAR(vencimento, 'DD/MM/YYYY'), detalhe_divergencia
		FROM public.prazo_auditoria
		WHERE user_id = $1 AND divergencia = true
		ORDER BY created_at DESC
		LIMIT 20
	`, userID)
	if err != nil {
		return fmt.Sprintf("[FERRAMENTA buscar_divergencias] Erro: %v", err)
	}
	defer rows.Close()

	var sb strings.Builder
	count := 0
	for rows.Next() {
		var proc, regra, venc, detalhe string
		if err := rows.Scan(&proc, &regra, &venc, &detalhe); err == nil {
			sb.WriteString(fmt.Sprintf("• Processo %s | %s | Vencimento (DJEN): %s\n  Detalhe: %s\n", proc, regra, venc, detalhe))
			count++
		}
	}
	if count == 0 {
		return "[FERRAMENTA buscar_divergencias] Nenhuma divergência detectada. Prazos DJEN e DataJud estão consistentes."
	}
	return fmt.Sprintf("[FERRAMENTA buscar_divergencias] ⚠️ %d divergência(s) detectada(s):\n\n%s\nAlerte o usuário sobre cada processo e recomende verificação manual.", count, sb.String())
}

func executarCriarVigilancia(userID string, args map[string]string) string {
	doc := args["documento"]
	nome := args["nome"]
	if doc == "" {
		return "[FERRAMENTA criar_vigilancia] Não consegui identificar o CPF ou CNPJ no texto. Peça ao usuário que informe o número."
	}
	if len(doc) != 11 && len(doc) != 14 {
		return "[FERRAMENTA criar_vigilancia] Documento inválido. Deve ter 11 dígitos (CPF) ou 14 (CNPJ)."
	}
	if nome == "" {
		nome = "Parte monitorada"
	}

	if DBPool == nil {
		return fmt.Sprintf("[FERRAMENTA criar_vigilancia] Modo offline — vigília para %s (%s) seria criada aqui.", nome, doc)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var vigID string
	err := DBPool.QueryRow(ctx, `
		INSERT INTO public.vigilancias (user_id, documento, nome, tipo, tribunais)
		VALUES ($1, $2, $3, 'parte', '{}')
		ON CONFLICT (user_id, documento)
		DO UPDATE SET nome = EXCLUDED.nome, ativo = true, updated_at = now()
		RETURNING id
	`, userID, doc, nome).Scan(&vigID)
	if err != nil {
		return fmt.Sprintf("[FERRAMENTA criar_vigilancia] Erro ao salvar: %v", err)
	}

	// Inicia verificação inicial
	go RunVigilanciaCheck(vigID, userID, doc, nome, "parte", []string{})

	return fmt.Sprintf("[FERRAMENTA criar_vigilancia] ✅ Vigília criada com sucesso para %s (doc: %s). A primeira verificação está em andamento.", nome, doc)
}

func executarListarVigilancias(userID string) string {
	if DBPool == nil {
		return "[FERRAMENTA listar_vigilancias] Banco de dados offline."
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := DBPool.Query(ctx, `
		SELECT documento, nome, tipo, array_to_string(tribunais, ','), ultima_verificacao, ativo
		FROM public.vigilancias
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return fmt.Sprintf("[FERRAMENTA listar_vigilancias] Erro: %v", err)
	}
	defer rows.Close()

	var sb strings.Builder
	count := 0
	for rows.Next() {
		var doc, nome, tipo, tribunais string
		var ultimaVer *time.Time
		var ativo bool
		if err := rows.Scan(&doc, &nome, &tipo, &tribunais, &ultimaVer, &ativo); err == nil {
			status := "🟢 ativo"
			if !ativo {
				status = "🔴 inativo"
			}
			ult := "nunca verificado"
			if ultimaVer != nil {
				ult = ultimaVer.Format("02/01/2006 15:04")
			}
			sb.WriteString(fmt.Sprintf("• %s (%s) | %s | %s | Última verificação: %s\n", nome, doc, tipo, status, ult))
			count++
		}
	}
	if count == 0 {
		return "[FERRAMENTA listar_vigilancias] Nenhuma vigília cadastrada."
	}
	return fmt.Sprintf("[FERRAMENTA listar_vigilancias] %d vigília(s):\n\n%s", count, sb.String())
}

func executarListarAlertasVigia(userID string) string {
	if DBPool == nil {
		return "[FERRAMENTA listar_alertas_vigia] Banco de dados offline."
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := DBPool.Query(ctx, `
		SELECT tipo_alerta, numero_processo, descricao, lido, TO_CHAR(created_at, 'DD/MM/YYYY')
		FROM public.vigilancia_alertas
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 30
	`, userID)
	if err != nil {
		return fmt.Sprintf("[FERRAMENTA listar_alertas_vigia] Erro: %v", err)
	}
	defer rows.Close()

	var sb strings.Builder
	count := 0
	for rows.Next() {
		var tipoAlerta, descricao, data string
		var numProc *string
		var lido bool
		if err := rows.Scan(&tipoAlerta, &numProc, &descricao, &lido, &data); err == nil {
			lidoStr := ""
			if !lido {
				lidoStr = " [NOVO]"
			}
			proc := ""
			if numProc != nil {
				proc = " | Processo: " + *numProc
			}
			sb.WriteString(fmt.Sprintf("• [%s]%s%s — %s (%s)\n", tipoAlerta, lidoStr, proc, descricao, data))
			count++
		}
	}
	if count == 0 {
		return "[FERRAMENTA listar_alertas_vigia] Nenhum alerta de vigília encontrado."
	}
	return fmt.Sprintf("[FERRAMENTA listar_alertas_vigia] %d alerta(s):\n\n%s\nDestacar os [NOVO]s ao usuário.", count, sb.String())
}

// ─── Integração no chatRequest ────────────────────────────────────────────────

// EnrichChatWithAgentContext verifica se a última mensagem do usuário contém
// intenção de carteira e, se sim, injeta o resultado da ferramenta no DocumentContext.
// Retorna o chatRequest enriquecido (sem modificar o original).
func EnrichChatWithAgentContext(req chatRequest, userID string) chatRequest {
	if len(req.Messages) == 0 {
		return req
	}
	lastMsg := req.Messages[len(req.Messages)-1]
	if lastMsg.Role != "user" {
		return req
	}

	intent := detectIntent(lastMsg.Content)
	if intent.Tool == toolNenhuma {
		return req
	}

	toolResult := ExecuteAgentTool(intent, userID)
	if toolResult == "" {
		return req
	}

	enriched := req
	if enriched.DocumentContext != "" {
		enriched.DocumentContext = enriched.DocumentContext + "\n\n" + toolResult
	} else {
		enriched.DocumentContext = toolResult
	}

	// Log para observabilidade
	_, _ = json.Marshal(map[string]string{"tool": string(intent.Tool), "user": userID})

	return enriched
}
