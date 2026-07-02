package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// searchLegalBasis busca no banco public.base_juridica os artigos correspondentes por palavra-chave.
func searchLegalBasis(query string) string {
	if DBPool == nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := DBPool.Query(ctx, `
		SELECT fonte, artigo_titulo, conteudo
		FROM public.base_juridica
		WHERE fts_document @@ plainto_tsquery('portuguese', $1)
		ORDER BY ts_rank(fts_document, plainto_tsquery('portuguese', $1)) DESC
		LIMIT 5
	`, query)
	if err != nil {
		log.Printf("[RAG] Erro ao buscar base jurídica: %v", err)
		return ""
	}
	defer rows.Close()

	var sb strings.Builder
	found := false
	for rows.Next() {
		var fonte, artigo, conteudo string
		if err := rows.Scan(&fonte, &artigo, &conteudo); err == nil {
			found = true
			sb.WriteString(fmt.Sprintf("[%s - %s]\n%s\n\n", fonte, artigo, conteudo))
		}
	}

	if !found {
		return ""
	}

	return sb.String()
}

// SeedBaseJuridica popula a base com artigos e súmulas comuns caso esteja vazia.
func SeedBaseJuridica() {
	if DBPool == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var count int
	err := DBPool.QueryRow(ctx, "SELECT COUNT(*) FROM public.base_juridica").Scan(&count)
	if err == nil && count > 0 {
		return
	}

	log.Println("[RAG] Semeando dados da base jurídica pública (CF, CPC, CLT, CTN)...")

	seeds := []struct {
		fonte    string
		artigo   string
		conteudo string
	}{
		{"CTN", "Art. 173", "O direito de a Fazenda Pública constituir o crédito tributário extingue-se após 5 (cinco) anos, contados: I - do primeiro dia do exercício seguinte àquele em que o lançamento poderia ter sido efetuado; II - da data em que se tornar definitiva a decisão que houver anulado, por vício formal, o lançamento anteriormente efetuado."},
		{"CPC", "Art. 335", "O réu poderá oferecer contestação, por petição, no prazo de 15 (quinze) dias, cujo termo inicial será a data: I - da audiência de conciliação ou de mediação, ou da última sessão de conciliação, quando qualquer parte não comparecer ou, comparecendo, não houver acordo; II - do protocolo do pedido de cancelamento da audiência de conciliação ou de mediação apresentado pelo réu, quando ocorrer a hipótese do art. 334, § 4º, inciso I; III - prevista no art. 231, de acordo com o modo como foi feita a citação, nos demais casos."},
		{"CPC", "Art. 350", "Se o réu alegar fato impeditivo, modificativo ou extintivo do direito do autor, este será ouvido no prazo de 15 (quinze) dias, permitindo-lhe o juiz a produção de prova."},
		{"CPC", "Art. 321", "O juiz, ao verificar que a petição inicial não preenche os requisitos dos arts. 319 e 320 ou que apresenta defeitos e irregularidades capazes de dificultar o julgamento de mérito, determinará que o autor, no prazo de 15 (quinze) dias, a emende ou a complete, indicando com precisão o que deve ser corrigido."},
		{"CPC", "Art. 523", "No caso de condenação em quantia certa, ou já fixada em liquidação, e no caso de decisão sobre parcela incontroversa, o cumprimento da sentença far-se-á a requerimento do exequente, sendo o executado intimado para pagar o débito, no prazo de 15 (quinze) dias, acrescido de custas, se houver."},
		{"CPC", "Art. 525", "Transcorrido o prazo previsto no art. 523 sem o pagamento voluntário, inicia-se o prazo de 15 (quinze) dias para que o executado, independentemente de penhora ou nova intimação, apresente, nos próprios autos, sua impugnação."},
		{"STF", "Súmula 284", "É inadmissível o recurso extraordinário, quando a deficiência na sua fundamentação não permitir a exata compreensão da controvérsia."},
	}

	for _, s := range seeds {
		_, err := DBPool.Exec(ctx, `
			INSERT INTO public.base_juridica (fonte, artigo_titulo, conteudo, fts_document)
			VALUES ($1, $2, $3, to_tsvector('portuguese', $2 || ' ' || $3))
		`, s.fonte, s.artigo, s.conteudo)
		if err != nil {
			log.Printf("[RAG] Erro ao semear %s %s: %v", s.fonte, s.artigo, err)
		}
	}
	log.Println("[RAG] Semeadura da base jurídica concluída com sucesso.")
}
