package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Estrutura do processo como sai do parser do portal (Python).
type portalProcess struct {
	Numero                string `json:"numero"`
	Classe                string `json:"classe"`
	Assunto               string `json:"assunto"`
	PoloAtivo             string `json:"polo_ativo"`
	PoloPassivo           string `json:"polo_passivo"`
	UltimaMovimentacao    string `json:"ultima_movimentacao"`
	DataUltimaMovimentacao string `json:"data_ultima_movimentacao"`
	DetalheURL            string `json:"detalhe_url"`
}

type portalOutput struct {
	Status    string          `json:"status"`
	Message   string          `json:"message"`
	Court     string          `json:"court"`
	Total     int             `json:"total"`
	Processos []portalProcess `json:"processos"`
}

func pythonBin() string {
	if p := os.Getenv("ROSES_PYTHON"); p != "" {
		return p
	}
	return "python3"
}

func scraperPath() string {
	if p := os.Getenv("ROSES_SCRAPER"); p != "" {
		return p
	}
	// Default: roses/scrapers/pje_portal_scraper.py (server roda em roses/server).
	return filepath.Join("..", "scrapers", "pje_portal_scraper.py")
}

// portalConsulta roda o scraper Python e converte a saida para Result.
func portalConsulta(tipo, valor, uf string, timeoutSec int) Result {
	query := map[string]string{"tipo": tipo, "valor": valor, "uf": uf}

	args := []string{scraperPath(), "--json-only"}
	switch tipo {
	case "oab":
		args = append(args, "--oab", valor)
	case "nome":
		args = append(args, "--nome-parte", valor)
	case "advogado":
		args = append(args, "--nome-advogado", valor)
	case "cnj":
		args = append(args, "--cnj", valor)
	}
	if uf != "" {
		args = append(args, "--uf", uf)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, pythonBin(), args...)
	out, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return Result{Status: "TIMEOUT", Message: "Portal excedeu o tempo limite.",
			Tribunal: "N/A", TribunalName: "N/A", Query: query}
	}
	if err != nil && len(out) == 0 {
		return errorResult("Falha ao executar o scraper do portal: "+err.Error(), query)
	}

	var po portalOutput
	if err := json.Unmarshal(out, &po); err != nil {
		return Result{Status: "UNKNOWN", Message: "Saida do portal nao-JSON.",
			Tribunal: "N/A", TribunalName: "N/A", Query: query}
	}

	procs := make([]Process, 0, len(po.Processos))
	for _, p := range po.Processos {
		partes := []Party{}
		if p.PoloAtivo != "" {
			partes = append(partes, Party{Tipo: "Polo Ativo", Nome: p.PoloAtivo})
		}
		if p.PoloPassivo != "" {
			partes = append(partes, Party{Tipo: "Polo Passivo", Nome: p.PoloPassivo})
		}
		movs := []Movement{}
		if p.UltimaMovimentacao != "" {
			movs = append(movs, Movement{Data: p.DataUltimaMovimentacao, Descricao: p.UltimaMovimentacao})
		}
		procs = append(procs, Process{
			Numero: p.Numero, Classe: p.Classe, Assunto: p.Assunto, Tribunal: po.Court,
			Partes: partes, Movimentacoes: movs, URLProcesso: p.DetalheURL,
		})
	}

	status := po.Status
	if status == "" {
		status = "UNKNOWN"
	}
	return Result{
		Status: status, Message: po.Message, Tribunal: po.Court, TribunalName: po.Court,
		Query: query, Total: po.Total, Processos: procs, Fonte: "portal",
	}
}
