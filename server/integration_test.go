package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// ─── Testes de cálculos judiciais ────────────────────────────────────────────

func TestCorrecaoMonetariaSELIC(t *testing.T) {
	req := CalculoRequest{
		Tipo:           "correcao_monetaria",
		ValorPrincipal: 10000.00,
		DataBase:       "01/01/2020",
		DataFim:        "31/12/2022",
		Indice:         "selic",
	}
	res, err := calcularCorrecaoMonetaria(req)
	if err != nil {
		t.Fatalf("calcularCorrecaoMonetaria retornou erro: %v", err)
	}
	if res.ValorCorrigido <= res.ValorPrincipal {
		t.Errorf("Esperava valor corrigido > principal. Got: %.2f", res.ValorCorrigido)
	}
	if res.FatorCorrecao <= 1.0 {
		t.Errorf("Fator de correção deveria ser > 1. Got: %.4f", res.FatorCorrecao)
	}
	if len(res.Observacoes) == 0 {
		t.Error("Resultado deveria conter observações")
	}
}

func TestCorrecaoMonetariaTR(t *testing.T) {
	req := CalculoRequest{
		Tipo:           "correcao_monetaria",
		ValorPrincipal: 5000.00,
		DataBase:       "01/01/2020",
		DataFim:        "01/01/2023",
		Indice:         "tr",
	}
	res, err := calcularCorrecaoMonetaria(req)
	if err != nil {
		t.Fatalf("TR retornou erro: %v", err)
	}
	// TR pós-2017 ≈ 0%, valor corrigido deve ser igual ao principal
	if res.ValorCorrigido != res.ValorPrincipal {
		t.Errorf("TR esperava valor igual ao principal (%.2f), obteve %.2f", res.ValorPrincipal, res.ValorCorrigido)
	}
}

func TestLiquidacaoSentenca(t *testing.T) {
	req := CalculoRequest{
		Tipo:           "liquidacao",
		ValorPrincipal: 20000.00,
		DataBase:       "01/01/2021",
		DataFim:        "01/01/2024",
		Indice:         "selic",
		JurosMes:       1.0,
		Honorarios:     10.0,
	}
	res, err := calcularLiquidacao(req)
	if err != nil {
		t.Fatalf("calcularLiquidacao retornou erro: %v", err)
	}
	if res.TotalGeral <= res.ValorPrincipal {
		t.Errorf("Total da liquidação deveria ser > principal. Got: %.2f", res.TotalGeral)
	}
	if res.Honorarios <= 0 {
		t.Errorf("Honorários deveriam ser > 0. Got: %.2f", res.Honorarios)
	}
	if len(res.Parcelas) < 3 {
		t.Errorf("Liquidação deveria ter pelo menos 3 parcelas. Got: %d", len(res.Parcelas))
	}
	// Verifica que honorários = 10% do subtotal
	subtotal := res.ValorCorrigido + res.JurosMontante
	expectedHon := arredondar2(subtotal * 0.10)
	if res.Honorarios != expectedHon {
		t.Errorf("Honorários esperados: %.2f, obtido: %.2f", expectedHon, res.Honorarios)
	}
}

func TestCalcTrabalhista(t *testing.T) {
	req := CalculoRequest{
		Tipo:             "trabalhista",
		SalarioMensal:    3000.00,
		MesesTrabalhados: 24,
		AvisoPrevio:      true,
		FaltaGrave:       false,
	}
	res, err := calcularTrabalhista(req)
	if err != nil {
		t.Fatalf("calcularTrabalhista retornou erro: %v", err)
	}
	if res.TotalGeral <= 0 {
		t.Errorf("Total trabalhista deve ser > 0. Got: %.2f", res.TotalGeral)
	}
	// FGTS = 8% x 3000 x 24 = 5760
	expectedFGTS := arredondar2(3000 * 0.08 * 24)
	foundFGTS := false
	for _, p := range res.Parcelas {
		if strings.Contains(p.Descricao, "FGTS") && p.Valor == expectedFGTS {
			foundFGTS = true
		}
	}
	if !foundFGTS {
		t.Errorf("Parcela FGTS não encontrada ou com valor incorreto (esperado %.2f)", expectedFGTS)
	}
	// Com falta grave = false e aviso prévio = true, deve ter multa FGTS
	foundMulta := false
	for _, p := range res.Parcelas {
		if strings.Contains(p.Descricao, "Multa FGTS") {
			foundMulta = true
		}
	}
	if !foundMulta {
		t.Error("Parcela 'Multa FGTS' ausente sem falta grave")
	}
}

