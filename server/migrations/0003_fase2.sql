-- Base de dados jurídica para o RAG
CREATE TABLE IF NOT EXISTS public.base_juridica (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  fonte TEXT NOT NULL,               -- CF | CPC | CLT | CTN | CC | STJ | STF
  artigo_titulo TEXT NOT NULL,       -- ex.: "Art. 173" ou "Súmula 284"
  conteudo TEXT NOT NULL,            -- texto da lei/súmula
  fts_document TSVECTOR,
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_base_juridica_fts ON public.base_juridica USING gin(fts_document);

-- Tabela de análises concluídas
CREATE TABLE IF NOT EXISTS public.analises (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  documento_id TEXT NOT NULL,        -- caminho no Supabase Storage
  nome_documento TEXT NOT NULL,
  processo_id UUID REFERENCES public.processos(id) ON DELETE SET NULL,
  resultado JSONB DEFAULT '{}',      -- JSON com os 13 campos
  created_at TIMESTAMPTZ DEFAULT now()
);

-- Fila de jobs de análise resiliente
CREATE TABLE IF NOT EXISTS public.jobs_analise (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  documento_id TEXT NOT NULL,
  nome_documento TEXT NOT NULL,
  processo_id UUID REFERENCES public.processos(id) ON DELETE SET NULL,
  status TEXT NOT NULL DEFAULT 'pending', -- pending | running | waiting_retry | done | error
  resultado JSONB,
  error_message TEXT,
  retry_count INT NOT NULL DEFAULT 0,
  next_retry_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

-- RLS para análises e jobs
ALTER TABLE public.analises ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.jobs_analise ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Users can only view their own analises"
  ON public.analises FOR ALL USING (auth.uid() = user_id);

CREATE POLICY "Users can only view their own analysis jobs"
  ON public.jobs_analise FOR ALL USING (auth.uid() = user_id);
