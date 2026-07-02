package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Motor de prazos — estimativa a partir das movimentações, agora com prazo
// específico por TIPO DE ATO (CPC/2015), não mais um valor único.
//
// Contagem em DIAS ÚTEIS (art. 219), a partir do dia útil seguinte ao ato,
// pulando fins de semana, feriados nacionais fixos e o recesso forense
// (20/12 a 06/01). Continua uma estimativa — o juízo pode fixar prazo diverso.

// prazoRule associa palavras-chave do ato ao prazo legal em dias úteis.
// A ORDEM importa: regras mais específicas vêm primeiro.
type prazoRule struct {
	keywords []string
	dias     int
	rotulo   string
	base     string
}

var prazoRules = []prazoRule{
	// Embargos de declaração — 5 dias (art. 1.023)
	{[]string{"embargos de declaração", "embargos de declaracao", "embargos declaratórios", "embargos declaratorios"}, 5, "Embargos de declaração", "CPC art. 1.023"},

	// Recurso inominado (Juizado Especial) — 10 dias (Lei 9.099 art. 42)
	{[]string{"recurso inominado"}, 10, "Recurso inominado", "Lei 9.099/95 art. 42"},

	// Agravo interno — 15 dias (art. 1.021 §2)
	{[]string{"agravo interno"}, 15, "Agravo interno", "CPC art. 1.021"},
	// Agravo de instrumento — 15 dias (art. 1.003 §5)
	{[]string{"agravo de instrumento"}, 15, "Agravo de instrumento", "CPC art. 1.003"},

	// Apelação — 15 dias (art. 1.003 §5)
	{[]string{"apelação", "apelacao", "apele", "sentença", "sentenca"}, 15, "Apelação", "CPC art. 1.003"},

	// Recurso especial / extraordinário e contrarrazões — 15 dias
	{[]string{"recurso especial", "recurso extraordinário", "recurso extraordinario", "contrarrazões", "contrarrazoes"}, 15, "Recurso/contrarrazões", "CPC art. 1.003"},

	// Contestação — 15 dias (art. 335); citação para responder
	{[]string{"contestação", "contestacao", "conteste", "citação", "citacao", "cite-se", "para responder", "apresentar defesa"}, 15, "Contestação", "CPC art. 335"},

	// Réplica / impugnação à contestação — 15 dias (art. 350/351)
	{[]string{"réplica", "replica", "impugnação à contestação", "impugnacao a contestacao"}, 15, "Réplica", "CPC art. 350"},

	// Cumprimento de sentença / pagamento voluntário — 15 dias (art. 523)
	{[]string{"cumprimento de sentença", "cumprimento de sentenca", "pagamento voluntário", "pagamento voluntario", "art. 523", "artigo 523"}, 15, "Pagamento voluntário", "CPC art. 523"},
	// Impugnação ao cumprimento de sentença — 15 dias (art. 525)
	{[]string{"impugnação ao cumprimento", "impugnacao ao cumprimento"}, 15, "Impugnação ao cumprimento", "CPC art. 525"},
	// Embargos à execução — 15 dias (art. 915)
	{[]string{"embargos à execução", "embargos a execucao", "embargos do devedor"}, 15, "Embargos à execução", "CPC art. 915"},

	// Emenda à inicial — 15 dias (art. 321)
	{[]string{"emenda à inicial", "emenda a inicial", "emende a inicial", "emendar a inicial"}, 15, "Emenda à inicial", "CPC art. 321"},

	// Alegações finais / memoriais — 15 dias (art. 364 §2)
	{[]string{"alegações finais", "alegacoes finais", "memoriais", "razões finais", "razoes finais"}, 15, "Alegações finais", "CPC art. 364"},

	// Especificação de provas — em regra 5 dias
	{[]string{"especificar provas", "especifiquem as provas", "especifiquem provas", "provas que pretende"}, 5, "Especificar provas", "CPC art. 218 §3"},

	// Manifestação sobre documentos juntados — 15 dias (art. 437 §1)
	{[]string{"manifestar sobre os documentos", "manifestação sobre documentos", "manifestacao sobre documentos", "sobre a juntada"}, 15, "Manifestação s/ documentos", "CPC art. 437"},

	// Manifestação / vista genérica / "diga" — default legal 5 dias (art. 218 §3)
	{[]string{"manifestar", "manifestação", "manifestacao", "vista dos autos", "abertura de vista", "diga o autor", "diga a parte", "digam as partes", "ciência", "ciencia"}, 5, "Manifestação", "CPC art. 218 §3"},

	// Intimação / publicação / despacho genéricos — default 5 dias (art. 218 §3)
	{[]string{"intimação", "intimacao", "intimado", "intime-se", "publicação", "publicacao", "publicado", "disponibiliz", "despacho", "decisão", "decisao", "acórdão", "acordao"}, 5, "Providência (prazo geral)", "CPC art. 218 §3"},
}

