package api

//go:generate protoc --go_out=plugins=grpc:.. cloud_controller.proto common.proto bosh_dns_adapter.proto
