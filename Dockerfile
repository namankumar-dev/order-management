FROM golang:1.22-alpine AS build
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/order-api .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/order-api /order-api
USER nonroot:nonroot
EXPOSE 8080 9090
ENTRYPOINT ["/order-api"]
