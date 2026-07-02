# CHANGELOG — Roses

## [Transversais] - Testes de Integração (2026-07-01)

### Adicionado
- **Suite de testes (`server/integration_test.go`)**: 18 testes cobrindo cálculos judiciais (SELIC/INPC/TR, liquidação, trabalhista com e sem falta grave, juros simples), portal do cliente (situação leiga, simplificação de movimentos), Ross agêntico (detecção de intenção, extração de CPF/CNPJ), validação dos 13 campos de análise, parser DJEN (datas, marco legal, matchRule) e LGPD (maskID).

## [Transversais] - Observabilidade (2026-07-01)

### Adicionado
- **Contadores in-process (`server/metricas.go`)**: `Metricas` com `sync/atomic` rastreando syncs DJEN (total/falha/intimações novas), requisições de IA (total/429/outros erros/latência média), uploads e processamentos de documentos, minutas geradas, checks de vigília, cálculos realizados e acessos ao portal do cliente.
- **Endpoint de métricas**: `GET /api/admin/metricas` protegido por `X-Admin-Key` (`ROSES_ADMIN_KEY`). Retorna snapshot JSON de todos os contadores + uptime.
- **Instrumentação**: `RegistrarDJENSync`, `RegistrarMinutaGerada`, `RegistrarIA429`, `RegistrarIALatencia`, `M.CalculosRealizados`, `M.PortalAcessos` chamados nos pontos certos do código.

## [Transversais] - LGPD (2026-07-01)

### Adicionado
- **Exportação de dados (`server/lgpd.go`)**: `GET /api/conta/exportar` retorna ZIP JSON com perfil, processos, intimações, minutas, cálculos e vigílias do titular (art. 18 V LGPD). Download com header `Content-Disposition`.
- **Exclusão de conta**: `DELETE /api/conta` com body `{"confirmar": true}` apaga todos os dados do usuário em cascata em todas as tabelas do Roses. Aviso explícito que o usuário em `auth.users` deve ser removido pelo Supabase Auth separadamente.
- **Logs sem PII**: `maskID` ofusca UUID nos logs de auditoria LGPD.

## [Diferencial 9.5] - Portal do Cliente (2026-07-01)

### Adicionado
- **Portal público (`server/portal_cliente.go`)**: Advogado gera link com token via `POST /api/portal/links`. O cliente acessa `GET /api/portal/{token}` sem login e vê andamento do processo em linguagem leiga (situação, última movimentação simplificada, próximo passo).
- **Controle de acesso**: Links podem expirar (`dias_validade`), ser revogados (`DELETE /api/portal/links/{id}`) e têm contador de acessos + timestamp do último acesso.
- **Tradução de jargão jurídico**: `situacaoLeiga`, `simplificarMovimento`, `proximoPassoLeigo` convertem termos processuais em linguagem acessível.
- **Migration**: `server/migrations/0007_calculos_portal.sql` com tabela `public.portal_links` (token único via `gen_random_bytes`, RLS por advogado).

## [Diferencial 9.5] - Motor de Cálculos Judiciais (2026-07-01)

### Adicionado
- **Motor de cálculos (`server/calculos.go`)**: Suporta 4 tipos via `POST /api/calculos`:
  - `correcao_monetaria`: SELIC, INPC ou TR com fator por período (data-base → data-fim)
  - `liquidacao`: principal + correção + juros moratórios + honorários, com parcelas detalhadas
  - `trabalhista`: FGTS, 13º, férias + 1/3, aviso prévio, multa 477 CLT (com/sem falta grave)
  - `juros`: simples ou compostos a taxa configurável (padrão 1%/mês)
- **Histórico auditável**: Cada cálculo é persistido em `public.calculos` com parâmetros e resultado JSON. `GET /api/calculos/historico` retorna os últimos 50.
- **Integração com Ross agêntico**: `executarCalculoRoss` detecta intenção de cálculo na conversa e orienta o usuário sobre os parâmetros necessários.

## [Diferencial 9.4] - Ross Agêntico com Ferramentas da Carteira (2026-07-01)

