FROM python:3.12-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends git && \
    rm -rf /var/lib/apt/lists/*

RUN groupadd -g 1000 indexer && \
    useradd -u 1000 -g indexer -m indexer

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY indexer.py .

USER indexer

ENTRYPOINT ["python", "-u", "/app/indexer.py"]
