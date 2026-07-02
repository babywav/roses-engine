import argparse
import os
import time
import json
import multiprocessing
import re
import sys

from pathlib import Path


def resolve_headless(headed: bool):
    """
    Resolve o modo de execucao do navegador.

    Env ROSES_HEADLESS controla (ideal para VPS):
      - "virtual" -> Camoufox sobe display virtual (Xvfb). RECOMENDADO em servidor.
      - "true"    -> headless puro (mais detectavel; evite no portal).
      - "false"   -> com janela (so em desktop/debug).
    Sem a env: usa a flag --headed (False => headless puro; True => janela).
    """
    mode = (os.environ.get("ROSES_HEADLESS") or "").strip().lower()
    if mode == "virtual":
        return "virtual"
    if mode == "true":
        return True
    if mode == "false":
        return False
    return not headed

# requests so e usado pelo sidecar antigo (desativado). Import opcional para
# nao virar dependencia obrigatoria do fluxo principal (Camoufox/Playwright).
try:
    import requests
except ImportError:
    requests = None

# ── Ghost Engine v2 ────────────────────────────────────────────────────────
# Camoufox: Firefox com patches em C++ (fingerprint nativo, não JS shim)
try:
    from camoufox.sync_api import Camoufox
    CAMOUFOX_AVAILABLE = True
except ImportError:
    CAMOUFOX_AVAILABLE = False

# ── Pacote roses (autossuficiente) ─────────────────────────────────────────
# Este scraper vive DENTRO de roses/. Tudo que ele precisa esta no proprio
# pacote: engine/ (datalake, stealth, solver) e parsers/ (parser de resultados).
# Nao depende de nenhuma pasta externa.
ROSES_ROOT = Path(__file__).resolve().parent.parent          # .../roses
if str(ROSES_ROOT) not in sys.path:
    sys.path.insert(0, str(ROSES_ROOT))

# Carrega roses/.env (proxy Bright Data etc.) se existir, sem sobrescrever o ambiente.
def _load_env_file():
    envp = ROSES_ROOT / ".env"
    if not envp.exists():
        return
    try:
        for line in envp.read_text(encoding="utf-8").splitlines():
            line = line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            k, v = line.split("=", 1)
            k = k.strip(); v = v.strip().strip('"').strip("'")
            if k and k not in os.environ:
                os.environ[k] = v
    except Exception:
        pass
_load_env_file()

# Hardware Datalake v2: M1 → M4 Max com rotação LCG garantida
try:
    from engine.hardware_datalake import generate_hardware_fingerprint
    from engine.stealth_layer import BASE_STEALTH_SCRIPT, get_gecko_firefox_prefs
    HARDWARE_DATALAKE_AVAILABLE = True
except Exception as _e:
    print(f"[GhostEngine] Hardware datalake indisponivel: {_e}")
    HARDWARE_DATALAKE_AVAILABLE = False

# Roses Native Solver: Biometric CDP Clicker
try:
    from engine.turnstile_solver import NativeTurnstileSolver
    NATIVE_SOLVER_AVAILABLE = True
except Exception as _e:
    print(f"[GhostEngine] Native solver indisponivel: {_e}")
    NATIVE_SOLVER_AVAILABLE = False

# Parser de resultados (extrai dados estruturados dos processos da tabela PJe).
try:
    from parsers.pje_results_parser import parse_results_html
    PARSER_AVAILABLE = True
except Exception as _e:
    print(f"[GhostEngine] Parser de resultados indisponivel: {_e}")
    PARSER_AVAILABLE = False
# ──────────────────────────────────────────────────────────────────────────

# Mapping of court codes (digits 14-16 of CNJ) to names and URLs
COURT_MAP = {
    "15": ("TJPB", "https://consultapublica.tjpb.jus.br/pje/ConsultaPublica/listView.seam", "0x4AAAAAAAAjq6WYeRDKmebM"),
    "19": ("TJRJ", "https://tjrj.pje.jus.br/pje/ConsultaPublica/listView.seam", None),
    "17": ("TJPE", "https://pje.tjpe.jus.br/pje/ConsultaPublica/listView.seam", None),
    "13": ("TJMG", "https://pje.tjmg.jus.br/pje/ConsultaPublica/listView.seam", None),
    "05": ("TJBA", "https://pje.tjba.jus.br/pje/ConsultaPublica/listView.seam", None),
}

# Fallback mapping based on UF
UF_MAP = {
    "PB": ("TJPB", "https://consultapublica.tjpb.jus.br/pje/ConsultaPublica/listView.seam", "0x4AAAAAAAAjq6WYeRDKmebM"),
    "RJ": ("TJRJ", "https://tjrj.pje.jus.br/pje/ConsultaPublica/listView.seam", None),
    "PE": ("TJPE", "https://pje.tjpe.jus.br/pje/ConsultaPublica/listView.seam", None),
    "MG": ("TJMG", "https://pje.tjmg.jus.br/pje/ConsultaPublica/listView.seam", None),
    "BA": ("TJBA", "https://pje.tjba.jus.br/pje/ConsultaPublica/listView.seam", None),
}

OUTPUT_DIR = ROSES_ROOT / "data"
DEFAULT_WAIT_SECONDS = 10
DEFAULT_OVERALL_TIMEOUT = 70
DEFAULT_MANUAL_CHALLENGE_SECONDS = 25

