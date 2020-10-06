# Accept the Go version for the image to be set as a build argument.
ARG GO_VERSION=1.15-alpine

FROM golang:${GO_VERSION}

ENV BIN /usr/local/bin/publiccode-validator

WORKDIR /go/src

COPY ./src/ .
COPY .git/ .

RUN apk add git

RUN go get -d

RUN go build -ldflags \
    "-X main.version=$(git describe --abbrev=0 --tags) -X main.date=$(date +%Y-%m-%d)" \
    -o $BIN \
    && chmod +x $BIN

CMD ["sh","-c","${BIN}"]
