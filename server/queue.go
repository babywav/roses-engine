package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StartAnalysisQueueWorker inicia o loop periódico do processador de tarefas da fila.
func StartAnalysisQueueWorker() {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				processNextJob()
			}
		}
	}()
	log.Println("[Queue] Processador de fila de análise inicializado.")
}

func processNextJob() {
	if DBPool == nil {
		return
	}

	ctx := context.Background()
	tx, err := DBPool.Begin(ctx)
	if err != nil {
		log.Printf("[Queue] Erro ao iniciar transação: %v", err)
		return
	}
	defer tx.Rollback(ctx)

	// Busca a próxima tarefa da fila travando a linha (FOR UPDATE SKIP LOCKED)
	var jobID, userID, docID, docName string
	var procID *string
	var retryCount int

	err = tx.QueryRow(ctx, `
		SELECT id, user_id, documento_id, nome_documento, processo_id, retry_count
		FROM public.jobs_analise
		WHERE status IN ('pending', 'waiting_retry')
		  AND (next_retry_at IS NULL OR next_retry_at <= now())
		ORDER BY created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`).Scan(&jobID, &userID, &docID, &docName, &procID, &retryCount)

	if err != nil {
		// Nenhuma tarefa encontrada, transação aborta silenciosamente
		return
	}

	// Altera status para 'running'
	_, err = tx.Exec(ctx, `
		UPDATE public.jobs_analise
		SET status = 'running', updated_at = now()
		WHERE id = $1
	`, jobID)
	if err != nil {
		log.Printf("[Queue] Erro ao atualizar status do job %s para running: %v", jobID, err)
		return
	}

	// Commit para liberar a linha como 'running' para outros workers
	if err := tx.Commit(ctx); err != nil {
		log.Printf("[Queue] Erro ao commitar transação inicial para o job %s: %v", jobID, err)
		return
	}

	log.Printf("[Queue] Iniciando processamento da tarefa %s (Doc: %s)...", jobID, docName)

	// Inicia processamento real fora da transação de fila
	resultJSON, err := executeJobAnalysis(userID, docID, docName)

	if err != nil {
		log.Printf("[Queue] Falha ao processar job %s: %v", jobID, err)
		handleJobFailure(jobID, retryCount, err.Error())
	} else {
		log.Printf("[Queue] Job %s concluído com sucesso!", jobID)
		handleJobSuccess(jobID, userID, docID, docName, procID, resultJSON)
	}
}

func executeJobAnalysis(userID, docID, docName string) ([]byte, error) {
	// 1. Download do Storage do Supabase
	fileBytes, err := downloadFromStorage(docID)
	if err != nil {
		return nil, fmt.Errorf("download falhou: %w", err)
	}

	// 2. Extração de texto
	text, err := ExtractText(docName, fileBytes)
	if err != nil {
		return nil, fmt.Errorf("extração de texto falhou: %w", err)
	}

	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("texto extraído está vazio ou o documento não possui camada de texto legível")
	}

	// 3. Grounding Legal (RAG)
	// Busca na base jurídica termos chave do texto do documento
	var searchTerms []string
	words := strings.Fields(text)
	for i, w := range words {
		if i > 15 { // Usa apenas as primeiras palavras-chave relevantes para o ranking
			break
		}
		if len(w) > 4 {
			searchTerms = append(searchTerms, w)
		}
	}
	legalContext := ""
	if len(searchTerms) > 0 {
		legalContext = searchLegalBasis(strings.Join(searchTerms, " "))
	}

	// 4. Executa chamada da IA e processa os 13 campos
	result, err := runAIAnalysis(text, legalContext)
	if err != nil {
		return nil, fmt.Errorf("análise de IA falhou: %w", err)
	}

	return json.Marshal(result)
}

func runAIAnalysis(text string, legalBasis string) (AnaliseResult, error) {
	// Limita contexto do prompt
	promptContext := text
	if len(promptContext) > 60000 {
		promptContext = promptContext[:60000] + "\n... [Documento longo cortado]"
	}

	systemInstruction := AnaliseEstruturadaPrompt
	if legalBasis != "" {
		systemInstruction += "\n\nBASES LEGAIS DO RAG ENCONTRADAS NO BANCO (Use para fundamentação):\n" + legalBasis
	}

	var chatReq chatRequest
	chatReq.Mode = string(ModeAnalise)
	chatReq.Agent = string(AgentGeral)
	chatReq.DocumentContext = promptContext
	chatReq.Messages = []chatMsg{
		{Role: "user", Content: "Analise o caso estruturado do documento em anexo."},
	}

	var reply string
	var err error

	critProvider := os.Getenv("AI_CRITICAL_PROVIDER")
	critModel := os.Getenv("AI_CRITICAL_MODEL")

	if critProvider != "" && critModel != "" {
		systemPrompt := buildSystemPrompt(ModeAnalise, AgentGeral, promptContext)
		if legalBasis != "" {
			systemPrompt = systemPrompt + "\n\nBASES LEGAIS DO RAG ENCONTRADAS NO BANCO (Use para fundamentação):\n" + legalBasis
		}
		reply, err = callCriticalAI(critProvider, critModel, chatReq.Messages, systemPrompt)
	} else {
		reply, _, err = chatComplete(chatReq)
	}

	if err != nil {
		return AnaliseResult{}, err
	}

	return repairAndParseJSON(reply)
}

