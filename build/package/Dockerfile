FROM golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/monitor ./cmd/monitor

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/monitor /monitor

EXPOSE 8080

ENTRYPOINT ["/monitor"]
