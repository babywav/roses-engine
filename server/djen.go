package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type AdvogadoObj struct {
	ID        int64  `json:"id"`
	Nome      string `json:"nome"`
	NumeroOAB string `json:"numero_oab"`
	UFOAB     string `json:"uf_oab"`
}

type DestinatarioAdvogado struct {
	ID            int64       `json:"id"`
	ComunicacaoID int64       `json:"comunicacao_id"`
	AdvogadoID    int64       `json:"advogado_id"`
	Advogado      AdvogadoObj `json:"advogado"`
}

type DestinatarioItem struct {
	ComunicacaoID int64  `json:"comunicacao_id"`
	Nome          string `json:"nome"`
	Polo          string `json:"polo"`
}

type ComunicacaoItem struct {
	ID                    int64                  `json:"id"`
	DataDisponibilizacao  string                 `json:"data_disponibilizacao"` // YYYY-MM-DD
	SiglaTribunal         string                 `json:"siglaTribunal"`
	TipoComunicacao       string                 `json:"tipoComunicacao"`
	Texto                 string                 `json:"texto"`
	NumeroProcesso        string                 `json:"numero_processo"`
	TipoDocumento         string                 `json:"tipoDocumento"`
	Destinatarios         []DestinatarioItem     `json:"destinatarios"`
	DestinatarioAdvogados []DestinatarioAdvogado `json:"destinatarioadvogados"`
}

type ComunicaResponse struct {
	Status  string            `json:"status"`
	Message string            `json:"message"`
	Count   int               `json:"count"`
	Items   []ComunicacaoItem `json:"items"`
}

type Perfil struct {
	ID        string
	OABNumero string
	OABUF     string
}

// SyncDJEN realiza a consulta paginada de comunicações no CNJ e as salva no banco.
func SyncDJEN(ctx context.Context, userID string, oab string, uf string, desde string) (int, error) {
	if desde == "" {
		desde = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}

	pagina := 1
	novosContados := 0

	for {
		apiURL := fmt.Sprintf("https://comunicaapi.pje.jus.br/api/v1/comunicacao?numeroOab=%s&ufOab=%s&dataDisponibilizacaoInicio=%s&pagina=%d",
			url.QueryEscape(oab),
			url.QueryEscape(uf),
			url.QueryEscape(desde),
			pagina,
		)

		log.Printf("[DJEN] Buscando página %d para OAB %s-%s desde %s...", pagina, oab, uf, desde)

		var respData ComunicaResponse
		err := getWithRetry(apiURL, &respData)
		if err != nil {
			return novosContados, fmt.Errorf("erro ao consultar DJEN na página %d: %w", pagina, err)
		}

		if len(respData.Items) == 0 {
			break
		}

		for _, item := range respData.Items {
			// Filtra para garantir que apenas intimações direcionadas ao advogado em questão sejam adicionadas
			corresponde := false
			for _, da := range item.DestinatarioAdvogados {
				if strings.TrimSpace(da.Advogado.NumeroOAB) == oab && strings.ToUpper(strings.TrimSpace(da.Advogado.UFOAB)) == strings.ToUpper(uf) {
					corresponde = true
					break
				}
			}

			if !corresponde && len(item.DestinatarioAdvogados) > 0 {
				continue
			}

			// Parse da data de disponibilização
			dispDate, err := time.Parse("2006-01-02", item.DataDisponibilizacao)
			if err != nil {
				log.Printf("[DJEN] Erro ao analisar data_disponibilizacao '%s': %v", item.DataDisponibilizacao, err)
				continue
			}

			// Marco legal: publicação é o próximo dia útil da disponibilização
			pubDate := adicionaDiasUteis(dispDate, 1, uf)

			// Identificar regra de prazos com base no teor do texto ou documento
			rule, hit := matchRule(item.Texto)
			if !hit {
				rule, hit = matchRule(item.TipoDocumento)
			}
			if !hit {
				// Default 5 dias úteis (manifestação geral)
				rule = prazoRule{dias: 5, rotulo: "Providência (prazo geral)", base: "CPC art. 218 §3"}
			}

			vencimento := adicionaDiasUteis(pubDate, rule.dias, uf)
			restantes := diasUteisEntre(time.Now(), vencimento, uf)

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

			if DBPool == nil {
				// Desenvolvimento sem banco, ignorar inserção
				continue
			}

			var intimacaoID string
			err = DBPool.QueryRow(ctx, `
				INSERT INTO public.intimacoes (
					user_id, numero_processo, tribunal, tipo_comunicacao, texto, data_disponibilizacao, data_publicacao, prazo_dias, prazo_rotulo, prazo_base, vencimento, status
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
				ON CONFLICT (user_id, numero_processo, data_disponibilizacao, tipo_comunicacao) DO NOTHING
				RETURNING id
			`, userID, item.NumeroProcesso, item.SiglaTribunal, item.TipoComunicacao, item.Texto, dispDate, pubDate, rule.dias, rule.rotulo, rule.base, vencimento, status).Scan(&intimacaoID)

			if err == nil && intimacaoID != "" {
				novosContados++

				// Registra trilha auditável (9.2)
				ufProc := extractUFFromTribunal(item.SiglaTribunal)
				RegistrarAuditoriaPrazo(ctx, userID, intimacaoID,
					item.NumeroProcesso, "djen",
					dispDate, pubDate, rule, vencimento, ufProc,
				)

				// Cross-check DJEN × DataJud/portal em background (9.2)
				go CrossCheckPrazo(ctx, userID, item.NumeroProcesso, vencimento, intimacaoID, ufProc)

				// Dispara a geração automática de minuta da petição em background (9.1)
				go func(uID, iID, numProc, trib, tipoCom, txt string) {
					err := GenerateMinutaForIntimacao(uID, iID, numProc, trib, tipoCom, txt)
					if err != nil {
						log.Printf("[Minuta Auto] Falha ao gerar minuta para intimação %s: %v", iID, err)
					}
				}(userID, intimacaoID, item.NumeroProcesso, item.SiglaTribunal, item.TipoComunicacao, item.Texto)
			}
		}

		if len(respData.Items) < 100 {
			break
		}

		pagina++
		time.Sleep(300 * time.Millisecond) // Respeitar a API
	}

	RegistrarDJENSync(novosContados, false)
	return novosContados, nil
}

