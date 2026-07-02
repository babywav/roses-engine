package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dslipak/pdf"
	"github.com/xuri/excelize/v2"
)

// ExtractText identifica a extensão do arquivo e extrai o texto correspondente.
func ExtractText(filename string, data []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".pdf":
		return ExtractTextFromPDF(data)
	case ".docx":
		return ExtractTextFromDOCX(data)
	case ".xlsx", ".xls":
		return ExtractTextFromXLSX(data)
	case ".txt", ".csv", ".json", ".xml", ".html", ".md":
		return string(data), nil
	default:
		return "", fmt.Errorf("formato de arquivo %s não suportado para extração de texto", ext)
	}
}

// ExtractTextFromPDF extrai o texto de um PDF página a página.
func ExtractTextFromPDF(data []byte) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("falha ao ler PDF: %w", err)
	}

	var sb strings.Builder
	numPages := r.NumPage()
	for p := 1; p <= numPages; p++ {
		page := r.Page(p)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("[Página %d]\n", p))
		sb.WriteString(strings.TrimSpace(text))
		sb.WriteString("\n\n")
	}

	return strings.TrimSpace(sb.String()), nil
}

// ExtractTextFromDOCX lê o arquivo w:t do xml interno do DOCX.
func ExtractTextFromDOCX(data []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("falha ao abrir zip do docx: %w", err)
	}

	var xmlContent []byte
	for _, f := range reader.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			xmlContent, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return "", err
			}
			break
		}
	}

	if len(xmlContent) == 0 {
		return "", fmt.Errorf("word/document.xml não encontrado no arquivo docx")
	}

	// Regex simples e robusta para capturar o texto dentro das tags <w:t>
	re := regexp.MustCompile(`<w:t[^>]*>([^<]*)</w:t>`)
	matches := re.FindAllSubmatch(xmlContent, -1)

	var sb strings.Builder
	for _, m := range matches {
		if len(m) > 1 {
			sb.Write(m[1])
			sb.WriteString(" ")
		}
	}

	return strings.TrimSpace(sb.String()), nil
}

// ExtractTextFromXLSX extrai dados de planilhas Excel formatando como CSV por aba.
func ExtractTextFromXLSX(data []byte) (string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("falha ao ler planilha: %w", err)
	}
	defer f.Close()

	var sb strings.Builder
	sheets := f.GetSheetList()
	for _, sheetName := range sheets {
		rows, err := f.GetRows(sheetName)
		if err != nil {
			continue
		}

		sb.WriteString(fmt.Sprintf("--- Planilha: %s (%d linhas) ---\n", sheetName, len(rows)))
		for _, row := range rows {
			var line []string
			for _, colCell := range row {
				line = append(line, colCell)
			}
			sb.WriteString(strings.Join(line, ", "))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return strings.TrimSpace(sb.String()), nil
}
