FROM golang:1.27.1 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN mkdir -p /app/bin \
    && CGO_ENABLED=0 go build -o /app/bin/attributor ./cmd/attributor \
    && if [ -d cmd/migrate ]; then CGO_ENABLED=0 go build -o /app/bin/migrate ./cmd/migrate; fi \
    && for dir in config resources templates; do \
        if [ -d "$dir" ]; then cp -r "$dir" /app/; fi; \
    done

FROM gcr.io/distroless/static-debian13:nonroot
WORKDIR /app
COPY --from=build /app/ /app/
EXPOSE 8080
CMD ["/app/bin/attributor"]
