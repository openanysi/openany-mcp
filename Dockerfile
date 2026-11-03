FROM golang:1.27.3-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/openany-mcp ./cmd/openany-mcp
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/openany-mcp /app/openany-mcp
COPY config /app/config
ENV MCP_LISTEN_ADDR=0.0.0.0:3333
EXPOSE 3333
ENTRYPOINT ["/app/openany-mcp"]
