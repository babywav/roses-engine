CREATE TABLE IF NOT EXISTS public.minutas (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  intimacao_id UUID REFERENCES public.intimacoes(id) ON DELETE CASCADE,
  processo_numero TEXT NOT NULL,
  tipo_peca TEXT NOT NULL,           -- Contestação | Réplica | Apelação | etc.
  conteudo TEXT NOT NULL,
  status TEXT DEFAULT 'rascunho',     -- rascunho | revisado | aprovado
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

-- Habilitar RLS
ALTER TABLE public.minutas ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Users can only access their own minutas"
  ON public.minutas FOR ALL
  TO authenticated
  USING (auth.uid() = user_id)
  WITH CHECK (auth.uid() = user_id);
