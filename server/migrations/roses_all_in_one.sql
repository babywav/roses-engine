-- =====================================================================
-- Roses — todas as migrations (0001–0007) em um arquivo só.
-- Rode isto no Supabase do projeto São Paulo (rmaauxerpkjucyqflovm):
--   Dashboard > SQL Editor > New query > cole tudo > Run.
-- É idempotente: pode rodar de novo sem quebrar.
-- =====================================================================

-- ── 0001: perfis + processos ─────────────────────────────────────────
CREATE TABLE IF NOT EXISTS public.perfis (
  id UUID PRIMARY KEY REFERENCES auth.users(id) ON DELETE CASCADE,
  nome TEXT,
  oab_numero TEXT,
  oab_uf TEXT,
  escritorio_id UUID,
  created_at TIMESTAMPTZ DEFAULT now()
);
ALTER TABLE public.perfis ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS "Users can only view their own profile" ON public.perfis;
CREATE POLICY "Users can only view their own profile"
  ON public.perfis FOR SELECT TO authenticated USING (auth.uid() = id);
DROP POLICY IF EXISTS "Users can manage their own profile" ON public.perfis;
CREATE POLICY "Users can manage their own profile"
  ON public.perfis FOR ALL TO authenticated USING (auth.uid() = id) WITH CHECK (auth.uid() = id);

CREATE TABLE IF NOT EXISTS public.processos (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  numero TEXT NOT NULL,
  tribunal TEXT,
  classe TEXT,
  assunto TEXT,
  orgao_julgador TEXT,
  fonte TEXT,
  data_distribuicao DATE,
  partes JSONB DEFAULT '[]',
  movimentacoes JSONB DEFAULT '[]',
  last_seen TIMESTAMPTZ DEFAULT now(),
  UNIQUE (user_id, numero)
);
CREATE INDEX IF NOT EXISTS idx_processos_user_id ON public.processos(user_id);
ALTER TABLE public.processos ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS "Users can only access their own processes" ON public.processos;
CREATE POLICY "Users can only access their own processes"
  ON public.processos FOR ALL TO authenticated USING (auth.uid() = user_id) WITH CHECK (auth.uid() = user_id);

-- ── 0002: intimacoes (DJEN) ──────────────────────────────────────────
CREATE TABLE IF NOT EXISTS public.intimacoes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  numero_processo TEXT NOT NULL,
  tribunal TEXT,
  tipo_comunicacao TEXT,
  texto TEXT,
  data_disponibilizacao DATE,
  data_publicacao DATE,
  prazo_dias INT,
  prazo_rotulo TEXT,
  prazo_base TEXT,
  vencimento DATE,
  status TEXT,
  lido BOOLEAN DEFAULT false,
  created_at TIMESTAMPTZ DEFAULT now(),
  UNIQUE (user_id, numero_processo, data_disponibilizacao, tipo_comunicacao)
);
ALTER TABLE public.intimacoes ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS "Users can only access their own intimacoes" ON public.intimacoes;
CREATE POLICY "Users can only access their own intimacoes"
  ON public.intimacoes FOR ALL TO authenticated USING (auth.uid() = user_id) WITH CHECK (auth.uid() = user_id);

