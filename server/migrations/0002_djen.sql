CREATE TABLE IF NOT EXISTS public.intimacoes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  numero_processo TEXT NOT NULL,
  tribunal TEXT,
  tipo_comunicacao TEXT,            -- intimação | citação | etc.
  texto TEXT,                       -- inteiro teor
  data_disponibilizacao DATE,       -- data no DJEN (data_disponibilizacao)
  data_publicacao DATE,             -- dia útil seguinte (marco legal)
  prazo_dias INT,                   -- casado com prazos.go
  prazo_rotulo TEXT,
  prazo_base TEXT,                  -- fundamento (ex.: CPC art. 335)
  vencimento DATE,
  status TEXT,                      -- vencido | hoje | urgente | emdia
  lido BOOLEAN DEFAULT false,
  created_at TIMESTAMPTZ DEFAULT now(),
  UNIQUE (user_id, numero_processo, data_disponibilizacao, tipo_comunicacao)
);

-- Habilitar RLS em intimacoes
ALTER TABLE public.intimacoes ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Users can only access their own intimacoes"
  ON public.intimacoes FOR ALL
  TO authenticated
  USING (auth.uid() = user_id)
  WITH CHECK (auth.uid() = user_id);
