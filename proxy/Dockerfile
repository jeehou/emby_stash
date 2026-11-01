FROM golang:1.24.9-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY third_party ./third_party
RUN go mod download
COPY *.go ./
COPY testdata ./testdata
RUN (cd third_party/metatube && sha256sum -c SHA256SUMS) && \
    go test ./... && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/stash-poster-proxy .
# Ship license texts for all linked modules with the binary.
RUN mkdir -p /out/licenses && \
    cp /src/third_party/metatube/LICENSE /out/licenses/metatube.txt && \
    cp /src/third_party/metatube/PROVENANCE.md /out/licenses/metatube-provenance.md && \
    cp /go/pkg/mod/github.com/disintegration/imaging@*/LICENSE /out/licenses/imaging.txt && \
    cp /go/pkg/mod/github.com/esimov/pigo@*/LICENSE /out/licenses/pigo.txt && \
    cp /go/pkg/mod/golang.org/x/image@*/LICENSE /out/licenses/x-image.txt

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
COPY --from=build /out/stash-poster-proxy /usr/local/bin/stash-poster-proxy
COPY --from=build /out/licenses /usr/share/licenses/stash-poster-proxy
COPY LICENSE /usr/share/licenses/stash-poster-proxy/project.txt
USER 65532:65532
EXPOSE 18080
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD ["/usr/local/bin/stash-poster-proxy", "--healthcheck"]
ENTRYPOINT ["/usr/local/bin/stash-poster-proxy"]
