FROM golang:1.14-alpine

ADD . /awsignal
WORKDIR /awsignal

RUN GOFLAGS='-mod=vendor' go build -tags=jsoniter -o /bin/main main.go

FROM alpine

COPY --from=0 /bin/main /main
USER nobody
ENTRYPOINT ["/main"]
CMD [""]