SELECTORS = {
    "cnj": 'css:input[name*="inputNumeroProcesso"]',
    "cnj_digito": 'css:input[name*="inputDigitoProcesso"]',
    "cnj_ano": 'css:input[name*="inputAnoProcesso"]',
    "cnj_origem": 'css:input[name*="inputOrigemProcesso"]',
    "processo_referencia": 'css:input[name*="processoReferenciaInput"]',
    "nome_parte": 'css:input[name*="nomeParte"]',
    "nome_advogado": 'css:input[name*="nomeAdv"]',
    "documento": 'css:input[name*="documentoParte"]',
    "numero_oab": 'css:input[name*="numeroOAB"]',
    "uf_oab": 'css:select[name*="estadoComboOAB"]',
    "pesquisar": 'css:input[name*="searchProcessos"]',
}


def log(message: str, quiet: bool = False) -> None:
    if not quiet:
        print(message)


def normalize_filename(value: str) -> str:
    cleaned = re.sub(r"[^a-zA-Z0-9_-]+", "_", value.strip())
    return cleaned.strip("_") or "consulta"


def resolve_court(query: dict) -> tuple[str, str, str | None]:
    # 1. Check CNJ
    cnj = query.get("cnj")
    if cnj:
        digits = re.sub(r"\D", "", cnj)
        if len(digits) == 20:
            court_code = digits[14:16]
            if court_code in COURT_MAP:
                return COURT_MAP[court_code]
    
    # 2. Check UF
    uf = query.get("uf_oab") or query.get("uf")
    if uf:
        norm_uf = uf.strip().upper()
        if norm_uf in UF_MAP:
            return UF_MAP[norm_uf]
            
    # Default fallback to TJPB
    return COURT_MAP["15"]


def detect_status(html: str, court_name: str) -> tuple[str, str]:
    lowered = html.lower()
    if "nãºmero de processo invã¡lido" in html or "número de processo inválido" in html or "processo inválido" in lowered:
        if "resultados encontrados" not in lowered:
            return "INVALID", "Numero de processo invalido"
    
    if "resultados encontrados" in lowered or "resultado encontrado" in lowered or "processostable" in lowered or "classeprocesso" in lowered:
        return "OK", "Lista de processos localizada"
        
    if "nenhum resultado encontrado" in html or "não foram encontrados" in lowered or "nenhum registro" in lowered or "não há processos" in lowered:
        return "NOT_FOUND", "Nenhum resultado encontrado"
        
    if "cf-turnstile" in html or "challenge-platform" in html or "g-recaptcha" in html:
        return "CHALLENGE", "Desafio de seguranca presente"
        
    if "listaprocesso" in html or "detalhes do processo" in html or "processo" in lowered:
        return "OK", f"Resposta do portal {court_name} recebida"
        
    return "UNKNOWN", "Resposta recebida sem classificador deterministico"


def get_token(url: str, sitekey: str | None, quiet: bool = False):
    if not sitekey:
        return None
    log(f"[Sidecar] Solicitando token Turnstile para sitekey {sitekey}...", quiet)
    sidecar_url = f"http://127.0.0.1:5000/turnstile?url={url}&sitekey={sitekey}"
    try:
        response = requests.get(sidecar_url, timeout=10)
        response.raise_for_status()
        task_id = response.json().get("task_id")
        if not task_id:
            return None

        for _ in range(20):
            time.sleep(2)
            result = requests.get(f"http://127.0.0.1:5000/result?id={task_id}", timeout=10)
            result.raise_for_status()
            value = result.json().get("value")
            if value and value != "CAPTCHA_NOT_READY":
                return value
    except Exception:
        return None
    return None


def parse_args():
    parser = argparse.ArgumentParser(description="Consulta publica PJe unificada via emulador local.")
    parser.add_argument("cnj_posicional", nargs="?", help="CNJ para consulta direta.")
    parser.add_argument("--cnj", dest="cnj")
    parser.add_argument("--processo-referencia", dest="processo_referencia")
    parser.add_argument("--nome-parte", dest="nome_parte")
    parser.add_argument("--nome-advogado", dest="nome_advogado")
    parser.add_argument("--cpf")
    parser.add_argument("--cnpj")
    parser.add_argument("--oab")
    parser.add_argument("--uf")
    parser.add_argument("--wait-seconds", type=int, default=DEFAULT_WAIT_SECONDS)
    parser.add_argument("--overall-timeout", type=int, default=DEFAULT_OVERALL_TIMEOUT)
    parser.add_argument("--manual-challenge-seconds", type=int, default=DEFAULT_MANUAL_CHALLENGE_SECONDS)
    parser.add_argument("--max-pages", type=int, default=200,
                        help="Maximo de paginas a coletar (sincronizacao OAB).")
    parser.add_argument("--port", type=int, default=9260)
    parser.add_argument("--headed", action="store_true", help="Mostra o browser. O padrao e headless.")
    parser.add_argument("--quiet", action="store_true")
    parser.add_argument("--json-only", action="store_true", help="Emite apenas JSON no stdout.")
    return parser.parse_args()


def build_query(args):
    query = {
        "cnj": args.cnj or args.cnj_posicional,
        "processo_referencia": args.processo_referencia,
        "nome_parte": args.nome_parte,
        "nome_advogado": args.nome_advogado,
        "documento": args.cpf or args.cnpj,
        "documento_tipo": "CPF" if args.cpf else "CNPJ" if args.cnpj else None,
        "numero_oab": args.oab,
        "uf_oab": (args.uf or "").upper() or None,
        "uf": (args.uf or "").upper() or None,
    }
    active = {key: value for key, value in query.items() if value}
    if not active:
        raise SystemExit("Nenhum criterio de busca informado.")
    return query, active


