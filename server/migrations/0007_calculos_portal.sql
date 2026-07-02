-- Migration 0007: cálculos judiciais + portal do cliente (9.5)

-- ─── Cálculos judiciais ────────────────────────────────────────────────────
-- Histórico auditável de cada cálculo executado.
CREATE TABLE IF NOT EXISTS public.calculos (
  id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         UUID        NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  processo_id     UUID        REFERENCES public.processos(id) ON DELETE SET NULL,
  tipo            TEXT        NOT NULL,  -- correcao_monetaria | liquidacao | trabalhista | juros
  parametros      JSONB       NOT NULL DEFAULT '{}',
  resultado       JSONB       NOT NULL DEFAULT '{}',
  created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_calculos_user    ON public.calculos (user_id);
CREATE INDEX IF NOT EXISTS idx_calculos_proc    ON public.calculos (processo_id);

ALTER TABLE public.calculos ENABLE ROW LEVEL SECURITY;
CREATE POLICY "calculos_own" ON public.calculos FOR ALL TO authenticated
  USING (auth.uid() = user_id) WITH CHECK (auth.uid() = user_id);

-- ─── Portal do cliente ─────────────────────────────────────────────────────
-- Links públicos gerados pelo advogado para o cliente acompanhar o processo.
CREATE TABLE IF NOT EXISTS public.portal_links (
  id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         UUID        NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  numero_processo TEXT        NOT NULL,
  token           TEXT        NOT NULL UNIQUE DEFAULT encode(gen_random_bytes(24), 'base64url'),
  nome_cliente    TEXT,
  ativo           BOOLEAN     DEFAULT TRUE,
  acessos         INT         DEFAULT 0,
  ultimo_acesso   TIMESTAMPTZ,
  expira_em       TIMESTAMPTZ,
  created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_portal_token   ON public.portal_links (token);
CREATE INDEX IF NOT EXISTS idx_portal_user    ON public.portal_links (user_id);

ALTER TABLE public.portal_links ENABLE ROW LEVEL SECURITY;
-- Advogado gerencia seus próprios links
CREATE POLICY "portal_links_own" ON public.portal_links FOR ALL TO authenticated
  USING (auth.uid() = user_id) WITH CHECK (auth.uid() = user_id);
-- Leitura pública pelo token (sem JWT) não passa pelo RLS — tratada no handler Go
