"""
roses/engine/hardware_datalake.py  — v2.0

Gerador de perfis de hardware falsos estilo Hackintosh/SMBIOS.
Suporta Apple Silicon M1 → M5 + Mac Pro + Mac Studio + MacBook Air.

Algoritmo de rotação: LCG (Linear Congruential Generator) com período completo.
Garantia matemática: nunca repete o mesmo perfil antes de esgotar todos os outros.

Teorema de Hull-Dobell (condições para período completo):
  X_{n+1} = (a * X_n + c) mod m
  1. c e m são coprimos (gcd(c,m) = 1) → satisfeito com c=1
  2. a-1 é divisível por todos os fatores primos de m
  3. Se m divisível por 4, então a-1 também deve ser divisível por 4
"""

import hashlib
import json
import random
import secrets
import time
import uuid
from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Optional


# --------------------------------------------------------------------------
# Modelos de dados
# --------------------------------------------------------------------------

@dataclass
class SystemProfile:
    platform: str
    os_name: str
    bios_vendor: str
    bios_version: str
    smbios_uuid: str
    serial_number: str
    model_identifier: str
    board_id: str


@dataclass
class HardwareProfile:
    chip: str          # "M4 Pro", "M2 Ultra", etc.
    cpu_brand: str
    cpu_cores: int
    cpu_perf_cores: int
    cpu_eff_cores: int
    ram_gb: int
    gpu_vendor: str
    gpu_renderer: str
    gpu_cores: int
    screen_width: int
    screen_height: int
    color_depth: int
    pixel_ratio: float


@dataclass
class NetworkProfile:
    mac_address: str
    timezone: str
    locale: str


@dataclass
class BrowserProfile:
    user_agent: str
    accept_language: str


@dataclass
class HardwareFingerprint:
    profile_id: str
    model_name: str
    system: SystemProfile
    hardware: HardwareProfile
    network: NetworkProfile
    browser: BrowserProfile

    def to_dict(self) -> dict:
        return asdict(self)

    def to_js_init_script(self) -> str:
        """
        Script JS injetado via page.addInitScript() ANTES de qualquer
        recurso do site. Sobrescreve APIs nativas com dados do perfil.
        Camoufox já faz a maioria disso em C++, mas este script adiciona
        camadas extras de consistência.
        """
        hw = self.hardware
        sys_ = self.system
        net = self.network

        return f"""
(function() {{
    // Hardware concurrency (CPU cores visíveis ao JS)
    Object.defineProperty(navigator, 'hardwareConcurrency', {{
        get: () => {hw.cpu_cores}, configurable: false
    }});

    // Device memory (GB — arredondado para potência de 2)
    Object.defineProperty(navigator, 'deviceMemory', {{
        get: () => {min(hw.ram_gb, 8)}, configurable: false
    }});

    // Platform
    Object.defineProperty(navigator, 'platform', {{
        get: () => '{sys_.platform}', configurable: false
    }});

    // Screen
    Object.defineProperty(screen, 'width', {{ get: () => {hw.screen_width} }});
    Object.defineProperty(screen, 'height', {{ get: () => {hw.screen_height} }});
    Object.defineProperty(screen, 'availWidth', {{ get: () => {hw.screen_width} }});
    Object.defineProperty(screen, 'availHeight', {{ get: () => {hw.screen_height - 25} }});
    Object.defineProperty(screen, 'colorDepth', {{ get: () => {hw.color_depth} }});
    Object.defineProperty(window, 'devicePixelRatio', {{ get: () => {hw.pixel_ratio} }});

    // WebGL — Renderer spoofing (redundante com Camoufox mas por segurança)
    const _getParam = WebGLRenderingContext.prototype.getParameter;
    WebGLRenderingContext.prototype.getParameter = function(p) {{
        if (p === 37445) return '{hw.gpu_vendor}';
        if (p === 37446) return '{hw.gpu_renderer}';
        return _getParam.apply(this, [p]);
    }};
    if (typeof WebGL2RenderingContext !== 'undefined') {{
        const _getParam2 = WebGL2RenderingContext.prototype.getParameter;
        WebGL2RenderingContext.prototype.getParameter = function(p) {{
            if (p === 37445) return '{hw.gpu_vendor}';
            if (p === 37446) return '{hw.gpu_renderer}';
            return _getParam2.apply(this, [p]);
        }};
    }}

    // Languages
    Object.defineProperty(navigator, 'languages', {{
        get: () => ['{net.locale}', '{net.locale.split("-")[0]}', 'en-US', 'en']
    }});
}})();
"""


