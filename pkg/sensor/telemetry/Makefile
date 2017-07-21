PROTOC_GEN_GO=$$GOPATH/bin/protoc-gen-go
GRPC_OUT=.
VENDOR=../../../vendor
API=../../../api/v0

INC=-I. -I$(VENDOR)/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis/ \
	-I$$GOPATH/src \
	-I$(API)

.PHONY: telemetry
telemetry: *.proto
        # Compile grpc and gateway stubs
	protoc --plugin=protoc-gen-go=$(PROTOC_GEN_GO) \
		--go_out=plugins=grpc:$(GRPC_OUT) \
		$(INC) \
		$?
