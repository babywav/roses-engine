package main

import (
	"net/http"
	"sort"
)

// "Meus casos" — lista enxuta dos processos que o usuário já consultou.
// Alimentado pelo mesmo store que as Oportunidades (data/processos.json).

type casoResumo struct {
	Numero        string   `json:"numero"`
	Classe        string   `json:"classe"`
	Assunto       string   `json:"assunto,omitempty"`
	Tribunal      string   `json:"tribunal"`
	OrgaoJulgador string   `json:"orgao_julgador,omitempty"`
	Partes        []Party  `json:"partes,omitempty"`
	UltimaData    string   `json:"ultima_data,omitempty"`
	UltimaDesc    string   `json:"ultima_desc,omitempty"`
	QtdMovs       int      `json:"qtd_movs"`
	LastSeen      string   `json:"last_seen,omitempty"`
}

func handleCasos(w http.ResponseWriter, r *http.Request) {
	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}
	stored := loadProcessos(userID)

	// Mais recentes primeiro (pela última vez que foram vistos/consultados).
	sort.Slice(stored, func(i, j int) bool {
		return stored[i].LastSeen.After(stored[j].LastSeen)
	})

	casos := make([]casoResumo, 0, len(stored))
	for _, sp := range stored {
		c := casoResumo{
			Numero:        sp.Numero,
			Classe:        sp.Classe,
			Assunto:       sp.Assunto,
			Tribunal:      sp.Tribunal,
			OrgaoJulgador: sp.OrgaoJulgador,
			Partes:        sp.Partes,
			QtdMovs:       len(sp.Movimentacoes),
		}
		if len(sp.Movimentacoes) > 0 {
			ult := sp.Movimentacoes[len(sp.Movimentacoes)-1]
			c.UltimaData = ult.Data
			c.UltimaDesc = ult.Descricao
		}
		if !sp.LastSeen.IsZero() {
			c.LastSeen = sp.LastSeen.Format("2006-01-02")
		}
		casos = append(casos, c)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total": len(casos),
		"casos": casos,
	})
}