# --------------------------------------------------------------------------
# Apple Silicon — catálogo completo M1 → M5
# --------------------------------------------------------------------------

APPLE_SILICON_MODELS = [
    # ─── M1 ─────────────────────────────────────────────────────────────────
    {
        "chip": "M1", "model_name": "MacBook Air (M1, 2020)",
        "model_id": "MacBookAir10,1", "board_id": "Mac-63001698E7A34814",
        "cpu_brand": "Apple M1", "cpu_cores": 8, "perf": 4, "eff": 4,
        "ram": [8, 16], "gpu_vendor": "Apple", "gpu_renderer": "Apple M1",
        "gpu_cores": 7, "screen": (2560, 1600), "px": 2.0, "depth": 24,
        "platform": "MacIntel", "bios_vendor": "Apple Inc.",
        "bios_versions": ["1554.140.15.0.0", "1554.140.16.0.0"],
        "oui": ["3c:22:fb", "a4:c3:f0"],
    },
    {
        "chip": "M1", "model_name": "MacBook Pro 13\" (M1, 2020)",
        "model_id": "MacBookPro17,1", "board_id": "Mac-BE088AF8C5EB4FA2",
        "cpu_brand": "Apple M1", "cpu_cores": 8, "perf": 4, "eff": 4,
        "ram": [8, 16], "gpu_vendor": "Apple", "gpu_renderer": "Apple M1",
        "gpu_cores": 8, "screen": (2560, 1600), "px": 2.0, "depth": 24,
        "platform": "MacIntel", "bios_vendor": "Apple Inc.",
        "bios_versions": ["1715.80.3.0.0"],
        "oui": ["8c:85:90", "ac:bc:32"],
    },
    {
        "chip": "M1 Pro", "model_name": "MacBook Pro 14\" (M1 Pro, 2021)",
        "model_id": "MacBookPro18,3", "board_id": "Mac-CFF7D910A743CAAF",
        "cpu_brand": "Apple M1 Pro", "cpu_cores": 10, "perf": 8, "eff": 2,
        "ram": [16, 32], "gpu_vendor": "Apple", "gpu_renderer": "Apple M1 Pro",
        "gpu_cores": 16, "screen": (3024, 1964), "px": 2.0, "depth": 30,
        "platform": "MacIntel", "bios_vendor": "Apple Inc.",
        "bios_versions": ["1715.80.3.0.0", "1715.80.4.0.0"],
        "oui": ["f0:18:98", "14:98:77"],
    },
    {
        "chip": "M1 Max", "model_name": "MacBook Pro 16\" (M1 Max, 2021)",
        "model_id": "MacBookPro18,2", "board_id": "Mac-A5C67F76ED83108C",
        "cpu_brand": "Apple M1 Max", "cpu_cores": 10, "perf": 8, "eff": 2,
        "ram": [32, 64], "gpu_vendor": "Apple", "gpu_renderer": "Apple M1 Max",
        "gpu_cores": 32, "screen": (3456, 2234), "px": 2.0, "depth": 30,
        "platform": "MacIntel", "bios_vendor": "Apple Inc.",
        "bios_versions": ["1715.80.3.0.0"],
        "oui": ["b8:f6:b1", "3c:22:fb"],
    },
    {
        "chip": "M1 Ultra", "model_name": "Mac Studio (M1 Ultra, 2022)",
        "model_id": "Mac13,2", "board_id": "Mac-AA6D100E4E55F28C",
        "cpu_brand": "Apple M1 Ultra", "cpu_cores": 20, "perf": 16, "eff": 4,
        "ram": [64, 128], "gpu_vendor": "Apple", "gpu_renderer": "Apple M1 Ultra",
        "gpu_cores": 64, "screen": (5120, 2880), "px": 2.0, "depth": 30,
        "platform": "MacIntel", "bios_vendor": "Apple Inc.",
        "bios_versions": ["2069.0.0.0.0"],
        "oui": ["00:cd:fe", "ac:bc:32"],
    },
    # ─── M2 ─────────────────────────────────────────────────────────────────
    {
        "chip": "M2", "model_name": "MacBook Air 13\" (M2, 2022)",
        "model_id": "Mac14,2", "board_id": "Mac-52DA4728DB4C9B57",
        "cpu_brand": "Apple M2", "cpu_cores": 8, "perf": 4, "eff": 4,
        "ram": [8, 16, 24], "gpu_vendor": "Apple", "gpu_renderer": "Apple M2",
        "gpu_cores": 10, "screen": (2560, 1664), "px": 2.0, "depth": 24,
        "platform": "MacIntel", "bios_vendor": "Apple Inc.",
        "bios_versions": ["7459.141.2.0.0"],
        "oui": ["a4:c3:f0", "8c:85:90"],
    },
    {
        "chip": "M2 Pro", "model_name": "MacBook Pro 14\" (M2 Pro, 2023)",
        "model_id": "Mac14,9", "board_id": "Mac-4B682C642B45593E",
        "cpu_brand": "Apple M2 Pro", "cpu_cores": 12, "perf": 8, "eff": 4,
        "ram": [16, 32], "gpu_vendor": "Apple", "gpu_renderer": "Apple M2 Pro",
        "gpu_cores": 19, "screen": (3024, 1964), "px": 2.0, "depth": 30,
        "platform": "MacIntel", "bios_vendor": "Apple Inc.",
        "bios_versions": ["7459.141.2.0.0", "7459.141.3.0.0"],
        "oui": ["f0:18:98", "3c:22:fb"],
    },
    {
        "chip": "M2 Max", "model_name": "MacBook Pro 16\" (M2 Max, 2023)",
        "model_id": "Mac14,6", "board_id": "Mac-7FM33B4G7CD6D536",
        "cpu_brand": "Apple M2 Max", "cpu_cores": 12, "perf": 8, "eff": 4,
        "ram": [32, 96], "gpu_vendor": "Apple", "gpu_renderer": "Apple M2 Max",
        "gpu_cores": 38, "screen": (3456, 2234), "px": 2.0, "depth": 30,
        "platform": "MacIntel", "bios_vendor": "Apple Inc.",
        "bios_versions": ["7459.141.2.0.0"],
        "oui": ["b8:f6:b1", "14:98:77"],
    },
    {
        "chip": "M2 Ultra", "model_name": "Mac Pro (M2 Ultra, 2023)",
        "model_id": "Mac14,8", "board_id": "Mac-4B682C642B45593E",
        "cpu_brand": "Apple M2 Ultra", "cpu_cores": 24, "perf": 16, "eff": 8,
        "ram": [192, 384], "gpu_vendor": "Apple", "gpu_renderer": "Apple M2 Ultra",
        "gpu_cores": 76, "screen": (5120, 2880), "px": 2.0, "depth": 30,
        "platform": "MacIntel", "bios_vendor": "Apple Inc.",
        "bios_versions": ["7459.141.2.0.0"],
        "oui": ["00:cd:fe", "ac:bc:32"],
    },
    # ─── M3 ─────────────────────────────────────────────────────────────────
    {
        "chip": "M3", "model_name": "MacBook Pro 14\" (M3, 2023)",
        "model_id": "Mac15,3", "board_id": "Mac-CFC2C8F01FFFCB52",
        "cpu_brand": "Apple M3", "cpu_cores": 8, "perf": 4, "eff": 4,
        "ram": [8, 16, 24], "gpu_vendor": "Apple", "gpu_renderer": "Apple M3",
        "gpu_cores": 10, "screen": (3024, 1964), "px": 2.0, "depth": 30,
        "platform": "MacIntel", "bios_vendor": "Apple Inc.",
        "bios_versions": ["10151.140.19.0.0"],
        "oui": ["a4:c3:f0", "f0:18:98"],
    },
    {
        "chip": "M3 Pro", "model_name": "MacBook Pro 16\" (M3 Pro, 2023)",
        "model_id": "Mac15,7", "board_id": "Mac-B6D54C9BFAA58B24",
        "cpu_brand": "Apple M3 Pro", "cpu_cores": 12, "perf": 6, "eff": 6,
        "ram": [18, 36], "gpu_vendor": "Apple", "gpu_renderer": "Apple M3 Pro",
        "gpu_cores": 18, "screen": (3456, 2234), "px": 2.0, "depth": 30,
        "platform": "MacIntel", "bios_vendor": "Apple Inc.",
        "bios_versions": ["10151.140.19.0.0"],
        "oui": ["3c:22:fb", "b8:f6:b1"],
    },
    {
        "chip": "M3 Max", "model_name": "MacBook Pro 16\" (M3 Max, 2023)",
        "model_id": "Mac15,9", "board_id": "Mac-C4536DBBE63B8D21",
        "cpu_brand": "Apple M3 Max", "cpu_cores": 16, "perf": 12, "eff": 4,
        "ram": [36, 128], "gpu_vendor": "Apple", "gpu_renderer": "Apple M3 Max",
        "gpu_cores": 40, "screen": (3456, 2234), "px": 2.0, "depth": 30,
        "platform": "MacIntel", "bios_vendor": "Apple Inc.",
        "bios_versions": ["10151.140.19.0.0"],
        "oui": ["14:98:77", "f0:18:98"],
    },
    # ─── M4 ─────────────────────────────────────────────────────────────────
    {
        "chip": "M4", "model_name": "MacBook Pro 14\" (M4, 2024)",
        "model_id": "Mac16,1", "board_id": "Mac-CFC2C8F01FFFCB52",
        "cpu_brand": "Apple M4", "cpu_cores": 10, "perf": 4, "eff": 6,
        "ram": [16, 24, 32], "gpu_vendor": "Apple", "gpu_renderer": "Apple M4",
        "gpu_cores": 10, "screen": (3024, 1964), "px": 2.0, "depth": 30,
        "platform": "MacIntel", "bios_vendor": "Apple Inc.",
        "bios_versions": ["11881.141.1.0.0"],
        "oui": ["a4:c3:f0", "3c:22:fb"],
    },
    {
        "chip": "M4 Pro", "model_name": "MacBook Pro 14\" (M4 Pro, 2024)",
        "model_id": "Mac16,6", "board_id": "Mac-4B682C642B45593E",
        "cpu_brand": "Apple M4 Pro", "cpu_cores": 14, "perf": 10, "eff": 4,
        "ram": [24, 48], "gpu_vendor": "Apple", "gpu_renderer": "Apple M4 Pro",
        "gpu_cores": 20, "screen": (3024, 1964), "px": 2.0, "depth": 30,
        "platform": "MacIntel", "bios_vendor": "Apple Inc.",
        "bios_versions": ["11881.141.1.0.0", "11881.141.2.0.0"],
        "oui": ["f0:18:98", "b8:f6:b1"],
    },
    {
        "chip": "M4 Pro", "model_name": "MacBook Pro 16\" (M4 Pro, 2024)",
        "model_id": "Mac16,7", "board_id": "Mac-7FM33B4G7CD6D536",
        "cpu_brand": "Apple M4 Pro", "cpu_cores": 14, "perf": 10, "eff": 4,
        "ram": [24, 48], "gpu_vendor": "Apple", "gpu_renderer": "Apple M4 Pro",
        "gpu_cores": 24, "screen": (3456, 2234), "px": 2.0, "depth": 30,
        "platform": "MacIntel", "bios_vendor": "Apple Inc.",
        "bios_versions": ["11881.141.1.0.0"],
        "oui": ["3c:22:fb", "14:98:77"],
    },
    {
        "chip": "M4 Max", "model_name": "MacBook Pro 16\" (M4 Max, 2024)",
        "model_id": "Mac16,8", "board_id": "Mac-C4536DBBE63B8D21",
        "cpu_brand": "Apple M4 Max", "cpu_cores": 16, "perf": 12, "eff": 4,
        "ram": [36, 128], "gpu_vendor": "Apple", "gpu_renderer": "Apple M4 Max",
        "gpu_cores": 40, "screen": (3456, 2234), "px": 2.0, "depth": 30,
        "platform": "MacIntel", "bios_vendor": "Apple Inc.",
        "bios_versions": ["11881.141.1.0.0"],
        "oui": ["b8:f6:b1", "a4:c3:f0"],
    },
    # ─── MacBook Air M2/M3 ──────────────────────────────────────────────────
    {
        "chip": "M2", "model_name": "MacBook Air 15\" (M2, 2023)",
        "model_id": "Mac14,15", "board_id": "Mac-52DA4728DB4C9B57",
        "cpu_brand": "Apple M2", "cpu_cores": 8, "perf": 4, "eff": 4,
        "ram": [8, 16, 24], "gpu_vendor": "Apple", "gpu_renderer": "Apple M2",
        "gpu_cores": 10, "screen": (2880, 1864), "px": 2.0, "depth": 24,
        "platform": "MacIntel", "bios_vendor": "Apple Inc.",
        "bios_versions": ["7459.141.2.0.0"],
        "oui": ["8c:85:90", "00:cd:fe"],
    },
    {
        "chip": "M3", "model_name": "MacBook Air 13\" (M3, 2024)",
        "model_id": "Mac15,12", "board_id": "Mac-CFC2C8F01FFFCB52",
        "cpu_brand": "Apple M3", "cpu_cores": 8, "perf": 4, "eff": 4,
        "ram": [8, 16, 24], "gpu_vendor": "Apple", "gpu_renderer": "Apple M3",
        "gpu_cores": 10, "screen": (2560, 1664), "px": 2.0, "depth": 24,
        "platform": "MacIntel", "bios_vendor": "Apple Inc.",
        "bios_versions": ["10151.140.19.0.0"],
        "oui": ["f0:18:98", "a4:c3:f0"],
    },
    # ─── Mac Studio M2 ──────────────────────────────────────────────────────
    {
        "chip": "M2 Max", "model_name": "Mac Studio (M2 Max, 2023)",
        "model_id": "Mac14,13", "board_id": "Mac-4B682C642B45593E",
        "cpu_brand": "Apple M2 Max", "cpu_cores": 12, "perf": 8, "eff": 4,
        "ram": [32, 96], "gpu_vendor": "Apple", "gpu_renderer": "Apple M2 Max",
        "gpu_cores": 30, "screen": (5120, 2880), "px": 2.0, "depth": 30,
        "platform": "MacIntel", "bios_vendor": "Apple Inc.",
        "bios_versions": ["7459.141.2.0.0"],
        "oui": ["00:cd:fe", "ac:bc:32"],
    },
]

