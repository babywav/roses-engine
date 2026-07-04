package main

import (
	"bufio"
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

// loadDotEnv carrega variaveis de um arquivo .env (KEY=VALUE) sem sobrescrever
// as que ja existem no ambiente. Procura em ./.env e ../.env (raiz do roses).
func loadDotEnv() {
	for _, path := range []string{".env", filepath.Join("..", ".env")} {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			eq := strings.Index(line, "=")
			if eq < 1 {
				continue
			}
			key := strings.TrimSpace(line[:eq])
			val := strings.TrimSpace(line[eq+1:])
			val = strings.Trim(val, `"'`)
			if _, exists := os.LookupEnv(key); !exists {
				os.Setenv(key, val)
			}
		}
		f.Close()
	}
}

type consultaRequest struct {
	Tipo  string `json:"tipo"`  // "cnj" | "oab" | "nome" | "advogado"
	Valor string `json:"valor"`
	UF    string `json:"uf"`
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// --- middleware ------------------------------------------------------------

func withCommon(h http.HandlerFunc) http.HandlerFunc {
	origins := getenv("ROSES_CORS_ORIGINS", "*")
	apiKey := os.Getenv("ROSES_API_KEY")
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[Request Common] Method: %s, Path: %s, URI: %s", r.Method, r.URL.Path, r.RequestURI)
		w.Header().Set("Access-Control-Allow-Origin", origins)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if apiKey != "" && r.Header.Get("X-API-Key") != apiKey {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "API key invalida ou ausente."})
			return
		}
		h(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// --- handlers --------------------------------------------------------------

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "online", "engine": "Roses", "version": "1.0.0",
	})
}

func handleConsulta(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}
	var req consultaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON invalido"})
		return
	}
	req.Tipo = strings.ToLower(strings.TrimSpace(req.Tipo))
	req.UF = strings.ToUpper(strings.TrimSpace(req.UF))

	start := time.Now()
	var res Result
	switch req.Tipo {
	case "cnj":
		res = datajudByCNJ(req.Valor)
		if res.Status == "NOT_FOUND" || res.Status == "ERROR" {
			// fallback opcional pro portal por numero
			if alt := portalConsulta("cnj", req.Valor, req.UF, 120); alt.Status == "OK" {
				res = alt
			}
		}
	case "oab", "nome", "advogado":
		res = portalConsulta(req.Tipo, req.Valor, req.UF, 120)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tipo deve ser cnj, oab, nome ou advogado"})
		return
	}
	res.ElapsedSeconds = time.Since(start).Seconds()
	if res.Status == "OK" && len(res.Processos) > 0 {
		saveProcessos(userID, res.Processos) // alimenta o cálculo de Oportunidades
	}
	writeJSON(w, http.StatusOK, res)
}

func handleOportunidades(w http.ResponseWriter, r *http.Request) {
	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}
	ops := computeOportunidades(userID)
	writeJSON(w, http.StatusOK, map[string]any{
		"total":         len(ops),
		"oportunidades": ops,
	})
}

func handleBrightdata(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("url")
	if target == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "passe ?url=..."})
		return
	}
	html, status, err := brightdataFetch(target)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"error": err.Error()})
		return
	}
	snippet := html
	if len(snippet) > 600 {
		snippet = snippet[:600]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"http_status": status,
		"bytes":       len(html),
		"snippet":     snippet,
	})
}

func handleTranscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	if err := r.ParseMultipartForm(25 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "form invalido"})
		return
	}
	file, hdr, err := r.FormFile("audio")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "campo 'audio' ausente"})
		return
	}
	defer file.Close()
	data, _ := io.ReadAll(file)
	mime := hdr.Header.Get("Content-Type")

	text, provider, err := transcribeAudio(data, mime, hdr.Filename)
	if err != nil {
		log.Printf("[transcribe] falha (%d bytes, mime=%s): %v", len(data), mime, err)
		writeJSON(w, http.StatusOK, map[string]string{"text": "", "error": err.Error()})
		return
	}
	log.Printf("[transcribe] ok via %s (%d bytes)", provider, len(data))
	writeJSON(w, http.StatusOK, map[string]string{"text": text, "provider": provider})
}

