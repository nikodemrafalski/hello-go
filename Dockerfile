FROM golang:alpine as build

WORKDIR /build
RUN apk update && apk upgrade && \
    apk add --no-cache bash git openssh
RUN go get -u github.com/gin-gonic/gin

COPY . .

RUN go build -o main .
WORKDIR /dist
RUN cp /build/main .

FROM alpine
COPY --from=build /dist/main /
ENV GIN_MODE=release
ENV PORT=80
EXPOSE 80
ENTRYPOINT ["/main"]