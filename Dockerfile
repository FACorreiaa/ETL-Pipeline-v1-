FROM golang:1.23.5 AS builder

LABEL maintainer="fc"
LABEL description="Base image score-app dev"

WORKDIR /app

ENV GOOS=linux
ENV GOARCH=amd64

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN echo "Contents of /app after COPY:" && ls -al /app && sleep 1
RUN echo "Listing contents of /app after COPY:" && ls -al /app && sleep 1
RUN echo "Contents of /app/config:" && ls -al /app/config || echo "/app/config does not exist" && sleep 1
RUN ls -al /app/internal || echo "No /app/internal found" && sleep 1
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/score-app -ldflags="-w -s" ./*.go
ENTRYPOINT ["/app/score-app"]

# Development stage with hot reload
FROM golang:1.23-alpine AS dev
WORKDIR /app
RUN go install github.com/air-verse/air@latest
COPY go.mod go.sum ./
RUN go mod download
COPY . .
EXPOSE 8080
CMD ["air"]

FROM alpine:latest AS prod
WORKDIR /app

RUN apk add --no-cache bash
COPY --from=builder /app/score-app /usr/bin/score-app
#COPY --from=builder /app/config /app/config

RUN echo "Contents of /app after COPY:" && ls -al /app && sleep 1
RUN echo "Listing contents of /app after COPY:" && ls -al /app && sleep 1
RUN echo "Contents of /app/config:" && ls -al /app/config || echo "/app/config does not exist" && sleep 1
RUN echo "Contents of /app/config during build:" && ls -al /app/config || echo "/app/config does not exist" && sleep 10
RUN ls -al /app/internal || echo "No /app/internal found" && sleep 1

EXPOSE 8000
EXPOSE 8181
CMD ["score-app"]
