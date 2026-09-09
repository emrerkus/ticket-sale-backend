# --- Go build (api + worker + migrate tek imajda) ---
FROM golang:1.27-alpine AS build
WORKDIR /src

# Once bagimliliklar (katman onbellegi icin)
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/api      ./cmd/api  && \
    CGO_ENABLED=0 go build -o /out/worker   ./cmd/worker && \
    CGO_ENABLED=0 go build -o /out/migrate  ./cmd/migrate && \
    CGO_ENABLED=0 go build -o /out/seed     ./cmd/seed

# --- Calisma imaji (minik) ---
FROM alpine:3.20
RUN adduser -D -u 10001 app
COPY --from=build /out/ /usr/local/bin/
USER app
EXPOSE 8080
# Varsayilan komut api; docker-compose worker servisi bunu 'worker' ile ezer.
ENTRYPOINT ["api"]
