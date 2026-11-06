FROM golang:1.25.1-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/api ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/palsctl ./cmd/palsctl && \
    CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/realtime ./cmd/realtime
FROM gcr.io/distroless/static-debian12:nonroot AS cli
COPY --from=build /out/palsctl /palsctl
ENTRYPOINT ["/palsctl"]
FROM gcr.io/distroless/static-debian12:nonroot AS realtime
COPY --from=build /out/realtime /realtime
EXPOSE 8081
ENTRYPOINT ["/realtime"]
FROM gcr.io/distroless/static-debian12:nonroot AS api
COPY --from=build /out/api /api
EXPOSE 8080
ENTRYPOINT ["/api"]