// --- chat IA (Ross AI) ------------------------------------------------------

type chatRequest struct {
	Messages        []chatMsg `json:"messages"`
	Mode            string    `json:"mode,omitempty"`
	Agent           string    `json:"agent,omitempty"`
	DocumentContext string    `json:"documentContext,omitempty"`
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	userID, _ := GetUserIDFromContext(r.Context()) // opcional: sem auth não enriquece
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON invalido"})
		return
	}
	if len(req.Messages) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mensagens vazias"})
		return
	}
	// Mantem apenas o histórico recente (custo/limite de contexto).
	if len(req.Messages) > 24 {
		req.Messages = req.Messages[len(req.Messages)-24:]
	}
	// Ross agêntico (9.4): injeta dados reais da carteira quando detecta intenção
	if userID != "" {
		req = EnrichChatWithAgentContext(req, userID)
	}
	reply, model, err := chatComplete(req)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"reply": "Não consegui falar com a IA agora: " + err.Error(),
			"error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"reply": reply, "model": model})
}

// handleChatStream responde em SSE — tokens chegam em tempo real.
func handleChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"use POST"}`, http.StatusMethodNotAllowed)
		return
	}
	userID, _ := GetUserIDFromContext(r.Context())
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"JSON invalido"}`, http.StatusBadRequest)
		return
	}
	if len(req.Messages) == 0 {
		http.Error(w, `{"error":"mensagens vazias"}`, http.StatusBadRequest)
		return
	}
	if len(req.Messages) > 24 {
		req.Messages = req.Messages[len(req.Messages)-24:]
	}
	// Ross agêntico (9.4): injeta dados reais da carteira quando detecta intenção
	if userID != "" {
		req = EnrichChatWithAgentContext(req, userID)
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		fmt.Fprintf(w, "data: {\"error\":\"streaming nao suportado\"}\n\ndata: [DONE]\n\n")
		return
	}

	if err := chatCompleteStream(req, w, flusher); err != nil {
		log.Printf("[stream] providers falharam: %v — fallback word-by-word", err)
		reply, _, fallbackErr := chatComplete(req)
		if fallbackErr != nil {
			fmt.Fprintf(w, "data: {\"error\":\"IA indisponível\"}\n\n")
			flusher.Flush()
		} else {
			words := strings.Fields(reply)
			for i, word := range words {
				tok := word
				if i < len(words)-1 {
					tok += " "
				}
				tj, _ := json.Marshal(map[string]string{"t": tok})
				fmt.Fprintf(w, "data: %s\n\n", tj)
				flusher.Flush()
				time.Sleep(20 * time.Millisecond)
			}
		}
	}
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// --- static (SPA) ----------------------------------------------------------

func spaHandler(dir string) http.HandlerFunc {
	fs := http.FileServer(http.Dir(dir))
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[Request SPA] Method: %s, Path: %s, URI: %s", r.Method, r.URL.Path, r.RequestURI)
		path := filepath.Join(dir, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		// fallback SPA -> index.html
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	}
}

