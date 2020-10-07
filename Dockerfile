FROM golang:1.15 as build-env
WORKDIR /dockerenv
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go get -tags netgo -ldflags '-w -extldflags "-static"' github.com/go-delve/delve/cmd/dlv
RUN env GOOS=linux GARCH=amd64 CGO_ENABLED=0 go build -gcflags="all=-N -l" -v -installsuffix cgo -o /homework

# Final stage
FROM alpine
RUN apk add bash curl
EXPOSE 3000 40000
WORKDIR /
COPY --from=build-env /homework /
COPY --from=build-env /go/bin/dlv /
#CMD ["/homework"]
CMD ["/dlv", "--listen=:40000", "--headless=true", "--api-version=2", "--continue", "--accept-multiclient", "exec", "/homework"]