package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// IA conversacional do Ross AI — self-contained no Roses.
// Usa modelos GRATUITOS: OpenRouter (:free) com fallback para Google Gemini.
// Chaves vêm do ambiente (OPENROUTER_API_KEY, GEMINI_API_KEY), carregadas do .env.

const (
	openRouterURL = "https://openrouter.ai/api/v1/chat/completions"
	geminiBase    = "https://generativelanguage.googleapis.com/v1beta/models"
)

// Modelos free padrão (espelham o Ross AI).
var defaultOpenRouterModels = []string{
	"openai/gpt-oss-20b:free",
	"openai/gpt-oss-120b:free",
	"google/gemma-4-31b-it:free",
	"nex-agi/nex-n2-pro:free",
	"nvidia/nemotron-nano-12b-v2-vl:free",
}

var defaultGeminiModels = []string{"gemini-2.5-flash", "gemini-2.0-flash"}

type Mode string
const (
	ModeChat       Mode = "chat"
	ModeAnalise    Mode = "analise"
	ModeParecer    Mode = "parecer"
	ModePeca       Mode = "peca"
	ModeImpugnacao Mode = "impugnacao"
)

type AgentArea string
const (
	AgentGeral       AgentArea = "geral"
	AgentTributario  AgentArea = "tributario"
	AgentTrabalhista AgentArea = "trabalhista"
	AgentCivel       AgentArea = "civel"
	AgentPenal       AgentArea = "penal"
	AgentEmpresarial AgentArea = "empresarial"
	AgentConsumidor  AgentArea = "consumidor"
	AgentFamilia     AgentArea = "familia"
)

const basePersona = `Você é o Ross AI, assistente jurídica brasileira de altíssimo padrão técnico, no nível de um advogado sênior de grande escritório. Fale de igual para igual, como um colega de profissão experiente.

REGRAS INVIOLÁVEIS:
- O usuário é sempre um ADVOGADO. Portanto, NUNCA adicione avisos de isenção de responsabilidade (disclaimers) dizendo que você não é advogado, que o usuário deve consultar um profissional, ou que você não pode praticar atos de advocacia. Fale de forma direta e técnica.
- Responda SEMPRE em português do Brasil, com linguagem jurídica natural, técnica e humana.
- Fundamente em legislação brasileira real (CF/88, CTN, CC, CPC, CLT, CDC, CP).
- NUNCA invente fatos. Use APENAS o que consta nos documentos anexados ou no histórico.
- NUNCA invente jurisprudência. Se não tiver certeza, use formulação segura.
- Antes de redigir peças, faça leitura global dos anexos.
- IMPORTANTE: a busca de processos reais é feita por outro caminho do sistema. Se o usuário quiser consultar um processo, oriente-o a enviar o NÚMERO (CNJ), a OAB (ex: "OAB 14233 PB") ou o nome da parte — que o sistema busca automaticamente. Não invente números de processo, datas ou movimentações; se não souber, diga que não tem o dado.`

var agentPrompts = map[AgentArea]string{
	AgentGeral:       "",
	AgentTributario:  "\n\nESPECIALIDADE: Direito Tributário. Domina CTN, processo administrativo fiscal, ICMS, ISS, IRPJ/CSLL, execução fiscal, decadência (art. 173 CTN), prescrição.",
	AgentTrabalhista: "\n\nESPECIALIDADE: Direito do Trabalho. Domina CLT, processo do trabalho, verbas rescisórias, horas extras, súmulas do TST.",
	AgentCivel:       "\n\nESPECIALIDADE: Direito Civil e processo civil. Domina CC, CPC/2015, contratos, responsabilidade civil.",
	AgentPenal:       "\n\nESPECIALIDADE: Direito Penal. Domina CP, CPP, garantias, dosimetria, nulidades.",
	AgentEmpresarial: "\n\nESPECIALIDADE: Direito Empresarial. Societário, recuperação judicial, falência, M&A.",
	AgentConsumidor:  "\n\nESPECIALIDADE: Direito do Consumidor. CDC, vícios, práticas abusivas.",
	AgentFamilia:     "\n\nESPECIALIDADE: Direito de Família e Sucessões.",
}

