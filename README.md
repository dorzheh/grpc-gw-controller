gRPC Gateway Controller
===========================
gRPC Gateway Controller performs functions to manage the lifecycle of micro-services as well as manage Kubernetes cluster.Controller enables its adapters to present both a HTTP 1.1 REST/JSON API and an efficient gRPC interface on a single TCP port.This provides developers with compatibility with the REST web ecosystem, while advancing a new, high-efficiency RPC protocol.

gRPC gateway is the core component of the gRPC Gateway Controller. It helps to provide APIs in both gRPC and RESTful style at the same time.It reads gRPC service definition and generates a reverse-proxy server which translates a RESTful JSON API into gRPC.

Overview
===========

![Image1](https://github.com/dorzheh/grpc-gw-controller/blob/master/arch/grpc-gw-controller.png)

Extending Controller functionality
=====================================

 - Create a new directory (api/v1/<your_api>) 
 - Create a new file <protofile_name>.proto
 - Compile the proto file
 - Create a new gRPC server (app/grpc/<your_grpc_server)




