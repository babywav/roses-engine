package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Persistência dos processos. Salva e carrega do PostgreSQL.
// Se DBPool for nulo (desenvolvimento offline), faz fallback para o arquivo JSON.

var storeMu sync.Mutex

type StoredProcess struct {
	Process
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

func dataDir() string {
	return getenv("ROSES_DATA_DIR", filepath.Join("..", "data"))
}

func processosPath() string {
	return filepath.Join(dataDir(), "processos.json")
}

// loadProcessos carrega os processos associados ao userID.
func loadProcessos(userID string) []StoredProcess {
	if DBPool == nil {
		return loadProcessosJSON()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := DBPool.Query(ctx, `
		SELECT numero, tribunal, classe, assunto, orgao_julgador, data_distribuicao, partes, movimentacoes, last_seen
		FROM public.processos
		WHERE user_id = $1
	`, userID)
	if err != nil {
		log.Printf("[store] Erro ao carregar processos do banco (usando fallback JSON): %v", err)
		return loadProcessosJSON()
	}
	defer rows.Close()

	var procs []StoredProcess
	for rows.Next() {
		var sp StoredProcess
		var partesBlob, movsBlob []byte
		var dataDist *time.Time
		err := rows.Scan(
			&sp.Numero,
			&sp.Tribunal,
			&sp.Classe,
			&sp.Assunto,
			&sp.OrgaoJulgador,
			&dataDist,
			&partesBlob,
			&movsBlob,
			&sp.LastSeen,
		)
		if err != nil {
			log.Printf("[store] Erro ao ler campos do processo: %v", err)
			continue
		}

		if dataDist != nil {
			sp.DataDistribuicao = dataDist.Format("02/01/2006")
		}

		if len(partesBlob) > 0 {
			_ = json.Unmarshal(partesBlob, &sp.Partes)
		}
		if len(movsBlob) > 0 {
			_ = json.Unmarshal(movsBlob, &sp.Movimentacoes)
		}
		sp.FirstSeen = sp.LastSeen

		procs = append(procs, sp)
	}

	return procs
}

// saveProcessos faz upsert dos processos do usuário no banco.
func saveProcessos(userID string, procs []Process) {
	if len(procs) == 0 {
		return
	}
	if DBPool == nil {
		saveProcessosJSON(procs)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now()
	for _, p := range procs {
		if p.Numero == "" {
			continue
		}

		partesBlob, _ := json.Marshal(p.Partes)
		movsBlob, _ := json.Marshal(p.Movimentacoes)
		dataDist := parseDateOrNil(p.DataDistribuicao)

		// Identifica a fonte
		fonte := "datajud"
		if p.URLProcesso != "" {
			fonte = "portal"
		}

		_, err := DBPool.Exec(ctx, `
			INSERT INTO public.processos (
				user_id, numero, tribunal, classe, assunto, orgao_julgador, fonte, data_distribuicao, partes, movimentacoes, last_seen
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (user_id, numero) DO UPDATE SET
				classe = COALESCE(NULLIF(EXCLUDED.classe, ''), public.processos.classe),
				assunto = COALESCE(NULLIF(EXCLUDED.assunto, ''), public.processos.assunto),
				orgao_julgador = COALESCE(NULLIF(EXCLUDED.orgao_julgador, ''), public.processos.orgao_julgador),
				fonte = EXCLUDED.fonte,
				data_distribuicao = COALESCE(EXCLUDED.data_distribuicao, public.processos.data_distribuicao),
				partes = EXCLUDED.partes,
				movimentacoes = EXCLUDED.movimentacoes,
				last_seen = EXCLUDED.last_seen
		`, userID, p.Numero, p.Tribunal, p.Classe, p.Assunto, p.OrgaoJulgador, fonte, dataDist, partesBlob, movsBlob, now)
		if err != nil {
			log.Printf("[store] Erro ao salvar processo %s no banco: %v", p.Numero, err)
		}
	}
}

// Helpers para compatibilidade/fallback local JSON

func parseDateOrNil(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, fmtStr := range []string{"02/01/2006", "2006-01-02", "2006-01-02T15:04:05Z"} {
		if t, err := time.Parse(fmtStr, s); err == nil {
			return &t
		}
	}
	return nil
}

func loadProcessosJSON() []StoredProcess {
	storeMu.Lock()
	defer storeMu.Unlock()
	data, err := os.ReadFile(processosPath())
	if err != nil {
		return nil
	}
	var out []StoredProcess
	if json.Unmarshal(data, &out) != nil {
		return nil
	}
	return out
}

func saveProcessosJSON(procs []Process) {
	storeMu.Lock()
	defer storeMu.Unlock()

	existing := loadProcessosJSON()
	byNum := make(map[string]int, len(existing))
	for i, p := range existing {
		byNum[p.Numero] = i
	}
	now := time.Now()
	for _, p := range procs {
		if p.Numero == "" {
			continue
		}
		if idx, ok := byNum[p.Numero]; ok {
			existing[idx].Process = p
			existing[idx].LastSeen = now
		} else {
			existing = append(existing, StoredProcess{Process: p, FirstSeen: now, LastSeen: now})
			byNum[p.Numero] = len(existing) - 1
		}
	}

	_ = os.MkdirAll(dataDir(), 0o755)
	if blob, err := json.MarshalIndent(existing, "", "  "); err == nil {
		_ = os.WriteFile(processosPath(), blob, 0o644)
	}
}