def wait_for_form(page):
    for _ in range(30):
        if page.ele(SELECTORS["pesquisar"], timeout=2):
            return True
        time.sleep(1)
    return False


def inject_token_if_available(page, url: str, sitekey: str | None, quiet: bool = False):
    if not sitekey:
        return False
    token = get_token(url, sitekey, quiet)
    if not token:
        return False
    log("[Sidecar] Token Turnstile recebido. Injetando...", quiet)
    page.run_js(
        'const field = document.querySelector("[name=\'cf-turnstile-response\']");'
        f'if (field) field.value = "{token}";'
    )
    return True


def wait_for_manual_challenge(page, seconds: int, court_name: str, quiet: bool = False) -> None:
    if seconds <= 0:
        return
    log(
        f"[{court_name}] Desafio de seguranca detectado. Resolva manualmente em ate {seconds}s.",
        quiet,
    )
    deadline = time.time() + seconds
    while time.time() < deadline:
        has_challenge = False
        # Check top level
        has_challenge = bool(
            page.evaluate(
                """
                Boolean(
                    document.querySelector('.cf-turnstile')
                    || document.querySelector('[name="cf-turnstile-response"]')
                    || document.querySelector('.g-recaptcha')
                )
                """
            )
        )
        
        # Check frames
        if not has_challenge:
            for frame in page.frames:
                if frame.url and ("turnstile" in frame.url.lower() or "recaptcha" in frame.url.lower()):
                    has_challenge = True
                    break

        if not has_challenge:
            log(f"[{court_name}] Desafio resolvido ou nao visivel. Prosseguindo.", quiet)
            return
        time.sleep(1)
    log(f"[{court_name}] Tempo de resolucao manual expirou. Continuando...", quiet)


def select_oab_uf(page, uf: str) -> bool:
    if not uf:
        return False

    select_element = page.ele(SELECTORS["uf_oab"], timeout=5)
    if not select_element:
        return False

    normalized_uf = uf.strip().upper()
    try:
        options = page.evaluate(
            """
            (selector) => {
                const select = document.querySelector(selector);
                if (!select) return [];
                return Array.from(select.options).map(option => ({
                    value: option.value,
                    text: (option.textContent || '').trim().toUpperCase(),
                }));
            }
            """,
            'select[name*="estadoComboOAB"]',
        )
    except Exception:
        options = []

    target_value = normalized_uf
    for option in options or []:
        if option.get("text") == normalized_uf:
            target_value = option.get("value") or normalized_uf
            break

    page.evaluate(
        """
        ([selector, wanted]) => {
            const select = document.querySelector(selector);
            if (!select) return false;
            const option = Array.from(select.options).find(
                item => (item.value || '').toUpperCase() === wanted.toUpperCase()
                    || (item.textContent || '').trim().toUpperCase() === wanted.toUpperCase()
            );
            if (!option) return false;
            select.value = option.value;
            select.dispatchEvent(new Event('change', { bubbles: true }));
            return true;
        }
        """,
        ['select[name*="estadoComboOAB"]', target_value],
    )
    return True


def select_document_type(page, document_type: str):
    radio_index = 0 if document_type == "CPF" else 1
    radios = page.eles('css:input[name*="tipoMascaraDocumento"]')
    if len(radios) > radio_index:
        radios[radio_index].click()


def fill_if_present(page, selector_key: str, value: str):
    if not value:
        return
    
    # Check split CNJ structure
    if selector_key == "cnj" and len(value.replace("-","").replace(".","")) >= 20:
        digits = value.replace("-", "").replace(".", "")
        try:
            page.ele(SELECTORS["cnj"]).input(digits[0:7])
            page.ele(SELECTORS["cnj_digito"]).input(digits[7:9])
            page.ele(SELECTORS["cnj_ano"]).input(digits[9:13])
            page.ele(SELECTORS["cnj_origem"]).input(digits[16:20])
            return
        except:
            pass

    element = page.ele(SELECTORS[selector_key], timeout=5)
    if element:
        element.clear()
        element.input(value)


def run_search(page, query, quiet: bool = False):
    if query["documento_tipo"]:
        select_document_type(page, query["documento_tipo"])

    fill_if_present(page, "cnj", query["cnj"])
    fill_if_present(page, "processo_referencia", query["processo_referencia"])
    fill_if_present(page, "nome_parte", query["nome_parte"])
    fill_if_present(page, "nome_advogado", query["nome_advogado"])
    fill_if_present(page, "documento", query["documento"])
    fill_if_present(page, "numero_oab", query["numero_oab"])

    if query["uf_oab"]:
        select_oab_uf(page, query["uf_oab"])

    search_button = page.ele(SELECTORS["pesquisar"], timeout=5)
    if not search_button:
        raise RuntimeError("Botao Pesquisar nao encontrado.")

    try:
        if page.handle_alert(timeout=1):
            log("[PJe] Alerta previo detectado e fechado.", quiet)
    except Exception:
        pass

    log("[PJe] Disparando pesquisa...", quiet)
    search_button.click()


def write_json(path: Path, payload: dict) -> None:
    path.write_text(json.dumps(payload, ensure_ascii=True, indent=2), encoding="utf-8")