func TestCalcTrabalhistaFaltaGrave(t *testing.T) {
	req := CalculoRequest{
		Tipo:             "trabalhista",
		SalarioMensal:    2500.00,
		MesesTrabalhados: 12,
		AvisoPrevio:      false,
		FaltaGrave:       true,
	}
	res, err := calcularTrabalhista(req)
	if err != nil {
		t.Fatalf("calcularTrabalhista (falta grave) retornou erro: %v", err)
	}
	for _, p := range res.Parcelas {
		if strings.Contains(p.Descricao, "Multa FGTS") {
			t.Error("Com falta grave não deve haver multa FGTS 40%")
		}
		if strings.Contains(p.Descricao, "Aviso prévio") {
			t.Error("Com falta grave não deve haver aviso prévio")
		}
	}
}

func TestJurosSimples(t *testing.T) {
	base := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	fim := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	juros := calcularJuros(10000, base, fim, 1.0, false)
	// 12 meses x 1% = 12% sobre 10000 = 1200
	expected := 1200.0
	if juros != expected {
		t.Errorf("Juros simples esperados: %.2f, obtido: %.2f", expected, juros)
	}
}

func TestExtrairValor(t *testing.T) {
	cases := []struct {
		texto    string
		expected float64
	}{
		{"o valor é R$ 5.000,00 de principal", 5000.0},
		{"preciso calcular 12500 de correção", 12500.0},
		{"R$ 1.234.567,89 de débito", 1234567.89},
	}
	for _, tc := range cases {
		got := extrairValor(tc.texto)
		if got != tc.expected {
			t.Errorf("extrairValor(%q) = %.2f, esperado %.2f", tc.texto, got, tc.expected)
		}
	}
}

// ─── Testes do Portal do Cliente ──────────────────────────────────────────────

func TestSituacaoLeiga(t *testing.T) {
	cases := []struct {
		descricao string
		expected  string
	}{
		{"Sentença prolatada pelo juízo", "sentenca_proferida"},
		{"Acórdão publicado no DJE", "julgado_em_segunda_instancia"},
		{"Penhora de ativos financeiros", "em_execucao"},
		{"Audiência de instrução realizada", "aguardando_audiencia"},
		{"Processo arquivado definitivamente", "encerrado"},
		{"Despacho ordinatório", "em_andamento"},
	}
	for _, tc := range cases {
		movs := []Movement{{Descricao: tc.descricao}}
		got := situacaoLeiga("Cível", movs)
		if got != tc.expected {
			t.Errorf("situacaoLeiga(%q) = %q, esperado %q", tc.descricao, got, tc.expected)
		}
	}
}

func TestSimplificarMovimento(t *testing.T) {
	cases := []struct {
		input    string
		contains string
	}{
		{"Ato ordinatório de citação expedido", "A outra parte foi informada"},
		{"Contestação apresentada tempestivamente", "A outra parte apresentou sua defesa"},
		{"Sentença procedente publicada", "O juiz proferiu uma decisão"},
	}
	for _, tc := range cases {
		got := simplificarMovimento(tc.input)
		if !strings.Contains(got, tc.contains) {
			t.Errorf("simplificarMovimento(%q) = %q; esperava conter %q", tc.input, got, tc.contains)
		}
	}
}

// ─── Testes de detecção de intenção do Ross agêntico ─────────────────────────

func TestDetectIntent(t *testing.T) {
	cases := []struct {
		msg      string
		expected agentTool
	}{
		{"quais prazos vencem esta semana?", toolListarPrazos},
		{"me mostre as minutas geradas", toolListarMinutas},
		{"tem alguma divergência de prazo?", toolBuscarDivergencias},
		{"vigiar o CNPJ 12345678000190", toolCriarVigilancia},
		{"quem estou monitorando?", toolListarVigilancias},
		{"o que mudou nas vigilâncias?", toolListarAlertasVigia},
		{"como funciona o sistema?", toolNenhuma},
	}
	for _, tc := range cases {
		got := detectIntent(tc.msg)
		if got.Tool != tc.expected {
			t.Errorf("detectIntent(%q) = %q, esperado %q", tc.msg, got.Tool, tc.expected)
		}
	}
}

func TestExtractDocumento(t *testing.T) {
	cases := []struct {
		texto    string
		expected string
	}{
		{"vigiar CPF 12345678901 da parte adversa", "12345678901"},
		{"monitorar CNPJ 12345678000190", "12345678000190"},
		{"vigiar João Silva sem documento", ""},
	}
	for _, tc := range cases {
		got := extractDocumento(tc.texto)
		if got != tc.expected {
			t.Errorf("extractDocumento(%q) = %q, esperado %q", tc.texto, got, tc.expected)
		}
	}
}