func callCriticalAI(provider string, model string, messages []chatMsg, systemPrompt string) (string, error) {
	switch strings.ToLower(provider) {
	case "openrouter":
		return callOpenRouter(model, messages, systemPrompt)
	case "gemini":
		return callGemini(model, messages, systemPrompt)
	default:
		return "", fmt.Errorf("provider crítico '%s' não mapeado", provider)
	}
}

func repairAndParseJSON(raw string) (AnaliseResult, error) {
	clean := strings.TrimSpace(raw)

	// Remove cercas de código markdown do JSON
	if strings.HasPrefix(clean, "```json") {
		clean = strings.TrimPrefix(clean, "```json")
		if strings.HasSuffix(clean, "```") {
			clean = strings.TrimSuffix(clean, "```")
		}
	} else if strings.HasPrefix(clean, "```") {
		clean = strings.TrimPrefix(clean, "```")
		if strings.HasSuffix(clean, "```") {
			clean = strings.TrimSuffix(clean, "```")
		}
	}
	clean = strings.TrimSpace(clean)

	// Filtra tudo antes do primeiro { e depois do último }
	first := strings.Index(clean, "{")
	last := strings.LastIndex(clean, "}")
	if first >= 0 && last >= 0 && last > first {
		clean = clean[first : last+1]
	}

	var res AnaliseResult
	err := json.Unmarshal([]byte(clean), &res)
	if err != nil {
		return AnaliseResult{}, fmt.Errorf("falha ao analisar JSON estruturado da IA: %w", err)
	}

	// Normaliza campos vazios para não quebrar contrato de frontend
	if res.Summary == "" {
		res.Summary = "Resumo indisponível"
	}
	if res.KeyObservations == "" {
		res.KeyObservations = "Nenhuma observação cadastrada"
	}

	return res, nil
}

func downloadFromStorage(documentoID string) ([]byte, error) {
	supabaseURL := os.Getenv("SUPABASE_URL")
	serviceKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
	if supabaseURL == "" || serviceKey == "" {
		return nil, fmt.Errorf("SUPABASE_URL ou SUPABASE_SERVICE_ROLE_KEY não configurada")
	}

	// URL do Supabase Storage
	apiURL := fmt.Sprintf("%s/storage/v1/object/documents/%s", supabaseURL, documentoID)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+serviceKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("download storage falhou com status %d: %s", resp.StatusCode, string(raw))
	}

	return io.ReadAll(resp.Body)
}

func handleJobSuccess(jobID, userID, docID, docName string, procID *string, resultJSON []byte) {
	if DBPool == nil {
		return
	}

	ctx := context.Background()
	// Salva na tabela analises
	_, err := DBPool.Exec(ctx, `
		INSERT INTO public.analises (user_id, documento_id, nome_documento, processo_id, resultado)
		VALUES ($1, $2, $3, $4, $5)
	`, userID, docID, docName, procID, resultJSON)
	if err != nil {
		log.Printf("[Queue] Erro ao gravar resultado na tabela analises: %v", err)
	}

	// Marca o job como concluído
	_, err = DBPool.Exec(ctx, `
		UPDATE public.jobs_analise
		SET status = 'done', resultado = $1, error_message = NULL, updated_at = now()
		WHERE id = $2
	`, resultJSON, jobID)
	if err != nil {
		log.Printf("[Queue] Erro ao marcar job %s como done: %v", jobID, err)
	}
}

func handleJobFailure(jobID string, retryCount int, errorMsg string) {
	if DBPool == nil {
		return
	}

	ctx := context.Background()
	maxRetries := 3

	if retryCount < maxRetries {
		// Agenda nova retentativa para daqui a 2 minutos
		nextRetry := time.Now().Add(2 * time.Minute)
		_, err := DBPool.Exec(ctx, `
			UPDATE public.jobs_analise
			SET status = 'waiting_retry', error_message = $1, retry_count = retry_count + 1, next_retry_at = $2, updated_at = now()
			WHERE id = $3
		`, errorMsg, nextRetry, jobID)
		if err != nil {
			log.Printf("[Queue] Erro ao reagendar job %s: %v", jobID, err)
		}
	} else {
		// Falha definitiva
		_, err := DBPool.Exec(ctx, `
			UPDATE public.jobs_analise
			SET status = 'error', error_message = $1, updated_at = now()
			WHERE id = $2
		`, errorMsg, jobID)
		if err != nil {
			log.Printf("[Queue] Erro ao marcar job %s como falhado definitivamente: %v", jobID, err)
		}
	}
}

func uploadToStorage(filename string, data []byte, contentType string) (string, error) {
	supabaseURL := os.Getenv("SUPABASE_URL")
	serviceKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")

	if supabaseURL == "" || serviceKey == "" {
		// Fallback para armazenamento em disco local durante desenvolvimento offline
		uniqueName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filename)
		localDir := filepath.Join("..", "data", "storage")
		_ = os.MkdirAll(localDir, 0755)
		localPath := filepath.Join(localDir, uniqueName)
		err := os.WriteFile(localPath, data, 0644)
		if err != nil {
			return "", fmt.Errorf("falha ao salvar arquivo localmente: %w", err)
		}
		log.Printf("[Storage Offline] Arquivo salvo em: %s", localPath)
		return uniqueName, nil
	}

	uniqueName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filename)
	apiURL := fmt.Sprintf("%s/storage/v1/object/documents/%s", supabaseURL, uniqueName)

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+serviceKey)
	req.Header.Set("Content-Type", contentType)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload storage falhou com status %d: %s", resp.StatusCode, string(raw))
	}

	return uniqueName, nil
}

