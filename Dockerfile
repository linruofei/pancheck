# Multi-stage build Dockerfile

# ====================================================================
# Stage 1: Build React frontend
# ====================================================================
FROM node:18-alpine AS frontend-builder

RUN npm install -g pnpm

WORKDIR /app/frontend

COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY frontend/ ./
RUN pnpm run build

# ====================================================================
# Stage 2: Build Go backend
# ====================================================================
FROM golang:1.24-alpine AS backend-builder

WORKDIR /app/backend

COPY go.mod go.sum ./
RUN go mod download

COPY . .

COPY --from=frontend-builder /app/frontend/dist ./static

RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/api

# ====================================================================
# Stage 3: Final runtime image
# ====================================================================
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=backend-builder /app/backend/main .
COPY --from=backend-builder /app/backend/static ./static

EXPOSE 6080

CMD ["./main"]
