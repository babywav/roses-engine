package main

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestExtractTextFromDOCX(t *testing.T) {
	// Cria um DOCX em memória (arquivo ZIP contendo word/document.xml)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	xmlContent := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
	<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:body>
			<w:p>
				<w:r>
					<w:t>Olá, esta é uma petição inicial de teste do Ross AI.</w:t>
				</w:r>
			</w:p>
		</w:body>
	</w:document>`

	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatalf("Erro ao criar arquivo no zip do docx: %v", err)
	}
	_, _ = w.Write([]byte(xmlContent))
	_ = zw.Close()

	text, err := ExtractTextFromDOCX(buf.Bytes())
	if err != nil {
		t.Fatalf("Erro ao extrair texto do DOCX: %v", err)
	}

	expected := "Olá, esta é uma petição inicial de teste do Ross AI."
	if text != expected {
		t.Errorf("Esperava '%s', obteve '%s'", expected, text)
	}
}

func TestExtractTextFromXLSX(t *testing.T) {
	// Cria um arquivo Excel XLSX em memória
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Sheet1"
	_ = f.SetCellValue(sheetName, "A1", "Competência")
	_ = f.SetCellValue(sheetName, "B1", "Valor Devido")
	_ = f.SetCellValue(sheetName, "A2", "Janeiro/2026")
	_ = f.SetCellValue(sheetName, "B2", "1500.25")

	var buf bytes.Buffer
	err := f.Write(&buf)
	if err != nil {
		t.Fatalf("Erro ao escrever XLSX: %v", err)
	}

	text, err := ExtractTextFromXLSX(buf.Bytes())
	if err != nil {
		t.Fatalf("Erro ao extrair texto do XLSX: %v", err)
	}

	if !bytes.Contains([]byte(text), []byte("Janeiro/2026, 1500.25")) {
		t.Errorf("Esperava que o texto contivesse 'Janeiro/2026, 1500.25', obteve:\n%s", text)
	}
}

func TestRepairAndParseJSON(t *testing.T) {
	// Simulação de resposta poluída de LLM com cercas markdown e explicações extras
	mockResponse := `Com certeza! Aqui está o resultado da análise estruturada:

	` + "```json" + `
	{
		"summary": "Contestação em face de cobrança indevida",
		"key_observations": "Discussão de preliminar de ilegitimidade",
		"critical_clauses": [
			{"clause": "Cláusula de foro de eleição", "risk_level": "medio"}
		],
		"risk_points": [],
		"obligations": [],
		"timeline": [],
		"thesis_party_a": "Tese do Autor",
		"thesis_party_b": "Tese do Réu",
		"legal_basis": ["Art. 337 CPC"],
		"procedural_risks": "Sem riscos iminentes",
		"probability_assessment": "possível",
		"lawyer_recommendations": [],
		"client_observations": "Conversar com o cliente."
	}
	` + "```" + `

	Espero que essa análise ajude!`

	res, err := repairAndParseJSON(mockResponse)
	if err != nil {
		t.Fatalf("Falha ao reparar e parsear JSON: %v", err)
	}

	if res.Summary != "Contestação em face de cobrança indevida" {
		t.Errorf("Campo summary incorreto: '%s'", res.Summary)
	}
	if len(res.CriticalClauses) != 1 || res.CriticalClauses[0].Clause != "Cláusula de foro de eleição" {
		t.Errorf("Campo critical_clauses incorreto")
	}
}
