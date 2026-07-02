-- Migration 0006: vigilancias de CPF/CNPJ multi-tribunal (Diferencial 9.3)
-- O advogado cadastra partes (cliente, devedor, adversário) e o Roses
-- monitora diariamente, notificando apenas o que mudou.

CREATE TABLE IF NOT EXISTS public.vigilancias (
  id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id              UUID        NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  documento            TEXT        NOT NULL,            -- CPF ou CNPJ (somente dígitos)
  nome                 TEXT        NOT NULL,
  tipo                 TEXT        DEFAULT 'parte',     -- cliente | adversario | devedor | parte
  tribunais            TEXT[]      DEFAULT '{}',        -- UFs/siglas; vazio = todos
  ultima_verificacao   TIMESTAMPTZ,
  snapshot_anterior    JSONB       DEFAULT '[]',        -- lista de processos anteriores
  ativo                BOOLEAN     DEFAULT TRUE,
  created_at           TIMESTAMPTZ DEFAULT now(),
  updated_at           TIMESTAMPTZ DEFAULT now(),
  UNIQUE (user_id, documento)
);

CREATE INDEX IF NOT EXISTS idx_vigilancias_user ON public.vigilancias (user_id);
CREATE INDEX IF NOT EXISTS idx_vigilancias_doc  ON public.vigilancias (documento);

ALTER TABLE public.vigilancias ENABLE ROW LEVEL SECURITY;

CREATE POLICY "vigilancias_own"
  ON public.vigilancias FOR ALL
  TO authenticated
  USING (auth.uid() = user_id)
  WITH CHECK (auth.uid() = user_id);

-- Alertas gerados pela vigília (append-only, nunca deletar)
CREATE TABLE IF NOT EXISTS public.vigilancia_alertas (
  id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  vigilancia_id  UUID        NOT NULL REFERENCES public.vigilancias(id) ON DELETE CASCADE,
  user_id        UUID        NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  tipo_alerta    TEXT        NOT NULL,  -- novo_processo | nova_execucao | atualizacao
  numero_processo TEXT,
  descricao      TEXT,
  lido           BOOLEAN     DEFAULT FALSE,
  created_at     TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_vig_alertas_user ON public.vigilancia_alertas (user_id);
CREATE INDEX IF NOT EXISTS idx_vig_alertas_vig  ON public.vigilancia_alertas (vigilancia_id);

ALTER TABLE public.vigilancia_alertas ENABLE ROW LEVEL SECURITY;

CREATE POLICY "vig_alertas_own"
  ON public.vigilancia_alertas FOR ALL
  TO authenticated
  USING (auth.uid() = user_id)
  WITH CHECK (auth.uid() = user_id);
