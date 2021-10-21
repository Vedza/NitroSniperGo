FROM golang:alpine3.14 AS builder

RUN apk add build-base --no-cache

WORKDIR app

COPY . .

RUN apk add --no-cache gcc libc-dev alsa-lib-dev \
	&& go mod download \
	&& go build

FROM alpine:3.14 AS final

WORKDIR app

COPY --from=builder /go/app/NitroSniperGo /app/NitroSniperGo

RUN apk add --no-cache alsa-lib-dev

CMD ["/app/NitroSniperGo"]
