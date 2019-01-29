FROM golang:1.11.5
COPY . /onionbox
WORKDIR /onionbox
#WORKDIR /onionbox/cmd
RUN go get github.com/cespare/reflex
RUN go get -u -a -v -x github.com/ipsn/go-libtor
#RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -gcflags=-m -a -tags netgo -ldflags '-w -extldflags "-static"' -o onionbox .
#RUN go test -v -race -bench -cpu=1,2,4 ./...
#FROM scratch
#COPY --from=builder /onionbox/cmd/onionbox .
COPY reflex.conf .
EXPOSE 80
ENTRYPOINT ["reflex", "-c", "reflex.conf"]
#CMD ["./onionbox", "-debug"]