var feriadosFixos = map[string]bool{
	"01/01": true, // Confraternização
	"21/04": true, // Tiradentes
	"01/05": true, // Trabalho
	"07/09": true, // Independência
	"12/10": true, // N. Sra. Aparecida
	"02/11": true, // Finados
	"15/11": true, // Proclamação da República
	"25/12": true, // Natal
}

func ehRecessoForense(t time.Time) bool {
	m, d := t.Month(), t.Day()
	if m == time.December && d >= 20 {
		return true
	}
	if m == time.January && d <= 20 { // CPC art. 220 suspende prazos até 20/01 inclusive
		return true
	}
	return false
}

func ehFeriadoLocal(t time.Time, uf string) bool {
	uf = strings.ToUpper(strings.TrimSpace(uf))
	diaMes := t.Format("02/01")

	switch uf {
	case "SP":
		return diaMes == "09/07" // Revolução Constitucionalista
	case "RJ":
		return diaMes == "23/04" || diaMes == "20/11" // São Jorge, Zumbi
	case "PB":
		return diaMes == "05/08" // Fundação da Paraíba
	case "PE":
		return diaMes == "06/03" // Data Magna de Pernambuco
	case "BA":
		return diaMes == "02/07" // Independência da Bahia
	case "RS":
		return diaMes == "20/09" // Revolução Farroupilha
	case "PR":
		return diaMes == "19/12" // Emancipação Política do Paraná
	}
	return false
}

func ehDiaUtil(t time.Time, uf string) bool {
	switch t.Weekday() {
	case time.Saturday, time.Sunday:
		return false
	}
	if feriadosFixos[t.Format("02/01")] {
		return false
	}
	if ehRecessoForense(t) {
		return false
	}
	if ehFeriadoLocal(t, uf) {
		return false
	}
	return true
}

func adicionaDiasUteis(inicio time.Time, n int, uf string) time.Time {
	t := inicio
	contados := 0
	for contados < n {
		t = t.AddDate(0, 0, 1)
		if ehDiaUtil(t, uf) {
			contados++
		}
	}
	return t
}

func extractUFFromTribunal(tribunal string) string {
	t := strings.ToUpper(strings.TrimSpace(tribunal))
	if strings.HasPrefix(t, "TJ") && len(t) >= 4 {
		return t[2:4]
	}
	return ""
}

// matchRule devolve a primeira regra cujo texto casa com a descrição do ato.
func matchRule(desc string) (prazoRule, bool) {
	d := strings.ToLower(desc)
	for _, rule := range prazoRules {
		for _, kw := range rule.keywords {
			if strings.Contains(d, kw) {
				return rule, true
			}
		}
	}
	return prazoRule{}, false
}

// gatilhoPrazoMaisRecente acha o ato mais recente (por data) que dispara prazo,
// devolvendo também a regra (tipo de prazo) aplicável.
func gatilhoPrazoMaisRecente(p Process) (time.Time, string, prazoRule, bool) {
	var melhor time.Time
	var ato string
	var melhorRule prazoRule
	found := false
	for _, m := range p.Movimentacoes {
		d, ok := parseBRDate(m.Data)
		if !ok {
			continue
		}
		rule, hit := matchRule(m.Descricao)
		if !hit {
			continue
		}
		if !found || d.After(melhor) {
			melhor = d
			ato = m.Descricao
			melhorRule = rule
			found = true
		}
	}
	return melhor, ato, melhorRule, found
}