### Adicionado
- **Ross Agêntico (`server/ross_agent.go`)**: Detecção de intenção na última mensagem do usuário (prazos, minutas, divergências, vigílias). Quando detectada, uma ferramenta Go é executada e o resultado é injetado como contexto antes da chamada à IA — sem dependência de tool-calling nativo, funciona com qualquer provider.
- **Ferramentas disponíveis**: `listar_prazos`, `listar_minutas`, `buscar_divergencias`, `criar_vigilancia`, `listar_vigilancias`, `listar_alertas_vigia`. Cada ferramenta retorna dados reais do banco com instruções para o Ross responder com fidelidade.
- **Integração nos handlers de chat** (`/api/chat` e `/api/chat/stream`): `EnrichChatWithAgentContext` é chamado automaticamente em ambos os fluxos quando o usuário está autenticado.
- **Exemplos de frases que ativam o agente**: "quais prazos vencem esta semana?", "me mostre as minutas", "vigiar o CNPJ 12345678000190", "tem alguma divergência de prazo?", "o que tenho para hoje?".

## [Diferencial 9.3] - Vigília de CPF/CNPJ Multi-Tribunal (2026-07-01)

### Adicionado
- **Tabela de Vigílias (`server/migrations/0006_vigilancias.sql`)**: Tabela `public.vigilancias` com documento (CPF/CNPJ), nome, tipo, tribunais monitorados e snapshot anterior. Tabela `public.vigilancia_alertas` (append-only) para notificações de processos novos.
- **Motor de Comparação (`server/vigilancia.go`)**: `RunVigilanciaCheck` consulta processos do nome via portal, cruza com snapshot anterior e gera alertas apenas para o que é novo. Detecta `nova_execucao` automaticamente por classe processual.
- **Worker Diário**: `StartVigilanciaWorker` executa a verificação de todas as vigílias ativas com delay de rate limiting. Inicializado automaticamente com o servidor.
- **Endpoints CRUD**: `GET/POST /api/vigilancias`, `DELETE/PATCH /api/vigilancias/{id}`, `GET /api/vigilancias/alertas`.
- **Integração com Alertas (Fase 3)**: Cada novo processo detectado dispara push notification via `GlobalDispatcher`.

## [Diferencial 9.2] - Blindagem de Prazo com Trilha Auditável (2026-07-01)

### Adicionado
- **Tabela de Auditoria (`server/migrations/0005_auditoria.sql`)**: Tabela `public.prazo_auditoria` append-only com todos os campos exigidos pelo spec: `fonte`, `data_disponibilizacao`, `data_publicacao`, `regra`, `base_legal`, `dias_uteis`, `feriados_considerados` (JSONB), `divergencia`, `detalhe_divergencia`.
- **Registro Automático de Auditoria (`server/auditoria.go`)**: `RegistrarAuditoriaPrazo` é chamado pelo `SyncDJEN` para cada intimação processada, gravando a prova completa da contagem incluindo lista de feriados/recesso pulados.
- **Cross-Check DJEN × DataJud (`CrossCheckPrazo`)**: Compara vencimento da intimação oficial com a estimativa por movimentação do mesmo processo. Divergências de ≥1 dia útil são registradas e ficam consultáveis. Movimentações sem intimação correspondente geram log de alerta.
- **Endpoints de Consulta**: `GET /api/auditoria/{numero_processo}` retorna trilha completa; `GET /api/auditoria/divergencias` lista todos os processos com divergência detectada.
- **Integração no SyncDJEN (`server/djen.go`)**: Registro de auditoria e cross-check disparados automaticamente para cada nova intimação sincronizada.

## [Fase 3] - Alertas e Entrega de Canais (WhatsApp, E-mail, Push) (2026-07-01)

### Adicionado
- **Arquitetura de Alertas Multicanal (`server/alertas.go`)**: Nova estrutura modular contendo as interfaces de comunicação para `WhatsAppProvider`, `EmailProvider` e `PushProvider`.
- **Provedores de Simulação (Mock)**: Provedores baseados em logs do sistema que simulam a integração com Twilio/Resend/FCM formatando as mensagens de alerta dinamicamente com base nas intimações e vencimentos reais, preparando o ambiente para a futura contratação das APIs.
- **Integração com Minutas Automáticas (`server/minutas.go`)**: Disparo automático de alertas no momento em que a minuta de petição sugerida é concluída e salva no banco de dados.
- **Boletim Diário Consolidado**: Função de compilação de e-mail diário (`SendDailyDigestEmail`) agrupando todos os prazos do advogado em formato de tabela HTML estruturada.
- **Testes de Alertas (`server/flagship_test.go`)**: Teste unitário automatizado que dispara a compilação e validação do fluxo completo de alerta e envio de boletins.

