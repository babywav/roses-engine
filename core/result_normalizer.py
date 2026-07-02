"""
roses/core/result_normalizer.py

Normaliza a saida do PORTAL (dicts do pje_results_parser) para o mesmo
schema do resto do motor (parsers.models): Process / Party / Movement /
RosesResult — identico ao que o DataJud ja devolve.

Assim, as duas fontes (DataJud e portal) produzem exatamente a mesma
estrutura, e o consumidor nao precisa saber de onde veio.
"""

from __future__ import annotations

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))
from parsers.models import Process, Party, Movement, RosesResult  # noqa: E402


def _parties_from_portal(proc: dict) -> list[Party]:
    partes = []
    if proc.get("polo_ativo"):
        partes.append(Party(tipo="Polo Ativo", nome=proc["polo_ativo"]))
    if proc.get("polo_passivo"):
        partes.append(Party(tipo="Polo Passivo", nome=proc["polo_passivo"]))
    return partes


def _movements_from_portal(proc: dict) -> list[Movement]:
    desc = proc.get("ultima_movimentacao")
    if not desc:
        return []
    return [Movement(data=proc.get("data_ultima_movimentacao") or "", descricao=desc)]


def portal_process_to_model(proc: dict, tribunal: str) -> Process:
    """Converte um dict de processo do parser do portal em um Process."""
    return Process(
        numero=proc.get("numero", ""),
        classe=proc.get("classe", ""),
        assunto=proc.get("assunto", ""),
        tribunal=tribunal,
        orgao_julgador=None,
        data_distribuicao=None,
        partes=_parties_from_portal(proc),
        movimentacoes=_movements_from_portal(proc),
        url_processo=proc.get("detalhe_url"),
    )


def portal_to_roses_result(
    portal_result: dict,
    tribunal: str,
    tribunal_name: str | None = None,
    query: dict | None = None,
) -> RosesResult:
    """
    Converte o dict completo retornado pelo scraper do portal em um RosesResult.
    Aceita tanto o formato com 'processos' (novo) quanto o antigo (so status).
    """
    tribunal_name = tribunal_name or tribunal
    query = query or portal_result.get("query") or {}
    status = portal_result.get("status", "UNKNOWN")

    processos = [
        portal_process_to_model(p, tribunal)
        for p in (portal_result.get("processos") or [])
    ]

    return RosesResult(
        status=status,
        message=portal_result.get("message", ""),
        tribunal=tribunal,
        tribunal_name=tribunal_name,
        query=query,
        total=portal_result.get("total", len(processos)),
        processos=processos,
    )
