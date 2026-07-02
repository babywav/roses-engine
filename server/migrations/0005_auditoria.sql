-- Migration 0005: prazo_auditoria (trilha auditável append-only)
-- Registra cada cálculo de prazo com a prova completa da contagem,
-- detecta divergências entre fonte DJEN e DataJud/portal.

CREATE TABLE IF NOT EXISTS public.prazo_auditoria (
  id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id               UUID        NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  intimacao_id          UUID        REFERENCES public.intimacoes(id) ON DELETE SET NULL,
  numero_processo       TEXT        NOT NULL,
  fonte                 TEXT        NOT NULL,           -- djen | datajud | portal
  data_disponibilizacao DATE,
  data_publicacao       DATE,
  regra                 TEXT,                           -- ex.: "Contestação"
  base_legal            TEXT,                           -- ex.: "CPC art. 335"
  dias_uteis            INT,
  vencimento            DATE,
  feriados_considerados JSONB       DEFAULT '[]',       -- lista de datas puladas
  divergencia           BOOLEAN     DEFAULT FALSE,
  detalhe_divergencia   TEXT,
  created_at            TIMESTAMPTZ DEFAULT now()
);

-- Índices de consulta
CREATE INDEX IF NOT EXISTS idx_prazo_auditoria_user    ON public.prazo_auditoria (user_id);
CREATE INDEX IF NOT EXISTS idx_prazo_auditoria_proc    ON public.prazo_auditoria (numero_processo);
CREATE INDEX IF NOT EXISTS idx_prazo_auditoria_intim   ON public.prazo_auditoria (intimacao_id);

-- RLS: cada advogado só vê sua própria trilha
ALTER TABLE public.prazo_auditoria ENABLE ROW LEVEL SECURITY;

CREATE POLICY "auditoria_own"
  ON public.prazo_auditoria FOR ALL
  TO authenticated
  USING (auth.uid() = user_id)
  WITH CHECK (auth.uid() = user_id);