GECKO_USER_AGENTS = [
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:130.0) Gecko/20100101 Firefox/130.0",
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_5; rv:129.0) Gecko/20100101 Firefox/129.0",
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 14.5; rv:128.0) Gecko/20100101 Firefox/128.0",
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 15_0; rv:130.0) Gecko/20100101 Firefox/130.0",
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 15.1; rv:132.0) Gecko/20100101 Firefox/132.0",
]

APPLE_SERIAL_CHARS = "ABCDEFGHJKLMNPQRSTUVWXYZ0123456789"


# --------------------------------------------------------------------------
# LCG — Linear Congruential Generator com período completo (Hull-Dobell)
# --------------------------------------------------------------------------

class ProfileRotatorLCG:
    """
    Rotador de perfis baseado em LCG com período completo.
    
    Garante que todos os N perfis são visitados antes de qualquer repetição.
    Teorema de Hull-Dobell:
      X_{n+1} = (a * X_{n} + c) mod m
      Período completo ↔ gcd(c, m)=1 ∧ ∀p primo de m: (a-1)%p=0 ∧ (4|m → 4|(a-1))
    """

    STATE_FILE = Path(__file__).parent.parent / "datalake" / "lcg_state.json"

    def __init__(self, n: int):
        self.m = n
        self.a = self._compute_multiplier(n)
        self.c = 1  # gcd(1, n) = 1 sempre ✓
        self.state = self._load_state()

    def _compute_multiplier(self, m: int) -> int:
        """
        Calcula multiplier 'a' que satisfaz Hull-Dobell para m dado.
        Algoritmo: a = (produto de todos fatores primos de m) + 1
        Se m divisível por 4: multiplica por 2 para garantir 4|(a-1)
        """
        if m <= 1:
            return 1
        # Factorização de m
        primes = set()
        n = m
        d = 2
        while d * d <= n:
            while n % d == 0:
                primes.add(d)
                n //= d
            d += 1
        if n > 1:
            primes.add(n)

        # a - 1 deve ser divisível por todos os primos de m
        product = 1
        for p in primes:
            product *= p

        # Se 4 | m, então 4 | (a-1)
        if m % 4 == 0:
            # Garante que product seja par (o que satisfaz 4 | a-1 quando multiplicado por 2 se necessário)
            if product % 2 != 0:
                product *= 2
            if product % 4 != 0:
                product *= 2

        return product + 1  # a = product + 1

    def _load_state(self) -> int:
        try:
            if self.STATE_FILE.exists():
                data = json.loads(self.STATE_FILE.read_text())
                return data.get("state", 0) % self.m
        except Exception:
            pass
        # Estado inicial aleatório para não começar sempre no mesmo lugar
        return secrets.randbelow(self.m)

    def _save_state(self):
        self.STATE_FILE.parent.mkdir(parents=True, exist_ok=True)
        self.STATE_FILE.write_text(json.dumps({
            "state": self.state,
            "m": self.m,
            "a": self.a,
            "c": self.c,
        }))

    def next_index(self) -> int:
        """Retorna o próximo índice na sequência LCG."""
        self.state = (self.a * self.state + self.c) % self.m
        self._save_state()
        return self.state


