FROM golang:1.24-alpine AS builder
WORKDIR /app

# 1. Root level go.mod for discovery-agent
COPY discovery-agent/go.mod discovery-agent/go.mod
# We assume discovery-agent is standalone and backend is standalone
# but we need to build both. We'll copy both directories.
COPY discovery-agent/ discovery-agent/
RUN cd discovery-agent && go build -o /app/discovery-agent-bin .

# 2. Build Backend
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
RUN CGO_ENABLED=0 go build -p 1 -o /app/securestream-backend .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/securestream-backend .
COPY --from=builder /app/discovery-agent-bin .
# We place discovery-agent-bin one level up or same level so backend can find it.
# In backend main.go we look for "../discovery-agent-bin", but since working dir is /app,
# if we put it in /app/discovery-agent-bin, we should set env var.
ENV DISCOVERY_AGENT_PATH="/app/discovery-agent-bin"
EXPOSE 8080
CMD ["./securestream-backend"]
