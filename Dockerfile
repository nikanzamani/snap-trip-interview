FROM golang:latest
WORKDIR /app
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
ENV REDIS_HOST=rdb REDIS_PORT=6379 POSTGRES_HOST=pdb POSTGRES_PORT=5432 
RUN go build
CMD [ "./snap-trip-interview" ]