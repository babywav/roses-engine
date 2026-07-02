-- Perfil do advogado (estende auth.users; NÃO recriar identidade)
CREATE TABLE IF NOT EXISTS public.perfis (
  id UUID PRIMARY KEY REFERENCES auth.users(id) ON DELETE CASCADE,
  nome TEXT,
  oab_numero TEXT,
  oab_uf TEXT,
  escritorio_id UUID,
  created_at TIMESTAMPTZ DEFAULT now()
);

-- Habilitar RLS em perfis
ALTER TABLE public.perfis ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Users can only view their own profile"
  ON public.perfis FOR SELECT
  TO authenticated
  USING (auth.uid() = id);

CREATE POLICY "Users can manage their own profile"
  ON public.perfis FOR ALL
  TO authenticated
  USING (auth.uid() = id)
  WITH CHECK (auth.uid() = id);

-- Processos do usuário
CREATE TABLE IF NOT EXISTS public.processos (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  numero TEXT NOT NULL,
  tribunal TEXT,
  classe TEXT,
  assunto TEXT,
  orgao_julgador TEXT,
  fonte TEXT,                       -- datajud | portal | djen
  data_distribuicao DATE,
  partes JSONB DEFAULT '[]',
  movimentacoes JSONB DEFAULT '[]',
  last_seen TIMESTAMPTZ DEFAULT now(),
  UNIQUE (user_id, numero)
);

CREATE INDEX IF NOT EXISTS idx_processos_user_id ON public.processos(user_id);

-- Habilitar RLS em processos
ALTER TABLE public.processos ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Users can only access their own processes"
  ON public.processos FOR ALL
  TO authenticated
  USING (auth.uid() = user_id)
  WITH CHECK (auth.uid() = user_id);
