FROM golang:1.25-trixie
WORKDIR /usr/src/app
COPY go.mod go.sum main.go ./
RUN go mod download && go build -o /usr/local/bin/collector ./main.go
ENV GODEBUG=multipathtcp=0
CMD ["collector"]
