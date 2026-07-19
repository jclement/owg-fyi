# capsule — gemini + web server for owg.fyi
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG GIT_SHA=""
ARG BUILD_DATE=""
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X github.com/jclement/owg-fyi/internal/content.BuildSHA=${GIT_SHA} -X github.com/jclement/owg-fyi/internal/content.BuildDate=${BUILD_DATE}" \
    -o /out/capsule .

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && adduser -D -H -u 10001 capsule
COPY --from=build /out/capsule /usr/local/bin/capsule
COPY content /srv/content

ENV CAPSULE_CONTENT=/srv/content \
    CAPSULE_DATA=/data

# The binary binds :80/:443/:1965 as root-in-container; fine for a scratch-
# style single-purpose container. /data holds ACME cache + gemini TOFU cert.
VOLUME /data
EXPOSE 80 443 1965

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD ["capsule", "health"]

ENTRYPOINT ["capsule"]