def save_artifacts(court_name: str, html: str, active_query: dict, result: dict) -> dict:
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    label = "__".join(f"{key}-{normalize_filename(str(value))}" for key, value in active_query.items())
    html_path = OUTPUT_DIR / f"{court_name.lower()}_{label}.html"
    json_path = OUTPUT_DIR / f"{court_name.lower()}_{label}.json"
    latest_html_path = OUTPUT_DIR / "tjpb_heroi_result.html"
    latest_json_path = OUTPUT_DIR / "tjpb_heroi_result.json"

    html_path.write_text(html, encoding="utf-8")
    latest_html_path.write_text(html, encoding="utf-8")

    result["artifacts"] = {
        "html": str(html_path),
        "json": str(json_path),
        "latest_html": str(latest_html_path),
        "latest_json": str(latest_json_path),
    }

    write_json(json_path, result)
    write_json(latest_json_path, result)
    return result


def save_json_only_artifacts(court_name: str, active_query: dict, result: dict) -> dict:
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    label = "__".join(f"{key}-{normalize_filename(str(value))}" for key, value in active_query.items())
    json_path = OUTPUT_DIR / f"{court_name.lower()}_{label}.json"
    latest_json_path = OUTPUT_DIR / "tjpb_heroi_result.json"

    result["artifacts"] = {
        "html": None,
        "json": str(json_path),
        "latest_html": None,
        "latest_json": str(latest_json_path),
    }

    write_json(json_path, result)
    write_json(latest_json_path, result)
    return result


def save_best_effort_artifacts(court_name: str, page, active_query: dict, result: dict) -> dict:
    if page is None:
        return save_json_only_artifacts(court_name, active_query, result)
    try:
        html = page.html
    except Exception:
        return save_json_only_artifacts(court_name, active_query, result)
    if html:
        return save_artifacts(court_name, html, active_query, result)
    return save_json_only_artifacts(court_name, active_query, result)


def _worker(config: dict, result_path: str) -> None:
    """
    Ghost Engine v2 Worker.

    Camadas ativas por sessão:
      L1 — IPv6 Synthetic Rotator (BLAKE2b + alias macOS)  [se disponível]
      L2 — Camoufox Firefox C++ patched                    [se disponível]
      L3 — Hardware Datalake v2 (M1→M4 Max, rotação LCG)
    """
    court_name = config["court_name"]
    court_url  = config["court_url"]
    sitekey    = config["sitekey"]

    result = {
        "status": "ERROR",
        "message": "Falha nao classificada",
        "query": config["active_query"],
        "token_injected": False,
        "headless": config["headless"],
        "wait_seconds": config["wait_seconds"],
        "stage": "boot",
        "court": court_name,
        "ghost_engine": {
            "camoufox": CAMOUFOX_AVAILABLE,
            "hardware_datalake": HARDWARE_DATALAKE_AVAILABLE,
            "native_solver": NATIVE_SOLVER_AVAILABLE,
        },
    }

    browser = None
    page    = None
    ghost_ip = None
    try:
        # ── L3: Hardware Datalake v2 ──────────────────────────────────────
        hw_fingerprint = None
        firefox_prefs  = {}
        if HARDWARE_DATALAKE_AVAILABLE:
            result["stage"] = "hardware_profile"
            hw_fingerprint = generate_hardware_fingerprint(engine="gecko")
            firefox_prefs  = get_gecko_firefox_prefs()
            result["ghost_engine"]["hardware_profile"] = hw_fingerprint.model_name
            result["ghost_engine"]["hardware_id"]      = hw_fingerprint.profile_id
            log(f"[GhostEngine] Perfil: {hw_fingerprint.model_name} "
                f"({hw_fingerprint.hardware.chip}, {hw_fingerprint.hardware.ram_gb}GB)",
                config["quiet"])

        # ── Proxy residencial Bright Data (IP do Brasil) — se configurado ──
        bd_proxy = None
        _ph = os.environ.get("BRIGHTDATA_PROXY_HOST")
        if _ph:
            _srv = _ph if _ph.startswith("http") else "http://" + _ph
            bd_proxy = {"server": _srv}
            if os.environ.get("BRIGHTDATA_PROXY_USER"):
                _user = os.environ["BRIGHTDATA_PROXY_USER"]
                _country = os.environ.get("BRIGHTDATA_PROXY_COUNTRY", "br")
                if _country and "-country-" not in _user:
                    _user = f"{_user}-country-{_country}"  # força IP do país (Brasil)
                bd_proxy["username"] = _user
                bd_proxy["password"] = os.environ.get("BRIGHTDATA_PROXY_PASS", "")
            result["ghost_engine"]["brightdata_proxy"] = True
            log("[GhostEngine] Proxy residencial Bright Data ATIVO (IP residencial).", config["quiet"])

        # ── L2: Camoufox (Firefox C++ patched) ───────────────────────────
        result["stage"] = "launch_browser"

        if CAMOUFOX_AVAILABLE:
            log(f"[GhostEngine] Iniciando Camoufox (Firefox patched C++)...",
                config["quiet"])

            launch_kwargs = {
                "headless": config["headless"],
                "os": "macos",
                # geoip=True requer camoufox[geoip] com DB baixado separado
                # Usamos locale="pt-BR" + timezone manualmente
            }
            if bd_proxy:
                launch_kwargs["proxy"] = bd_proxy

            # A tela e outros aspectos de hardware já são injetados em nível de Javascript
            # Omitimos screen do Camoufox para evitar conflitos com o browserforge interno

            with Camoufox(**launch_kwargs) as cam_browser:
                ctx = cam_browser.new_context(
                    locale="pt-BR",
                    timezone_id="America/Sao_Paulo",
                    ignore_https_errors=True,
                )

                page_pw = ctx.new_page()

                # Executa a sessão de scraping
                result = _run_playwright_session(
                    page_pw, ctx, court_name, court_url, sitekey,
                    config, result
                )

        else:
            # ── Fallback: Playwright Firefox puro (sem Camoufox) ──────────
            log("[GhostEngine] Camoufox nao disponivel. Usando Playwright Firefox.",
                config["quiet"])
            from playwright.sync_api import sync_playwright

            with sync_playwright() as p:
                # Playwright puro nao tem display virtual; "virtual" vira headless.
                pw_headless = True if config["headless"] == "virtual" else bool(config["headless"])
                launch_kwargs = {
                    "headless": pw_headless,
                    "firefox_user_prefs": firefox_prefs or {
                        "media.peerconnection.enabled": False,
                        "dom.webdriver.enabled": False,
                    },
                }
                if bd_proxy:
                    launch_kwargs["proxy"] = bd_proxy
                browser = p.firefox.launch(**launch_kwargs)
                ua = (hw_fingerprint.browser.user_agent
                      if hw_fingerprint
                      else "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:130.0) Gecko/20100101 Firefox/130.0")

                ctx_kwargs = {"user_agent": ua, "locale": "pt-BR", "timezone_id": "America/Sao_Paulo", "ignore_https_errors": True}
                if hw_fingerprint:
                    ctx_kwargs["viewport"] = {
                        "width":  hw_fingerprint.hardware.screen_width  // 2,
                        "height": hw_fingerprint.hardware.screen_height // 2,
                    }

                ctx = browser.new_context(**ctx_kwargs)
                if hw_fingerprint:
                    ctx.add_init_script(BASE_STEALTH_SCRIPT)
                    ctx.add_init_script(hw_fingerprint.to_js_init_script())

                page_pw = ctx.new_page()
                result = _run_playwright_session(
                    page_pw, ctx, court_name, court_url, sitekey,
                    config, result
                )
                browser.close()

    except Exception as exc:
        result["status"]  = "ERROR"
        result["message"] = str(exc)
    finally:
        # Cleanup IPv6 alias
        if ghost_ip:
            ghost_ip.teardown_session()
        write_json(Path(result_path), result)