func main() {
	loadDotEnv()
	initDB()
	initCognitoJWKS()
	
	// Registra métricas de inicialização
	StartBackgroundSyncWorker()
	StartAnalysisQueueWorker()
	StartVigilanciaWorker()
	SeedBaseJuridica()
	port := getenv("ROSES_PORT", "8080")
	webDir := getenv("ROSES_WEB_DIR", filepath.Join("..", "public"))

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", withCommon(handleHealth))
	mux.HandleFunc("/api/consulta", withAuth(handleConsulta))
	mux.HandleFunc("/api/chat", withAuth(handleChat))
	mux.HandleFunc("/api/chat/stream", withAuth(handleChatStream))
	mux.HandleFunc("/api/oportunidades", withAuth(handleOportunidades))
	mux.HandleFunc("/api/casos", withAuth(handleCasos))
	mux.HandleFunc("/api/dashboard", withAuth(handleDashboard))
	mux.HandleFunc("/api/notificacoes", withAuth(handleNotificacoes))
	mux.HandleFunc("/api/transcribe", withAuth(handleTranscribe))
	mux.HandleFunc("/api/brightdata", withCommon(handleBrightdata))
	mux.HandleFunc("/api/djen/sync", withAuth(handleDjenSync))
	mux.HandleFunc("/api/analise/upload", withAuth(handleAnaliseUpload))
	mux.HandleFunc("/api/analise/job/", withAuth(handleAnaliseJobStatus))
	mux.HandleFunc("/api/minutas/", withAuth(handleMinutas))
	mux.HandleFunc("/api/auditoria/divergencias", withAuth(handleAuditoriaDivergencias))
	mux.HandleFunc("/api/auditoria/", withAuth(handleAuditoriaProcesso))
	mux.HandleFunc("/api/vigilancias/alertas", withAuth(handleVigilancias))
	mux.HandleFunc("/api/vigilancias/", withAuth(handleVigilancias))
	mux.HandleFunc("/api/vigilancias", withAuth(handleVigilancias))
	mux.HandleFunc("/api/calculos", withAuth(handleCalculos))
	mux.HandleFunc("/api/calculos/historico", withAuth(handleHistoricoCalculos))
	mux.HandleFunc("/api/portal/links", withAuth(handlePortalLinks))
	mux.HandleFunc("/api/portal/links/", withAuth(handlePortalLinks))
	mux.HandleFunc("/api/portal/", handlePortalPublico) // sem auth — acesso público por token
	mux.HandleFunc("/api/conta/exportar", withAuth(handleExportarDados))
	mux.HandleFunc("/api/conta", withAuth(handleExcluirConta))
	mux.HandleFunc("/api/perfil", withAuth(handleGetPerfil))
	mux.HandleFunc("/api/equipe/membros", withAuth(handleGetMembros))
	mux.HandleFunc("/api/equipe/membros/adicionar", withAuth(handleAdicionarMembro))
	mux.HandleFunc("/api/equipe/membros/remover", withAuth(handleRemoverMembro))
	mux.HandleFunc("/api/admin/metricas", withAdminKey(handleMetricas))
	mux.HandleFunc("/api/config", withCommon(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"COGNITO_REGION":    os.Getenv("AWS_REGION"),
			"COGNITO_CLIENT_ID": os.Getenv("COGNITO_CLIENT_ID"),
		})
	}))

	// Front-end (build do Vite). Se nao existir, so a API responde.
	if _, err := os.Stat(webDir); err == nil {
		mux.HandleFunc("/", spaHandler(webDir))
		log.Printf("[Roses] Servindo front-end de %s", webDir)
	} else {
		log.Printf("[Roses] web/dist nao encontrado (%s) — rodando so a API", webDir)
	}

	log.Printf("[Roses] Backend Go ouvindo em http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

type djenSyncRequest struct {
	Oab   string `json:"oab"`
	Uf    string `json:"uf"`
	Desde string `json:"desde"`
}

func handleDjenSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}

	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	var req djenSyncRequest
	_ = json.NewDecoder(r.Body).Decode(&req) // Ignore decoding errors to allow empty body/query defaults

	oab := strings.TrimSpace(req.Oab)
	uf := strings.TrimSpace(req.Uf)
	desde := strings.TrimSpace(req.Desde)

	// Busca OAB/UF no perfil do banco se ausentes na requisição
	if (oab == "" || uf == "") && DBPool != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := DBPool.QueryRow(ctx, `
			SELECT oab_numero, oab_uf
			FROM public.perfis
			WHERE id = $1
		`, userID).Scan(&oab, &uf)

		if err != nil {
			log.Printf("[DJEN] Perfil de OAB/UF não encontrado para o usuário %s: %v", userID, err)
		}
	}

	if oab == "" || uf == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "OAB e UF são obrigatórios (devem ser passados na requisição ou cadastrados no perfil)"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	novas, err := SyncDJEN(ctx, userID, oab, uf, desde)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":          true,
		"novas_intimacoes": novas,
		"oab":              oab,
		"uf":               uf,
		"desde":            desde,
	})
}

func handleAnaliseUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}

	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	err = r.ParseMultipartForm(50 << 20) // limite de 50MB
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "form inválido ou arquivo muito grande"})
		return
	}

	file, header, err := r.FormFile("documento")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "campo 'documento' ausente"})
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "falha ao ler arquivo"})
		return
	}

	contentType := header.Header.Get("Content-Type")
	filename := header.Filename

	// Opcional processo_id
	var processoID *string
	if pid := r.FormValue("processo_id"); pid != "" {
		processoID = &pid
	}

	// 1. Upload do arquivo (local ou Supabase)
	docID, err := uploadToStorage(filename, fileBytes, contentType)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 2. Insere na fila jobs_analise
	var jobID string
	if DBPool != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = DBPool.QueryRow(ctx, `
			INSERT INTO public.jobs_analise (user_id, documento_id, nome_documento, processo_id, status)
			VALUES ($1, $2, $3, $4, 'pending')
			RETURNING id
		`, userID, docID, filename, processoID).Scan(&jobID)

		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("falha ao enfileirar job: %v", err)})
			return
		}
	} else {
		// Mock local sem banco
		jobID = "mock-job-id-12345"
		log.Printf("[Queue Mode Offline] Recebido arquivo %s. Job %s agendado offline.", filename, jobID)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"job_id":  jobID,
		"doc_id":  docID,
	})
}

func handleAnaliseJobStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET"})
		return
	}

	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	// Extrai o ID do caminho, ex: /api/analise/job/some-uuid
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 || parts[4] == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID do job ausente"})
		return
	}
	jobID := parts[4]

	if DBPool == nil {
		// Mock local
		writeJSON(w, http.StatusOK, map[string]any{
			"id": jobID,
			"status": "done",
			"resultado": map[string]any{
				"summary": "Processamento em modo offline",
				"key_observations": "Fila executada localmente.",
			},
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var status string
	var resultJSON []byte
	var errMsg *string

	err = DBPool.QueryRow(ctx, `
		SELECT status, resultado, error_message 
		FROM public.jobs_analise 
		WHERE id = $1 AND user_id = $2
	`, jobID, userID).Scan(&status, &resultJSON, &errMsg)

	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job não encontrado"})
		return
	}

	var res map[string]any
	if len(resultJSON) > 0 {
		_ = json.Unmarshal(resultJSON, &res)
	}

	var errorStr string
	if errMsg != nil {
		errorStr = *errMsg
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":            jobID,
		"status":        status,
		"resultado":     res,
		"error_message": errorStr,
	})
}

func handleMinutas(w http.ResponseWriter, r *http.Request) {
	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	if r.Method == http.MethodGet {
		path := r.URL.Path
		switch {
		case strings.HasPrefix(path, "/api/minutas/intimacao/"):
			intimacaoID := strings.TrimPrefix(path, "/api/minutas/intimacao/")
			if intimacaoID == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID da intimação ausente"})
				return
			}
			queryMinutaByIntimacao(w, userID, intimacaoID)
		case path == "/api/minutas" || path == "/api/minutas/":
			listAllMinutas(w, userID)
		default:
			minutaID := strings.TrimPrefix(path, "/api/minutas/")
			queryMinutaByID(w, userID, minutaID)
		}
		return
	}

	if r.Method == http.MethodPut {
		minutaID := strings.TrimPrefix(r.URL.Path, "/api/minutas/")
		if minutaID == "" || strings.Contains(minutaID, "/") {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID da minuta inválido ou ausente"})
			return
		}
		updateMinuta(w, r, userID, minutaID)
		return
	}

	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "método não suportado"})
}

