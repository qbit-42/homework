FROM golang:1.15 as build-env
WORKDIR /dockerenv
COPY go.* ./
RUN go mod download
COPY . .
RUN env GOOS=linux GARCH=amd64 CGO_ENABLED=0 go build -v -installsuffix cgo -o /homework

# Final stage
FROM alpine
RUN apk add bash curl
EXPOSE 3000
WORKDIR /
COPY --from=build-env /go/bin/dlv /
COPY --from=build-env /homework /
CMD ["/homework"]