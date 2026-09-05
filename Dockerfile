# Builds both Go binaries; each service picks its own via the start command
# (api: /bin/api, worker: /bin/worker) defined in .railway/railway.ts.

FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -o /out/worker ./cmd/worker

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
 && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/api /bin/api
COPY --from=build /out/worker /bin/worker
CMD ["/bin/api"]
