FROM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY product-service.go ./
RUN CGO_ENABLED=0 go build -o /product-service product-service.go

FROM alpine:3.22
RUN adduser -D app
USER app
COPY --from=build /product-service /product-service
EXPOSE 8080
ENTRYPOINT ["/product-service"]
