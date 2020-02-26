# publiccode.yml web validator for Go

[![Join the #publiccode channel](https://img.shields.io/badge/Slack%20channel-%23publiccode-blue.svg?logo=slack)](https://developersitalia.slack.com/messages/CAM3F785T)
[![Get invited](https://slack.developers.italia.it/badge.svg)](https://slack.developers.italia.it/)

This is a Go web interface validator for [publiccode.yml](https://github.com/italia/publiccode.yml) files, it uses [publiccode-parser-go](https://github.com/italia/publiccode-parser-go).

publiccode.yml is an international standard for describing public software. It is expected to be published in the root of open source repositories. This parser performs syntactic and semantic validation according to the official spec.

## Features
See related project for details: [publiccode-parser-go](https://github.com/italia/publiccode-parser-go)


## Validation from command line

This repository also contains an executable tool which can be used for validating a publiccode.yml file locally.

```sh
$ go run src/main.go
$ curl -XPOST localhost:5000/pc/validate -d '{
  "localisation": {
    "localisationReady": false
  },
  "description": {
    "it": {
      "shortDescription": "test"
    }
  },
  "publiccodeYmlVersion": "0.2"
}'
```
## Docker support

This project can be packaged and executed using Docker as follow:

```sh
$ docker build -t pc-web-validator .
$ docker run -p5000:5000 --name pc-web-validator -it --rm pc-web-validator
```


## Contributing

Contributing is always appreciated.
Feel free to open issues, fork or submit a Pull Request.
If you want to know more about how to add new fields, check out [CONTRIBUTING.md](CONTRIBUTING.md). In order to support other country-specific extensions in addition to Italy some refactoring might be needed.

## See also

* [Developers Italia backend & crawler](https://github.com/italia/developers-italia-backend) - a Go crawler for PC.
* [publiccode-parser-go](https://github.com/italia/publiccode-parser-go) - a Go parser and validator for publiccode.yml files.

## Maintainers

This software is maintained by the [Developers Italia](https://developers.italia.it/) team.

## License

© 2018-2019 Team per la Trasformazione Digitale - Presidenza del Consiglio dei Minstri

Licensed under the EUPL.
The version control system provides attribution for specific lines of code.
