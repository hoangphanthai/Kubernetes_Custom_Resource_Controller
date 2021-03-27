FROM golang:1.15-alpine
RUN mkdir /app
ADD . /app
WORKDIR /app/src/acontroller
RUN go clean --modcache
RUN go build -o main
CMD ["/app/src/acontroller/main"]