## [Diferenciais 9.1 & 9.2] - Flagship "Intimação → Minuta Pronta" + "Blindagem de Prazo" (2026-07-01)

### Adicionado
- **Geração Automática de Minutas (`server/minutas.go`)**: Ao registrar uma nova intimação no banco (via sync do DJEN), um processo assíncrono é disparado para identificar o tipo de peça (Contestação, Réplica, Recurso, Emenda), buscar no RAG o contexto legal, consultar a IA e gravar uma minuta de petição sugerida vinculada à intimação na tabela `public.minutas`.
- **Tabela de Minutas (`server/migrations/0004_flagship.sql`)**: Nova tabela para gerenciar o ciclo de vida das minutas sugeridas (`rascunho | revisado | aprovado`) com RLS isolando por usuário.
- **Detecção de Feriados Estaduais**: Novo validador `ehFeriadoLocal(t, uf)` em `server/prazos.go` cobrindo feriados forenses regionais nos estados de SP, RJ, PB, PE, BA, RS e PR.
- **Suspensão Forense CPC 220**: Ajustado o intervalo de suspensão de prazos processuais para transcorrer de 20/12 a 20/01 inclusive.
- **Mapeamento de Rotas CRUD para Minutas**: Novas rotas no backend `/api/minutas`, `/api/minutas/intimacao/{id}` e `/api/minutas/{id}` para listar, detalhar e atualizar minutas.
- **Testes Unitários de Feriado/Recesso (`server/flagship_test.go`)**: Validadores de conformidade do cálculo de prazos estaduais e do recesso sob o artigo 220 do CPC.

## [Fase 2] - IA Aterrada + Extração de Documentos + Análise 13 Campos (2026-07-01)

### Adicionado
- **Extração de Documentos Nativa**: Criado `server/extractor.go` com rotinas nativas de decodificação de texto de documentos PDF (via `dslipak/pdf`), DOCX (unzip + parse do XML `word/document.xml`) e planilhas XLSX (via `excelize/v2` renderizando para layout CSV por aba).
- **Estruturas de Prompt Jurídico (`server/legal_prompts.go`)**: Definição detalhada do prompt de análise estruturada cobrindo 13 campos de análise jurídica do caso, com validações de consistência aritmética para planilhas.
- **RAG com Busca em Texto do PostgreSQL (`server/rag.go`)**: Função de busca semântica aproximada por palavras-chave com indexação e ranking utilizando PostgreSQL full-text search (`tsvector` e `plainto_tsquery`) sobre a tabela `public.base_juridica`, semeada automaticamente no boot com legislações de referência do Brasil (CF, CPC, CLT, CTN e Súmulas).
- **Fila de Jobs Resiliente (`server/queue.go`)**: Loop de processamento assíncrono para os jobs em banco (`pending | running | waiting_retry | done | error`), com backoff de retentativa automático de 2 minutos para contornar limites de rate-limiting (erros 429) e suporte a modelos críticos dedicados configuráveis em ambiente (`AI_CRITICAL_MODEL`).
- **Endpoints de Upload e Acompanhamento**: Endpoints `/api/analise/upload` e `/api/analise/job/:id` permitindo o envio de documentos (salvos no Supabase Storage ou localmente em pasta de cache sob modo offline) e verificação do progresso do processamento assíncrono.
- **Testes Unitários de Extração (`server/extractor_test.go`)**: Testes cobrindo a extração de DOCX falso montado em memória, tabelas XLSX estruturadas e rotinas de reparo de JSON corrompido ou embrulhado em markdown de LLMs.

## [Fase 1] - Prazos Oficiais via DJEN/Comunica (2026-07-01)

