# Multi-stage build for Bifrost Semantic Router Service
FROM python:3.12-slim AS builder

WORKDIR /app

# Install uv (fast Python package installer)
RUN pip install --no-cache-dir uv -i https://pypi.tuna.tsinghua.edu.cn/simple

# Copy dependency and build files
COPY pyproject.toml ./
COPY uv.lock* ./
COPY README.md ./
COPY src/ ./src/

# Install dependencies and the project itself
# Explicitly avoid CUDA and clean up cache after install
RUN uv sync --frozen --no-dev && \
    uv cache clean

# Final stage
FROM python:3.12-slim

WORKDIR /app

# Only copy necessary files from builder
# We copy the virtualenv and the source code
COPY --from=builder /app/.venv /app/.venv
COPY --from=builder /app/src /app/src

# Set environment variables
# Ensure we use the virtualenv's Python
ENV PATH="/app/.venv/bin:$PATH"
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
ENV HF_ENDPOINT="https://hf-mirror.com"

# Expose port
EXPOSE 8001

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD python -c "import requests; requests.get('http://localhost:8001/health', timeout=5)"

# Run the application via the entry point defined in pyproject.toml
CMD ["server"]