def _run_playwright_session(
    page, ctx, court_name: str, court_url: str,
    sitekey: str | None, config: dict, result: dict
) -> dict:
    """
    Executa a sessão de scraping usando a API do Playwright.
    Compatível com Camoufox e Playwright Firefox puro.
    Mantém toda a lógica original de negócio (formulário, captcha, resultados).
    """
    try:
        result["stage"] = "open_page"
        log(f"[{court_name}] Navegando para {court_url}...", config["quiet"])
        page.goto(court_url, timeout=60000, wait_until="domcontentloaded")

        result["stage"] = "inject_token"
        result["token_injected"] = _inject_token_playwright(
            page, court_url, sitekey, config["quiet"]
        )

        if not result["token_injected"] and not config["headless"]:
            result["stage"] = "manual_challenge"
            _wait_for_manual_challenge_pw(page, config["manual_challenge_seconds"],
                                          court_name, config["quiet"])
        elif not result["token_injected"] and config["headless"]:
            if NATIVE_SOLVER_AVAILABLE:
                log(f"[{court_name}] Acionando Roses Native Solver (Biometria CDP)...", config["quiet"])
                solver = NativeTurnstileSolver(page, config["quiet"])
                solved = solver.solve(timeout_sec=config["manual_challenge_seconds"])
                if solved:
                    result["token_injected"] = True
            if not result.get("token_injected"):
                time.sleep(3)

        result["stage"] = "wait_form"
        if not _wait_for_form_pw(page):
            result["status"]  = "TIMEOUT"
            result["message"] = f"Formulario {court_name} nao carregou (bloqueado por WAF/Captcha)"
        else:
            result["stage"] = "submit_search"
            _run_search_pw(page, config["query"], config["quiet"])

            result["stage"] = "await_results"
            log(f"[{court_name}] Busca disparada. Aguardando sinal de resultados...",
                config["quiet"])
            signal = _wait_for_results_pw(page, config["wait_seconds"], config["quiet"])
            log(f"[{court_name}] Sinal: {signal}", config["quiet"])

            result["stage"] = "extract"
            html = page.content()
            if signal == "OK" and PARSER_AVAILABLE:
                processos, total_reported, pages = _collect_all_results_pw(
                    page, config.get("max_pages", 200), config["wait_seconds"], config["quiet"]
                )
                result["status"]  = "OK"
                result["message"] = (
                    f"{len(processos)} processo(s) extraido(s) em {pages} pagina(s)."
                )
                result["processos"]      = processos
                result["total"]          = len(processos)
                result["total_reported"] = total_reported
                result["pages_collected"] = pages
                html = page.content()  # ultima pagina, para o artifact de debug
            elif signal in ("NOT_FOUND", "INVALID"):
                result["status"]  = signal
                result["message"] = (
                    "Nenhum resultado encontrado" if signal == "NOT_FOUND"
                    else "Numero de processo invalido"
                )
                result["processos"] = []
                result["total"]     = 0
            else:
                # Fallback: classificacao pelo HTML cru (challenge/unknown).
                status, message = detect_status(html, court_name)
                result["status"]  = status
                result["message"] = message

            result = save_artifacts(court_name, html, config["active_query"], result)

    except Exception as exc:
        if "artifacts" not in result:
            try:
                html = page.content()
                result = save_artifacts(court_name, html, config["active_query"], result)
            except Exception:
                result = save_json_only_artifacts(court_name, config["active_query"], result)
        
        # Se falhou com timeout, a mensagem será mantida. Caso contrário, registra a exception.
        if result.get("message") == "Falha nao classificada" or "message" not in result:
            result["message"] = str(exc)
        
        if result.get("status") not in ["TIMEOUT", "INVALID", "NOT_FOUND"]:
            result["status"] = "ERROR"
            
    return result


