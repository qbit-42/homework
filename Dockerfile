FROM golang:1.15 as build-env
#RUN go get github.com/go-delve/delve/cmd/dlv
WORKDIR /dockerenv
COPY go.* ./
RUN go mod download
COPY . .
#RUN go build -gcflags="all=-N -l" -o /homework
RUN env GOOS=linux GARCH=amd64 CGO_ENABLED=0 go build -v -installsuffix cgo -o /homework

# Final stage
FROM alpine
RUN apk add bash curl
#EXPOSE 3000 40000
EXPOSE 3000
WORKDIR /
#COPY --from=build-env /go/bin/dlv /
COPY --from=build-env /homework /
#CMD ["/dlv", "--listen=:40000", "--headless=true", "--api-version=2", "--accept-multiclient", "exec", "/homework"]
