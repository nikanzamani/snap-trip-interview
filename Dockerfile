FROM golang:latest
WORKDIR /app
COPY . .
RUN go mod download
ENV PORT=8080
RUN go build
CMD [ "./snap-trip-interview" ]