# --------------------------------------------------------------------------
# Geração de serial number Apple
# --------------------------------------------------------------------------

def _random_serial(chip_prefix: str) -> str:
    """Gera serial Apple plausível baseado no chip."""
    prefix_map = {
        "M1": "C02G", "M1 Pro": "C02H", "M1 Max": "C02J",
        "M1 Ultra": "C07M", "M2": "C02L", "M2 Pro": "C02N",
        "M2 Max": "C02P", "M2 Ultra": "C02Q", "M3": "FVFM",
        "M3 Pro": "FVFN", "M3 Max": "FVFP", "M4": "FVFW",
        "M4 Pro": "FVFX", "M4 Max": "FVFY",
    }
    prefix = prefix_map.get(chip_prefix, "C02G")
    body = "".join(random.choices(APPLE_SERIAL_CHARS, k=8))
    return f"{prefix}{body}"


def _random_mac(oui_prefix: str) -> str:
    suffix = ":".join(f"{random.randint(0, 255):02x}" for _ in range(3))
    return f"{oui_prefix}:{suffix}"


# --------------------------------------------------------------------------
# Gerador principal
# --------------------------------------------------------------------------

_rotator: Optional[ProfileRotatorLCG] = None


def _get_rotator() -> ProfileRotatorLCG:
    global _rotator
    if _rotator is None:
        _rotator = ProfileRotatorLCG(len(APPLE_SILICON_MODELS))
    return _rotator


