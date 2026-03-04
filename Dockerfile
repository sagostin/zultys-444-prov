FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod main.go ./
RUN go build -o /provisioning-proxy .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /provisioning-proxy /usr/local/bin/provisioning-proxy
EXPOSE 444
ENTRYPOINT ["provisioning-proxy"]
