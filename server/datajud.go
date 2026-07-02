package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Chave PUBLICA vigente do DataJud (publicada na wiki do CNJ).
// Sobrescrevivel por env DATAJUD_API_KEY.
const defaultDataJudKey = "cDZHYzlZa0JadVREZDJCendQbXY6SkJlTzNjLV9TRENyQk1RdnFKZGRQdw=="

func datajudKey() string {
	if k := os.Getenv("DATAJUD_API_KEY"); k != "" {
		return k
	}
	return defaultDataJudKey
}

// Estruturas minimas da resposta Elasticsearch do DataJud.
type esResponse struct {
	Hits struct {
		Hits []struct {
			Source esSource `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

type esNome struct {
	Nome string `json:"nome"`
}

type esMovimento struct {
	Nome          string `json:"nome"`
	DataHora      string `json:"dataHora"`
	OrgaoJulgador esNome `json:"orgaoJulgador"`
}

type esSource struct {
	NumeroProcesso  string        `json:"numeroProcesso"`
	Classe          esNome        `json:"classe"`
	Assuntos        []esNome      `json:"assuntos"`
	OrgaoJulgador   esNome        `json:"orgaoJulgador"`
	DataAjuizamento string        `json:"dataAjuizamento"`
	Movimentos      []esMovimento `json:"movimentos"`
}

var digitsRe = regexp.MustCompile(`\D`)

// normalizeDate: ISO (2024-10-30T..) ou numerico (YYYYMMDD..) -> DD/MM/AAAA.
func normalizeDate(s string) string {
	if s == "" {
		return ""
	}
	if strings.Contains(s, "-") || strings.Contains(s, "T") {
		clean := strings.SplitN(strings.ReplaceAll(s, "Z", ""), ".", 2)[0]
		for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02"} {
			if t, err := time.Parse(layout, clean); err == nil {
				return t.Format("02/01/2006")
			}
		}
	}
	d := digitsRe.ReplaceAllString(s, "")
	if len(d) >= 8 {
		return d[6:8] + "/" + d[4:6] + "/" + d[0:4]
	}
	return s
}

func datajudByCNJ(cnj string) Result {
	query := map[string]string{"cnj": cnj}
	digits := onlyDigits(cnj)
	if len(digits) != 20 {
		return Result{Status: "INVALID", Message: "Numero CNJ invalido (esperado 20 digitos).",
			Tribunal: "N/A", TribunalName: "N/A", Query: query}
	}
	alias := aliasFromCNJ(cnj)
	if alias == "" {
		return Result{Status: "INVALID", Message: "Tribunal nao mapeado no DataJud.",
			Tribunal: "N/A", TribunalName: "N/A", Query: query}
	}
	sigla := strings.ToUpper(alias[strings.LastIndex(alias, "_")+1:])

	body := map[string]any{
		"size":  10,
		"query": map[string]any{"match": map[string]any{"numeroProcesso": digits}},
	}
	raw, _ := json.Marshal(body)

	url := fmt.Sprintf("https://api-publica.datajud.cnj.jus.br/%s/_search", alias)
	req, err := http.NewRequest("POST", url, bytes.NewReader(raw))
	if err != nil {
		return Result{Status: "ERROR", Message: err.Error(), Tribunal: sigla, TribunalName: sigla, Query: query}
	}
	req.Header.Set("Authorization", "APIKey "+datajudKey())
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return Result{Status: "ERROR", Message: err.Error(), Tribunal: sigla, TribunalName: sigla, Query: query}
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 {
		return Result{Status: "ERROR", Message: "401: chave publica DataJud invalida/expirada.",
			Tribunal: sigla, TribunalName: sigla, Query: query}
	}
	if resp.StatusCode != 200 {
		return Result{Status: "ERROR", Message: fmt.Sprintf("HTTP %d do DataJud", resp.StatusCode),
			Tribunal: sigla, TribunalName: sigla, Query: query}
	}

	var es esResponse
	if err := json.Unmarshal(data, &es); err != nil {
		return Result{Status: "ERROR", Message: "resposta DataJud invalida", Tribunal: sigla, TribunalName: sigla, Query: query}
	}
	if len(es.Hits.Hits) == 0 {
		return Result{Status: "NOT_FOUND", Message: "Nenhum processo encontrado.",
			Tribunal: sigla, TribunalName: sigla, Query: query, Total: 0, Processos: []Process{}}
	}

	procs := make([]Process, 0, len(es.Hits.Hits))
	for _, h := range es.Hits.Hits {
		procs = append(procs, sourceToProcess(h.Source, sigla))
	}
	return Result{
		Status: "OK", Message: fmt.Sprintf("%d processo(s) via DataJud.", len(procs)),
		Tribunal: sigla, TribunalName: sigla, Query: query,
		Total: len(procs), Processos: procs, Fonte: "datajud",
	}
}

func sourceToProcess(s esSource, sigla string) Process {
	// Ordena movimentos pela dataHora bruta (YYYYMMDD..), mais recentes primeiro.
	movs := make([]esMovimento, len(s.Movimentos))
	copy(movs, s.Movimentos)
	sort.SliceStable(movs, func(i, j int) bool {
		return digitsRe.ReplaceAllString(movs[i].DataHora, "") > digitsRe.ReplaceAllString(movs[j].DataHora, "")
	})
	mvs := make([]Movement, 0, len(movs))
	for _, m := range movs {
		mvs = append(mvs, Movement{Data: normalizeDate(m.DataHora), Descricao: m.Nome, Orgao: m.OrgaoJulgador.Nome})
	}
	assunto := ""
	if len(s.Assuntos) > 0 {
		assunto = s.Assuntos[0].Nome
	}
	return Process{
		Numero:           formatCNJ(s.NumeroProcesso),
		Classe:           s.Classe.Nome,
		Assunto:          assunto,
		Tribunal:         sigla,
		OrgaoJulgador:    s.OrgaoJulgador.Nome,
		DataDistribuicao: normalizeDate(s.DataAjuizamento),
		Partes:           []Party{},
		Movimentacoes:    mvs,
	}
}