def generate_hardware_fingerprint(
    engine: str = "gecko",
    profile_type: str = "mac",
    seed: Optional[int] = None,
    use_rotation: bool = True,
) -> HardwareFingerprint:
    """
    Gera um perfil de hardware Apple Silicon único para esta sessão.
    
    Args:
        engine: "gecko" (Firefox/Zen) ou "chromium"
        profile_type: "mac" (Apple Silicon) — futuro: "windows"
        seed: Se definido, usa perfil fixo para testes reproduzíveis
        use_rotation: Se True, usa LCG para garantir não-repetição
    
    Returns:
        HardwareFingerprint completo com JS de injeção
    """
    if seed is not None:
        random.seed(seed)
        model = APPLE_SILICON_MODELS[seed % len(APPLE_SILICON_MODELS)]
    elif use_rotation:
        rotator = _get_rotator()
        idx = rotator.next_index()
        model = APPLE_SILICON_MODELS[idx]
    else:
        model = random.choice(APPLE_SILICON_MODELS)

    # Identificadores únicos por sessão
    smbios_uuid = str(uuid.uuid4()).upper()
    serial = _random_serial(model["chip"])
    oui = random.choice(model["oui"])
    mac_addr = _random_mac(oui)
    bios_ver = random.choice(model["bios_versions"])
    ram = random.choice(model["ram"])

    # User-Agent Firefox compatível com macOS
    ua = random.choice(GECKO_USER_AGENTS)

    # Profile ID — hash dos identificadores únicos desta sessão
    profile_id = hashlib.sha256(
        f"{smbios_uuid}{serial}{mac_addr}".encode()
    ).hexdigest()[:16]

    return HardwareFingerprint(
        profile_id=profile_id,
        model_name=model["model_name"],
        system=SystemProfile(
            platform=model["platform"],
            os_name="macOS 15.1",
            bios_vendor=model["bios_vendor"],
            bios_version=bios_ver,
            smbios_uuid=smbios_uuid,
            serial_number=serial,
            model_identifier=model["model_id"],
            board_id=model["board_id"],
        ),
        hardware=HardwareProfile(
            chip=model["chip"],
            cpu_brand=model["cpu_brand"],
            cpu_cores=model["cpu_cores"],
            cpu_perf_cores=model["perf"],
            cpu_eff_cores=model["eff"],
            ram_gb=ram,
            gpu_vendor=model["gpu_vendor"],
            gpu_renderer=model["gpu_renderer"],
            gpu_cores=model["gpu_cores"],
            screen_width=model["screen"][0],
            screen_height=model["screen"][1],
            color_depth=model["depth"],
            pixel_ratio=model["px"],
        ),
        network=NetworkProfile(
            mac_address=mac_addr,
            timezone="America/Sao_Paulo",
            locale="pt-BR",
        ),
        browser=BrowserProfile(
            user_agent=ua,
            accept_language="pt-BR,pt;q=0.9,en-US;q=0.8,en;q=0.7",
        ),
    )


