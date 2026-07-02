package main

// calculos.go — Motor de Cálculos Judiciais (Diferencial 9.5)
//
// Suporta:
//   • Correção monetária: SELIC acumulada, INPC simplificado, TR (pós-fixado 0%)
//   • Juros moratórios: simples (1%/mês) ou compostos
//   • Liquidação de sentença: principal + correção + juros + honorários
//   • Verbas trabalhistas: FGTS, férias, 13º, aviso prévio, multa 477/CLT
//
// Os índices de correção são aproximações embutidas no binário para
// funcionar sem chamadas externas. Em produção, substituir por
// consulta à API do Banco Central (SGS) ou IBGE.

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ─── Índices de correção (aproximações anuais acumuladas) ─────────────────────
// Fonte referencial: BCB/IBGE. Atualizar periodicamente via env ou tabela no banco.

type indiceAnual struct {
	Ano  int
	Taxa float64 // percentual ao ano, ex: 5.79 = 5,79% a.a.
}

// SELIC acumulada por ano (meta, referencial)
var selicAnual = []indiceAnual{
	{2015, 13.25}, {2016, 13.75}, {2017, 7.00}, {2018, 6.50},
	{2019, 4.50}, {2020, 2.00}, {2021, 9.25}, {2022, 13.75},
	{2023, 11.75}, {2024, 10.50}, {2025, 13.25}, {2026, 13.75},
}

// INPC acumulado por ano (IBGE)
var inpcAnual = []indiceAnual{
	{2015, 11.28}, {2016, 6.58}, {2017, 2.07}, {2018, 3.43},
	{2019, 4.48}, {2020, 5.45}, {2021, 10.16}, {2022, 5.93},
	{2023, 3.71}, {2024, 4.83}, {2025, 5.10}, {2026, 4.50},
}

func taxaAnual(indices []indiceAnual, ano int) float64 {
	for _, idx := range indices {
		if idx.Ano == ano {
			return idx.Taxa
		}
	}
	// Se não encontrar, usa o último disponível
	if len(indices) > 0 {
		return indices[len(indices)-1].Taxa
	}
	return 6.0
}

// fatorCorrecao retorna o fator multiplicativo acumulado entre dataBase e dataFim.
func fatorCorrecao(indices []indiceAnual, dataBase, dataFim time.Time) float64 {
	if !dataFim.After(dataBase) {
		return 1.0
	}
	fator := 1.0
	for ano := dataBase.Year(); ano <= dataFim.Year(); ano++ {
		taxa := taxaAnual(indices, ano)
		fracao := 1.0
		if ano == dataBase.Year() && ano == dataFim.Year() {
			// Fração do ano entre as datas
			diasAno := float64(time.Date(ano+1, 1, 1, 0, 0, 0, 0, time.UTC).Sub(time.Date(ano, 1, 1, 0, 0, 0, 0, time.UTC)).Hours() / 24)
			diasPeriodo := dataFim.Sub(dataBase).Hours() / 24
			fracao = diasPeriodo / diasAno
		} else if ano == dataBase.Year() {
			diasAno := float64(time.Date(ano+1, 1, 1, 0, 0, 0, 0, time.UTC).Sub(time.Date(ano, 1, 1, 0, 0, 0, 0, time.UTC)).Hours() / 24)
			diasRestantes := time.Date(ano+1, 1, 1, 0, 0, 0, 0, time.UTC).Sub(dataBase).Hours() / 24
			fracao = diasRestantes / diasAno
		} else if ano == dataFim.Year() {
			diasAno := float64(time.Date(ano+1, 1, 1, 0, 0, 0, 0, time.UTC).Sub(time.Date(ano, 1, 1, 0, 0, 0, 0, time.UTC)).Hours() / 24)
			diasDecorridos := dataFim.Sub(time.Date(ano, 1, 1, 0, 0, 0, 0, time.UTC)).Hours() / 24
			fracao = diasDecorridos / diasAno
		}
		fator *= math.Pow(1+taxa/100, fracao)
	}
	return fator
}

