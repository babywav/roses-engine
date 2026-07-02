#!/usr/bin/env python3
"""
test_datajud.py — Teste ao vivo da API Publica do DataJud (CNJ).

Varre o maximo de tribunais possivel comecando pelo TJPB e mostra, por tribunal:
  - se a API respondeu (status HTTP),
  - quantos documentos existem no indice (hits.total),
  - uma amostra de 1 processo (numero, classe, assunto, orgao, data, nº de movimentos).

Tudo via canal OFICIAL e GRATUITO do CNJ (Resolucao 331/2020). Sem Cloudflare,
sem captcha, sem proxy de terceiros, do seu proprio IP.

Uso:
    # Varredura padrao (PB primeiro, depois os demais TJs estaduais)
    python3 test_datajud.py

    # Consulta um numero CNJ especifico (detecta o tribunal pelos digitos 14-16)
    python3 test_datajud.py --cnj "0801234-56.2022.8.15.0001"

    # So um conjunto de siglas
    python3 test_datajud.py --apenas pb,pe,rj,sp

Sem dependencias externas: usa apenas a biblioteca padrao do Python 3.
"""

import argparse
import json
import re
import sys
import time
import urllib.request
import urllib.error

# --------------------------------------------------------------------------
# Configuracao
# --------------------------------------------------------------------------

# Chave PUBLICA vigente do DataJud (publicada na wiki do CNJ).
# Se algum dia retornar 401, atualize em: https://datajud-wiki.cnj.jus.br/api-publica/acesso/
API_KEY = "cDZHYzlZa0JadVREZDJCendQbXY6SkJlTzNjLV9TRENyQk1RdnFKZGRQdw=="

BASE = "https://api-publica.datajud.cnj.jus.br/{alias}/_search"

# Aliases dos Tribunais de Justica estaduais. PB primeiro (seu maior cliente).
TJS = [
    ("PB", "api_publica_tjpb", "15"),
    ("PE", "api_publica_tjpe", "17"),
    ("RJ", "api_publica_tjrj", "19"),
    ("SP", "api_publica_tjsp", "26"),
    ("MG", "api_publica_tjmg", "13"),
    ("BA", "api_publica_tjba", "05"),
    ("RS", "api_publica_tjrs", "21"),
    ("PR", "api_publica_tjpr", "16"),
    ("SC", "api_publica_tjsc", "24"),
    ("CE", "api_publica_tjce", "06"),
    ("GO", "api_publica_tjgo", "09"),
    ("DF", "api_publica_tjdft", "07"),
    ("ES", "api_publica_tjes", "08"),
    ("MA", "api_publica_tjma", "10"),
    ("MT", "api_publica_tjmt", "11"),
    ("MS", "api_publica_tjms", "12"),
    ("PA", "api_publica_tjpa", "14"),
    ("PI", "api_publica_tjpi", "18"),
    ("RN", "api_publica_tjrn", "20"),
    ("RO", "api_publica_tjro", "22"),
    ("RR", "api_publica_tjrr", "23"),
    ("SE", "api_publica_tjse", "25"),
    ("TO", "api_publica_tjto", "27"),
    ("AC", "api_publica_tjac", "01"),
    ("AL", "api_publica_tjal", "02"),
    ("AP", "api_publica_tjap", "03"),
    ("AM", "api_publica_tjam", "04"),
]

CODE_TO_ALIAS = {code: alias for _, alias, code in TJS}
SIGLA_TO_ALIAS = {sigla: alias for sigla, alias, _ in TJS}


# --------------------------------------------------------------------------
# HTTP
# --------------------------------------------------------------------------

def datajud_post(alias: str, body: dict, timeout: int = 30) -> tuple[int, dict]:
    """POST no endpoint do DataJud. Retorna (http_status, json|erro)."""
    url = BASE.format(alias=alias)
    data = json.dumps(body).encode("utf-8")
    req = urllib.request.Request(url, data=data, method="POST")
    req.add_header("Authorization", f"APIKey {API_KEY}")
    req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.status, json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        try:
            payload = json.loads(e.read().decode("utf-8"))
        except Exception:
            payload = {"error": str(e)}
        return e.code, payload
    except Exception as e:
        return 0, {"error": f"{type(e).__name__}: {e}"}


# --------------------------------------------------------------------------
# Formatacao
# --------------------------------------------------------------------------

