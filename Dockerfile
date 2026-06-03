FROM --platform=linux/amd64 golang:1.26 AS builder

WORKDIR /src

RUN go env -w GOPROXY=https://goproxy.cn,direct \
    && go env -w GOSUMDB=sum.golang.google.cn

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/micro-uac-server ./cmd/server

FROM --platform=linux/amd64 debian:bookworm-slim

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends python3 \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /app/.runtime /app/audio /app/scripts

COPY --from=builder /out/micro-uac-server /app/micro-uac-server
COPY config.json /app/config.json
COPY audio/demo.wav /app/audio/demo.wav
COPY scripts/create_call.py /app/scripts/create_call.py

EXPOSE 5061/udp
EXPOSE 8090/tcp

CMD ["/app/micro-uac-server", "-config", "/app/config.json"]