var modeInstructions = map[Mode]string{
	ModeChat:       "\n\nMODO: Conversa. Responda de forma direta, técnica, fluida e natural. Evite listas numeradas longas, bullet points excessivos ou formatações carregadas que prejudiquem a audição/leitura por voz (Text-to-Speech). Prefira parágrafos explicativos e fluidos quando possível.",
	ModeAnalise:    "\n\nMODO: Análise. Leia integralmente os documentos e produza: 1) Identificação; 2) Síntese dos fatos; 3) Pontos críticos; 4) Riscos jurídicos; 5) Inconsistências; 6) Próximos passos.",
	ModeParecer:    "\n\nMODO: Parecer jurídico estruturado: Ementa; Relatório; Fundamentação; Conclusão; Recomendações.",
	ModePeca:       "\n\nMODO: Peça processual completa com padrão de protocolo: endereçamento, qualificação, fatos, direito, pedidos, valor da causa, requerimentos finais.",
	ModeImpugnacao: "\n\nMODO: Impugnação Fiscal completa. Estrutura: endereçamento, qualificação, referência ao auto, tempestividade, preliminares (decadência CTN 173, nulidades), mérito, contestação dos cálculos, pedidos finais e subsidiários.",
}

const spreadsheetGuide = `
INSTRUÇÕES PARA PLANILHAS (XLSX/XLS/CSV):
Os dados tabulares chegam em formato CSV, uma seção por aba ("--- Planilha: nome ---"), com dimensões informadas.
Ao analisar planilhas você DEVE:
1. Identificar o que cada aba representa (apuração fiscal, folha, notas, extratos, cálculos de liquidação etc.).
2. Conferir a coerência aritmética dos valores apresentados: totais, somas de colunas, percentuais, multiplicações de quantidade × valor unitário. Aponte divergências com os números exatos.
3. Destacar valores juridicamente relevantes: principal, multa, juros, correção, base de cálculo, alíquotas, competências/períodos.
4. Sinalizar inconsistências típicas: períodos faltantes, lançamentos duplicados, alíquota incompatível com a legislação citada, datas fora do período fiscalizado, valores negativos inesperados.
5. Quando a planilha embasar autuação ou cálculo da parte adversa, verificar decadência/prescrição por competência e indicar quais linhas seriam atingidas.
6. Citar os números SEMPRE como constam na planilha (não arredonde sem avisar) e referenciar aba e coluna ao mencioná-los.
7. Se o conteúdo tabular já trouxer marcações como "DIVERGÊNCIA", "≠", "declarado" ou "calculado", você DEVE transportar essas inconsistências para "key_observations" e também para ao menos um item de "risk_points", usando a palavra DIVERGÊNCIA e os valores exatos envolvidos.`

func buildSystemPrompt(mode Mode, agent AgentArea, docContext string) string {
	var sb strings.Builder
	sb.WriteString(basePersona)

	if val, ok := agentPrompts[agent]; ok && val != "" {
		sb.WriteString(val)
	}
	if val, ok := modeInstructions[mode]; ok && val != "" {
		sb.WriteString(val)
	}

	if docContext != "" {
		sb.WriteString("\n\nCONTEXTO DA ANÁLISE EM DISCUSSÃO (única fonte factual):\n")
		sb.WriteString(docContext)
		if strings.Contains(strings.ToLower(docContext), "planilha") || strings.Contains(strings.ToLower(docContext), "csv") {
			sb.WriteString("\n\n")
			sb.WriteString(spreadsheetGuide)
		}
	}

	// Suggestions logic
	if mode == "" || mode == ModeChat {
		sb.WriteString("\n\nAo final de cada resposta conversacional, adicione exatamente esta linha com 2 ou 3 sugestões de próxima ação (sem alterar o formato):\nSUGESTÕES: [\"sugestão 1\", \"sugestão 2\", \"sugestão 3\"]")
	}

	return sb.String()
}

