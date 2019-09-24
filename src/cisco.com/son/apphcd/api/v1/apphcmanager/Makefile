# Copyright © 2019 Cisco Systems Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Author  <dorzheho@cisco.com>

all:
	protoc -I/usr/local/include -I. \
		-I${GOPATH}/src \
		-I${GOPATH}/src/cisco.com/son/apphcd/third_party/googleapis \
                -I${GOPATH}/src/cisco.com/son/apphcd/third_party/options \
                -I${GOPATH}/src/cisco.com/son/apphcd/vendor \
		--go_out=plugins=grpc:. \
                --validate_out="lang=go:." \
		apphcmanager.proto
	protoc -I/usr/local/include -I. \
		-I${GOPATH}/src \
                -I${GOPATH}/src/cisco.com/son/apphcd/third_party/googleapis \
                 -I${GOPATH}/src/cisco.com/son/apphcd/third_party/options \
                -I${GOPATH}/src/cisco.com/son/apphcd/vendor \
		--grpc-gateway_out=logtostderr=true,allow_delete_body=true:.\
                --validate_out="lang=go:." \
		apphcmanager.proto
	protoc -I/usr/local/include -I. \
		-I${GOPATH}/src \
                -I${GOPATH}/src/cisco.com/son/apphcd/third_party/googleapis \
                 -I${GOPATH}/src/cisco.com/son/apphcd/third_party/options \
                -I${GOPATH}/src/cisco.com/son/apphcd/vendor \
		--swagger_out=logtostderr=true,allow_delete_body=true:.\
                --validate_out="lang=go:." \
		apphcmanager.proto
	go generate .
	sed  -e '/description/ s#`#\\"#g' swagger.pb.go > swagger.pb.go.1
	mv swagger.pb.go.1 swagger.pb.go