-- ── 0003: base_juridica + analises + jobs_analise ────────────────────
CREATE TABLE IF NOT EXISTS public.base_juridica (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  fonte TEXT NOT NULL,
  artigo_titulo TEXT NOT NULL,
  conteudo TEXT NOT NULL,
  fts_document TSVECTOR,
  created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_base_juridica_fts ON public.base_juridica USING gin(fts_document);

CREATE TABLE IF NOT EXISTS public.analises (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  documento_id TEXT NOT NULL,
  nome_documento TEXT NOT NULL,
  processo_id UUID REFERENCES public.processos(id) ON DELETE SET NULL,
  resultado JSONB DEFAULT '{}',
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS public.jobs_analise (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  documento_id TEXT NOT NULL,
  nome_documento TEXT NOT NULL,
  processo_id UUID REFERENCES public.processos(id) ON DELETE SET NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  resultado JSONB,
  error_message TEXT,
  retry_count INT NOT NULL DEFAULT 0,
  next_retry_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);
ALTER TABLE public.analises ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.jobs_analise ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS "Users can only view their own analises" ON public.analises;
CREATE POLICY "Users can only view their own analises"
  ON public.analises FOR ALL USING (auth.uid() = user_id);
DROP POLICY IF EXISTS "Users can only view their own analysis jobs" ON public.jobs_analise;
CREATE POLICY "Users can only view their own analysis jobs"
  ON public.jobs_analise FOR ALL USING (auth.uid() = user_id);

-- ── 0004: minutas ────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS public.minutas (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  intimacao_id UUID REFERENCES public.intimacoes(id) ON DELETE CASCADE,
  processo_numero TEXT NOT NULL,
  tipo_peca TEXT NOT NULL,
  conteudo TEXT NOT NULL,
  status TEXT DEFAULT 'rascunho',
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);
ALTER TABLE public.minutas ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS "Users can only access their own minutas" ON public.minutas;
CREATE POLICY "Users can only access their own minutas"
  ON public.minutas FOR ALL TO authenticated USING (auth.uid() = user_id) WITH CHECK (auth.uid() = user_id);

-- ── 0005: prazo_auditoria ────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS public.prazo_auditoria (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  intimacao_id UUID REFERENCES public.intimacoes(id) ON DELETE SET NULL,
  numero_processo TEXT NOT NULL,
  fonte TEXT NOT NULL,
  data_disponibilizacao DATE,
  data_publicacao DATE,
  regra TEXT,
  base_legal TEXT,
  dias_uteis INT,
  vencimento DATE,
  feriados_considerados JSONB DEFAULT '[]',
  divergencia BOOLEAN DEFAULT FALSE,
  detalhe_divergencia TEXT,
  created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_prazo_auditoria_user ON public.prazo_auditoria (user_id);
CREATE INDEX IF NOT EXISTS idx_prazo_auditoria_proc ON public.prazo_auditoria (numero_processo);
CREATE INDEX IF NOT EXISTS idx_prazo_auditoria_intim ON public.prazo_auditoria (intimacao_id);
ALTER TABLE public.prazo_auditoria ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS "auditoria_own" ON public.prazo_auditoria;
CREATE POLICY "auditoria_own" ON public.prazo_auditoria FOR ALL TO authenticated
  USING (auth.uid() = user_id) WITH CHECK (auth.uid() = user_id);

-- ── 0006: vigilancias + vigilancia_alertas ──────────────────────────
CREATE TABLE IF NOT EXISTS public.vigilancias (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  documento TEXT NOT NULL,
  nome TEXT NOT NULL,
  tipo TEXT DEFAULT 'parte',
  tribunais TEXT[] DEFAULT '{}',
  ultima_verificacao TIMESTAMPTZ,
  snapshot_anterior JSONB DEFAULT '[]',
  ativo BOOLEAN DEFAULT TRUE,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now(),
  UNIQUE (user_id, documento)
);
CREATE INDEX IF NOT EXISTS idx_vigilancias_user ON public.vigilancias (user_id);
CREATE INDEX IF NOT EXISTS idx_vigilancias_doc ON public.vigilancias (documento);
ALTER TABLE public.vigilancias ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS "vigilancias_own" ON public.vigilancias;
CREATE POLICY "vigilancias_own" ON public.vigilancias FOR ALL TO authenticated
  USING (auth.uid() = user_id) WITH CHECK (auth.uid() = user_id);

CREATE TABLE IF NOT EXISTS public.vigilancia_alertas (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  vigilancia_id UUID NOT NULL REFERENCES public.vigilancias(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  tipo_alerta TEXT NOT NULL,
  numero_processo TEXT,
  descricao TEXT,
  lido BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_vig_alertas_user ON public.vigilancia_alertas (user_id);
CREATE INDEX IF NOT EXISTS idx_vig_alertas_vig ON public.vigilancia_alertas (vigilancia_id);
ALTER TABLE public.vigilancia_alertas ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS "vig_alertas_own" ON public.vigilancia_alertas;
CREATE POLICY "vig_alertas_own" ON public.vigilancia_alertas FOR ALL TO authenticated
  USING (auth.uid() = user_id) WITH CHECK (auth.uid() = user_id);

-- ── 0007: calculos + portal_links ───────────────────────────────────
CREATE TABLE IF NOT EXISTS public.calculos (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  processo_id UUID REFERENCES public.processos(id) ON DELETE SET NULL,
  tipo TEXT NOT NULL,
  parametros JSONB NOT NULL DEFAULT '{}',
  resultado JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_calculos_user ON public.calculos (user_id);
CREATE INDEX IF NOT EXISTS idx_calculos_proc ON public.calculos (processo_id);
ALTER TABLE public.calculos ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS "calculos_own" ON public.calculos;
CREATE POLICY "calculos_own" ON public.calculos FOR ALL TO authenticated
  USING (auth.uid() = user_id) WITH CHECK (auth.uid() = user_id);

CREATE TABLE IF NOT EXISTS public.portal_links (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  numero_processo TEXT NOT NULL,
  token TEXT NOT NULL UNIQUE DEFAULT encode(gen_random_bytes(24), 'base64'),
  nome_cliente TEXT,
  ativo BOOLEAN DEFAULT TRUE,
  acessos INT DEFAULT 0,
  ultimo_acesso TIMESTAMPTZ,
  expira_em TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_portal_token ON public.portal_links (token);
CREATE INDEX IF NOT EXISTS idx_portal_user ON public.portal_links (user_id);
ALTER TABLE public.portal_links ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS "portal_links_own" ON public.portal_links;
CREATE POLICY "portal_links_own" ON public.portal_links FOR ALL TO authenticated
  USING (auth.uid() = user_id) WITH CHECK (auth.uid() = user_id);

-- Fim. Deve criar 12 tabelas em public.
