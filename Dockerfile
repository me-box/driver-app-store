FROM golang:1.10.3-alpine3.8 as gobuild
WORKDIR /
ENV GOPATH=/
RUN apk update && apk add pkgconfig build-base bash autoconf git libzmq zeromq-dev
RUN go get -u gopkg.in/src-d/go-git.v4/...
RUN go get -v -d github.com/toshbrown/lib-go-databox
COPY . .
RUN addgroup -S databox && adduser -S -g databox databox
#RUN GGO_ENABLED=0 GOOS=linux go build -a -tags netgo -installsuffix netgo -ldflags '-s -w' -o app /*.go
RUN GGO_ENABLED=0 GOOS=linux go build -a -ldflags '-s -w' -o app /*.go

FROM amd64/alpine:3.8
COPY --from=gobuild /etc/passwd /etc/passwd
RUN apk update && apk add --no-cache libzmq ca-certificates
RUN mkdir -p /app && chown databox /app
USER databox
WORKDIR /app
COPY --from=gobuild /app .
LABEL databox.type="driver"
EXPOSE 8080

CMD ["./app"]
