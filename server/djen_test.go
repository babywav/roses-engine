package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLegalRulesCalculations(t *testing.T) {
	// Exemplo: Disponibilização numa Quinta-feira (04/06/2026)
	// Publicação deve ser Sexta-feira (05/06/2026)
	// Contagem começa na Segunda-feira (08/06/2026)
	// Prazo de 5 dias úteis:
	// Dia 1: Segunda (08)
	// Dia 2: Terça (09)
	// Dia 3: Quarta (10)
	// Dia 4: Quinta (11)
	// Dia 5: Sexta (12)
	// Vencimento em 12/06/2026 (adicionaDiasUteis conta a partir de sexta-feira)
	dispThur := time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)
	pubFri := adicionaDiasUteis(dispThur, 1, "")

	expectedPub := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	if !pubFri.Equal(expectedPub) {
		t.Errorf("Esperava publicação na Sexta-feira %s, obteve %s", expectedPub.Format("02/01/2006"), pubFri.Format("02/01/2006"))
	}

	// Prazo de 5 dias úteis a partir da publicação
	venc := adicionaDiasUteis(pubFri, 5, "")
	expectedVenc := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	if !venc.Equal(expectedVenc) {
		t.Errorf("Esperava vencimento na Sexta-feira %s, obteve %s", expectedVenc.Format("02/01/2006"), venc.Format("02/01/2006"))
	}
}

func TestParserCNJResponse(t *testing.T) {
	mockJSON := `{
		"status": "success",
		"message": "Sucesso",
		"count": 1,
		"items": [{
			"id": 12345,
			"data_disponibilizacao": "2026-07-01",
			"siglaTribunal": "STJ",
			"tipoComunicacao": "Intimação",
			"texto": "Intime-se para réplica...",
			"numero_processo": "08186489820238150000",
			"tipoDocumento": "Decisão",
			"destinatarios": [],
			"destinatarioadvogados": [{
				"advogado": {
					"numero_oab": "14233",
					"uf_oab": "PB"
				}
			}]
		}]
	}`

	var resp ComunicaResponse
	err := json.Unmarshal([]byte(mockJSON), &resp)
	if err != nil {
		t.Fatalf("Erro ao decodificar JSON de mock: %v", err)
	}

	if resp.Status != "success" || len(resp.Items) != 1 {
		t.Errorf("Esperava status success e 1 item, obteve status='%s' e %d itens", resp.Status, len(resp.Items))
	}

	item := resp.Items[0]
	if item.ID != 12345 || item.NumeroProcesso != "08186489820238150000" {
		t.Errorf("Campos decodificados incorretamente. ID=%d, Numero=%s", item.ID, item.NumeroProcesso)
	}

	if len(item.DestinatarioAdvogados) != 1 || item.DestinatarioAdvogados[0].Advogado.NumeroOAB != "14233" {
		t.Errorf("Advogados do destinatário decodificados incorretamente")
	}
}
