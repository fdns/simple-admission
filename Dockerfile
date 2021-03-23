FROM golang:1.16.2 as builder

WORKDIR $GOPATH/src/github.com/fdns/simple-admission
COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .


#RUN go get -d -v

RUN CGO_ENABLED=0 go build -o /go/bin/simple-admission

FROM scratch
COPY --from=builder /go/bin/simple-admission /go/bin/simple-admission
ENTRYPOINT ["/go/bin/simple-admission"]