func arredondar2(v float64) float64 {
	return math.Round(v*100) / 100
}

// ─── Tipos de requisição e resposta ──────────────────────────────────────────

type CalculoRequest struct {
	Tipo          string  `json:"tipo"`           // correcao_monetaria | liquidacao | trabalhista | juros
	ValorPrincipal float64 `json:"valor_principal"`
	DataBase      string  `json:"data_base"`      // DD/MM/YYYY ou YYYY-MM-DD
	DataFim       string  `json:"data_fim"`       // padrão: hoje
	Indice        string  `json:"indice"`         // selic | inpc | tr | nenhum
	JurosMes      float64 `json:"juros_mes"`      // percentual mensal (padrão 1.0)
	JurosCompostos bool   `json:"juros_compostos"`
	Honorarios    float64 `json:"honorarios_pct"` // % de honorários sobre o total
	// Trabalhista
	SalarioMensal float64 `json:"salario_mensal"`
	MesesTrabalhados int  `json:"meses_trabalhados"`
	AvisoPrevio   bool    `json:"aviso_previo"`
	FaltaGrave    bool    `json:"falta_grave"`    // se true: sem FGTS multa 40%
}

type CalculoResultado struct {
	Tipo              string             `json:"tipo"`
	ValorPrincipal    float64            `json:"valor_principal"`
	DataBase          string             `json:"data_base"`
	DataFim           string             `json:"data_fim"`
	Indice            string             `json:"indice"`
	FatorCorrecao     float64            `json:"fator_correcao,omitempty"`
	ValorCorrigido    float64            `json:"valor_corrigido,omitempty"`
	JurosMontante     float64            `json:"juros_montante,omitempty"`
	Honorarios        float64            `json:"honorarios,omitempty"`
	TotalGeral        float64            `json:"total_geral"`
	Parcelas          []ParcelaCalculo   `json:"parcelas,omitempty"`
	Observacoes       []string           `json:"observacoes"`
}

type ParcelaCalculo struct {
	Descricao string  `json:"descricao"`
	Valor     float64 `json:"valor"`
}

func parseData(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"02/01/2006", "2006-01-02", "01/2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("data inválida: %q", s)
}

// ─── Cálculos ─────────────────────────────────────────────────────────────────

func calcularCorrecaoMonetaria(req CalculoRequest) (CalculoResultado, error) {
	base, err := parseData(req.DataBase)
	if err != nil {
		return CalculoResultado{}, fmt.Errorf("data_base inválida: %w", err)
	}
	fim := time.Now()
	if req.DataFim != "" {
		fim, err = parseData(req.DataFim)
		if err != nil {
			return CalculoResultado{}, fmt.Errorf("data_fim inválida: %w", err)
		}
	}

	var indices []indiceAnual
	var nomeIndice string
	switch strings.ToLower(req.Indice) {
	case "inpc":
		indices = inpcAnual
		nomeIndice = "INPC/IBGE"
	case "tr":
		// TR pós-2017 ≈ 0% — mantida por tradição em alguns contratos
		nomeIndice = "TR (referencial — aproximado 0% pós-2017)"
		res := CalculoResultado{
			Tipo: "correcao_monetaria", Indice: nomeIndice,
			ValorPrincipal: req.ValorPrincipal, DataBase: base.Format("02/01/2006"), DataFim: fim.Format("02/01/2006"),
			FatorCorrecao: 1.0, ValorCorrigido: arredondar2(req.ValorPrincipal),
			TotalGeral: arredondar2(req.ValorPrincipal),
			Observacoes: []string{"TR pós-2017 é praticamente zero. Confirme com tabela do Banco Central (SGS série 7810)."},
		}
		return res, nil
	default:
		indices = selicAnual
		nomeIndice = "SELIC/BCB"
	}

	// 4 casas para o fator
	fator4 := math.Round(fatorCorrecao(indices, base, fim)*10000) / 10000
	corrigido := arredondar2(req.ValorPrincipal * fator4)

	return CalculoResultado{
		Tipo: "correcao_monetaria", Indice: nomeIndice,
		ValorPrincipal: req.ValorPrincipal, DataBase: base.Format("02/01/2006"), DataFim: fim.Format("02/01/2006"),
		FatorCorrecao: fator4, ValorCorrigido: corrigido, TotalGeral: corrigido,
		Observacoes: []string{
			fmt.Sprintf("Fator calculado com base nas taxas anuais do %s.", nomeIndice),
			"Resultado aproximado — confirme com tabela oficial antes de protocolar.",
		},
	}, nil
}

