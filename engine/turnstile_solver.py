import time
import random
import logging

# NOTA: HumanMousePhysics (curvas de Bezier) existe em turnstile_math.py mas
# NAO e usado aqui de proposito. O Camoufox ja humaniza os eventos de input
# em C++ (isTrusted=true). Reproduzir movimento "humano" por cima disso, via
# page.mouse.move com sleeps no Python, gera um padrao geometrico que o
# Cloudflare deteta e e justamente o que causava o loop de recarregamento.

logger = logging.getLogger(__name__)

class NativeTurnstileSolver:
    """
    Usa um clique nativo (via Camoufox/Playwright) para interagir com o
    Cloudflare Turnstile "Interactive", deixando a humanizacao do input por
    conta dos patches em C++ do Camoufox.
    """
    
    def __init__(self, page, quiet: bool = False):
        self.page = page
        self.quiet = quiet

    def log(self, message: str):
        if not self.quiet:
            print(message)

    def find_challenge_box(self) -> dict:
        """Procura o widget do Turnstile e retorna o BoundingRect via Playwright API."""
        try:
            # Seletores comuns para o Turnstile e Cloudflare Challenge
            selectors = [
                'iframe[src*="turnstile"]',
                'iframe[src*="cloudflare"]',
                '.cf-turnstile-wrapper',
                '#challenge-stage',
                'div.cf-turnstile'
            ]
            iframes_count = len(self.page.frames)
            self.log(f"[TurnstileSolver] {iframes_count} frames encontrados na arvore (Playwright frames).")
            
            for frame in self.page.frames:
                url = frame.url.lower()
                if 'turnstile' in url or 'cloudflare' in url:
                    self.log(f"[TurnstileSolver] Frame alvo encontrado: {url}")
                    # Pegamos o elemento HTML (iframe) que hospeda este frame
                    try:
                        frame_element = frame.frame_element()
                        rect = frame_element.bounding_box()
                        if rect and rect['width'] > 0 and rect['height'] > 0:
                            return {
                                'x': rect['x'],
                                'y': rect['y'],
                                'width': rect['width'],
                                'height': rect['height'],
                                'cx': rect['x'] + (rect['width'] / 2),
                                'cy': rect['y'] + (rect['height'] / 2)
                            }
                    except Exception as e:
                        if "detached" not in str(e).lower():
                            self.log(f"[TurnstileSolver] Erro ao pegar bounding box do frame: {e}")
                        continue
                        
            # Fallback para locators padrão caso falhe pelo frame_element
            for sel in selectors:
                try:
                    loc = self.page.locator(sel)
                    count = loc.count()
                    if count > 0:
                        for i in range(count):
                            l = loc.nth(i)
                            if l.is_visible():
                                rect = l.bounding_box()
                                if rect and rect['width'] > 0 and rect['height'] > 0:
                                    return {
                                        'x': rect['x'],
                                        'y': rect['y'],
                                        'width': rect['width'],
                                        'height': rect['height'],
                                        'cx': rect['x'] + (rect['width'] / 2),
                                        'cy': rect['y'] + (rect['height'] / 2)
                                    }
                except Exception:
                    pass
            return None
        except Exception as e:
            self.log(f"[TurnstileSolver] Erro ao buscar widget: {e}")
            return None

    def execute_human_click(self, target_x: int, target_y: int):
        """
        Delega o clique para o Playwright/Camoufox. Camoufox já patchou
        eventos de input no C++ para parecerem humanos (isTrusted = true).
        Movimentos complexos com sleep no Python causam rejeição.
        """
        self.log(f"[TurnstileSolver] Efetuando mouse click nativo em {target_x}, {target_y}...")
        
        # O Playwright faz hover e mousedown/mouseup suavemente
        self.page.mouse.click(target_x, target_y, delay=random.uniform(50, 150))
        
        return True

    def solve(self, timeout_sec: int = 25) -> bool:
        """
        Espera o Turnstile aparecer e aplica UM clique nativo coerente.

        Estrategia (corrigida):
        - Um unico clique nativo via execute_human_click(), deixando o Camoufox
          humanizar o evento em C++. Sem page.mouse.move(steps=) + sleeps, que
          geram o padrao linear detectavel e causavam o loop de recarregamento.
        - Falha rapida em ate 3 tentativas para nao "queimar" a reputacao do IP.
        - Erros reais de Python sao propagados, nao mascarados como bloqueio do
          Cloudflare.
        """
        start_time = time.time()
        attempts = 0

        self.log("[TurnstileSolver] Observando Cloudflare Turnstile...")
        while time.time() - start_time < timeout_sec:
            rect = self.find_challenge_box()
            if not rect:
                time.sleep(1)
                continue

            self.log(f"[TurnstileSolver] Rect do iframe: {rect}")

            # O Turnstile precisa de tempo para inicializar e validar telemetria.
            time.sleep(random.uniform(2.5, 4.0))

            # Alvo: checkbox fica a esquerda dentro do iframe.
            click_x = int(rect['x'] + 30 + random.uniform(-6, 6))
            click_y = int(rect['cy'] + random.uniform(-6, 6))

            # Clique nativo. Se algo de errado acontecer aqui, e um erro REAL
            # (frame detached, page fechada, etc.) e NAO o Cloudflare bloqueando.
            # Por isso propagamos a excecao em vez de engolir e fingir que foi o
            # site -- esse mascaramento era o que escondia bugs como o
            # 'UnboundLocalError' citado no relatorio.
            self.execute_human_click(click_x, click_y)

            # Espera para ver se o widget some (passou) ou recarrega.
            time.sleep(4)
            gone = True
            for _ in range(3):
                if self.find_challenge_box():
                    gone = False
                    break
                time.sleep(1)

            if gone:
                self.log("[TurnstileSolver] Cloudflare resolvido. ✅")
                return True

            attempts += 1
            if attempts >= 3:
                self.log(
                    "[TurnstileSolver] 3 tentativas sem sucesso. Falha rapida "
                    "para preservar a reputacao do IP (nao adianta insistir)."
                )
                return False

            self.log(f"[TurnstileSolver] Widget recarregou (tentativa {attempts}/3). Nova tentativa...")
            time.sleep(2)

        self.log("[TurnstileSolver] Timeout aguardando resolucao do Turnstile.")
        return False
