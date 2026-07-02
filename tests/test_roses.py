"""
roses/tests/test_roses.py

Suite de testes do motor Roses. Roda offline (sem rede/navegador), usando a
fixture HTML real do TJPB.

Uso:
    python3 -m pytest tests/            # se tiver pytest
    python3 tests/test_roses.py        # runner embutido (sem dependencia)
"""

import sys
from pathlib import Path

ROSES_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROSES_ROOT))

FIXTURE = ROSES_ROOT / "scrapers" / "fixtures" / "tjpb_oab_sample.html"


# -- Parser ----------------------------------------------------------------

def test_parser_extrai_31_processos():
    from parsers.pje_results_parser import parse_results_html
    html = FIXTURE.read_text(encoding="utf-8", errors="replace")
    data = parse_results_html(html)
    assert data["status"] == "OK"
    assert data["total_reported"] == 31
    assert data["total_extracted"] == 31
    p = data["processos"][0]
    assert p["numero"] == "0000166-95.1997.8.15.0211"
    assert p["classe"] == "CUMPRIMENTO DE SENTENÇA"
    assert p["polo_ativo"] and p["polo_passivo"]
    assert p["detalhe_url"].startswith("https://")


def test_parser_vazio_nao_quebra():
    from parsers.pje_results_parser import parse_results_html
    data = parse_results_html("<html><body>nada aqui</body></html>")
    assert data["status"] in ("UNKNOWN", "NOT_FOUND")
    assert data["processos"] == []


# -- CNJ -------------------------------------------------------------------

def test_cnj_valido_e_invalido():
    from core.cnj import is_valid
    assert is_valid("0000166-95.1997.8.15.0211")
    assert is_valid("4001220-77.2025.8.26.0037")
    assert not is_valid("0000166-95.1997.8.15.0212")   # digito trocado
    assert not is_valid("123")                          # curto demais


# -- Resolver de alias DataJud --------------------------------------------

def test_resolver_alias_por_segmento():
    from datajud.client import resolve_alias
    assert resolve_alias("0000166-95.1997.8.15.0211") == "api_publica_tjpb"   # estadual
    assert resolve_alias("4001220-77.2025.8.26.0037") == "api_publica_tjsp"   # estadual
    assert resolve_alias("0800000-00.2020.4.05.8200") == "api_publica_trf5"   # federal
    assert resolve_alias("0000100-00.2020.5.13.0001") == "api_publica_trt13"  # trabalho
    assert resolve_alias("0000100-00.2020.3.00.0000") == "api_publica_stj"    # superior


# -- Normalizador (portal -> models) --------------------------------------

def test_normalizer_portal_para_models():
    from parsers.pje_results_parser import parse_results_html
    from core.result_normalizer import portal_to_roses_result
    parsed = parse_results_html(FIXTURE.read_text(encoding="utf-8", errors="replace"))
    portal = {"status": "OK", "message": "ok", "total": parsed["total_extracted"],
              "processos": parsed["processos"]}
    rr = portal_to_roses_result(portal, "TJPB", "TJPB", {"oab": "14233"})
    assert rr.status == "OK" and rr.total == 31
    assert rr.processos[0].numero == "0000166-95.1997.8.15.0211"
    assert len(rr.processos[0].partes) == 2


# -- Persistencia + diff ---------------------------------------------------

def test_storage_diff_incremental():
    import tempfile, os
    from parsers.models import Process, Movement, RosesResult
    from core.storage import Storage

    db = tempfile.mktemp(suffix=".db")
    st = Storage(db)
    try:
        def res(movs):
            p = Process(numero="0000166-95.1997.8.15.0211", classe="X", assunto="Y",
                        tribunal="TJPB",
                        movimentacoes=[Movement(data=d, descricao=t) for d, t in movs])
            return RosesResult(status="OK", message="", tribunal="TJPB",
                               tribunal_name="TJPB", query={}, total=1, processos=[p])

        n1 = st.save_result(res([("01/01/2026", "A"), ("02/01/2026", "B")]))
        n2 = st.save_result(res([("01/01/2026", "A"), ("02/01/2026", "B")]))
        n3 = st.save_result(res([("01/01/2026", "A"), ("02/01/2026", "B"), ("03/01/2026", "C")]))
        assert len(n1) == 1            # 1 processo com novidade
        assert n2 == {}               # nada novo na 2a vez
        assert n3["0000166-95.1997.8.15.0211"][0].descricao == "C"
        assert st.count_processos() == 1
    finally:
        st.close()
        os.unlink(db)


# -- Runner embutido (sem pytest) -----------------------------------------

if __name__ == "__main__":
    testes = [v for k, v in sorted(globals().items()) if k.startswith("test_")]
    falhas = 0
    for t in testes:
        try:
            t()
            print(f"  PASS  {t.__name__}")
        except Exception as e:
            falhas += 1
            print(f"  FALHA {t.__name__}: {type(e).__name__}: {e}")
    print(f"\n{len(testes) - falhas}/{len(testes)} testes passaram.")
    sys.exit(1 if falhas else 0)