def save_profile(fp: HardwareFingerprint, output_dir: Path) -> Path:
    output_dir.mkdir(parents=True, exist_ok=True)
    out_path = output_dir / f"profile_{fp.profile_id}.json"
    out_path.write_text(json.dumps(fp.to_dict(), indent=2, ensure_ascii=False))
    return out_path


# Workaround para o f-string com min()
def min(a, b):
    return a if a < b else b


# --------------------------------------------------------------------------
# CLI
# --------------------------------------------------------------------------

if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser(description="🌹 Roses — Hardware Datalake v2")
    parser.add_argument("--engine", default="gecko", choices=["gecko", "chromium"])
    parser.add_argument("--seed", type=int, default=None)
    parser.add_argument("--no-rotation", action="store_true")
    parser.add_argument("--save", action="store_true")
    parser.add_argument("--js", action="store_true")
    parser.add_argument("--list", action="store_true", help="Lista todos os modelos disponíveis")
    args = parser.parse_args()

    if args.list:
        print(f"\n🌹 Roses — {len(APPLE_SILICON_MODELS)} perfis Apple Silicon disponíveis:\n")
        for i, m in enumerate(APPLE_SILICON_MODELS):
            print(f"  [{i:02d}] {m['model_name']} | {m['chip']} | {m['cpu_cores']}C | RAM: {m['ram']} | GPU: {m['gpu_cores']}C")
        import sys; sys.exit(0)

    fp = generate_hardware_fingerprint(
        engine=args.engine,
        seed=args.seed,
        use_rotation=not args.no_rotation,
    )

    print(f"\n🌹 Roses — Hardware Fingerprint Gerado")
    print(f"   Profile ID  : {fp.profile_id}")
    print(f"   Modelo      : {fp.model_name}")
    print(f"   Chip        : {fp.hardware.chip}")
    print(f"   CPU         : {fp.hardware.cpu_brand}")
    print(f"   CPU Cores   : {fp.hardware.cpu_cores} ({fp.hardware.cpu_perf_cores}P+{fp.hardware.cpu_eff_cores}E)")
    print(f"   RAM         : {fp.hardware.ram_gb}GB")
    print(f"   GPU         : {fp.hardware.gpu_renderer} ({fp.hardware.gpu_cores} cores)")
    print(f"   Resolução   : {fp.hardware.screen_width}x{fp.hardware.screen_height} @{fp.hardware.pixel_ratio}x")
    print(f"   MAC         : {fp.network.mac_address}")
    print(f"   UUID BIOS   : {fp.system.smbios_uuid}")
    print(f"   Serial      : {fp.system.serial_number}")
    print(f"   User-Agent  : {fp.browser.user_agent[:70]}...")

    if args.save:
        saved = save_profile(fp, Path("roses/datalake"))
        print(f"\n   💾 Perfil salvo em: {saved}")

    if args.js:
        print(f"\n   📜 Script JS de injeção:\n")
        print(fp.to_js_init_script())