type chatMsg struct {
	Role    string `json:"role"`    // "user" | "assistant" | "system"
	Content string `json:"content"`
}

func providerOrder() []string {
	if v := os.Getenv("AI_PROVIDER_ORDER"); v != "" {
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return []string{"openrouter", "gemini"}
}

func openRouterModels() []string {
	if v := os.Getenv("OPENROUTER_MODELS"); strings.TrimSpace(v) != "" {
		return splitCSV(v)
	}
	return defaultOpenRouterModels
}

func geminiModels() []string {
	if v := os.Getenv("GEMINI_MODELS"); strings.TrimSpace(v) != "" {
		return splitCSV(v)
	}
	return defaultGeminiModels
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// chatComplete tenta os provedores na ordem configurada até um responder.
func chatComplete(req chatRequest) (string, string, error) {
	systemPrompt := buildSystemPrompt(Mode(req.Mode), AgentArea(req.Agent), req.DocumentContext)
	var lastErr error
	for _, provider := range providerOrder() {
		switch provider {
		case "openrouter":
			if os.Getenv("OPENROUTER_API_KEY") == "" {
				continue
			}
			for _, model := range openRouterModels() {
				out, err := callOpenRouter(model, req.Messages, systemPrompt)
				if err == nil && strings.TrimSpace(out) != "" {
					return out, "openrouter:" + model, nil
				}
				lastErr = err
			}
		case "gemini":
			if os.Getenv("GEMINI_API_KEY") == "" {
				continue
			}
			for _, model := range geminiModels() {
				out, err := callGemini(model, req.Messages, systemPrompt)
				if err == nil && strings.TrimSpace(out) != "" {
					return out, "gemini:" + model, nil
				}
				lastErr = err
			}
		}
	}
	if lastErr == nil {
		lastErr = errors.New("nenhum provedor de IA configurado (defina OPENROUTER_API_KEY ou GEMINI_API_KEY)")
	}
	return "", "", lastErr
}

// --- OpenRouter (formato OpenAI chat completions) -------------------------

func callOpenRouter(model string, history []chatMsg, systemPrompt string) (string, error) {
	msgs := make([]map[string]string, 0, len(history)+1)
	msgs = append(msgs, map[string]string{"role": "system", "content": systemPrompt})
	for _, m := range history {
		msgs = append(msgs, map[string]string{"role": m.Role, "content": m.Content})
	}
	body, _ := json.Marshal(map[string]any{"model": model, "messages": msgs, "stream": false})

	req, _ := http.NewRequest("POST", openRouterURL, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+os.Getenv("OPENROUTER_API_KEY"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://roses.local")
	req.Header.Set("X-Title", "Roses Ross AI")

	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("openrouter %d: %s", resp.StatusCode, truncate(string(data), 180))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("openrouter: resposta vazia")
	}
	return parsed.Choices[0].Message.Content, nil
}

// --- Google Gemini --------------------------------------------------------

func callGemini(model string, history []chatMsg, systemPrompt string) (string, error) {
	contents := make([]map[string]any, 0, len(history))
	for _, m := range history {
		role := "user"
		if m.Role == "assistant" {
			role = "model"
		}
		if m.Role == "system" {
			continue // system vai em systemInstruction
		}
		contents = append(contents, map[string]any{
			"role":  role,
			"parts": []map[string]string{{"text": m.Content}},
		})
	}
	body, _ := json.Marshal(map[string]any{
		"contents":          contents,
		"systemInstruction": map[string]any{"parts": []map[string]string{{"text": systemPrompt}}},
	})

	url := fmt.Sprintf("%s/%s:generateContent?key=%s", geminiBase, model, os.Getenv("GEMINI_API_KEY"))
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("gemini %d: %s", resp.StatusCode, truncate(string(data), 180))
	}
	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("gemini: resposta vazia")
	}
	var sb strings.Builder
	for _, p := range parsed.Candidates[0].Content.Parts {
		sb.WriteString(p.Text)
	}
	return sb.String(), nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// --- Streaming SSE --------------------------------------------------------

// chatCompleteStream tenta providers em ordem enviando tokens SSE conforme chegam.
func chatCompleteStream(req chatRequest, w http.ResponseWriter, flusher http.Flusher) error {
	systemPrompt := buildSystemPrompt(Mode(req.Mode), AgentArea(req.Agent), req.DocumentContext)
	for _, provider := range providerOrder() {
		switch provider {
		case "openrouter":
			if os.Getenv("OPENROUTER_API_KEY") == "" {
				continue
			}
			for _, model := range openRouterModels() {
				if err := streamOpenRouter(model, req.Messages, systemPrompt, w, flusher); err == nil {
					return nil
				}
			}
		case "gemini":
			if os.Getenv("GEMINI_API_KEY") == "" {
				continue
			}
			for _, model := range geminiModels() {
				if err := streamGemini(model, req.Messages, systemPrompt, w, flusher); err == nil {
					return nil
				}
			}
		}
	}
	return errors.New("todos os providers falharam no streaming")
}

func streamOpenRouter(model string, history []chatMsg, systemPrompt string, w http.ResponseWriter, flusher http.Flusher) error {
	msgs := make([]map[string]string, 0, len(history)+1)
	msgs = append(msgs, map[string]string{"role": "system", "content": systemPrompt})
	for _, m := range history {
		msgs = append(msgs, map[string]string{"role": m.Role, "content": m.Content})
	}
	body, _ := json.Marshal(map[string]any{"model": model, "messages": msgs, "stream": true})

	req, _ := http.NewRequest("POST", openRouterURL, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+os.Getenv("OPENROUTER_API_KEY"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://roses.local")
	req.Header.Set("X-Title", "Roses Ross AI")

	resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("openrouter stream %d: %s", resp.StatusCode, truncate(string(raw), 180))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 512*1024), 512*1024)
	hasContent := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content *string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil || len(chunk.Choices) == 0 || chunk.Choices[0].Delta.Content == nil {
			continue
		}
		token := *chunk.Choices[0].Delta.Content
		if token == "" {
			continue
		}
		hasContent = true
		tj, _ := json.Marshal(map[string]string{"t": token})
		fmt.Fprintf(w, "data: %s\n\n", tj)
		flusher.Flush()
	}
	if !hasContent {
		return errors.New("openrouter: stream vazio")
	}
	return scanner.Err()
}

func streamGemini(model string, history []chatMsg, systemPrompt string, w http.ResponseWriter, flusher http.Flusher) error {
	contents := make([]map[string]any, 0, len(history))
	for _, m := range history {
		role := "user"
		if m.Role == "assistant" {
			role = "model"
		}
		if m.Role == "system" {
			continue
		}
		contents = append(contents, map[string]any{
			"role":  role,
			"parts": []map[string]string{{"text": m.Content}},
		})
	}
	body, _ := json.Marshal(map[string]any{
		"contents":          contents,
		"systemInstruction": map[string]any{"parts": []map[string]string{{"text": systemPrompt}}},
	})

	url := fmt.Sprintf("%s/%s:streamGenerateContent?key=%s&alt=sse", geminiBase, model, os.Getenv("GEMINI_API_KEY"))
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gemini stream %d: %s", resp.StatusCode, truncate(string(raw), 180))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 512*1024), 512*1024)
	hasContent := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		var chunk struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil || len(chunk.Candidates) == 0 {
			continue
		}
		for _, p := range chunk.Candidates[0].Content.Parts {
			if p.Text == "" {
				continue
			}
			hasContent = true
			tj, _ := json.Marshal(map[string]string{"t": p.Text})
			fmt.Fprintf(w, "data: %s\n\n", tj)
			flusher.Flush()
		}
	}
	if !hasContent {
		return errors.New("gemini: stream vazio")
	}
	return scanner.Err()
}
