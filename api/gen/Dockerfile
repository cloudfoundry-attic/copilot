FROM golang:1.11.0-stretch

# Install protobuf
RUN wget https://github.com/protocolbuffers/protobuf/releases/download/v3.6.1/protoc-3.6.1-linux-x86_64.zip
RUN apt-get update && apt-get install unzip
RUN unzip protoc-3.6.1-linux-x86_64.zip
RUN mv bin/protoc /usr/local/bin

# Install protoc-gen-go
RUN go get github.com/golang/protobuf/protoc-gen-go
RUN git -C /go/src/github.com/golang/protobuf/protoc-gen-go checkout v1.2.0
RUN go install github.com/golang/protobuf/protoc-gen-go

COPY ./bin/generate-protos /tmp/build/generate-protos

ENTRYPOINT ["/tmp/build/generate-protos"]

