package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Oportunidades: ranqueia os processos guardados por SINAIS REAIS.
// Nada de valor monetário inventado — usa urgência, tempo parado e
// reincidência de cliente.

type Oportunidade struct {
	Numero     string `json:"numero"`
	Tribunal   string `json:"tribunal"`
	Classe     string `json:"classe"`
	Tipo       string `json:"tipo"`   // categoria do sinal
	Motivo     string `json:"motivo"` // explicação legível
	Score      int    `json:"score"`  // 0-100
	DiasParado int    `json:"dias_parado,omitempty"`
}

// Palavras-chave de urgência por nível (espelha a lógica do JurisMiner).
var urgencyTiers = []struct {
	score   int
	words   []string
	acao    string
}{
	{95, []string{"prazo", "audiência", "audiencia", "liminar", "tutela", "leilão", "leilao", "penhora", "bloqueio"}, "Ação imediata: há prazo ou ato que exige resposta."},
	{85, []string{"intimação", "intimacao", "citação", "citacao", "sentença", "sentenca", "decisão", "decisao", "embargos"}, "Priorizar: andamento relevante recente."},
	{70, []string{"despacho", "recurso", "manifestação", "manifestacao", "petição", "peticao"}, "Revisar: movimentação que pode pedir providência."},
}

func parseBRDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if len(s) < 10 {
		return time.Time{}, false
	}
	t, err := time.Parse("02/01/2006", s[:10])
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func latestMovimentacaoDate(p Process) (time.Time, bool) {
	var best time.Time
	found := false
	for _, m := range p.Movimentacoes {
		if d, ok := parseBRDate(m.Data); ok {
			if !found || d.After(best) {
				best = d
				found = true
			}
		}
	}
	return best, found
}

func urgencySignal(p Process) (int, string, string) {
	// olha as movimentações mais recentes (até 6)
	limit := 6
	if len(p.Movimentacoes) < limit {
		limit = len(p.Movimentacoes)
	}
	blob := strings.ToLower(p.Assunto + " " + p.Classe)
	for i := 0; i < limit; i++ {
		blob += " " + strings.ToLower(p.Movimentacoes[i].Descricao)
	}
	for _, tier := range urgencyTiers {
		for _, w := range tier.words {
			if strings.Contains(blob, w) {
				return tier.score, tier.acao, w
			}
		}
	}
	return 0, "", ""
}

func computeOportunidades(userID string) []Oportunidade {
	stored := loadProcessos(userID)
	now := time.Now()
	out := []Oportunidade{}

	// Reincidência de cliente: conta processos por nome de parte.
	clienteCount := map[string]int{}
	for _, sp := range stored {
		for _, parte := range sp.Process.Partes {
			n := strings.TrimSpace(strings.ToUpper(parte.Nome))
			if n != "" {
				clienteCount[n]++
			}
		}
	}

	for _, sp := range stored {
		p := sp.Process

		// dias desde a última movimentação
		diasParado := 0
		if d, ok := latestMovimentacaoDate(p); ok {
			diasParado = int(now.Sub(d).Hours() / 24)
		}

		score, acao, palavra := urgencySignal(p)
		var tipo, motivo string

		if score >= 70 {
			tipo = "Ação urgente"
			motivo = fmt.Sprintf("%s (sinal: \"%s\").", acao, palavra)
		} else if diasParado >= 180 {
			score = 60
			tipo = "Processo parado"
			motivo = fmt.Sprintf("Sem movimentação há %d dias — avaliar cobrança de andamento ou arquivamento.", diasParado)
		} else {
			continue // sem sinal relevante
		}

		// boost por cliente recorrente
		for _, parte := range p.Partes {
			n := strings.TrimSpace(strings.ToUpper(parte.Nome))
			if clienteCount[n] >= 2 {
				score += 8
				motivo += fmt.Sprintf(" Cliente recorrente (%s aparece em %d processos).", parte.Nome, clienteCount[n])
				break
			}
		}
		if score > 100 {
			score = 100
		}

		out = append(out, Oportunidade{
			Numero: p.Numero, Tribunal: p.Tribunal, Classe: p.Classe,
			Tipo: tipo, Motivo: motivo, Score: score, DiasParado: diasParado,
		})
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}
