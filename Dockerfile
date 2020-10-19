FROM golang:1.14 AS builder
ENV CGO_ENABLED 0
WORKDIR /go/src/app
ADD . .
RUN go build -o /template-autoops-cronjob

FROM alpine:3.12
COPY --from=builder /template-autoops-cronjob /template-autoops-cronjob
CMD ["/template-autoops-cronjob"]