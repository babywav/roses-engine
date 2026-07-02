# Roses — imagem para VPS (Linux headless)
#
# Roda o Camoufox em DISPLAY VIRTUAL (Xvfb) via ROSES_HEADLESS=virtual,
# que e o modo recomendado em servidor (headless puro e mais detectavel).

FROM python:3.11-slim

ENV DEBIAN_FRONTEND=noninteractive \
    PYTHONUNBUFFERED=1 \
    ROSES_HEADLESS=virtual

# Display virtual + utilitarios. As demais libs do Firefox entram via
# "playwright install --with-deps firefox" abaixo.
RUN apt-get update && apt-get install -y --no-install-recommends \
        xvfb ca-certificates wget \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 1) Dependencias Python
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# 2) Navegadores: Firefox (Playwright, com deps de SO) + Camoufox patched
RUN python -m playwright install --with-deps firefox \
    && python -m camoufox fetch

# 3) Codigo do motor
COPY . /app/roses
WORKDIR /app/roses

# API HTTP (DataJud + portal). Defina ROSES_API_KEY e ROSES_CORS_ORIGINS em producao.
EXPOSE 8001
CMD ["uvicorn", "api.roses_api:app", "--host", "0.0.0.0", "--port", "8001"]
