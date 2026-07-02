"""
roses/datajud/client.py

Cliente da API Publica do DataJud (CNJ).

- Resolve o tribunal a partir do numero CNJ (digitos 14-16).
- Consulta por numero de processo e devolve um RosesResult normalizado
  (mesmo formato de saida do resto do motor).
- Suporta _count e paginacao via search_after.
- Throttle interno para respeitar o limite de 120 req/min do Termo de Uso.

Sem dependencias externas: usa apenas a biblioteca padrao do Python 3.
"""

from __future__ import annotations

import json
import os
import re
import sys
import threading
import time
import urllib.request
import urllib.error
from pathlib import Path
from typing import Optional

# Importa os modelos do motor (Process, Party, Movement, RosesResult).
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))
from parsers.models import Process, Party, Movement, RosesResult  # noqa: E402
from core.cnj import format_cnj  # noqa: E402


# --------------------------------------------------------------------------
# Configuracao
# --------------------------------------------------------------------------

# Chave PUBLICA vigente (a mesma para todos). Pode ser sobrescrita por env var.
# Se retornar 401, atualize a partir de:
# https://datajud-wiki.cnj.jus.br/api-publica/acesso/
DEFAULT_API_KEY = "cDZHYzlZa0JadVREZDJCendQbXY6SkJlTzNjLV9TRENyQk1RdnFKZGRQdw=="

BASE_URL = "https://api-publica.datajud.cnj.jus.br/{alias}"

# Termo de Uso: no maximo 120 requisicoes/minuto por chave.
# Mantemos uma folga (110/min => intervalo minimo de ~0.55s entre chamadas).
MAX_REQ_PER_MIN = 110
_MIN_INTERVAL = 60.0 / MAX_REQ_PER_MIN


# Codigo CNJ (digitos 14-16) -> alias do indice DataJud. Justica Estadual.
ALIAS_BY_CODE: dict[str, str] = {
    "01": "api_publica_tjac", "02": "api_publica_tjal", "03": "api_publica_tjap",
    "04": "api_publica_tjam", "05": "api_publica_tjba", "06": "api_publica_tjce",
    "07": "api_publica_tjdft", "08": "api_publica_tjes", "09": "api_publica_tjgo",
    "10": "api_publica_tjma", "11": "api_publica_tjmt", "12": "api_publica_tjms",
    "13": "api_publica_tjmg", "14": "api_publica_tjpa", "15": "api_publica_tjpb",
    "16": "api_publica_tjpr", "17": "api_publica_tjpe", "18": "api_publica_tjpi",
    "19": "api_publica_tjrj", "20": "api_publica_tjrn", "21": "api_publica_tjrs",
    "22": "api_publica_tjro", "23": "api_publica_tjrr", "24": "api_publica_tjsc",
    "25": "api_publica_tjse", "26": "api_publica_tjsp", "27": "api_publica_tjto",
}

ALIAS_BY_SIGLA: dict[str, str] = {
    "TJAC": "api_publica_tjac", "TJAL": "api_publica_tjal", "TJAP": "api_publica_tjap",
    "TJAM": "api_publica_tjam", "TJBA": "api_publica_tjba", "TJCE": "api_publica_tjce",
    "TJDFT": "api_publica_tjdft", "TJES": "api_publica_tjes", "TJGO": "api_publica_tjgo",
    "TJMA": "api_publica_tjma", "TJMT": "api_publica_tjmt", "TJMS": "api_publica_tjms",
    "TJMG": "api_publica_tjmg", "TJPA": "api_publica_tjpa", "TJPB": "api_publica_tjpb",
    "TJPR": "api_publica_tjpr", "TJPE": "api_publica_tjpe", "TJPI": "api_publica_tjpi",
    "TJRJ": "api_publica_tjrj", "TJRN": "api_publica_tjrn", "TJRS": "api_publica_tjrs",
    "TJRO": "api_publica_tjro", "TJRR": "api_publica_tjrr", "TJSC": "api_publica_tjsc",
    "TJSE": "api_publica_tjse", "TJSP": "api_publica_tjsp", "TJTO": "api_publica_tjto",
}

