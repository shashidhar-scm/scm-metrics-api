FROM golang:1.22-alpine AS builder

WORKDIR /src

ARG TARGETOS
ARG TARGETARCH

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -o /out/metrics-api ./

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/metrics-api /app/metrics-api
COPY --from=builder /src/migrations /app/migrations

ENV PORT=8080
ENV DEBUG=1

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/app/metrics-api"]