func getWithRetry(apiURL string, target interface{}) error {
	var lastErr error
	backoff := 1 * time.Second

	for attempt := 1; attempt <= 3; attempt++ {
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(apiURL)
		if err != nil {
			lastErr = err
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("status HTTP inválido: %d", resp.StatusCode)
			time.Sleep(backoff)
			backoff *= 2
			continue
		}

		err = json.NewDecoder(resp.Body).Decode(target)
		if err != nil {
			return fmt.Errorf("erro ao decodificar resposta JSON: %w", err)
		}

		return nil
	}

	return fmt.Errorf("tentativas esgotadas. Último erro: %w", lastErr)
}

// StartBackgroundSyncWorker inicializa a sincronização em segundo plano diária
func StartBackgroundSyncWorker() {
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		for {
			select {
			case <-ticker.C:
				runGlobalSync()
			}
		}
	}()
	// Executa uma sincronização global assíncrona ao iniciar
	go runGlobalSync()
}

func runGlobalSync() {
	if DBPool == nil {
		log.Println("[DJEN Worker] Banco de dados offline. Sincronização cancelada.")
		return
	}

	log.Println("[DJEN Worker] Iniciando sincronização diária do DJEN...")
	ctx := context.Background()

	rows, err := DBPool.Query(ctx, `
		SELECT id, oab_numero, oab_uf
		FROM public.perfis
		WHERE oab_numero IS NOT NULL AND oab_uf IS NOT NULL
	`)
	if err != nil {
		log.Printf("[DJEN Worker] Erro ao buscar perfis de advogados: %v", err)
		return
	}
	defer rows.Close()

	var perfis []Perfil
	for rows.Next() {
		var p Perfil
		if err := rows.Scan(&p.ID, &p.OABNumero, &p.OABUF); err == nil {
			perfis = append(perfis, p)
		}
	}

	for _, p := range perfis {
		log.Printf("[DJEN Worker] Sincronizando OAB %s-%s para usuário %s...", p.OABNumero, p.OABUF, p.ID)
		count, err := SyncDJEN(ctx, p.ID, p.OABNumero, p.OABUF, "")
		if err != nil {
			log.Printf("[DJEN Worker] Falha ao sincronizar OAB do usuário %s: %v", p.ID, err)
		} else {
			log.Printf("[DJEN Worker] Sincronizado com sucesso! %d novas intimações.", count)
		}
		time.Sleep(1 * time.Second) // Delay para rate limiting
	}

	log.Println("[DJEN Worker] Sincronização diária do DJEN concluída.")
}
