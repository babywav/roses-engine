package main

import (
	"testing"
	"time"
)

func TestRegionalHolidays(t *testing.T) {
	// 09/07 é feriado estadual em São Paulo (Revolução Constitucionalista)
	// Mas não é em outros estados como o Rio de Janeiro (RJ) ou Paraíba (PB)

	// Começamos na quarta-feira 08/07/2026 e adicionamos 2 dias úteis
	start := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)

	// Para SP:
	// Dia 1: Sexta 10/07 (pula Quinta 09/07)
	// Dia 2: Segunda 13/07
	vencSP := adicionaDiasUteis(start, 2, "SP")
	expectedSP := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	if !vencSP.Equal(expectedSP) {
		t.Errorf("[SP] Esperava vencimento em %s, obteve %s", expectedSP.Format("02/01/2006"), vencSP.Format("02/01/2006"))
	}

	// Para RJ (onde 09/07 não é feriado):
	// Dia 1: Quinta 09/07
	// Dia 2: Sexta 10/07
	vencRJ := adicionaDiasUteis(start, 2, "RJ")
	expectedRJ := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	if !vencRJ.Equal(expectedRJ) {
		t.Errorf("[RJ] Esperava vencimento em %s, obteve %s", expectedRJ.Format("02/01/2006"), vencRJ.Format("02/01/2006"))
	}
}

func TestCPC220RecessDeadlines(t *testing.T) {
	// CPC art. 220 suspende prazos processuais de 20 de dezembro a 20 de janeiro (inclusive)
	// Se começamos a contagem em 17/12/2026 (Quinta-feira) e adicionamos 3 dias úteis:
	// Dia 1: Sexta 18/12/2026
	// --- SUSPENSÃO DO RECESSO (20/12 a 20/01) ---
	// Dia 2: Quinta 21/01/2027 (próximo dia útil após a suspensão)
	// Dia 3: Sexta 22/01/2027 (Vencimento)

	start := time.Date(2026, 12, 17, 0, 0, 0, 0, time.UTC)
	venc := adicionaDiasUteis(start, 3, "")
	expectedVenc := time.Date(2027, 1, 22, 0, 0, 0, 0, time.UTC)

	if !venc.Equal(expectedVenc) {
		t.Errorf("[CPC 220] Esperava vencimento em %s, obteve %s", expectedVenc.Format("02/01/2006"), venc.Format("02/01/2006"))
	}
}

func TestMockAlerts(t *testing.T) {
	// Garante que o despachante e formatadores de mensagens rodam sem erro/pânico
	TriggerNewIntimacaoAlert("test-user", "0818648-98.2023.8.15.0000", "TJPB", "Contestação", "22/01/2027")
	SendDailyDigestEmail("test-user")
}

