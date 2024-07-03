FROM golang:1.22-alpine

WORKDIR /app

COPY ./go.mod ./go.sum ./

ENV CGO_ENABLED=1

RUN apk add --no-cache gcc musl-dev
RUN go mod download

COPY ./src .

EXPOSE 8080

CMD ["go", "run", "."]
