package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

// Motor de transcrição (voz -> texto) em Go, usando API barata.
// Ordem padrão: Groq (Whisper, free tier, aceita webm do navegador) e
// Gemini como fallback (reaproveita a GEMINI_API_KEY que ja existe).

const groqTranscribeURL = "https://api.groq.com/openai/v1/audio/transcriptions"

func transcribeProviderOrder() []string {
	if v := os.Getenv("TRANSCRIBE_PROVIDER_ORDER"); v != "" {
		return splitCSV(v)
	}
	return []string{"groq", "gemini"} // Groq (Whisper) primeiro; Gemini de reserva
}

// transcribeAudio tenta os provedores na ordem ate um devolver texto.
func transcribeAudio(data []byte, mime, filename string) (string, string, error) {
	var lastErr error
	for _, p := range transcribeProviderOrder() {
		switch p {
		case "groq":
			if os.Getenv("GROQ_API_KEY") == "" {
				continue
			}
			if txt, err := callGroqTranscribe(data, filename); err == nil && strings.TrimSpace(txt) != "" {
				return txt, "groq", nil
			} else {
				lastErr = err
			}
		case "gemini":
			if os.Getenv("GEMINI_API_KEY") == "" {
				continue
			}
			if txt, err := callGeminiTranscribe(data, mime); err == nil && strings.TrimSpace(txt) != "" {
				return txt, "gemini", nil
			} else {
				lastErr = err
			}
		}
	}
	if lastErr == nil {
		lastErr = errors.New("nenhum provedor de transcricao configurado (defina GROQ_API_KEY ou GEMINI_API_KEY)")
	}
	return "", "", lastErr
}

// --- Groq (Whisper, OpenAI-compatible multipart) --------------------------

func callGroqTranscribe(data []byte, filename string) (string, error) {
	if filename == "" {
		filename = "audio.webm"
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	fw.Write(data)
	w.WriteField("model", getenv("GROQ_MODEL", "whisper-large-v3-turbo"))
	w.WriteField("language", "pt")
	w.WriteField("response_format", "json")
	w.Close()

	req, _ := http.NewRequest("POST", groqTranscribeURL, &buf)
	req.Header.Set("Authorization", "Bearer "+os.Getenv("GROQ_API_KEY"))
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := (&http.Client{Timeout: 90 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("groq %d: %s", resp.StatusCode, truncate(string(body), 180))
	}
	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	return strings.TrimSpace(parsed.Text), nil
}

// --- Gemini (áudio inline) ------------------------------------------------

func callGeminiTranscribe(data []byte, mime string) (string, error) {
	if mime == "" {
		mime = "audio/webm"
	}
	model := getenv("GEMINI_TRANSCRIBE_MODEL", "gemini-2.0-flash")
	body, _ := json.Marshal(map[string]any{
		"contents": []map[string]any{{
			"parts": []any{
				map[string]any{"inline_data": map[string]string{
					"mime_type": mime,
					"data":      base64.StdEncoding.EncodeToString(data),
				}},
				map[string]string{"text": "Transcreva este áudio em português do Brasil. Devolva apenas o texto falado, sem comentários."},
			},
		}},
	})
	url := fmt.Sprintf("%s/%s:generateContent?key=%s", geminiBase, model, os.Getenv("GEMINI_API_KEY"))
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 90 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("gemini %d: %s", resp.StatusCode, truncate(string(raw), 180))
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
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("gemini: resposta vazia")
	}
	var sb strings.Builder
	for _, p := range parsed.Candidates[0].Content.Parts {
		sb.WriteString(p.Text)
	}
	return strings.TrimSpace(sb.String()), nil
}