func listAllMinutas(w http.ResponseWriter, userID string) {
	if DBPool == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := DBPool.Query(ctx, `
		SELECT id, intimacao_id, processo_numero, tipo_peca, conteudo, status, created_at, updated_at
		FROM public.minutas
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type minutaItem struct {
		ID             string    `json:"id"`
		IntimacaoID    *string   `json:"intimacao_id"`
		ProcessoNumero string    `json:"processo_numero"`
		TipoPeca       string    `json:"tipo_peca"`
		Conteudo       string    `json:"conteudo"`
		Status         string    `json:"status"`
		CreatedAt      time.Time `json:"created_at"`
		UpdatedAt      time.Time `json:"updated_at"`
	}

	list := []minutaItem{}
	for rows.Next() {
		var mi minutaItem
		err := rows.Scan(&mi.ID, &mi.IntimacaoID, &mi.ProcessoNumero, &mi.TipoPeca, &mi.Conteudo, &mi.Status, &mi.CreatedAt, &mi.UpdatedAt)
		if err == nil {
			list = append(list, mi)
		}
	}

	writeJSON(w, http.StatusOK, list)
}

func queryMinutaByIntimacao(w http.ResponseWriter, userID string, intimacaoID string) {
	if DBPool == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "minuta não encontrada"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var id, procNum, tipoPeca, conteudo, status string
	var createdAt, updatedAt time.Time
	err := DBPool.QueryRow(ctx, `
		SELECT id, processo_numero, tipo_peca, conteudo, status, created_at, updated_at
		FROM public.minutas
		WHERE user_id = $1 AND intimacao_id = $2
		LIMIT 1
	`, userID, intimacaoID).Scan(&id, &procNum, &tipoPeca, &conteudo, &status, &createdAt, &updatedAt)

	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "minuta não encontrada para a intimação informada"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":              id,
		"intimacao_id":    intimacaoID,
		"processo_numero": procNum,
		"tipo_peca":       tipoPeca,
		"conteudo":        conteudo,
		"status":          status,
		"created_at":      createdAt,
		"updated_at":      updatedAt,
	})
}

func queryMinutaByID(w http.ResponseWriter, userID string, minutaID string) {
	if DBPool == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "minuta não encontrada"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var intID *string
	var procNum, tipoPeca, conteudo, status string
	var createdAt, updatedAt time.Time
	err := DBPool.QueryRow(ctx, `
		SELECT intimacao_id, processo_numero, tipo_peca, conteudo, status, created_at, updated_at
		FROM public.minutas
		WHERE user_id = $1 AND id = $2
	`, userID, minutaID).Scan(&intID, &procNum, &tipoPeca, &conteudo, &status, &createdAt, &updatedAt)

	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "minuta não encontrada"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":              minutaID,
		"intimacao_id":    intID,
		"processo_numero": procNum,
		"tipo_peca":       tipoPeca,
		"conteudo":        conteudo,
		"status":          status,
		"created_at":      createdAt,
		"updated_at":      updatedAt,
	})
}

type updateMinutaRequest struct {
	Conteudo string `json:"conteudo"`
	Status   string `json:"status"`
}

func updateMinuta(w http.ResponseWriter, r *http.Request, userID string, minutaID string) {
	if DBPool == nil {
		writeJSON(w, http.StatusOK, map[string]any{"success": true})
		return
	}

	var req updateMinutaRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON de atualização inválido"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if req.Conteudo != "" && req.Status != "" {
		_, err = DBPool.Exec(ctx, `
			UPDATE public.minutas
			SET conteudo = $1, status = $2, updated_at = now()
			WHERE id = $3 AND user_id = $4
		`, req.Conteudo, req.Status, minutaID, userID)
	} else if req.Conteudo != "" {
		_, err = DBPool.Exec(ctx, `
			UPDATE public.minutas
			SET conteudo = $1, updated_at = now()
			WHERE id = $2 AND user_id = $3
		`, req.Conteudo, minutaID, userID)
	} else if req.Status != "" {
		_, err = DBPool.Exec(ctx, `
			UPDATE public.minutas
			SET status = $1, updated_at = now()
			WHERE id = $2 AND user_id = $3
		`, req.Status, minutaID, userID)
	} else {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "nada para atualizar"})
		return
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