# ── Playwright wrappers (espelham a API DrissionPage original) ────────────

def _inject_token_playwright(page, url: str, sitekey: str | None, quiet: bool) -> bool:
    # A API Sidecar original foi removida. Usaremos o NativeSolver (Biométrico)
    return False


def _wait_for_form_pw(page) -> bool:
    for _ in range(60):
        try:
            el = page.query_selector('input[name*="searchProcessos"]')
            if el:
                return True
        except Exception:
            pass
        time.sleep(1)
        
    # Timeout hit. Take screenshot and HTML for debug.
    try:
        
        ts = int(time.time())
        page.screenshot(path=f"timeout_form_{ts}.png")
        with open(f"timeout_form_{ts}.html", "w", encoding="utf-8") as f:
            f.write(page.content())
    except Exception:
        pass
        
    return False


def _wait_for_manual_challenge_pw(page, seconds: int, court_name: str,
                                   quiet: bool) -> None:
    if seconds <= 0:
        return
    log(f"[{court_name}] Desafio detectado. Resolva manualmente em {seconds}s.", quiet)
    deadline = time.time() + seconds
    # Wait up to 5 seconds for the challenge to appear before assuming it's absent
    challenge_appeared = False
    
    while time.time() < deadline:
        has_challenge = False
        has_challenge = bool(
            page.evaluate("""
                Boolean(
                    document.querySelector('.cf-turnstile')
                    || document.querySelector('.g-recaptcha')
                    || document.querySelector('[name="cf-turnstile-response"]')
                )
            """)
        )
        
        if not has_challenge:
            for frame in page.frames:
                if frame.url and ("turnstile" in frame.url.lower() or "recaptcha" in frame.url.lower()):
                    has_challenge = True
                    break
                    
        if has_challenge:
            challenge_appeared = True
            
        if not has_challenge:
            if challenge_appeared or (time.time() > (deadline - seconds + 5)):
                log(f"[{court_name}] Desafio resolvido ou ausente. Prosseguindo.", quiet)
                return
        time.sleep(1)
    log(f"[{court_name}] Tempo de resolucao manual expirou. Continuando...", quiet)


PW_SELECTORS = {
    "cnj":              'input[name*="inputNumeroProcesso"]',
    "cnj_digito":       'input[name*="inputDigitoProcesso"]',
    "cnj_ano":          'input[name*="inputAnoProcesso"]',
    "cnj_origem":       'input[name*="inputOrigemProcesso"]',
    "processo_referencia": 'input[name*="processoReferenciaInput"]',
    "nome_parte":       'input[name*="nomeParte"]',
    "nome_advogado":    'input[name*="nomeAdv"]',
    "documento":        'input[name*="documentoParte"]',
    "numero_oab":       'input[name*="numeroOAB"]',
    "uf_oab":           'select[name*="estadoComboOAB"]',
    "pesquisar":        'input[name*="searchProcessos"]',
}


def _fill_pw(page, selector, value: str) -> bool:
    """
    Preenche um campo tolerando variacao de layout: aceita um seletor unico
    ou uma LISTA de candidatos, tentando cada um ate funcionar.
    Retorna True se conseguiu preencher.
    """
    if not value:
        return False
    candidates = selector if isinstance(selector, (list, tuple)) else [selector]
    for sel in candidates:
        try:
            el = page.query_selector(sel)
            if el:
                try:
                    el.fill("")
                except Exception:
                    pass
                el.fill(value)
                return True
        except Exception:
            continue
    return False


# Caminhos alternativos por campo (resiliencia a mudanca de id/name do JSF).
PW_FALLBACKS = {
    "numero_oab":    ['input[name*="numeroOAB"]', 'input[id*="numeroOAB"]', 'input[name*="OAB"]'],
    "nome_advogado": ['input[name*="nomeAdv"]', 'input[id*="nomeAdv"]', 'input[name*="Advogado"]'],
    "nome_parte":    ['input[name*="nomeParte"]', 'input[id*="nomeParte"]'],
    "documento":     ['input[name*="documentoParte"]', 'input[id*="documentoParte"]'],
    "pesquisar":     ['input[name*="searchProcessos"]', 'input[id*="searchProcessos"]',
                      'input[type="submit"][value*="Pesquisar"]', 'button[type="submit"]'],
}