func calcularJuros(principal float64, dataBase, dataFim time.Time, jurosMes float64, compostos bool) float64 {
	if jurosMes <= 0 {
		jurosMes = 1.0 // 1% ao mês (padrão CPC art. 406 / SELIC)
	}
	meses := (dataFim.Year()-dataBase.Year())*12 + int(dataFim.Month()-dataBase.Month())
	if meses <= 0 {
		return 0
	}
	taxa := jurosMes / 100
	if compostos {
		return arredondar2(principal * (math.Pow(1+taxa, float64(meses)) - 1))
	}
	return arredondar2(principal * taxa * float64(meses))
}

func calcularLiquidacao(req CalculoRequest) (CalculoResultado, error) {
	base, err := parseData(req.DataBase)
	if err != nil {
		return CalculoResultado{}, fmt.Errorf("data_base inválida: %w", err)
	}
	fim := time.Now()
	if req.DataFim != "" {
		if fim, err = parseData(req.DataFim); err != nil {
			return CalculoResultado{}, fmt.Errorf("data_fim inválida: %w", err)
		}
	}

	var indices []indiceAnual
	nomeIndice := "SELIC/BCB"
	if strings.ToLower(req.Indice) == "inpc" {
		indices = inpcAnual
		nomeIndice = "INPC/IBGE"
	} else {
		indices = selicAnual
	}

	fator := math.Round(fatorCorrecao(indices, base, fim)*10000) / 10000
	corrigido := arredondar2(req.ValorPrincipal * fator)
	juros := calcularJuros(corrigido, base, fim, req.JurosMes, req.JurosCompostos)
	subtotal := arredondar2(corrigido + juros)
	honorarios := arredondar2(subtotal * req.Honorarios / 100)
	total := arredondar2(subtotal + honorarios)

	parcelas := []ParcelaCalculo{
		{"Principal", arredondar2(req.ValorPrincipal)},
		{fmt.Sprintf("Correção monetária (%s, fator %.4f)", nomeIndice, fator), arredondar2(corrigido - req.ValorPrincipal)},
		{"Juros moratórios", juros},
	}
	if honorarios > 0 {
		parcelas = append(parcelas, ParcelaCalculo{fmt.Sprintf("Honorários advocatícios (%.1f%%)", req.Honorarios), honorarios})
	}

	obs := []string{
		"Cálculo de liquidação de sentença (CPC art. 509).",
		"Correção monetária: data-base = citação ou inadimplemento; juros: data-base = citação (STJ Súm. 54 / 163).",
		"Confirme a data-base correta com a decisão que fixou os critérios.",
	}

	return CalculoResultado{
		Tipo: "liquidacao", Indice: nomeIndice,
		ValorPrincipal: req.ValorPrincipal, DataBase: base.Format("02/01/2006"), DataFim: fim.Format("02/01/2006"),
		FatorCorrecao: fator, ValorCorrigido: corrigido,
		JurosMontante: juros, Honorarios: honorarios,
		TotalGeral: total, Parcelas: parcelas, Observacoes: obs,
	}, nil
}

