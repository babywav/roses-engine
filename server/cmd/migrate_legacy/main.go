package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Party representa uma parte processual.
type Party struct {
	Tipo      string `json:"tipo"`
	Nome      string `json:"nome"`
	Documento string `json:"documento,omitempty"`
	OAB       string `json:"oab,omitempty"`
}

// Movement representa uma movimentação processual.
type Movement struct {
	Data      string `json:"data"`
	Descricao string `json:"descricao"`
	Orgao     string `json:"orgao,omitempty"`
}

// Process representa um processo.
type Process struct {
	Numero          string     `json:"numero"`
	Classe          string     `json:"classe"`
	Assunto         string     `json:"assunto"`
	Tribunal        string     `json:"tribunal"`
	OrgaoJulgador   string     `json:"orgao_julgador,omitempty"`
	DataDistribuicao string    `json:"data_distribuicao,omitempty"`
	Partes          []Party    `json:"partes"`
	Movimentacoes   []Movement `json:"movimentacoes"`
	URLProcesso     string     `json:"url_processo,omitempty"`
}

// StoredProcess representa a estrutura persistida do processo legado.
type StoredProcess struct {
	Process
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

func loadDotEnv() {
	// Procura em múltiplos níveis para achar o .env
	for _, path := range []string{".env", filepath.Join("..", ".env"), filepath.Join("..", "..", ".env"), filepath.Join("..", "..", "..", ".env")} {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			eq := strings.Index(line, "=")
			if eq < 1 {
				continue
			}
			key := strings.TrimSpace(line[:eq])
			val := strings.TrimSpace(line[eq+1:])
			val = strings.Trim(val, `"'`)
			if _, exists := os.LookupEnv(key); !exists {
				os.Setenv(key, val)
			}
		}
		f.Close()
	}
}

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

func main() {
	loadDotEnv()
	log.Println("[Migration] Iniciando processo de migração do processos.json legado...")

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" || strings.Contains(databaseURL, "<SENHA_DO_BANCO>") {
		log.Fatal("[Migration] Erro: DATABASE_URL não definida ou senha fictícia presente no .env. Migração cancelada.")
	}

	// Caminho do processos.json legado
	var dataPath string
	pathsToTry := []string{
		filepath.Join("..", "data", "processos.json"),
		filepath.Join("data", "processos.json"),
		filepath.Join("..", "..", "data", "processos.json"),
	}

	for _, p := range pathsToTry {
		if _, err := os.Stat(p); err == nil {
			dataPath = p
			break
		}
	}

	if dataPath == "" {
		log.Println("[Migration] Arquivo processos.json não encontrado nas pastas conhecidas. Nada a migrar.")
		return
	}

	log.Printf("[Migration] Lendo arquivo legado em: %s", dataPath)

	data, err := os.ReadFile(dataPath)
	if err != nil {
		log.Fatalf("[Migration] Falha ao ler processos.json: %v", err)
	}

	var legacyProcs []StoredProcess
	if err := json.Unmarshal(data, &legacyProcs); err != nil {
		log.Fatalf("[Migration] Falha ao decodificar JSON dos processos: %v", err)
	}

	log.Printf("[Migration] Carregados %d processos do JSON legado.", len(legacyProcs))

	// Conectar ao Banco de Dados
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("[Migration] Erro ao conectar ao banco de dados: %v", err)
	}
	defer pool.Close()

	legacyUserID := "00000000-0000-0000-0000-000000000000"

	// Garantir que o usuário e perfil correspondente existam
	_, err = pool.Exec(ctx, `
		INSERT INTO auth.users (id, aud, role, email)
		VALUES ($1, 'authenticated', 'authenticated', 'legacy@roses.ai')
		ON CONFLICT (id) DO NOTHING
	`, legacyUserID)
	if err != nil {
		log.Printf("[Migration] Aviso ao criar usuário legado em auth.users: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO public.perfis (id, nome)
		VALUES ($1, 'Usuário Legado')
		ON CONFLICT (id) DO NOTHING
	`, legacyUserID)
	if err != nil {
		log.Printf("[Migration] Aviso ao criar perfil legado em public.perfis: %v", err)
	}

	migratedCount := 0
	for _, sp := range legacyProcs {
		p := sp.Process
		if p.Numero == "" {
			continue
		}

		partesBlob, _ := json.Marshal(p.Partes)
		movsBlob, _ := json.Marshal(p.Movimentacoes)
		dataDist := parseDateOrNil(p.DataDistribuicao)

		fonte := "datajud"
		if p.URLProcesso != "" {
			fonte = "portal"
		}

		_, err := pool.Exec(ctx, `
			INSERT INTO public.processos (
				user_id, numero, tribunal, classe, assunto, orgao_julgador, fonte, data_distribuicao, partes, movimentacoes, last_seen
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (user_id, numero) DO UPDATE SET
				classe = EXCLUDED.classe,
				assunto = EXCLUDED.assunto,
				orgao_julgador = EXCLUDED.orgao_julgador,
				fonte = EXCLUDED.fonte,
				data_distribuicao = EXCLUDED.data_distribuicao,
				partes = EXCLUDED.partes,
				movimentacoes = EXCLUDED.movimentacoes,
				last_seen = EXCLUDED.last_seen
		`, legacyUserID, p.Numero, p.Tribunal, p.Classe, p.Assunto, p.OrgaoJulgador, fonte, dataDist, partesBlob, movsBlob, sp.LastSeen)

		if err != nil {
			log.Printf("[Migration] Falha ao salvar processo %s: %v", p.Numero, err)
		} else {
			migratedCount++
		}
	}

	log.Printf("[Migration] Concluído! %d/%d processos migrados com sucesso.", migratedCount, len(legacyProcs))

	// Renomear arquivo para evitar execução repetida
	bakPath := dataPath + ".bak"
	if err := os.Rename(dataPath, bakPath); err != nil {
		log.Printf("[Migration] Erro ao renomear arquivo para bak: %v", err)
	} else {
		log.Printf("[Migration] Arquivo de dados legado renomeado para %s", bakPath)
	}
}
