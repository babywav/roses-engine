package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"time"
)

// Cliente Bright Data Web Unlocker — resolve Cloudflare/anti-bot e devolve o
// HTML da página alvo. Usado no caminho do portal (OAB/nome) que tem Cloudflare.
// Requer BRIGHTDATA_API_TOKEN e BRIGHTDATA_ZONE no .env.

const brightdataURL = "https://api.brightdata.com/request"

func brightdataFetch(targetURL string) (string, int, error) {
	token := os.Getenv("BRIGHTDATA_API_TOKEN")
	if token == "" {
		return "", 0, errors.New("BRIGHTDATA_API_TOKEN ausente no .env")
	}
	zone := getenv("BRIGHTDATA_ZONE", "web_unlocker1")

	payload, _ := json.Marshal(map[string]string{
		"zone":   zone,
		"url":    targetURL,
		"format": "raw",
	})
	req, err := http.NewRequest("POST", brightdataURL, bytes.NewReader(payload))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return string(data), resp.StatusCode, nil
}
