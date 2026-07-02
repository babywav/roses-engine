package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Dashboard + Radar Jurídico (notificações) — tudo calculado dos processos
// reais salvos em data/processos.json. Sem números fixos.

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

// contagens de movimentações por janela de tempo.
func contaMovimentacoes(stored []StoredProcess, hoje time.Time) (mHoje, m7d int) {
	limite7 := hoje.AddDate(0, 0, -7)
	for _, sp := range stored {
		for _, m := range sp.Movimentacoes {
			d, ok := parseBRDate(m.Data)
			if !ok {
				continue
			}
			if sameDay(d, hoje) {
				mHoje++
			}
			if !d.Before(limite7) && !d.After(hoje) {
				m7d++
			}
		}
	}
	return
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}
	stored := loadProcessos(userID)
	hoje := time.Now()

	prazos := computePrazos(userID)
	var prazosHoje, prazosUrgentes, prazosVencidos int
	for _, p := range prazos {
		switch p.Status {
		case "hoje":
			prazosHoje++
		case "urgente":
			prazosUrgentes++
		case "vencido":
			prazosVencidos++
		}
	}

	mHoje, m7d := contaMovimentacoes(stored, hoje)

	writeJSON(w, http.StatusOK, map[string]any{
		"total_casos":        len(stored),
		"prazos_hoje":        prazosHoje,
		"prazos_urgentes":    prazosUrgentes,
		"prazos_vencidos":    prazosVencidos,
		"movimentacoes_hoje": mHoje,
		"movimentacoes_7d":   m7d,
	})
}

type Notificacao struct {
	Tipo       string `json:"tipo"`       // prazo | movimentacao | parado | urgente
	Prioridade string `json:"prioridade"` // alta | media | baixa
	Titulo     string `json:"titulo"`
	Descricao  string `json:"descricao"`
	Data       string `json:"data,omitempty"`
	Numero     string `json:"numero,omitempty"`
}

func handleNotificacoes(w http.ResponseWriter, r *http.Request) {
	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}
	stored := loadProcessos(userID)
	hoje := time.Now()
	out := []Notificacao{}

	// 1) Prazos vencidos / hoje / urgentes (prioridade alta).
	prazos := computePrazos(userID)
	sort.SliceStable(prazos, func(i, j int) bool {
		return prazos[i].DiasRestantes < prazos[j].DiasRestantes
	})
	for _, p := range prazos {
		if p.Status == "emdia" {
			continue
		}
		var titulo, prio string
		switch p.Status {
		case "vencido":
			titulo = fmt.Sprintf("Prazo VENCIDO há %d dia(s) útil(eis)", -p.DiasRestantes)
			prio = "alta"
		case "hoje":
			titulo = "Prazo vence HOJE"
			prio = "alta"
		case "urgente":
			titulo = fmt.Sprintf("Prazo vence em %d dia(s) útil(eis)", p.DiasRestantes)
			prio = "alta"
		}
		out = append(out, Notificacao{
			Tipo: "prazo", Prioridade: prio, Titulo: titulo,
			Descricao: fmt.Sprintf("%s (%d dias úteis · %s) • %s — venc. %s", p.Rotulo, p.PrazoDias, p.Base, p.Tribunal, p.Vencimento),
			Data:      p.Vencimento, Numero: p.Numero,
		})
	}

	// 2) Movimentações recentes (últimos 3 dias) — prioridade média.
	limite := hoje.AddDate(0, 0, -3)
	type mv struct {
		numero, tribunal, classe, desc string
		data                           time.Time
	}
	recentes := []mv{}
	for _, sp := range stored {
		p := sp.Process
		d, ok := latestMovimentacaoDate(p)
		if !ok || d.Before(limite) || d.After(hoje) {
			continue
		}
		desc := ""
		for _, m := range p.Movimentacoes {
			if md, ok := parseBRDate(m.Data); ok && md.Equal(d) {
				desc = m.Descricao
				break
			}
		}
		recentes = append(recentes, mv{p.Numero, p.Tribunal, p.Classe, desc, d})
	}
	sort.SliceStable(recentes, func(i, j int) bool { return recentes[i].data.After(recentes[j].data) })
	for _, m := range recentes {
		out = append(out, Notificacao{
			Tipo: "movimentacao", Prioridade: "media",
			Titulo:    "Nova movimentação",
			Descricao: fmt.Sprintf("%s • %s — %s", m.classe, m.tribunal, resumoAto(m.desc)),
			Data:      m.data.Format("02/01/2006"), Numero: m.numero,
		})
	}

	// 3) Processos parados há muito tempo (>=180 dias) — prioridade baixa.
	for _, sp := range stored {
		p := sp.Process
		if d, ok := latestMovimentacaoDate(p); ok {
			dias := int(hoje.Sub(d).Hours() / 24)
			if dias >= 180 {
				out = append(out, Notificacao{
					Tipo: "parado", Prioridade: "baixa",
					Titulo:    fmt.Sprintf("Processo parado há %d dias", dias),
					Descricao: fmt.Sprintf("%s • %s — avaliar cobrança de andamento.", p.Classe, p.Tribunal),
					Data:      d.Format("02/01/2006"), Numero: p.Numero,
				})
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total":        len(out),
		"notificacoes": out,
	})
}

func resumoAto(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "—"
	}
	runes := []rune(s)
	if len(runes) > 90 {
		return string(runes[:90]) + "…"
	}
	return s
}
