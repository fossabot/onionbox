FROM golang:1.11.5
COPY . /onionbox
WORKDIR /onionbox
RUN go get github.com/cespare/reflex
RUN go get -u -a -v -x github.com/ipsn/go-libtor
EXPOSE 80
ENTRYPOINT ["reflex", "-c", "reflex.conf"]