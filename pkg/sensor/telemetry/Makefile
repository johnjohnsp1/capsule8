PROTOC_GEN_GO=$$GOPATH/bin/protoc-gen-go
GRPC_OUT=.
VENDOR=../../../vendor
API=$(VENDOR)

INC=-I. -I$(VENDOR)/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis/ \
	-I$(VENDOR)/github.com/

.PHONY: telemetry
telemetry: *.proto
        # Compile grpc and gateway stubs
	protoc --plugin=protoc-gen-go=$(PROTOC_GEN_GO) \
		--go_out=plugins=grpc:$(GRPC_OUT) \
		$(INC) \
		$?
