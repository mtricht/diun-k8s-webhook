FROM ghcr.io/astral-sh/uv:alpine

WORKDIR /app

COPY . .
RUN uv sync

EXPOSE 8080

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8080"]