type Prazo struct {
	Numero        string `json:"numero"`
	Tribunal      string `json:"tribunal"`
	Classe        string `json:"classe"`
	Ato           string `json:"ato"`
	Rotulo        string `json:"rotulo"`         // tipo de prazo (ex.: Contestação)
	Base          string `json:"base"`           // fundamento legal
	PrazoDias     int    `json:"prazo_dias"`     // dias úteis aplicados
	Inicio        string `json:"inicio"`
	Vencimento    string `json:"vencimento"`
	DiasRestantes int    `json:"dias_restantes"` // úteis; negativo = vencido
	Status        string `json:"status"`         // vencido | hoje | urgente | emdia
	TipoContagem  string `json:"tipo_contagem"`  // oficial | estimado (não oficial)
}

// diasUteisEntre conta dias úteis de a (exclusivo) até b (inclusivo) considerando a UF.
func diasUteisEntre(a, b time.Time, uf string) int {
	a = time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, a.Location())
	b = time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, b.Location())
	if b.Equal(a) {
		return 0
	}
	sinal := 1
	ini, fim := a, b
	if b.Before(a) {
		sinal = -1
		ini, fim = b, a
	}
	cont := 0
	t := ini
	for t.Before(fim) {
		t = t.AddDate(0, 0, 1)
		if ehDiaUtil(t, uf) {
			cont++
		}
	}
	return sinal * cont
}

func computePrazos(userID string) []Prazo {
	hoje := time.Now()
	out := []Prazo{}
	hasOfficial := make(map[string]bool)

	// 1) Carregar intimações oficiais do banco
	if DBPool != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		rows, err := DBPool.Query(ctx, `
			SELECT numero_processo, tribunal, tipo_comunicacao, data_publicacao, prazo_dias, prazo_rotulo, prazo_base, vencimento, status
			FROM public.intimacoes
			WHERE user_id = $1
		`, userID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var pz Prazo
				var pubDate, vencDate time.Time
				var tipoCom string
				err := rows.Scan(
					&pz.Numero,
					&pz.Tribunal,
					&tipoCom,
					&pubDate,
					&pz.PrazoDias,
					&pz.Rotulo,
					&pz.Base,
					&vencDate,
					&pz.Status,
				)
				if err != nil {
					continue
				}

				uf := extractUFFromTribunal(pz.Tribunal)
				pz.Ato = fmt.Sprintf("Intimação oficial via DJEN (%s)", tipoCom)
				pz.Inicio = pubDate.Format("02/01/2006")
				pz.Vencimento = vencDate.Format("02/01/2006")
				pz.DiasRestantes = diasUteisEntre(hoje, vencDate, uf)
				pz.TipoContagem = "oficial"

				hasOfficial[pz.Numero] = true
				out = append(out, pz)
			}
		}
	}

	// 2) Carregar processos e estimar prazos para os que não têm prazo oficial
	stored := loadProcessos(userID)
	for _, sp := range stored {
		p := sp.Process
		if hasOfficial[p.Numero] {
			continue // Já possui prazo oficial
		}

		inicio, ato, rule, ok := gatilhoPrazoMaisRecente(p)
		if !ok {
			continue
		}
		uf := extractUFFromTribunal(p.Tribunal)
		venc := adicionaDiasUteis(inicio, rule.dias, uf)
		restantes := diasUteisEntre(hoje, venc, uf)

		var status string
		switch {
		case restantes < 0:
			status = "vencido"
		case restantes == 0:
			status = "hoje"
		case restantes <= 3:
			status = "urgente"
		default:
			status = "emdia"
		}

		out = append(out, Prazo{
			Numero:        p.Numero,
			Tribunal:      p.Tribunal,
			Classe:        p.Classe,
			Ato:           ato,
			Rotulo:        rule.rotulo,
			Base:          rule.base,
			PrazoDias:     rule.dias,
			Inicio:        inicio.Format("02/01/2006"),
			Vencimento:    venc.Format("02/01/2006"),
			DiasRestantes: restantes,
			Status:        status,
			TipoContagem:  "estimado (não oficial)",
		})
	}
	return out
}