// ─── Testes de validação de JSON dos 13 campos da análise ────────────────────

func TestValidacaoJSON13Campos(t *testing.T) {
	// Simula o JSON que a IA deveria retornar para análise estruturada
	exemploJSON := `{
		"summary": "Ação de cobrança de dívida...",
		"key_observations": ["Documento vencido em 2021"],
		"critical_clauses": [{"texto": "cláusula de multa 20%", "risco": "alto"}],
		"risk_points": [{"descricao": "Prescrição 3 anos CC art. 206", "recomendacao": "ajuizar urgente"}],
		"obligations": [{"parte": "réu", "obrigacao": "pagar R$ 10.000", "prazo": "30 dias"}],
		"timeline": [{"data": "2021-01", "evento": "Inadimplemento"}],
		"thesis_party_a": "Cobrança é legítima e tempestiva",
		"thesis_party_b": "Prescrição já ocorreu",
		"legal_basis": ["CC art. 206 §3 VIII", "STJ Súm. 149"],
		"procedural_risks": "Risco de prescrição se não ajuizado em 30 dias",
		"probability_assessment": "Favorável ao autor se não prescrito (60%)",
		"lawyer_recommendations": ["Ajuizar imediatamente", "Requerer tutela de urgência"],
		"client_observations": "Processo tem boa chance de êxito se ajuizado esta semana"
	}`

	camposObrigatorios := []string{
		"summary", "key_observations", "critical_clauses",
		"risk_points", "obligations", "timeline",
		"thesis_party_a", "thesis_party_b", "legal_basis",
		"procedural_risks", "probability_assessment",
		"lawyer_recommendations", "client_observations",
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(exemploJSON), &parsed); err != nil {
		t.Fatalf("JSON de análise inválido: %v", err)
	}

	for _, campo := range camposObrigatorios {
		if _, ok := parsed[campo]; !ok {
			t.Errorf("Campo obrigatório ausente na análise: %q", campo)
		}
	}

	if len(camposObrigatorios) != 13 {
		t.Errorf("Deveria haver 13 campos. Got: %d", len(camposObrigatorios))
	}
}

// ─── Testes do parser DJEN ────────────────────────────────────────────────────

func TestParseDJENData(t *testing.T) {
	// Verifica que datas no formato YYYY-MM-DD são parseadas corretamente
	dataStr := "2026-06-15"
	data, err := time.Parse("2006-01-02", dataStr)
	if err != nil {
		t.Fatalf("Erro ao parsear data DJEN: %v", err)
	}
	if data.Year() != 2026 || data.Month() != 6 || data.Day() != 15 {
		t.Errorf("Data parseada incorretamente: %v", data)
	}

	// Marco legal: publicação é o próximo dia útil
	pubDate := adicionaDiasUteis(data, 1, "SP")
	if !pubDate.After(data) {
		t.Errorf("Data de publicação deveria ser após disponibilização")
	}
	if pubDate.Weekday() == time.Saturday || pubDate.Weekday() == time.Sunday {
		t.Errorf("Data de publicação não pode ser fim de semana. Got: %v", pubDate.Weekday())
	}
}

func TestMatchRuleDJEN(t *testing.T) {
	cases := []struct {
		texto           string
		expectedRotulo  string
		expectedDias    int
	}{
		{"intime-se para contestação", "Contestação", 15},
		{"prazo para interpor apelação", "Apelação", 15},
		{"embargos de declaração", "Embargos de declaração", 5},
		{"recurso inominado da sentença", "Recurso inominado", 10},
	}
	for _, tc := range cases {
		rule, hit := matchRule(tc.texto)
		if !hit {
			t.Errorf("matchRule(%q): nenhuma regra encontrada", tc.texto)
			continue
		}
		if rule.rotulo != tc.expectedRotulo {
			t.Errorf("matchRule(%q): rótulo = %q, esperado %q", tc.texto, rule.rotulo, tc.expectedRotulo)
		}
		if rule.dias != tc.expectedDias {
			t.Errorf("matchRule(%q): dias = %d, esperado %d", tc.texto, rule.dias, tc.expectedDias)
		}
	}
}

// ─── Testes do fluxo LGPD ─────────────────────────────────────────────────────

func TestMaskID(t *testing.T) {
	id := "550e8400-e29b-41d4-a716-446655440000"
	masked := maskID(id)
	if strings.Contains(masked, id[4:len(id)-4]) {
		t.Errorf("maskID deveria ofuscar o meio do ID. Got: %s", masked)
	}
	if !strings.HasPrefix(masked, id[:4]) {
		t.Errorf("maskID deveria manter os primeiros 4 chars. Got: %s", masked)
	}
}