### Adicionado
- **Integração com a API de Comunicações do CNJ**: Novo módulo `server/djen.go` que se conecta com a API pública do CNJ (`https://comunicaapi.pje.jus.br/api/v1/comunicacao`) para sincronizar intimações oficiais por OAB/UF com paginação por `pagina` e controle de rate-limiting.
- **Tabela de Intimações**: Migração `server/migrations/0002_djen.sql` criando a tabela `public.intimacoes` com políticas de RLS e chave de idempotência `unique (user_id, numero_processo, data_disponibilizacao, tipo_comunicacao)`.
- **Regras do Marco Legal do CNJ**: Cálculo automático de datas seguindo as regras do Diário de Justiça Eletrônico Nacional (DJEN) reutilizando o calendário e contagem de feriados de `prazos.go`.
- **Endpoint de Sincronização Manual**: Endpoint `POST /api/djen/sync` protegido por JWT que sincroniza intimações para a OAB/UF fornecida na requisição ou cadastrada no perfil do usuário.
- **Worker em Segundo Plano**: Rotina rodando periodicamente (a cada 24h) para buscar atualizações automáticas no DJEN para todos os advogados cadastrados em `public.perfis`.
- **Testes Unitários para DJEN**: Suite de teste `server/djen_test.go` verificando o algoritmo de cálculo de datas legais e decodificação do payload JSON da API.

### Modificado
- **Prazos Inteligentes (`server/prazos.go`)**: O cálculo de prazos agora junta e prioriza intimações oficiais do DJEN (rotuladas como `"oficial"`) e mantém o cálculo heurístico baseado em movimentações como fallback (rotuladas como `"estimado (não oficial)"`) evitando duplicações por processo.
- **Rotas de Backend (`server/main.go`)**: Registrado o novo endpoint `/api/djen/sync` e inicializado o worker em segundo plano.

## [Fase 0] - Fundação: Postgres + Auth Multi-Tenant (2026-07-01)

### Adicionado
- **Migração do Banco de Dados**: Script SQL `server/migrations/0001_init.sql` com tabelas `public.perfis` (para estender a auth do Supabase) e `public.processos` (tabela de dados dos processos do usuário).
- **Segurança & Isolamento (RLS)**: Row Level Security habilitado nas tabelas `perfis` e `processos` para garantir o isolamento completo de dados por inquilino (`user_id`).
- **Conectividade Relacional**: Novo módulo `server/db.go` para gerenciar pools de conexão PostgreSQL resilientes com o driver oficial `pgx/v5`.
- **Middleware de Autenticação JWT**: Novo módulo `server/auth.go` validando tokens de acesso JWT emitidos pelo Supabase (algoritmo HS256 com `SUPABASE_JWT_SECRET`). O middleware injeta o `userID` no contexto do request HTTP.
- **Importador de Processos Legados**: Script one-shot `server/cmd/migrate_legacy/main.go` para ler, validar e migrar os dados legados do arquivo local `data/processos.json` para o PostgreSQL sob um ID de usuário legado.
- **Suíte de Testes para Auth**: Novo arquivo `server/auth_test.go` validando o middleware de autenticação (casos sem header, tokens bem assinados e assinaturas inválidas).

### Modificado
- **Store Descentralizado (`server/store.go`)**: Substituição de leitura/gravação em JSON local por consultas e upserts transacionais diretos no PostgreSQL. Mantido suporte a fallback local offline para facilidade em desenvolvimento local.
- **Endpatch Multi-Tenant (`server/main.go`)**: Endpoints de processos e dashboards agora exigem token JWT e estão isolados por usuário.
- **Mapeamento de Regras e Negócios**: `oportunidades.go`, `casos.go`, `dashboard.go` e `prazos.go` atualizados para propagar e filtrar informações processuais por usuário.

### Como Executar
1. Configure as variáveis de ambiente necessárias no arquivo `server/.env`:
   ```env
   SUPABASE_JWT_SECRET="<sua_chave_jwt_secret>"
   DATABASE_URL="postgresql://postgres:<sua_senha>@db.utzoxnitkvhpeivdqgdu.supabase.co:5432/postgres"
   ```
2. Execute o servidor Go normalmente:
   ```bash
   cd server
   go run .
   ```
3. Para migrar dados legados:
   ```bash
   cd server/cmd/migrate_legacy
   go run .
   ```
4. Para executar a suíte de testes do backend:
   ```bash
   cd server
   go test -v .
   ```
