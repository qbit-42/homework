FROM golang:1.15 as build-env
#RUN go get github.com/go-delve/delve/cmd/dlv
COPY . /dockerenv
WORKDIR /dockerenv
#RUN go build -gcflags="all=-N -l" -o /homework
RUN go build -o /homework

# Final stage
FROM ubuntu:latest
#EXPOSE 3000 40000
EXPOSE 3000
WORKDIR /
#COPY --from=build-env /go/bin/dlv /
COPY --from=build-env /homework /
#CMD ["/dlv", "--listen=:40000", "--headless=true", "--api-version=2", "--accept-multiclient", "exec", "/homework"]