CODE_TO_SIGLA: dict[str, str] = {
    code: alias.split("_")[-1].upper() for code, alias in ALIAS_BY_CODE.items()
}

# Tribunais Superiores e demais segmentos (uso por sigla e via resolver por CNJ).
ALIAS_BY_SIGLA.update({
    "STJ": "api_publica_stj",   # Superior Tribunal de Justica (segmento J=3)
    "TST": "api_publica_tst",   # Tribunal Superior do Trabalho
    "TSE": "api_publica_tse",   # Tribunal Superior Eleitoral
    "STM": "api_publica_stm",   # Superior Tribunal Militar (J=7)
})
# Justica Federal — TRF1..TRF6 (segmento J=4, TT = regiao).
for _r in range(1, 7):
    ALIAS_BY_SIGLA[f"TRF{_r}"] = f"api_publica_trf{_r}"
# Justica do Trabalho — TRT1..TRT24 (segmento J=5, TT = regiao).
for _r in range(1, 25):
    ALIAS_BY_SIGLA[f"TRT{_r}"] = f"api_publica_trt{_r}"


def _alias_from_cnj_segment(digits: str) -> Optional[str]:
    """
    Resolve o alias usando o segmento de justica (digito 13 = J) e o
    codigo do tribunal (digitos 14-15 = TT) do numero CNJ.

      J=8  Justica Estadual  -> ALIAS_BY_CODE[TT]
      J=4  Justica Federal    -> api_publica_trf{TT}
      J=5  Justica Trabalho   -> api_publica_trt{TT}  (TT=00 -> TST)
      J=3  Superior            -> STJ
      J=7  Militar da Uniao    -> STM
    """
    if len(digits) != 20:
        return None
    j = digits[13]
    tt = digits[14:16]
    if j == "8":
        return ALIAS_BY_CODE.get(tt)
    if j == "4":
        n = int(tt)
        return f"api_publica_trf{n}" if 1 <= n <= 6 else None
    if j == "5":
        n = int(tt)
        if n == 0:
            return "api_publica_tst"
        return f"api_publica_trt{n}" if 1 <= n <= 24 else None
    if j == "3":
        return "api_publica_stj"
    if j == "7":
        return "api_publica_stm"
    return None


class DataJudError(Exception):
    """Erro de comunicacao ou resposta invalida da API DataJud."""


# --------------------------------------------------------------------------
# Helpers
# --------------------------------------------------------------------------

def clean_cnj(cnj: str) -> str:
    """Remove tudo que nao for digito."""
    return re.sub(r"\D", "", cnj or "")


def resolve_alias(cnj: str) -> Optional[str]:
    """Resolve o alias DataJud a partir do numero CNJ (segmento J + codigo TT)."""
    return _alias_from_cnj_segment(clean_cnj(cnj))


def normalize_date(value) -> Optional[str]:
    """
    Normaliza datas do DataJud para DD/MM/AAAA.
    Aceita ISO ('2020-09-16T12:28:55.000Z') e numerico ('20070704000000').
    """
    if not value:
        return None
    s = str(value).strip()
    # ISO 8601
    if "-" in s or "T" in s:
        try:
            from datetime import datetime
            s2 = s.replace("Z", "").split(".")[0]
            dt = datetime.fromisoformat(s2)
            return dt.strftime("%d/%m/%Y")
        except Exception:
            pass
    # Numerico YYYYMMDD... (>=8 digitos)
    digits = re.sub(r"\D", "", s)
    if len(digits) >= 8:
        y, m, d = digits[0:4], digits[4:6], digits[6:8]
        if y.isdigit() and m.isdigit() and d.isdigit():
            return f"{d}/{m}/{y}"
    return s


def _first_name(lst, key="nome", default="") -> str:
    if isinstance(lst, list) and lst:
        return lst[0].get(key, default) if isinstance(lst[0], dict) else default
    return default


# --------------------------------------------------------------------------
# Cliente
# --------------------------------------------------------------------------