def _run_search_pw(page, query: dict, quiet: bool = False):
    # Documento tipo (CPF/CNPJ radio)
    if query.get("documento_tipo"):
        radio_idx = 0 if query["documento_tipo"] == "CPF" else 1
        radios = page.query_selector_all('input[name*="tipoMascaraDocumento"]')
        if len(radios) > radio_idx:
            radios[radio_idx].click()

    # CNJ (split em campos separados)
    cnj = query.get("cnj")
    if cnj:
        digits = re.sub(r"\D", "", cnj)
        if len(digits) >= 20:
            _fill_pw(page, PW_SELECTORS["cnj"],        digits[0:7])
            _fill_pw(page, PW_SELECTORS["cnj_digito"], digits[7:9])
            _fill_pw(page, PW_SELECTORS["cnj_ano"],    digits[9:13])
            _fill_pw(page, PW_SELECTORS["cnj_origem"], digits[16:20])
        else:
            _fill_pw(page, PW_SELECTORS["cnj"], cnj)

    _fill_pw(page, PW_SELECTORS["processo_referencia"],     query.get("processo_referencia"))
    _fill_pw(page, PW_FALLBACKS["nome_parte"],              query.get("nome_parte"))
    _fill_pw(page, PW_FALLBACKS["nome_advogado"],           query.get("nome_advogado"))
    _fill_pw(page, PW_FALLBACKS["documento"],               query.get("documento"))
    oab_ok = _fill_pw(page, PW_FALLBACKS["numero_oab"],     query.get("numero_oab"))
    if query.get("numero_oab") and not oab_ok:
        log("[PJe] AVISO: campo numero da OAB nao localizado no formulario.", quiet)

    # UF OAB (select) — tenta API do Playwright e, se falhar, JS por texto/valor.
    uf = query.get("uf_oab")
    if uf:
        if not _select_uf_oab_pw(page, uf):
            log(f"[PJe] AVISO: UF da OAB '{uf}' nao selecionada.", quiet)

    # Clica pesquisar (com fallback de seletor).
    log("[PJe] Disparando pesquisa...", quiet)
    search_btn = None
    for sel in PW_FALLBACKS["pesquisar"]:
        search_btn = page.query_selector(sel)
        if search_btn:
            break
    if not search_btn:
        raise RuntimeError("Botao Pesquisar nao encontrado.")
    search_btn.click()


def _select_uf_oab_pw(page, uf: str) -> bool:
    """Seleciona a UF da OAB de forma resiliente (label, value e JS)."""
    uf = uf.strip().upper()
    for sel in (PW_SELECTORS["uf_oab"], 'select[name*="estadoComboOAB"]', 'select[name*="OAB"]'):
        try:
            page.select_option(sel, label=uf)
            return True
        except Exception:
            try:
                page.select_option(sel, value=uf)
                return True
            except Exception:
                continue
    # Fallback JS: casa por texto ou valor e dispara o evento change.
    try:
        return bool(page.evaluate(
            """
            (wanted) => {
              const sels = document.querySelectorAll('select[name*="estadoComboOAB"], select[name*="OAB"]');
              for (const select of sels) {
                const opt = Array.from(select.options).find(o =>
                  (o.value || '').toUpperCase() === wanted ||
                  (o.textContent || '').trim().toUpperCase() === wanted);
                if (opt) { select.value = opt.value;
                  select.dispatchEvent(new Event('change', {bubbles:true})); return true; }
              }
              return false;
            }
            """, uf))
    except Exception:
        return False


# ── Espera, paginacao e extracao de resultados ────────────────────────────

def _results_signal_pw(page) -> str | None:
    """
    Retorna um sinal deterministico do estado da pagina apos a busca:
      'OK'        -> tabela de resultados presente
      'NOT_FOUND' -> portal informou ausencia de resultados
      'INVALID'   -> numero de processo invalido
      'CHALLENGE' -> desafio de seguranca presente
    None se ainda indefinido (continuar aguardando).
    """
    try:
        return page.evaluate(
            """
            () => {
              const txt = (document.body.innerText || '').toLowerCase();
              const hasRow = !!document.querySelector('tr.rich-table-row, [id*="processosTable"] tr');
              const hasResultCount = /resultados? encontrados?/.test(txt);
              if (hasRow || hasResultCount) return 'OK';
              if (txt.includes('nenhum resultado encontrado') ||
                  txt.includes('não foram encontrados') ||
                  txt.includes('nenhum registro')) return 'NOT_FOUND';
              if (txt.includes('número de processo inválido') ||
                  txt.includes('processo inválido')) return 'INVALID';
              if (document.querySelector('.cf-turnstile, .g-recaptcha, [name="cf-turnstile-response"]'))
                return 'CHALLENGE';
              return null;
            }
            """
        )
    except Exception:
        return None


def _wait_for_results_pw(page, timeout: int, quiet: bool = False) -> str:
    """Aguarda ate a pagina dar um sinal claro (OK/NOT_FOUND/INVALID/CHALLENGE)."""
    deadline = time.time() + max(timeout, 1)
    last = None
    while time.time() < deadline:
        sig = _results_signal_pw(page)
        if sig in ("OK", "NOT_FOUND", "INVALID"):
            return sig
        last = sig or last
        time.sleep(1)
    return last or "UNKNOWN"


def _first_numero_pw(page) -> str:
    """Primeiro numero CNJ visivel na tabela (para detectar troca de pagina)."""
    try:
        return page.evaluate(
            r"""
            () => {
              const m = (document.body.innerText || '')
                .match(/\d{7}-\d{2}\.\d{4}\.\d\.\d{2}\.\d{4}/);
              return m ? m[0] : '';
            }
            """
        ) or ""
    except Exception:
        return ""


