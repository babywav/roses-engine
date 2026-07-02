# 🌹 Roses — Motor de Busca de Processos Jurídicos

> **Roses** é o motor unificado de consulta e scraping de processos jurídicos brasileiros.
> Projetado para ser invisível, resiliente e escalável, Roses integra automação de navegador,
> resolução de desafios e emulação de hardware para consultar portais de tribunais com máxima
> eficácia e mínima detecção.

---

## Índice

1. [Visão Geral](#visão-geral)
2. [Arquitetura](#arquitetura)
3. [Tribunais Suportados](#tribunais-suportados)
4. [Módulos e Componentes](#módulos-e-componentes)
5. [Hardware Datalake (Anti-Detect)](#hardware-datalake-anti-detect)
6. [Fluxo de Consulta](#fluxo-de-consulta)
7. [Motor de Navegador](#motor-de-navegador)
8. [Resolução de Captcha e Turnstile](#resolução-de-captcha-e-turnstile)
9. [API / Integração](#api--integração)
10. [Variáveis de Ambiente](#variáveis-de-ambiente)
11. [Como Executar](#como-executar)
12. [Roadmap](#roadmap)

---

## Visão Geral

Roses é o coração da plataforma **Astra Legal AI**. Ele foi criado para resolver um problema
crítico: os portais de tribunais brasileiros (PJe, e-SAJ, e-Proc etc.) são protegidos por
sistemas anti-bot (WAF, Cloudflare Turnstile, Google reCAPTCHA) que bloqueiam requisições
HTTP diretas (retornando `HTTP 503`).

A solução do Roses é emular um computador real — com fingerprint de hardware, User-Agent
legítimo, comportamento humano e resolução automática de desafios — para navegar nesses
portais de forma imperceptível.

```
Cliente (Frontend / API)
        │
        ▼
┌─────────────────────┐
│   Roses API Layer   │  ← Recebe consultas (CNJ, nome, OAB, CPF)
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│  Court Router       │  ← Identifica tribunal pelo número CNJ ou UF
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│  Hardware Datalake  │  ← Gera perfil de hardware falso (BIOS, UUID, MAC)
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│  Browser Engine     │  ← Gecko (Firefox/Zen) ou Chromium com stealth
│  (DrissionPage /    │
│   Playwright-extra) │
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│  Challenge Solver   │  ← Turnstile / reCAPTCHA via 2Captcha sidecar
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│  Result Parser      │  ← Extrai processos, partes, movimentações
└────────┬────────────┘
         │
         ▼
      JSON Output
```

---

## Arquitetura

O Roses é composto por três camadas:

### Camada 1 — Roteamento e Parsing
Responsável por analisar o input do usuário, identificar o tribunal alvo e montar a query.

**Arquivo principal:** `core/court_router.py`

- Analisa o número CNJ (20 dígitos) e extrai os dígitos 14-16 para identificar o tribunal
- Suporta fallback por sigla UF para buscas OAB
- Mapeia o tribunal para URL, sitekey de Turnstile e seletor CSS do formulário

### Camada 2 — Automação e Anti-Detect
Responsável por abrir o navegador, aplicar camuflagem e interagir com o formulário.

**Arquivos:** `engine/browser_engine.py`, `engine/hardware_datalake.py`, `engine/stealth_layer.py`

- Usa **DrissionPage** (Python) com Chromium emulado via `--remote-debugging-port`
- Suporta **Gecko (Firefox/Zen Browser)** via Playwright Firefox
- Aplica fingerprint de hardware gerado pelo **Hardware Datalake** antes de cada sessão
- Injeta scripts de stealth: `navigator.webdriver = undefined`, WebGL spoofing, etc.

### Camada 3 — Parsing e Saída
Responsável por extrair os dados do HTML retornado e estruturá-los.

**Arquivo:** `parsers/pje_parser.py`

- Extrai nome, número, classe, assunto, partes e movimentações
- Salva resultado em JSON estruturado em `data/`
- Retorna JSON via stdout para integração com o backend Node.js

---

## Tribunais Suportados

| Código CNJ | Sigla | Tribunal                               | Status     | WAF/Captcha         |
|:----------:|:-----:|:---------------------------------------|:----------:|:--------------------|
| `15`       | TJPB  | Tribunal de Justiça da Paraíba         | ✅ Operacional | Turnstile CF    |
| `19`       | TJRJ  | Tribunal de Justiça do Rio de Janeiro  | ✅ Operacional | reCAPTCHA v2    |
| `17`       | TJPE  | Tribunal de Justiça de Pernambuco      | ✅ Operacional | Nenhum          |
| `13`       | TJMG  | Tribunal de Justiça de Minas Gerais    | 🔧 Beta        | Nenhum          |
| `05`       | TJBA  | Tribunal de Justiça da Bahia           | 🔧 Beta        | Nenhum          |
| `08`       | TJSP  | Tribunal de Justiça de São Paulo       | 🗓️ Roadmap    | Turnstile CF    |
| `04`       | TJRS  | Tribunal de Justiça do Rio Grande do Sul | 🗓️ Roadmap  | Nenhum          |
| —          | STJ   | Superior Tribunal de Justiça           | 🗓️ Roadmap    | Nenhum          |
| —          | TST   | Tribunal Superior do Trabalho          | 🗓️ Roadmap    | Nenhum          |

### Dígitos CNJ — Tabela de Referência

O número CNJ segue o padrão: `NNNNNNN-DD.AAAA.J.TT.OOOO`

- **J** = Segmento de Justiça (ex: `8` = Justiça Estadual)
- **TT** = Código do tribunal (ex: `19` = TJRJ, `15` = TJPB)

Roses extrai os dígitos na posição 14-16 do número limpo (sem pontuação) para identificar
automaticamente o tribunal.

---

## Módulos e Componentes

```
roses/
├── DETAILS.md                    ← Este arquivo
├── README.md                     ← Início rápido (quickstart)
│
├── core/
│   ├── court_router.py           ← Roteador de tribunais por CNJ/UF
│   ├── query_builder.py          ← Monta os critérios de busca
│   └── result_normalizer.py      ← Normaliza a saída para formato padrão
│
├── engine/
│   ├── browser_engine.py         ← Instancia e gerencia o navegador
│   ├── hardware_datalake.py      ← Gerador de perfis de hardware falsos
│   ├── stealth_layer.py          ← Scripts JS de evasão anti-bot
│   └── captcha_solver.py         ← Integração com 2Captcha / Turnstile sidecar
│
├── scrapers/
│   ├── pje_scraper.py            ← Scraper PJe genérico (multiportal)
│   ├── tjpb_scraper.py           ← Scraper especializado TJPB
│   ├── tjrj_scraper.py           ← Scraper especializado TJRJ
│   └── esaj_scraper.py           ← (Roadmap) Scraper e-SAJ (TJSP, TJBA)
│
├── parsers/
│   ├── pje_parser.py             ← Parser HTML do PJe
│   └── models.py                 ← Modelos de dados (Process, Party, Movement)
│
├── datalake/
│   ├── hardware_profiles.json    ← Banco de perfis de hardware gerados
│   ├── user_agents.json          ← Pool de User-Agents reais coletados
│   └── mac_bios_seeds.json       ← Seeds de BIOS/MAC para geração de perfis
│
├── api/
│   ├── roses_api.py              ← FastAPI — ponto de entrada HTTP
│   └── routes.py                 ← Rotas: /search, /status, /profiles
│
└── tests/
    ├── test_router.py
    ├── test_scraper_tjpb.py
    └── test_scraper_tjrj.py
```

---

## Hardware Datalake (Anti-Detect)

> **Esta é a feature mais avançada do Roses.** Inspirada nas técnicas de Hackintosh e
> emulação de SMBIOS, o Hardware Datalake gera perfis completos de máquina virtual
> para enganar sistemas WAF que analisam fingerprint de hardware via JavaScript/WebGL.

### Como Funciona

Portais modernos com proteção avançada (ex: Cloudflare, DataDome) usam scripts JavaScript
para coletar informações sobre o hardware do visitante:

- **WebGL Renderer** — identifica a GPU
- **Canvas fingerprint** — hash do canvas renderizado
- **AudioContext fingerprint** — hash do contexto de áudio
- **Navigator.platform / deviceMemory / hardwareConcurrency**
- **Screen.width/height e window.outerWidth/outerHeight**

O Hardware Datalake usa um banco de dados de perfis baseados em especificações reais de
MacBooks (via Apple SMBIOS dumps) e PCs Windows comuns, e aplica essas especificações
como patches JS antes que qualquer script do site seja executado.

### Estrutura de um Perfil de Hardware

```json
{
  "profile_id": "mbp-m1-2021-001",
  "system": {
    "platform": "MacIntel",
    "os": "macOS 14.5",
    "bios_vendor": "Apple Inc.",
    "bios_version": "1715.80.3.0.0",
    "smbios_uuid": "A3F2BC10-847C-11ED-9F2E-3C22FB1A7B39",
    "serial_number": "C02G7KQGQ6LR",
    "model_identifier": "MacBookPro18,3",
    "board_id": "Mac-CFF7D910A743CAAF"
  },
  "hardware": {
    "cpu_brand": "Apple M1 Pro",
    "cpu_cores": 10,
    "ram_gb": 16,
    "gpu_vendor": "Apple",
    "gpu_renderer": "Apple M1 Pro",
    "screen_width": 3456,
    "screen_height": 2234,
    "color_depth": 30,
    "pixel_ratio": 2.0
  },
  "network": {
    "mac_address": "3c:22:fb:1a:7b:39",
    "timezone": "America/Sao_Paulo",
    "locale": "pt-BR"
  },
  "browser": {
    "user_agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_5) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15",
    "accept_language": "pt-BR,pt;q=0.9,en-US;q=0.8,en;q=0.7"
  }
}
```

### Geração de Perfis

O módulo `hardware_datalake.py` usa os seeds de `mac_bios_seeds.json` para gerar perfis
únicos e plausíveis a cada sessão:

1. Seleciona um modelo base aleatório (MacBook Pro, iMac, Mac Mini, etc.)
2. Gera valores únicos para UUID, serial e MAC address mantendo o OUI (fabricante) correto
3. Randomiza levemente os valores de hardware (RAM, CPU cores) dentro de limites plausíveis
4. Associa o perfil a um User-Agent compatível

### Aplicação no Navegador

Antes de carregar qualquer página, o Roses injeta um script `initScript` que sobrescreve
as APIs nativas do browser:

```javascript
// Aplicado via page.addInitScript() ANTES de qualquer recurso do site
Object.defineProperty(navigator, 'platform', { get: () => 'MacIntel' });
Object.defineProperty(navigator, 'hardwareConcurrency', { get: () => 10 });
Object.defineProperty(navigator, 'deviceMemory', { get: () => 16 });
Object.defineProperty(screen, 'width', { get: () => 1728 });
Object.defineProperty(screen, 'height', { get: () => 1117 });

// WebGL Renderer Spoofing
const getParam = WebGLRenderingContext.prototype.getParameter;
WebGLRenderingContext.prototype.getParameter = function(p) {
  if (p === 37445) return 'Apple'; // UNMASKED_VENDOR_WEBGL
  if (p === 37446) return 'Apple M1 Pro'; // UNMASKED_RENDERER_WEBGL
  return getParam.apply(this, [p]);
};
```

---

## Fluxo de Consulta

```
1. Input recebido:
   { cnj: "0001234-56.2023.8.19.0001", tipo: "cnj" }

2. Court Router analisa:
   - Dígitos 14-16 do CNJ limpo = "19"
   - Tribunal identificado: TJRJ
   - URL alvo: https://tjrj.pje.jus.br/pje/ConsultaPublica/listView.seam

3. Hardware Datalake gera perfil:
   - Seleciona perfil: "mbp-m1-2021-003"
   - Injeta User-Agent e fingerprint JS

4. Browser Engine abre sessão:
   - Gecko (Firefox) com perfil stealth aplicado
   - Navega para a URL do tribunal

5. Challenge Solver:
   - Detecta reCAPTCHA v2 na página
   - Solicita token ao sidecar 2Captcha
   - Injeta token no campo oculto
   - Clica no botão de busca

6. Result Parser:
   - Aguarda tabela de resultados carregar
   - Extrai linhas de processos (número, classe, assunto, partes)
   - Navega para cada processo para obter movimentações (opcional)
   - Retorna JSON estruturado

7. Output:
   {
     "status": "OK",
     "tribunal": "TJRJ",
     "total": 3,
     "processos": [
       {
         "numero": "0001234-56.2023.8.19.0001",
         "classe": "Ação Penal Pública Incondicionada",
         "assunto": "Roubo Majorado",
         "partes": [
           { "tipo": "Ré", "nome": "João Victor Fernandes da Nóbrega" }
         ],
         "movimentacoes": [...]
       }
     ]
   }
```

---

## Motor de Navegador

Roses suporta dois motores de navegador, configuráveis via variável de ambiente:

### Gecko (Firefox / Zen Browser) — **Recomendado**

```bash
BROWSER_ENGINE=gecko
```

- Baseado no Firefox com perfis de privacidade do Zen Browser
- **Menor rastreabilidade**: sem Blink features de automação
- WebRTC desativado por padrão (`media.peerconnection.enabled = false`)
- Fingerprint mais difícil de associar a bots
- `dom.webdriver.enabled = false` — oculta flag de automação

**User-Agents Gecko disponíveis:**
- `Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:130.0) Gecko/20100101 Firefox/130.0` (Zen Browser macOS)
- `Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:128.0) Gecko/20100101 Firefox/128.0`
- `Mozilla/5.0 (Macintosh; Intel Mac OS X 14.5; rv:129.0) Gecko/20100101 Firefox/129.0`
- `Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0`

### Chromium (Playwright Extra + Stealth)

```bash
BROWSER_ENGINE=chromium
```

- Usa `playwright-extra` + `puppeteer-extra-plugin-stealth`
- Necessário para portais que exigem Chrome (ex: e-SAJ)
- Aplica patches: `--disable-blink-features=AutomationControlled`

---

## Resolução de Captcha e Turnstile

O Roses usa uma arquitetura de sidecar para resolução de desafios:

```
Roses Browser Engine
        │
        │ HTTP GET /turnstile?url=...&sitekey=...
        ▼
┌──────────────────────┐
│  2Captcha Sidecar    │  ← Serviço Python local (porta 5000)
│  (captcha_solver.py) │  ← Envia tarefa para API 2Captcha
└──────────────────────┘
        │
        │ HTTP GET /result?id=TASK_ID (polling até resolver)
        ▼
   Token Turnstile / reCAPTCHA

        │
        │ page.run_js() — injeta token no campo oculto
        ▼
   Botão de busca é clicado
```

**Suporte atual:**
- ✅ Cloudflare Turnstile (via `cf-turnstile-response`)
- ✅ Google reCAPTCHA v2 (via `g-recaptcha-response`)
- 🗓️ hCaptcha (Roadmap)
- 🗓️ reCAPTCHA v3 (Roadmap)

---

## API / Integração

### Uso via CLI (Python direto)

```bash
# Consulta por CNJ
python pje_unified_scraper.py --cnj "0001234-56.2023.8.19.0001"

# Consulta por nome de parte
python pje_unified_scraper.py --nome-parte "João Victor" --uf RJ

# Consulta por OAB
python pje_unified_scraper.py --oab "123456" --uf RJ

# Saída JSON apenas (para integração com Node.js)
python pje_unified_scraper.py --cnj "..." --json-only

# Com browser visível (debug)
python pje_unified_scraper.py --cnj "..." --headed
```

### Uso via Node.js (child_process.spawn)

```typescript
import { spawn } from 'child_process';
import path from 'path';

const pythonPath = path.join(SCRAPERS_PATH, 'venv2', 'bin', 'python3');
const scriptPath = path.join(SCRAPERS_PATH, 'pje_unified_scraper.py');

const proc = spawn(pythonPath, [
  scriptPath,
  '--cnj', '0001234-56.2023.8.19.0001',
  '--json-only',
]);

let output = '';
proc.stdout.on('data', (chunk) => { output += chunk.toString(); });
proc.on('close', (code) => {
  if (code === 0) {
    const result = JSON.parse(output);
    console.log(result);
  }
});
```

### Uso via HTTP (Roses API — FastAPI)

```bash
# Inicia a API
uvicorn roses.api.roses_api:app --port 8001

# Consulta
curl -X POST http://localhost:8001/search \
  -H "Content-Type: application/json" \
  -d '{"tipo": "cnj", "valor": "0001234-56.2023.8.19.0001"}'
```

---

## Variáveis de Ambiente

| Variável                    | Padrão      | Descrição                                         |
|:----------------------------|:-----------:|:--------------------------------------------------|
| `BROWSER_ENGINE`            | `chromium`  | Motor do navegador: `chromium` ou `gecko`         |
| `BROWSER_HEADLESS`          | `true`      | Modo headless. `false` para visualizar o browser  |
| `BROWSER_AUTOMATION_ENABLED`| `true`      | Liga/desliga automação (útil para testes)         |
| `TWOCAPTCHA_API_KEY`        | —           | Chave da API do 2Captcha para resolver desafios   |
| `RESIDENTIAL_PROXY_URL`     | —           | Proxy residencial (ex: `http://user:pass@host`)   |
| `ROSES_HARDWARE_PROFILE`    | `random`    | ID do perfil de hardware ou `random`              |
| `ROSES_OUTPUT_DIR`          | `data/`     | Diretório para salvar JSONs de resultado          |
| `ROSES_LOG_LEVEL`           | `info`      | Nível de log: `debug`, `info`, `warn`, `error`    |

---

## Como Executar

### Pré-requisitos

```bash
# Python 3.11+
python3 --version

# Ambiente virtual (já existente no projeto)
source /path/to/venv2/bin/activate

# Dependências Python
pip install drissionpage playwright requests fastapi uvicorn

# Instala binários Playwright (Firefox + Chromium)
playwright install firefox chromium
```

### Execução Básica

```bash
# Consulta direta por CNJ
cd /path/to/roses
python core/pje_unified_scraper.py --cnj "0001234-56.2023.8.19.0001" --headed

# Via API
uvicorn api.roses_api:app --reload --port 8001
```

### Docker (Roadmap)

```dockerfile
FROM python:3.11-slim
RUN pip install playwright drissionpage fastapi uvicorn
RUN playwright install firefox chromium
COPY . /roses
WORKDIR /roses
CMD ["uvicorn", "api.roses_api:app", "--host", "0.0.0.0", "--port", "8001"]
```

---

## Roadmap

### v1.0 (Atual)
- [x] Roteador de tribunais por CNJ (TJPB, TJRJ, TJPE, TJMG, TJBA)
- [x] Motor Gecko (Firefox/Zen) com perfis de privacidade
- [x] Resolução Turnstile via sidecar 2Captcha
- [x] Resolução reCAPTCHA v2
- [x] Integração com backend Node.js (child_process.spawn)
- [x] Busca por CNJ, nome de parte, OAB, CPF

### v1.1 (Em desenvolvimento)
- [ ] Hardware Datalake — Geração de perfis BIOS/SMBIOS estilo Hackintosh
- [ ] Rotação automática de perfis a cada N consultas
- [ ] API FastAPI completa (Roses API)
- [ ] Suporte a proxy residencial rotativo

### v1.2 (Próximo)
- [ ] TJSP (e-SAJ) — maior volume processual do Brasil
- [ ] TJRS (e-Proc)
- [ ] STJ (Superior Tribunal de Justiça)
- [ ] Consulta por CPF/CNPJ em múltiplos tribunais simultaneamente
- [ ] Concorrência: múltiplos tribunais em paralelo (asyncio)

### v2.0 (Visão)
- [ ] Dashboard de monitoramento de processos (alertas de movimentação)
- [ ] hCaptcha e reCAPTCHA v3
- [ ] Integração com serviços de IA para resumo de movimentações
- [ ] TST, TRF1 ao TRF6, Justiça Federal

---

## Sobre o Nome

**Roses** foi escolhido por representar algo que parece delicado por fora mas tem espinhos
que protegem o núcleo — assim como o motor: uma interface simples para os advogados,
mas com uma camada técnica robusta e agressiva de anti-detecção por baixo.

> *"Every rose has its thorn"* — e o Roses tem seus scrapers.

---

*Desenvolvido pela equipe Astra Legal AI. Versão 1.0 — 2025.*