class DataJudClient:
    """Cliente HTTP da API Publica do DataJud."""

    _lock = threading.Lock()
    _last_call = 0.0

    def __init__(self, api_key: Optional[str] = None, timeout: int = 30):
        self.api_key = api_key or os.environ.get("DATAJUD_API_KEY", DEFAULT_API_KEY)
        self.timeout = timeout

    # -- baixo nivel -------------------------------------------------------

    def _throttle(self):
        """Garante o intervalo minimo entre chamadas (respeita 120 req/min)."""
        with DataJudClient._lock:
            elapsed = time.time() - DataJudClient._last_call
            if elapsed < _MIN_INTERVAL:
                time.sleep(_MIN_INTERVAL - elapsed)
            DataJudClient._last_call = time.time()

    def _post(self, alias: str, path: str, body: dict) -> dict:
        """POST cru no endpoint do DataJud. Levanta DataJudError em falha."""
        self._throttle()
        url = BASE_URL.format(alias=alias) + path
        data = json.dumps(body).encode("utf-8")
        req = urllib.request.Request(url, data=data, method="POST")
        req.add_header("Authorization", f"APIKey {self.api_key}")
        req.add_header("Content-Type", "application/json")
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as resp:
                return json.loads(resp.read().decode("utf-8"))
        except urllib.error.HTTPError as e:
            detail = ""
            try:
                detail = e.read().decode("utf-8")[:300]
            except Exception:
                pass
            if e.code == 401:
                raise DataJudError(
                    "401: chave publica invalida/expirada. Atualize em "
                    "https://datajud-wiki.cnj.jus.br/api-publica/acesso/"
                ) from e
            if e.code == 429:
                raise DataJudError("429: rate limit atingido (>120 req/min). Reduza o ritmo.") from e
            raise DataJudError(f"HTTP {e.code}: {detail}") from e
        except Exception as e:
            raise DataJudError(f"{type(e).__name__}: {e}") from e

    def raw_search(self, alias: str, query: dict) -> dict:
        """Executa um _search bruto e retorna o JSON do Elasticsearch."""
        return self._post(alias, "/_search", query)

    def iter_all(
        self,
        tribunal: str,
        query: Optional[dict] = None,
        page_size: int = 1000,
        max_records: Optional[int] = None,
        sort: Optional[list] = None,
    ):
        """
        Itera por TODOS os documentos de um indice usando paginacao
        search_after (sem o teto de 10.000 de um unico _search).

        Gera (yield) cada hit (_source). `tribunal` pode ser sigla, codigo
        ou alias. `sort` precisa ser deterministico; por padrao usa
        @timestamp asc (campo padrao do DataJud).
        """
        alias = self._alias_from_tribunal(tribunal)
        sort = sort or [{"@timestamp": {"order": "asc"}}]
        search_after = None
        yielded = 0

        while True:
            body = {
                "size": min(page_size, 10000),
                "query": query or {"match_all": {}},
                "sort": sort,
            }
            if search_after is not None:
                body["search_after"] = search_after

            resp = self._post(alias, "/_search", body)
            hits = ((resp.get("hits") or {}).get("hits")) or []
            if not hits:
                break

            for h in hits:
                yield h.get("_source", {})
                yielded += 1
                if max_records is not None and yielded >= max_records:
                    return

            last = hits[-1]
            search_after = last.get("sort")
            if not search_after:
                break

    def count(self, tribunal: str, query: Optional[dict] = None) -> int:
        """
        Conta documentos do indice (endpoint _count, sem o teto de 10.000).
        `tribunal` pode ser sigla (TJPB), codigo CNJ (15) ou alias completo.
        """
        alias = self._alias_from_tribunal(tribunal)
        body = {"query": query or {"match_all": {}}}
        resp = self._post(alias, "/_count", body)
        return int(resp.get("count", 0))

    # -- alto nivel --------------------------------------------------------

    @staticmethod
    def _alias_from_tribunal(tribunal: str) -> str:
        t = (tribunal or "").strip().upper()
        if t.startswith("API_PUBLICA_"):
            return t.lower()
        if t in ALIAS_BY_SIGLA:
            return ALIAS_BY_SIGLA[t]
        if t in ALIAS_BY_CODE:
            return ALIAS_BY_CODE[t]
        raise DataJudError(f"Tribunal nao reconhecido: {tribunal}")

    def by_cnj(self, cnj: str, include_movimentos: bool = True) -> RosesResult:
        """
        Consulta um processo pelo numero CNJ e devolve um RosesResult.

        Obs.: a API publica NAO expoe nome das partes (LGPD). O campo `partes`
        vem vazio aqui; para nomes, use o portal (busca por nome).
        """
        digits = clean_cnj(cnj)
        query_meta = {"cnj": cnj}

        if len(digits) != 20:
            return RosesResult(
                status="INVALID",
                message=f"Numero CNJ invalido: esperado 20 digitos, veio {len(digits)}.",
                tribunal="N/A", tribunal_name="N/A", query=query_meta,
            )

        alias = _alias_from_cnj_segment(digits)
        if not alias:
            return RosesResult(
                status="INVALID",
                message=f"Tribunal nao mapeado no DataJud (J={digits[13]}, TT={digits[14:16]}).",
                tribunal="N/A", tribunal_name="N/A", query=query_meta,
            )

        sigla = alias.split("_")[-1].upper()
        body = {"size": 10, "query": {"match": {"numeroProcesso": digits}}}

        try:
            resp = self.raw_search(alias, body)
        except DataJudError as e:
            return RosesResult(
                status="ERROR", message=str(e),
                tribunal=sigla, tribunal_name=sigla, query=query_meta,
            )

        hits = ((resp.get("hits") or {}).get("hits")) or []
        if not hits:
            return RosesResult.not_found(sigla, sigla, query_meta)

        processos = [self._hit_to_process(h, sigla, include_movimentos) for h in hits]
        return RosesResult(
            status="OK",
            message=f"{len(processos)} processo(s) encontrado(s) via DataJud.",
            tribunal=sigla, tribunal_name=sigla, query=query_meta,
            total=len(processos), processos=processos,
        )

    def _hit_to_process(self, hit: dict, sigla: str, include_movimentos: bool) -> Process:
        src = hit.get("_source", {}) or {}

        movimentacoes = []
        if include_movimentos:
            # Ordena pelos digitos da dataHora bruta (YYYYMMDDHHMMSS), que e
            # cronologicamente correto — diferente de ordenar o texto DD/MM/AAAA.
            movs_raw = src.get("movimentos") or []
            movs_raw = sorted(
                movs_raw,
                key=lambda m: re.sub(r"\D", "", str(m.get("dataHora") or "")),
                reverse=True,  # mais recentes primeiro
            )
            for m in movs_raw:
                movimentacoes.append(Movement(
                    data=normalize_date(m.get("dataHora")) or "",
                    descricao=m.get("nome", ""),
                    orgao=(m.get("orgaoJulgador") or {}).get("nome") if isinstance(m.get("orgaoJulgador"), dict) else None,
                ))

        # A API publica geralmente nao traz nomes de partes; mapeamos se existir.
        partes = []
        for p in (src.get("partes") or []):
            if isinstance(p, dict):
                partes.append(Party(
                    tipo=p.get("polo", p.get("tipo", "")),
                    nome=p.get("nome", ""),
                ))

        return Process(
            numero=format_cnj(src.get("numeroProcesso", "")),
            classe=(src.get("classe") or {}).get("nome", ""),
            assunto=_first_name(src.get("assuntos")),
            tribunal=sigla,
            orgao_julgador=(src.get("orgaoJulgador") or {}).get("nome"),
            data_distribuicao=normalize_date(src.get("dataAjuizamento")),
            partes=partes,
            movimentacoes=movimentacoes,
            url_processo=None,
        )


# --------------------------------------------------------------------------
# CLI rapido
# --------------------------------------------------------------------------

if __name__ == "__main__":
    import argparse
    ap = argparse.ArgumentParser(description="Cliente DataJud (CNJ).")
    ap.add_argument("--cnj", help="Numero CNJ a consultar")
    ap.add_argument("--count", help="Sigla/codigo do tribunal para contar processos (ex: TJPB)")
    args = ap.parse_args()

    client = DataJudClient()
    if args.count:
        print(f"{args.count}: {client.count(args.count):,} processos indexados")
    elif args.cnj:
        result = client.by_cnj(args.cnj)
        print(result.to_json())
    else:
        ap.print_help()
