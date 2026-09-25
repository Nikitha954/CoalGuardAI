# ---- Build stage ----
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download || true
COPY . .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o coal-governance-backend .

# ---- Runtime stage ----
FROM alpine:3.19
WORKDIR /app
RUN mkdir -p /app/uploads
COPY --from=builder /app/coal-governance-backend .
EXPOSE 8080
CMD ["./coal-governance-backend"]
