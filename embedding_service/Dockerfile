# Multi-stage build for Bifrost Semantic Router Service
FROM python:3.12-slim AS builder

WORKDIR /app

# Install uv (fast Python package installer)
RUN pip install --no-cache-dir uv

# Copy dependency and build files
COPY pyproject.toml ./
COPY uv.lock* ./
COPY README.md ./
# Need src to build the package properly
COPY src/ ./src/

# Install dependencies and the project itself
# This populates .venv with everything needed
RUN uv sync --frozen --no-dev

# Final stage
FROM python:3.12-slim

WORKDIR /app

# Copy virtual environment AND source files from builder
# (Since uv sync default uses editable mode in development context, 
#  we keep src to ensure the package remains functional)
COPY --from=builder /app/.venv /app/.venv
COPY --from=builder /app/src /app/src

# Set environment variables
ENV PATH="/app/.venv/bin:$PATH"
ENV PYTHONUNBUFFERED=1
ENV HF_ENDPOINT="https://hf-mirror.com"

# Expose port
EXPOSE 8001

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD python -c "import requests; requests.get('http://localhost:8001/health', timeout=5)"

# Run the application via the entry point defined in pyproject.toml
CMD ["server"]
