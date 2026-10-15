# Build stage
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

ARG TARGETOS TARGETARCH

WORKDIR /app
COPY go.mod ./
COPY . .

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o main .

# Fonts stage: fetch the custom fonts so curl/unzip never reach the final image
FROM alpine:3.21 AS fonts

RUN apk add --no-cache curl unzip

# Inter font (https://rsms.me/inter/)
RUN mkdir -p /fonts/inter \
    && curl -sSL -o /tmp/inter.zip https://github.com/rsms/inter/releases/download/v4.0/Inter-4.0.zip \
    && unzip -j /tmp/inter.zip 'extras/ttf/*.ttf' -d /fonts/inter

# Twitter Color Emoji SVGinOT font
RUN mkdir -p /fonts/twemoji \
    && curl -sSL -o /fonts/twemoji/TwitterColorEmoji-SVGinOT.ttf \
        https://github.com/eosrei/twemoji-color-font/releases/download/v14.0.2/TwitterColorEmoji-SVGinOT.ttf

# Final stage
FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        chromium \
        ca-certificates \
        tini \
        fontconfig \
        fonts-liberation \
        fonts-dejavu-core \
        fonts-noto-core \
        fonts-noto-cjk \
        fonts-noto-color-emoji \
    && rm -rf /var/lib/apt/lists/*

COPY --from=fonts /fonts /usr/share/fonts/truetype
RUN fc-cache -f

WORKDIR /app
COPY --from=builder /app/main .

ENV APP_PORT=:8080
ENV CHROME_BIN=/usr/bin/chromium

EXPOSE 8080

ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["./main"]
