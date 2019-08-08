FROM golang:1.12.6

WORKDIR /go/src/pc-web-validator
COPY ./src/ .

RUN go get -d -v ./...
RUN go install -v ./...
RUN go build -o main

CMD ["./main"]