def _go_next_page_pw(page, quiet: bool = False) -> bool:
    """
    Tenta avancar para a proxima pagina do datascroller do PJe.
    Usa varias heuristicas (seta », titulos 'proxima'/'next', controles
    rich-datascr) para tolerar variacoes de layout. Retorna True se clicou.
    """
    try:
        clicked = page.evaluate(
            """
            () => {
              const norm = s => (s || '').trim().toLowerCase();
              // candidatos: controles do datascroller e links/botoes de navegacao
              const nodes = Array.from(document.querySelectorAll(
                '.rich-datascr-button, .rich-datascr-act, td[onclick], a[onclick], a, span'
              ));
              const isNext = el => {
                const t = norm(el.textContent);
                const title = norm(el.getAttribute && el.getAttribute('title'));
                const rel = norm(el.getAttribute && el.getAttribute('rel'));
                if (t === '»' || t === '>' || t === '›') return true;
                if (title.includes('próxim') || title.includes('proxim') ||
                    title.includes('next') || title.includes('avanç')) return true;
                if (rel === 'next') return true;
                return false;
              };
              // evita o "ultima pagina" (»») clicando so no proximo simples
              const isLast = el => {
                const t = norm(el.textContent);
                const title = norm(el.getAttribute && el.getAttribute('title'));
                return t === '»»' || t === '>>' || title.includes('últim') || title.includes('ultim');
              };
              const target = nodes.find(el => isNext(el) && !isLast(el));
              if (!target) return false;
              if (target.getAttribute && (target.getAttribute('disabled') !== null ||
                  norm(target.className).includes('dsabled') ||
                  norm(target.className).includes('disabled'))) return false;
              target.click();
              return true;
            }
            """
        )
        return bool(clicked)
    except Exception:
        return False


def _collect_all_results_pw(page, max_pages: int, wait_seconds: int, quiet: bool = False):
    """
    Coleta TODOS os processos, percorrendo todas as paginas (sincronizacao
    OAB -> todos os processos vinculados). Dedupe por numero.
    Retorna (processos: list[dict], total_reported: int|None, pages: int).
    """
    if not PARSER_AVAILABLE:
        return [], None, 0

    collected: dict[str, dict] = {}
    total_reported = None
    pages = 0

    for _ in range(max(max_pages, 1)):
        html = page.content()
        parsed = parse_results_html(html)
        if total_reported is None:
            total_reported = parsed.get("total_reported")
        for proc in parsed.get("processos", []):
            collected.setdefault(proc["numero"], proc)
        pages += 1
        log(f"[PJe] Pagina {pages}: +{len(parsed.get('processos', []))} "
            f"(acumulado {len(collected)}/{total_reported or '?'})", quiet)

        # ja coletou tudo que o portal reportou?
        if total_reported is not None and len(collected) >= total_reported:
            break

        prev_first = _first_numero_pw(page)
        if not _go_next_page_pw(page, quiet):
            break

        # aguarda a tabela trocar de conteudo
        changed = False
        deadline = time.time() + max(wait_seconds, 3)
        while time.time() < deadline:
            time.sleep(1)
            if _first_numero_pw(page) and _first_numero_pw(page) != prev_first:
                changed = True
                break
        if not changed:
            break

    return list(collected.values()), total_reported, pages


def run_unified():
    args = parse_args()
    query, active_query = build_query(args)
    
    # Dynamically resolve court
    court_name, court_url, sitekey = resolve_court(query)

    if not args.json_only:
        print(f"\n[INICIANDO BUSCA UNIFICADA - TRIBUNAL: {court_name}]")
        print(f"[PJe] URL Alvo: {court_url}")
        print(f"[PJe] Criterios: {active_query}")

    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    temp_result = OUTPUT_DIR / f"_pje_tmp_{int(time.time() * 1000)}.json"
    worker_config = {
        "query": query,
        "active_query": active_query,
        "court_name": court_name,
        "court_url": court_url,
        "sitekey": sitekey,
        "wait_seconds": args.wait_seconds,
        "headless": resolve_headless(args.headed),
        "manual_challenge_seconds": args.manual_challenge_seconds,
        "max_pages": args.max_pages,
        "port": args.port,
        "quiet": args.quiet,
    }

    try:
        _worker(worker_config, str(temp_result))
    except Exception as e:
        log(f"Erro no worker principal: {e}", args.quiet)
    
    if temp_result.exists():
        result = json.loads(temp_result.read_text(encoding="utf-8"))
        try:
            temp_result.unlink()
        except Exception:
            pass
    else:
        result = {
            "status": "ERROR",
            "message": "Worker finalizou sem produzir resultado (deadlock ou erro silencioso)",
            "query": active_query,
            "token_injected": False,
            "headless": not args.headed,
            "wait_seconds": args.wait_seconds,
            "stage": "missing_result",
            "court": court_name,
        }
        result = save_json_only_artifacts(court_name, active_query, result)

    return result


if __name__ == "__main__":
    multiprocessing.freeze_support()
    payload = run_unified()
    print(json.dumps(payload, ensure_ascii=True))
    sys.exit(0 if payload.get("status") in {"OK", "NOT_FOUND", "INVALID", "CHALLENGE", "UNKNOWN"} else 1)
