package main

// metricas.go — Observabilidade (Transversal)
//
// Contadores in-process thread-safe. Sem dependência externa.
// Em produção, exportar para Prometheus via /metrics ou enviar para
// Datadog/NewRelic lendo estes contadores periodicamente.
//
// Endpoint: GET /api/admin/metricas  (requer X-Admin-Key)

import (
	"net/http"
	"sync/atomic"
	"time"
)

// ─── Contadores globais ───────────────────────────────────────────────────────

type Metricas struct {
	// DJEN
	DJENSyncsTotal     atomic.Int64
	DJENSyncsFalha     atomic.Int64
	DJENIntimacoesNova atomic.Int64

	// IA
	IARequests         atomic.Int64
	IAErros429         atomic.Int64
	IAErrosOutros      atomic.Int64
	IALatenciaTotalMs  atomic.Int64 // soma para calcular média

	// Análise de documentos
	DocsUpload         atomic.Int64
	DocsProcessados    atomic.Int64
	DocsErro           atomic.Int64

	// Minutas
	MinutasGeradas     atomic.Int64
	MinutasErro        atomic.Int64

	// Vigília
	VigiliasChecks     atomic.Int64
	VigiliasAlertas    atomic.Int64

	// Cálculos
	CalculosRealizados atomic.Int64

	// Portal cliente
	PortalAcessos      atomic.Int64

	// HTTP
	HTTPRequests       atomic.Int64
	HTTP5xx            atomic.Int64

	iniciadoEm time.Time
}

var M = &Metricas{iniciadoEm: time.Now()}

// ─── Helpers de registro ──────────────────────────────────────────────────────

func RegistrarIALatencia(duracaoMs int64) {
	M.IARequests.Add(1)
	M.IALatenciaTotalMs.Add(duracaoMs)
}

func RegistrarIA429() {
	M.IAErros429.Add(1)
}

func RegistrarIAErro() {
	M.IAErrosOutros.Add(1)
}

func RegistrarDJENSync(novas int, err bool) {
	M.DJENSyncsTotal.Add(1)
	if err {
		M.DJENSyncsFalha.Add(1)
	} else {
		M.DJENIntimacoesNova.Add(int64(novas))
	}
}

func RegistrarMinutaGerada(ok bool) {
	if ok {
		M.MinutasGeradas.Add(1)
	} else {
		M.MinutasErro.Add(1)
	}
}

func RegistrarVigilia(alertas int) {
	M.VigiliasChecks.Add(1)
	M.VigiliasAlertas.Add(int64(alertas))
}

// ─── Handler ──────────────────────────────────────────────────────────────────

func handleMetricas(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET"})
		return
	}

	uptime := time.Since(M.iniciadoEm).Round(time.Second).String()
	totalIA := M.IARequests.Load()
	latMedia := int64(0)
	if totalIA > 0 {
		latMedia = M.IALatenciaTotalMs.Load() / totalIA
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"uptime":    uptime,
		"iniciado_em": M.iniciadoEm.Format(time.RFC3339),
		"djen": map[string]any{
			"syncs_total":      M.DJENSyncsTotal.Load(),
			"syncs_falha":      M.DJENSyncsFalha.Load(),
			"intimacoes_novas": M.DJENIntimacoesNova.Load(),
		},
		"ia": map[string]any{
			"requests_total":    totalIA,
			"erros_429":         M.IAErros429.Load(),
			"erros_outros":      M.IAErrosOutros.Load(),
			"latencia_media_ms": latMedia,
		},
		"documentos": map[string]any{
			"uploads":      M.DocsUpload.Load(),
			"processados":  M.DocsProcessados.Load(),
			"erros":        M.DocsErro.Load(),
		},
		"minutas": map[string]any{
			"geradas": M.MinutasGeradas.Load(),
			"erros":   M.MinutasErro.Load(),
		},
		"vigilancia": map[string]any{
			"checks":  M.VigiliasChecks.Load(),
			"alertas": M.VigiliasAlertas.Load(),
		},
		"calculos": map[string]any{
			"realizados": M.CalculosRealizados.Load(),
		},
		"portal": map[string]any{
			"acessos": M.PortalAcessos.Load(),
		},
		"http": map[string]any{
			"requests_total": M.HTTPRequests.Load(),
			"erros_5xx":      M.HTTP5xx.Load(),
		},
	})
}

// withAdminKey protege endpoints internos com X-Admin-Key (distinto da API key pública).
func withAdminKey(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminKey := getenv("ROSES_ADMIN_KEY", "")
		if adminKey == "" || r.Header.Get("X-Admin-Key") != adminKey {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "X-Admin-Key inválida"})
			return
		}
		h(w, r)
	}
}
