FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
ARG TARGETOS=linux
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-w -s" -o vtc-service .

FROM scratch
COPY --from=builder /app/vtc-service /vtc-service
EXPOSE 8080
CMD ["/vtc-service"]