func calcularTrabalhista(req CalculoRequest) (CalculoResultado, error) {
	if req.SalarioMensal <= 0 {
		return CalculoResultado{}, fmt.Errorf("salario_mensal deve ser maior que zero")
	}
	if req.MesesTrabalhados <= 0 {
		return CalculoResultado{}, fmt.Errorf("meses_trabalhados deve ser maior que zero")
	}
	sal := req.SalarioMensal
	meses := req.MesesTrabalhados

	// Saldo de salário (proporcional ao último mês)
	saldo := arredondar2(sal / 30 * 30) // simplificado: 1 mês completo

	// 13º proporcional: (sal / 12) * meses (limitado a 12)
	decimoTerceiro := arredondar2(sal / 12 * float64(min(meses, 12)))

	// Férias proporcionais + 1/3: (sal / 12) * meses * 4/3
	feriasProp := arredondar2((sal / 12) * float64(min(meses, 12)) * (4.0 / 3.0))

	// FGTS depositado: 8% × sal × meses
	fgts := arredondar2(sal * 0.08 * float64(meses))

	// Multa FGTS 40% (só sem falta grave)
	multaFGTS := 0.0
	if !req.FaltaGrave {
		multaFGTS = arredondar2(fgts * 0.40)
	}

	// Aviso prévio
	avisoPrevio := 0.0
	if req.AvisoPrevio && !req.FaltaGrave {
		diasAviso := 30 + min(meses/12, 30) // CLT art. 487: 30 dias + 3/ano (cap 90)
		if meses >= 12 {
			extras := (meses / 12) * 3
			if extras > 60 {
				extras = 60
			}
			diasAviso = 30 + extras
		}
		avisoPrevio = arredondar2(sal / 30 * float64(diasAviso))
	}

	// Multa CLT art. 477 (atraso nas verbas): 1 salário
	multa477 := sal

	total := arredondar2(saldo + decimoTerceiro + feriasProp + fgts + multaFGTS + avisoPrevio + multa477)

	parcelas := []ParcelaCalculo{
		{"Saldo de salário", saldo},
		{"13º salário proporcional", decimoTerceiro},
		{"Férias proporcionais + 1/3", feriasProp},
		{fmt.Sprintf("FGTS (8%% × %d meses)", meses), fgts},
	}
	if !req.FaltaGrave {
		parcelas = append(parcelas, ParcelaCalculo{"Multa FGTS 40% (CLT art. 18 §1)", multaFGTS})
	}
	if avisoPrevio > 0 {
		parcelas = append(parcelas, ParcelaCalculo{"Aviso prévio indenizado", avisoPrevio})
	}
	parcelas = append(parcelas, ParcelaCalculo{"Multa CLT art. 477 (1 salário)", multa477})

	obs := []string{
		"Cálculo simplificado — não inclui horas extras, adicional noturno, DSR, integrações.",
		"Multa art. 477 incluída como provisão; confirme se o prazo de 10 dias foi descumprido.",
	}
	if req.FaltaGrave {
		obs = append(obs, "Falta grave reconhecida: sem aviso prévio nem multa FGTS de 40%.")
	}

	return CalculoResultado{
		Tipo: "trabalhista", ValorPrincipal: sal,
		TotalGeral: total, Parcelas: parcelas, Observacoes: obs,
	}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ─── Handler HTTP ──────────────────────────────────────────────────────────────

func handleCalculos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	var req CalculoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON inválido"})
		return
	}

	var res CalculoResultado
	switch strings.ToLower(req.Tipo) {
	case "correcao_monetaria", "correcao":
		res, err = calcularCorrecaoMonetaria(req)
	case "liquidacao", "liquidação":
		res, err = calcularLiquidacao(req)
	case "trabalhista":
		res, err = calcularTrabalhista(req)
	case "juros":
		base, e := parseData(req.DataBase)
		if e != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": e.Error()})
			return
		}
		fim := time.Now()
		if req.DataFim != "" {
			fim, _ = parseData(req.DataFim)
		}
		juros := calcularJuros(req.ValorPrincipal, base, fim, req.JurosMes, req.JurosCompostos)
		res = CalculoResultado{
			Tipo: "juros", ValorPrincipal: req.ValorPrincipal,
			DataBase: base.Format("02/01/2006"), DataFim: fim.Format("02/01/2006"),
			JurosMontante: juros, TotalGeral: arredondar2(req.ValorPrincipal + juros),
			Parcelas: []ParcelaCalculo{
				{"Principal", req.ValorPrincipal},
				{fmt.Sprintf("Juros moratórios (%.2f%%/mês)", func() float64 { if req.JurosMes > 0 { return req.JurosMes }; return 1.0 }()), juros},
			},
			Observacoes: []string{"Juros de mora: 1%/mês (CPC art. 406, CC art. 406, SELIC como índice legal)."},
		}
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "tipo deve ser: correcao_monetaria, liquidacao, trabalhista ou juros",
		})
		return
	}

	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	M.CalculosRealizados.Add(1)

	// Persiste no banco (auditabilidade)
	if DBPool != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		paramsJSON, _ := json.Marshal(req)
		resJSON, _ := json.Marshal(res)
		_, _ = DBPool.Exec(ctx, `
			INSERT INTO public.calculos (user_id, tipo, parametros, resultado)
			VALUES ($1, $2, $3, $4)
		`, userID, req.Tipo, string(paramsJSON), string(resJSON))
	}

	writeJSON(w, http.StatusOK, res)
}

