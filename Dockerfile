FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /planner-elt

# Multi-stage build
FROM alpine:latest

ARG USERNAME=planner
ARG USER_UID=1000
ARG USER_GID=${USER_UID}

RUN addgroup -g ${USER_GID} ${USERNAME} \
    && adduser -u ${USER_UID} -G ${USERNAME} -D ${USERNAME}

COPY --from=builder /planner-elt /planner-elt

USER ${USERNAME}
CMD ["/planner-elt"]
