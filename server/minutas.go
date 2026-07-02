package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// GenerateMinutaForIntimacao gera assincronamente um rascunho de petição para a intimação e salva no banco.
func GenerateMinutaForIntimacao(userID string, intimacaoID string, numeroProcesso string, tribunal string, tipoComunicacao string, textoIntimacao string) error {
	if DBPool == nil {
		return nil
	}

	log.Printf("[Minuta Auto] Iniciando geração automática para processo %s (Intimação: %s)...", numeroProcesso, intimacaoID)

	// 1. RAG para obter base legal
	legalContext := searchLegalBasis(textoIntimacao)

	// 2. Identificar tipo da peça sugerida
	tipoPeca := "Manifestação"
	lowerText := strings.ToLower(textoIntimacao)
	switch {
	case strings.Contains(lowerText, "contestação") || strings.Contains(lowerText, "contestar") || strings.Contains(lowerText, "contestacao"):
		tipoPeca = "Contestação"
	case strings.Contains(lowerText, "réplica") || strings.Contains(lowerText, "replica") || strings.Contains(lowerText, "manifeste-se sobre a contestação") || strings.Contains(lowerText, "impugnação à contestação") || strings.Contains(lowerText, "impugnacao a contestacao"):
		tipoPeca = "Réplica à Contestação"
	case strings.Contains(lowerText, "apelação") || strings.Contains(lowerText, "apelacao") || strings.Contains(lowerText, "recurso"):
		tipoPeca = "Recurso de Apelação"
	case strings.Contains(lowerText, "emenda") || strings.Contains(lowerText, "emendar") || strings.Contains(lowerText, "emenda à inicial") || strings.Contains(lowerText, "emenda a inicial"):
		tipoPeca = "Emenda à Petição Inicial"
	}

	// 3. Montar prompt para Ross AI
	systemPrompt := `Você é o Ross AI, assistente jurídico sênior especializado na redação de minutas processuais de alta qualidade.
Seu objetivo é gerar uma minuta de petição inicial de manifestação baseada na intimação do processo e nas bases legais fornecidas pelo RAG.

ESTRUTURA DA PETIÇÃO:
1. Endereçamento e Qualificação genéricos (conforme tribunal e processo).
2. Seção "Dos Fatos" resumindo o andamento atual.
3. Seção "Do Direito" fundamentando juridicamente com base real.
4. Seção "Dos Pedidos" solicitando o deferimento.

REGRAS CRÍTICAS:
- Responda apenas com o texto completo da petição. Não adicione conversas.
- Cite artigos de leis reais (CPC, CC, etc.) fornecidos no RAG.
- Use placeholders entre colchetes para dados específicos da parte (ex: [Nome do Cliente], [CPF]).
`
	if legalContext != "" {
		systemPrompt += "\n\nBASES LEGAIS DO RAG ENCONTRADAS:\n" + legalContext
	}

	userPrompt := fmt.Sprintf("Gere uma minuta do tipo '%s' para o processo nº %s, tribunal %s, com base no seguinte texto da intimação:\n\n%s",
		tipoPeca, numeroProcesso, tribunal, textoIntimacao,
	)

	var reply string
	var err error
	critProvider := os.Getenv("AI_CRITICAL_PROVIDER")
	critModel := os.Getenv("AI_CRITICAL_MODEL")

	chatMessages := []chatMsg{
		{Role: "user", Content: userPrompt},
	}

	if critProvider != "" && critModel != "" {
		reply, err = callCriticalAI(critProvider, critModel, chatMessages, systemPrompt)
	} else {
		var chatReq chatRequest
		chatReq.Mode = string(ModePeca)
		chatReq.Agent = string(AgentGeral)
		chatReq.DocumentContext = textoIntimacao
		chatReq.Messages = chatMessages
		reply, _, err = chatComplete(chatReq)
	}

	if err != nil {
		return fmt.Errorf("falha ao gerar minuta via IA: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = DBPool.Exec(ctx, `
		INSERT INTO public.minutas (user_id, intimacao_id, processo_numero, tipo_peca, conteudo, status)
		VALUES ($1, $2, $3, $4, $5, 'rascunho')
	`, userID, intimacaoID, numeroProcesso, tipoPeca, reply)

	if err != nil {
		return fmt.Errorf("falha ao salvar minuta no banco de dados: %w", err)
	}

	log.Printf("[Minuta Auto] Minuta do tipo '%s' criada com sucesso para intimação %s.", tipoPeca, intimacaoID)
	RegistrarMinutaGerada(true)

	// Busca data de vencimento formatada para o alerta
	var vencStr string
	_ = DBPool.QueryRow(ctx, "SELECT TO_CHAR(vencimento, 'DD/MM/YYYY') FROM public.intimacoes WHERE id = $1", intimacaoID).Scan(&vencStr)
	if vencStr == "" {
		vencStr = "Não determinada"
	}

	// Dispara alertas multi-canal simulados (Fase 3)
	TriggerNewIntimacaoAlert(userID, numeroProcesso, tribunal, tipoPeca, vencStr)

	return nil
}
