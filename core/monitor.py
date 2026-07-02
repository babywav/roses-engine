"""
roses/core/monitor.py

Monitoramento de processos. Roda agendado (ex.: diariamente) e avisa quando
um processo monitorado tem movimentacao NOVA.

Estrategia:
  - CNJs monitorados -> verificados via DataJud (oficial, sem Cloudflare).
  - OAB -> 'sincroniza' pelo portal (uma vez) para descobrir os numeros dos
    processos e registra cada um como CNJ monitorado; dai em diante o
    acompanhamento diario e todo via DataJud (barato e sem Cloudflare).

Uso:
  python3 -m core.monitor --add-cnj "0000166-95.1997.8.15.0211"
  python3 -m core.monitor --sync-oab 14233 --uf PB     # descobre processos da OAB
  python3 -m core.monitor --run                        # checa tudo e reporta novidades
  python3 -m core.monitor --list
"""

from __future__ import annotations

import logging
import sys
from pathlib import Path

ROSES_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROSES_ROOT))

from datajud.client import DataJudClient, DataJudError  # noqa: E402
from core.storage import Storage  # noqa: E402

logger = logging.getLogger("roses.monitor")


def run_check(storage: Storage, datajud: DataJudClient | None = None) -> dict:
    """
    Verifica todos os CNJs monitorados via DataJud e persiste.
    Retorna {numero: [movimentos novos]} apenas dos que mudaram.
    """
    datajud = datajud or DataJudClient()
    novidades = {}
    for item in storage.list_tracking(tipo="cnj"):
        cnj = item["valor"]
        try:
            result = datajud.by_cnj(cnj)
        except DataJudError as e:
            logger.warning("Falha ao consultar %s: %s", cnj, e)
            continue
        if result.status == "OK":
            novas = storage.save_result(result)
            novidades.update(novas)
    return novidades


def sync_oab(storage: Storage, oab: str, uf: str, engine=None) -> int:
    """
    Descobre os processos de uma OAB pelo portal e registra cada numero
    para monitoramento via DataJud. Retorna quantos numeros foram registrados.
    """
    # Import tardio: o portal exige camoufox/playwright (so quando usado).
    from core.engine import RosesEngine
    engine = engine or RosesEngine()
    result = engine.consultar(oab=oab, uf=uf)
    storage.add_tracking("oab", oab, uf)
    n = 0
    for proc in result.processos:
        if proc.numero:
            storage.add_tracking("cnj", proc.numero, uf)
            storage.save_process(proc, result.status)
            n += 1
    return n


def _format_novidades(novidades: dict) -> str:
    if not novidades:
        return "Nenhuma movimentacao nova."
    linhas = [f"{len(novidades)} processo(s) com novidade:\n"]
    for numero, movs in novidades.items():
        linhas.append(f"  • {numero}")
        for m in movs:
            linhas.append(f"      {m.data}  {m.descricao}")
    return "\n".join(linhas)


if __name__ == "__main__":
    import argparse

    ap = argparse.ArgumentParser(description="Roses — monitor de processos.")
    ap.add_argument("--add-cnj", help="Registra um CNJ para monitorar")
    ap.add_argument("--sync-oab", help="Descobre processos de uma OAB (portal) e monitora")
    ap.add_argument("--uf", help="UF da OAB (com --sync-oab)")
    ap.add_argument("--run", action="store_true", help="Verifica todos e reporta novidades")
    ap.add_argument("--list", action="store_true", help="Lista o que esta sendo monitorado")
    args = ap.parse_args()

    logging.basicConfig(level=logging.INFO, format="%(levelname)s %(name)s: %(message)s")
    store = Storage()

    if args.add_cnj:
        store.add_tracking("cnj", args.add_cnj)
        print(f"Monitorando CNJ {args.add_cnj}")
    elif args.sync_oab:
        if not args.uf:
            sys.exit("--sync-oab exige --uf (ex: --sync-oab 14233 --uf PB)")
        n = sync_oab(store, args.sync_oab, args.uf)
        print(f"OAB {args.sync_oab}/{args.uf}: {n} processo(s) registrados para monitoramento.")
    elif args.list:
        for t in store.list_tracking():
            print(f"  [{t['tipo']}] {t['valor']}" + (f"/{t['uf']}" if t['uf'] else ""))
        print(f"\nTotal de processos no banco: {store.count_processos()}")
    elif args.run:
        novidades = run_check(store)
        print(_format_novidades(novidades))
    else:
        ap.print_help()
