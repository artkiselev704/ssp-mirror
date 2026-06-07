FROM    golang:1.26 AS build

WORKDIR /build

COPY    ./src/app.go ./

RUN     CGO_ENABLED=0 GOOS=linux go build -o app app.go

FROM    gcr.io/distroless/static:nonroot

WORKDIR /app

COPY    ./src/config.json ./
COPY    --from=build /build/app ./

CMD     ["/app/app"]

EXPOSE  443/tcp