def resumir_hit(hit: dict) -> str:
    """Extrai os campos principais de um documento DataJud."""
    src = hit.get("_source", {})
    numero = src.get("numeroProcesso", "?")
    classe = (src.get("classe") or {}).get("nome", "?")
    assuntos = src.get("assuntos") or []
    assunto = assuntos[0].get("nome", "?") if assuntos else "?"
    orgao = (src.get("orgaoJulgador") or {}).get("nome", "?")
    data_aj = src.get("dataAjuizamento", "?")
    n_mov = len(src.get("movimentos") or [])
    return (f"      numero={numero}\n"
            f"      classe={classe}\n"
            f"      assunto={assunto}\n"
            f"      orgao={orgao}\n"
            f"      ajuizamento={data_aj}  movimentos={n_mov}")


def total_de(resp: dict) -> str:
    total = (((resp.get("hits") or {}).get("total")) or {})
    if isinstance(total, dict):
        return str(total.get("value", "?"))
    return str(total)


# --------------------------------------------------------------------------
# Modos
# --------------------------------------------------------------------------

def sweep(siglas=None):
    """Varre os TJs, confirma conectividade e mostra 1 amostra de cada."""
    alvos = TJS
    if siglas:
        want = {s.strip().upper() for s in siglas}
        alvos = [t for t in TJS if t[0] in want]

    print("=" * 70)
    print("  TESTE DataJud (CNJ) — varredura de tribunais (PB primeiro)")
    print("=" * 70)

    ok, falhou = 0, 0
    for sigla, alias, code in alvos:
        body = {"size": 1, "query": {"match_all": {}}}
        t0 = time.time()
        status, resp = datajud_post(alias, body)
        dt = round(time.time() - t0, 2)

        if status == 200:
            hits = (resp.get("hits") or {}).get("hits") or []
            print(f"\n[OK]  TJ{sigla} ({alias})  http=200  {dt}s  total_indexado={total_de(resp)}")
            if hits:
                print(resumir_hit(hits[0]))
            ok += 1
        else:
            err = resp.get("error", resp)
            print(f"\n[ERRO] TJ{sigla} ({alias})  http={status}  {dt}s  -> {err}")
            falhou += 1
        time.sleep(0.3)  # gentil com a API

    print("\n" + "=" * 70)
    print(f"  RESUMO: {ok} tribunais OK, {falhou} com erro (de {len(alvos)})")
    print("=" * 70)


def consulta_cnj(cnj: str):
    """Consulta um numero CNJ especifico no tribunal correto."""
    digits = re.sub(r"\D", "", cnj)
    if len(digits) != 20:
        print(f"[ERRO] CNJ invalido (esperado 20 digitos, veio {len(digits)}): {cnj}")
        return
    code = digits[14:16]
    alias = CODE_TO_ALIAS.get(code)
    if not alias:
        print(f"[ERRO] Codigo de tribunal '{code}' nao mapeado.")
        return

    print(f"Consultando CNJ {cnj}  ->  codigo {code}  ->  {alias}")
    body = {"query": {"match": {"numeroProcesso": digits}}}
    status, resp = datajud_post(alias, body)
    if status != 200:
        print(f"[ERRO] http={status} -> {resp.get('error', resp)}")
        return
    hits = (resp.get("hits") or {}).get("hits") or []
    if not hits:
        print("Nenhum processo encontrado (verifique o numero ou se e processo publico).")
        return
    print(f"Encontrado(s) {len(hits)} documento(s):")
    for h in hits:
        print(resumir_hit(h))
        # mostra as 5 movimentacoes mais recentes
        movs = (h.get("_source") or {}).get("movimentos") or []
        for m in movs[:5]:
            print(f"        - {m.get('dataHora','?')}  {m.get('nome','?')}")


# --------------------------------------------------------------------------
# Main
# --------------------------------------------------------------------------

if __name__ == "__main__":
    ap = argparse.ArgumentParser(description="Teste ao vivo da API DataJud (CNJ).")
    ap.add_argument("--cnj", help="Numero CNJ especifico para consultar")
    ap.add_argument("--apenas", help="Lista de siglas separadas por virgula (ex: pb,pe,rj)")
    args = ap.parse_args()

    if args.cnj:
        consulta_cnj(args.cnj)
    else:
        sweep(args.apenas.split(",") if args.apenas else None)
