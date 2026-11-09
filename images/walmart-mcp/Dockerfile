# walmart-mcp: read-only Walmart catalog MCP server for Hermes Agent
# (server.py; README.md). Built by .github/workflows/build-walmart-mcp-image.yaml
# for amd64 and arm64.
FROM docker.io/library/python:3.13-slim@sha256:e48f59cdba69a3cbd9ed2cf0f348d90d45a8d350d98134cf1070bc106ee9ed90

RUN groupadd -g 10001 mcp && useradd -u 10001 -g mcp -M -s /usr/sbin/nologin mcp

WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1

COPY requirements.txt .
RUN pip install --no-cache-dir --no-deps -r requirements.txt && pip check

COPY server.py .

# Build check, run the way the container runs it (no credentials: the tools
# then answer with a "no credentials" error instead of failing to import).
RUN python -c "import server"

USER 10001
EXPOSE 8000
ENTRYPOINT ["python", "/app/server.py"]
