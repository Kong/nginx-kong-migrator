FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-X main.Version=${VERSION}" -o /migrator .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /migrator /migrator
EXPOSE 8080
ENTRYPOINT ["/migrator"]
