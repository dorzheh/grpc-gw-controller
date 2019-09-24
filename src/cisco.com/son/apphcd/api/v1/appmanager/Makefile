# Author  <dorzheho@cisco.com>

all:
	protoc -I/usr/local/include -I. \
		-I${GOPATH}/src \
		-I${GOPATH}/src/cisco.com/son/apphcd/third_party/googleapis \
                -I${GOPATH}/src/cisco.com/son/apphcd/third_party/options \
                -I${GOPATH}/src/cisco.com/son/apphcd/vendor \
		--go_out=plugins=grpc:. \
                --validate_out="lang=go:." \
		appmanager.proto
	protoc -I/usr/local/include -I. \
		-I${GOPATH}/src \
                -I${GOPATH}/src/cisco.com/son/apphcd/third_party/googleapis \
                 -I${GOPATH}/src/cisco.com/son/apphcd/third_party/options \
                -I${GOPATH}/src/cisco.com/son/apphcd/vendor \
		--grpc-gateway_out=logtostderr=true,allow_delete_body=true:.\
                --validate_out="lang=go:." \
		appmanager.proto
	protoc -I/usr/local/include -I. \
		-I${GOPATH}/src \
                -I${GOPATH}/src/cisco.com/son/apphcd/third_party/googleapis \
                 -I${GOPATH}/src/cisco.com/son/apphcd/third_party/options \
                -I${GOPATH}/src/cisco.com/son/apphcd/vendor \
		--swagger_out=logtostderr=true,allow_delete_body=true:.\
                --validate_out="lang=go:." \
		appmanager.proto
	go generate .
	sed  -e '/description/ s#`#\\"#g' swagger.pb.go > swagger.pb.go.1
	mv swagger.pb.go.1 swagger.pb.go
