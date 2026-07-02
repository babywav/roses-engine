"""
roses/engine/stealth_layer.py

Camada de evasão stealth para o motor Roses.
Scripts JS injetados antes do carregamento de qualquer recurso do portal alvo.
Combina técnicas de fingerpint spoofing com os dados do Hardware Datalake.
"""

from typing import Optional


# --------------------------------------------------------------------------
# Script base — aplicado sempre, independente do perfil de hardware
# --------------------------------------------------------------------------

BASE_STEALTH_SCRIPT = """
(function() {
    'use strict';

    // 1. Remove webdriver flag
    try {
        Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
    } catch(e) {}

    // 2. Remove automation-related properties
    try {
        delete window.cdc_adoQpoasnfa76pfcZLmcfl_Array;
        delete window.cdc_adoQpoasnfa76pfcZLmcfl_Promise;
        delete window.cdc_adoQpoasnfa76pfcZLmcfl_Symbol;
        delete window.__nightmare;
        delete window._phantom;
        delete window.callPhantom;
        delete window.__webdriver_evaluate;
        delete window.__selenium_evaluate;
        delete window.__fxdriver_evaluate;
        delete window.__driver_unwrapped;
        delete window.__webdriver_unwrapped;
        delete window.__driver_evaluate;
        delete window.__selenium_unwrapped;
        delete window.__fxdriver_unwrapped;
    } catch(e) {}

    // 3. Fake Chrome runtime (prevents "chrome is not defined" errors in Chrome UA)
    if (navigator.userAgent.indexOf('Chrome') > -1 && !window.chrome) {
        window.chrome = {
            runtime: {
                connect: function() {},
                sendMessage: function() {},
                onMessage: { addListener: function() {} }
            },
            loadTimes: function() { return {}; },
            csi: function() { return {}; }
        };
    }

    // 4. Override toString for native functions to prevent detection
    const originalToString = Function.prototype.toString;
    Function.prototype.toString = function() {
        const result = originalToString.call(this);
        if (result.includes('[native code]')) return result;
        // For patched functions, return native code format
        const fnName = this.name || '';
        if (fnName && (
            fnName.includes('getParameter') ||
            fnName.includes('getOwnProperty')
        )) {
            return `function ${fnName}() { [native code] }`;
        }
        return result;
    };

    // 5. Permissions API spoofing
    if (navigator.permissions) {
        const originalQuery = navigator.permissions.query.bind(navigator.permissions);
        navigator.permissions.query = function(parameters) {
            if (parameters.name === 'notifications') {
                return Promise.resolve({ state: 'prompt', onchange: null });
            }
            return originalQuery(parameters);
        };
    }

    // 6. Plugin spoofing (empty plugins = bot indicator)
    try {
        Object.defineProperty(navigator, 'plugins', {
            get: function() {
                const arr = [
                    { name: 'PDF Viewer', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
                    { name: 'Chrome PDF Viewer', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
                    { name: 'Chromium PDF Viewer', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
                    { name: 'Microsoft Edge PDF Viewer', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
                    { name: 'WebKit built-in PDF', filename: 'internal-pdf-viewer', description: 'Portable Document Format' }
                ];
                arr.__proto__ = PluginArray.prototype;
                return arr;
            }
        });
    } catch(e) {}

    // 7. MimeTypes spoofing
    try {
        Object.defineProperty(navigator, 'mimeTypes', {
            get: function() {
                const arr = [
                    { type: 'application/pdf', description: 'Portable Document Format', suffixes: 'pdf' },
                    { type: 'text/pdf', description: 'Portable Document Format', suffixes: 'pdf' }
                ];
                arr.__proto__ = MimeTypeArray.prototype;
                return arr;
            }
        });
    } catch(e) {}

    // 8. Prevent iframe detection
    try {
        Object.defineProperty(window, 'top', { get: () => window });
        Object.defineProperty(window, 'frameElement', { get: () => null });
    } catch(e) {}

})();
"""


def get_full_stealth_script(hardware_js: Optional[str] = None) -> str:
    """
    Combina o script stealth base com o script de hardware do datalake.
    
    Args:
        hardware_js: Script JS gerado pelo HardwareFingerprint.to_js_init_script()
    
    Returns:
        Script JS combinado pronto para injeção via page.addInitScript()
    """
    parts = [BASE_STEALTH_SCRIPT]
    if hardware_js:
        parts.append(hardware_js)
    return "\n".join(parts)


def get_gecko_firefox_prefs() -> dict:
    """
    Preferências do Firefox para modo stealth máximo.
    Aplica as configurações de privacidade do Zen Browser.
    
    Returns:
        Dict de preferências para usar em firefoxUserPrefs.
    """
    return {
        # Anti-tracking
        "media.peerconnection.enabled": False,          # Bloqueia WebRTC (IP leak)
        "media.peerconnection.ice.default_address_only": True,
        "privacy.trackingprotection.enabled": False,
        "privacy.trackingprotection.pbmode.enabled": False,

        # Esconde automação
        "dom.webdriver.enabled": False,
        "useAutomationExtension": False,

        # Performance / Telemetry
        "toolkit.telemetry.enabled": False,
        "toolkit.telemetry.unified": False,
        "datareporting.healthreport.uploadEnabled": False,
        "datareporting.policy.dataSubmissionEnabled": False,
        "browser.ping-centre.telemetry": False,

        # Fingerprinting (não ativar resistFingerprinting — bloqueia spoofing manual)
        "privacy.resistFingerprinting": False,

        # Network
        "network.http.sendRefererHeader": 2,
        "network.http.referer.XOriginPolicy": 0,
        "network.cookie.cookieBehavior": 0,

        # Canvas
        "canvas.poison.enabled": False,  # Deixamos spoofer manual cuidar disso

        # Segurança
        "security.ssl3.rsa_des_ede3_sha": False,
        "security.tls.version.min": 2,

        # Zona horária Brasil
        "intl.locale.requested": "pt-BR",
    }
