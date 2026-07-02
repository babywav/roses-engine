# 🌹 Roses — Motor de Consulta de Processos Jurídicos

**Versão:** 1.0.0  
**Stack:** Python 3.11 + DrissionPage + Playwright + FastAPI  
**Engine:** Gecko (Firefox/Zen Browser) + Chromium Stealth  

---

## Início Rápido

> **Tudo roda de dentro desta pasta (`roses/`).** O motor é autossuficiente —
> não depende de nenhum caminho externo.

```bash
# (a partir da pasta roses/)

# 1. Consulta por NÚMERO (CNJ) — caminho oficial DataJud, sem Cloudflare
python3 -m datajud.client --cnj "0001234-56.2023.8.19.0001"
python3 -m datajud.client --count TJPB          # total de processos do tribunal

# 2. Sincronização por OAB — portal (todos os processos vinculados)
python3 scrapers/pje_portal_scraper.py --oab 14233 --uf PB --json-only
python3 scrapers/pje_portal_scraper.py --oab 14233 --uf PB --headed   # com tela

# 3. Consulta por nome de parte — portal
python3 scrapers/pje_portal_scraper.py --nome-parte "João Victor Fernandes" --uf PB --json-only

# 4. Gerar um perfil de hardware anti-detect
python3 engine/hardware_datalake.py --type mac --engine gecko --js

# 5. Iniciar a API HTTP (DataJud primário + portal como fallback)
uvicorn api.roses_api:app --reload --port 8001
```

### Arquitetura de consulta (híbrida)

| Tipo de busca        | Caminho                | Cloudflare? |
|----------------------|------------------------|:-----------:|
| Número CNJ           | `datajud/` (oficial)   | Não         |
| Nome / OAB / CPF     | `scrapers/` (portal)   | Sim         |

## Tribunais Suportados

| Tribunal | Estado | Status |
|----------|--------|--------|
| TJPB | Paraíba | ✅ Operacional |
| TJRJ | Rio de Janeiro | ✅ Operacional |
| TJPE | Pernambuco | ✅ Operacional |
| TJMG | Minas Gerais | 🔧 Beta |
| TJBA | Bahia | 🔧 Beta |

## Documentação Completa

→ Ver [DETAILS.md](./DETAILS.md)