// handleHistoricoCalculos retorna os últimos cálculos do usuário.
// GET /api/calculos
func handleHistoricoCalculos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET"})
		return
	}
	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}
	if DBPool == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := DBPool.Query(ctx, `
		SELECT id, tipo, resultado, TO_CHAR(created_at,'DD/MM/YYYY HH24:MI')
		FROM public.calculos
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 50
	`, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type item struct {
		ID        string          `json:"id"`
		Tipo      string          `json:"tipo"`
		Resultado json.RawMessage `json:"resultado"`
		CreatedAt string          `json:"created_at"`
	}
	var lista []item
	for rows.Next() {
		var it item
		var resRaw []byte
		if err := rows.Scan(&it.ID, &it.Tipo, &resRaw, &it.CreatedAt); err == nil {
			it.Resultado = resRaw
			lista = append(lista, it)
		}
	}
	if lista == nil {
		lista = []item{}
	}

	// Converter []item para []any para satisfazer writeJSON
	out := make([]any, len(lista))
	for i, v := range lista {
		out[i] = v
	}
	writeJSON(w, http.StatusOK, out)
}

// Integração no Ross agêntico — calcula ao detectar intenção
func executarCalculoRoss(userID string, texto string) string {
	lower := strings.ToLower(texto)
	if !containsAny(lower, "calcul", "corrig", "atualiz", "liquidaç", "liquidacao",
		"fgts", "verbas", "rescisão", "rescisao", "juros") {
		return ""
	}

	// Tenta extrair valor da mensagem (ex: R$ 5.000,00 ou 5000)
	valor := extrairValor(texto)
	if valor <= 0 {
		return "[FERRAMENTA calculos] Detectei intenção de cálculo mas não encontrei o valor principal na mensagem. Pergunte ao usuário: valor principal, data-base e tipo de correção desejado (SELIC ou INPC)."
	}

	return fmt.Sprintf("[FERRAMENTA calculos] Para calcular, o usuário deve usar POST /api/calculos com: { \"tipo\": \"liquidacao\", \"valor_principal\": %.2f, \"data_base\": \"DD/MM/AAAA\", \"indice\": \"selic\" }. Informe-o sobre os campos necessários.", valor)
}

func extrairValor(texto string) float64 {
	// Procura padrões como "R$ 5.000,00" ou "5000" ou "5.000"
	texto = strings.ReplaceAll(texto, "R$", "")
	texto = strings.ReplaceAll(texto, "R $", "")
	for _, token := range strings.Fields(texto) {
		token = strings.Map(func(r rune) rune {
			if (r >= '0' && r <= '9') || r == ',' || r == '.' {
				return r
			}
			return -1
		}, token)
		// Normaliza separadores brasileiros
		if strings.Count(token, ",") == 1 && strings.Count(token, ".") <= 3 {
			token = strings.ReplaceAll(token, ".", "")
			token = strings.ReplaceAll(token, ",", ".")
		}
		if v, err := strconv.ParseFloat(token, 64); err == nil && v > 1 {
			return v
		}
	}
	return 0
}
