package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

type PerfilUsuario struct {
	ID           string  `json:"id"`
	Nome         string  `json:"nome"`
	OabNumero    string  `json:"oab_numero"`
	OabUF        string  `json:"oab_uf"`
	EscritorioID *string `json:"escritorio_id"`
	CreatedAt    string  `json:"created_at"`
}

// handleGetPerfil recupera o perfil do usuário ou cria um se não existir
func handleGetPerfil(w http.ResponseWriter, r *http.Request) {
	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	if DBPool == nil {
		// Mock sem banco: não retorna nome para não sobrescrever o localStorage
		writeJSON(w, http.StatusOK, map[string]any{
			"id": userID,
		})
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			Nome      string `json:"nome"`
			OabNumero string `json:"oab_numero"`
			OabUF     string `json:"oab_uf"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			_, _ = DBPool.Exec(context.Background(), `
				INSERT INTO public.perfis (id, nome, oab_numero, oab_uf)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (id) DO UPDATE SET
					nome = EXCLUDED.nome,
					oab_numero = EXCLUDED.oab_numero,
					oab_uf = EXCLUDED.oab_uf
			`, userID, req.Nome, req.OabNumero, req.OabUF)
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var p PerfilUsuario
	var createdAtTime time.Time
	err = DBPool.QueryRow(ctx, `
		SELECT id, nome, oab_numero, oab_uf, escritorio_id, created_at
		FROM public.perfis
		WHERE id = $1
	`, userID).Scan(&p.ID, &p.Nome, &p.OabNumero, &p.OabUF, &p.EscritorioID, &createdAtTime)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Perfil não existe, criar perfil padrão
			defaultNome := "Advogado Sem Nome"
			defaultOAB := "000.000"
			defaultUF := "SP"
			
			err = DBPool.QueryRow(ctx, `
				INSERT INTO public.perfis (id, nome, oab_numero, oab_uf)
				VALUES ($1, $2, $3, $4)
				RETURNING id, nome, oab_numero, oab_uf, escritorio_id, created_at
			`, userID, defaultNome, defaultOAB, defaultUF).Scan(&p.ID, &p.Nome, &p.OabNumero, &p.OabUF, &p.EscritorioID, &createdAtTime)
			
			if err != nil {
				log.Printf("[Equipe] Erro ao criar perfil padrão (usando fallback offline): %v", err)
				writeJSON(w, http.StatusOK, map[string]any{
					"id":            userID,
					"nome":          "Usuário Legado (Mock)",
					"oab_numero":    "SP 312.489",
					"oab_uf":        "SP",
					"escritorio_id": "00000000-0000-0000-0000-000000000000",
				})
				return
			}
		} else {
			log.Printf("[Equipe] Erro ao buscar perfil (usando fallback offline): %v", err)
			writeJSON(w, http.StatusOK, map[string]any{
				"id":            userID,
				"nome":          "Usuário Legado (Mock)",
				"oab_numero":    "SP 312.489",
				"oab_uf":        "SP",
				"escritorio_id": "00000000-0000-0000-0000-000000000000",
			})
			return
		}
	}

	p.CreatedAt = createdAtTime.Format(time.RFC3339)
	writeJSON(w, http.StatusOK, p)
}

// handleGetMembros retorna a lista de perfis do escritório do usuário
func handleGetMembros(w http.ResponseWriter, r *http.Request) {
	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	if DBPool == nil {
		writeJSON(w, http.StatusOK, []PerfilUsuario{
			{
				ID:           userID,
				Nome:         "Usuário Legado",
				OabNumero:    "SP 312.489",
				OabUF:        "SP",
				EscritorioID: ptr("00000000-0000-0000-0000-000000000000"),
			},
			{
				ID:           "11111111-1111-1111-1111-111111111111",
				Nome:         "Dr. Lucas Rezende",
				OabNumero:    "RJ 124.551",
				OabUF:        "RJ",
				EscritorioID: ptr("00000000-0000-0000-0000-000000000000"),
			},
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// 1. Pegar escritorio_id do usuário atual
	var escritorioID *string
	err = DBPool.QueryRow(ctx, "SELECT escritorio_id FROM public.perfis WHERE id = $1", userID).Scan(&escritorioID)
	if err != nil {
		log.Printf("[Equipe] Erro ao buscar escritorio_id (usando fallback offline): %v", err)
		writeJSON(w, http.StatusOK, []PerfilUsuario{
			{
				ID:           userID,
				Nome:         "Usuário Legado (Mock)",
				OabNumero:    "SP 312.489",
				OabUF:        "SP",
				EscritorioID: ptr("00000000-0000-0000-0000-000000000000"),
			},
			{
				ID:           "11111111-1111-1111-1111-111111111111",
				Nome:         "Dr. Lucas Rezende (Mock)",
				OabNumero:    "RJ 124.551",
				OabUF:        "RJ",
				EscritorioID: ptr("00000000-0000-0000-0000-000000000000"),
			},
		})
		return
	}

	if escritorioID == nil {
		// Sem equipe ainda
		writeJSON(w, http.StatusOK, []PerfilUsuario{})
		return
	}

	// 2. Buscar todos os perfis com o mesmo escritorio_id
	rows, err := DBPool.Query(ctx, `
		SELECT id, nome, oab_numero, oab_uf, escritorio_id, created_at
		FROM public.perfis
		WHERE escritorio_id = $1
		ORDER BY nome ASC
	`, *escritorioID)
	if err != nil {
		log.Printf("[Equipe] Erro ao buscar membros (usando fallback offline): %v", err)
		writeJSON(w, http.StatusOK, []PerfilUsuario{
			{
				ID:           userID,
				Nome:         "Usuário Legado (Mock)",
				OabNumero:    "SP 312.489",
				OabUF:        "SP",
				EscritorioID: ptr("00000000-0000-0000-0000-000000000000"),
			},
		})
		return
	}
	defer rows.Close()

	membros := []PerfilUsuario{}
	for rows.Next() {
		var p PerfilUsuario
		var createdAtTime time.Time
		if err := rows.Scan(&p.ID, &p.Nome, &p.OabNumero, &p.OabUF, &p.EscritorioID, &createdAtTime); err != nil {
			log.Printf("[Equipe] Erro ao scanear perfil: %v", err)
			continue
		}
		p.CreatedAt = createdAtTime.Format(time.RFC3339)
		membros = append(membros, p)
	}

	writeJSON(w, http.StatusOK, membros)
}

// handleAdicionarMembro adiciona outro perfil à equipe
func handleAdicionarMembro(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}

	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	var req struct {
		UsuarioID string `json:"usuario_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "corpo do JSON inválido"})
		return
	}

	if req.UsuarioID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "usuario_id é obrigatório"})
		return
	}

	if DBPool == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "sucesso (modo offline)"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// 1. Garantir que o usuário atual tem escritorio_id. Se não tiver, gera um.
	var escritorioID *string
	err = DBPool.QueryRow(ctx, "SELECT escritorio_id FROM public.perfis WHERE id = $1", userID).Scan(&escritorioID)
	if err != nil {
		log.Printf("[Equipe] Erro ao buscar escritorio_id do admin (usando mock fallback): %v", err)
		writeJSON(w, http.StatusOK, map[string]string{"status": "sucesso (mock fallback)"})
		return
	}

	if escritorioID == nil {
		// Gerar um novo escritorio_id via Postgres e associar ao usuário atual
		var newEscritorioID string
		err = DBPool.QueryRow(ctx, `
			UPDATE public.perfis 
			SET escritorio_id = gen_random_uuid() 
			WHERE id = $1 
			RETURNING escritorio_id
		`, userID).Scan(&newEscritorioID)
		if err != nil {
			log.Printf("[Equipe] Erro ao gerar escritorio_id: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "erro ao criar escritório"})
			return
		}
		escritorioID = &newEscritorioID
	}

	// 2. Verificar se o usuário que queremos adicionar existe na tabela auth.users
	var exists bool
	err = DBPool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM auth.users WHERE id = $1)", req.UsuarioID).Scan(&exists)
	if err != nil {
		log.Printf("[Equipe] Erro ao verificar existência do usuário: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "erro ao validar usuário"})
		return
	}

	if !exists {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Usuário não cadastrado no sistema (ID inválido)"})
		return
	}

	// Garantir que a linha correspondente existe em public.perfis
	var perfilExists bool
	_ = DBPool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM public.perfis WHERE id = $1)", req.UsuarioID).Scan(&perfilExists)
	if !perfilExists {
		_, err = DBPool.Exec(ctx, `
			INSERT INTO public.perfis (id, nome, oab_numero, oab_uf)
			VALUES ($1, 'Advogado Convidado', '000.000', 'SP')
		`, req.UsuarioID)
		if err != nil {
			log.Printf("[Equipe] Erro ao autocriar perfil para usuário convidado: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "erro ao criar perfil do membro"})
			return
		}
	}

	// 3. Atualizar o escritorio_id do usuário alvo para o mesmo do admin
	_, err = DBPool.Exec(ctx, `
		UPDATE public.perfis
		SET escritorio_id = $1
		WHERE id = $2
	`, *escritorioID, req.UsuarioID)
	if err != nil {
		log.Printf("[Equipe] Erro ao associar usuário ao escritório: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "erro ao adicionar membro"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "membro adicionado com sucesso"})
}

// handleRemoverMembro remove um membro da equipe (setando escritorio_id para NULL)
func handleRemoverMembro(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}

	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	var req struct {
		UsuarioID string `json:"usuario_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "corpo do JSON inválido"})
		return
	}

	if req.UsuarioID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "usuario_id é obrigatório"})
		return
	}

	if DBPool == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "sucesso (modo offline)"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var adminEscritorio, targetEscritorio *string
	_ = DBPool.QueryRow(ctx, "SELECT escritorio_id FROM public.perfis WHERE id = $1", userID).Scan(&adminEscritorio)
	_ = DBPool.QueryRow(ctx, "SELECT escritorio_id FROM public.perfis WHERE id = $1", req.UsuarioID).Scan(&targetEscritorio)

	if adminEscritorio == nil || targetEscritorio == nil || *adminEscritorio != *targetEscritorio {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "você não tem permissão para gerenciar este usuário"})
		return
	}

	_, err = DBPool.Exec(ctx, `
		UPDATE public.perfis
		SET escritorio_id = NULL
		WHERE id = $1
	`, req.UsuarioID)
	if err != nil {
		log.Printf("[Equipe] Erro ao remover membro: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "erro ao remover membro"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "membro removido da equipe"})
}

func ptr(s string) *string {
	return &s
}
