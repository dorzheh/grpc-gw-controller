package clustermanager 

const (
swagger = `{
  "swagger": "2.0",
  "info": {
    "title": "clustermanager.proto",
    "version": "version not set"
  },
  "schemes": [
    "http",
    "https"
  ],
  "consumes": [
    "application/json"
  ],
  "produces": [
    "application/json"
  ],
  "paths": {
    "/api/v1/apphoster": {
      "get": {
        "summary": "GetClusterInfo shows appropriate information about AppHoster cluster",
        "operationId": "GetClusterInfo",
        "responses": {
          "200": {
            "description": "A successful response.",
            "schema": {
              "$ref": "#/definitions/clustermanagerResponse"
            }
          }
        },
        "parameters": [
          {
            "name": "hostname",
            "in": "query",
            "required": false,
            "type": "string"
          }
        ],
        "tags": [
          "ClusterManager"
        ]
      },
      "put": {
        "summary": "UpgradeCluster responsible for upgrading AppHoster cluster",
        "operationId": "UpgradeCluster",
        "responses": {
          "200": {
            "description": "A successful response.",
            "schema": {
              "$ref": "#/definitions/clustermanagerResponse"
            }
          }
        },
        "tags": [
          "ClusterManager"
        ]
      }
    },
    "/api/v1/apphoster/nodes": {
      "post": {
        "summary": "CreateNode scales out an AppHoster cluster",
        "operationId": "CreateNode",
        "responses": {
          "200": {
            "description": "A successful response.",
            "schema": {
              "$ref": "#/definitions/clustermanagerResponse"
            }
          }
        },
        "parameters": [
          {
            "name": "body",
            "in": "body",
            "required": true,
            "schema": {
              "$ref": "#/definitions/clustermanagerCreateNodeRequest"
            }
          }
        ],
        "tags": [
          "ClusterManager"
        ]
      }
    },
    "/api/v1/apphoster/nodes/{hostname}": {
      "delete": {
        "summary": "DeleteNode scales in an AppHoster cluster",
        "operationId": "DeleteNode",
        "responses": {
          "200": {
            "description": "A successful response.",
            "schema": {
              "$ref": "#/definitions/clustermanagerResponse"
            }
          }
        },
        "parameters": [
          {
            "name": "hostname",
            "in": "path",
            "required": true,
            "type": "string"
          }
        ],
        "tags": [
          "ClusterManager"
        ]
      }
    },
    "/api/v1/apphoster/nodes/{hostname}/state": {
      "post": {
        "summary": "UpdateNodeState sets AppHoster node to appropriate state",
        "operationId": "UpdateNodeState",
        "responses": {
          "200": {
            "description": "A successful response.",
            "schema": {
              "$ref": "#/definitions/clustermanagerResponse"
            }
          }
        },
        "parameters": [
          {
            "name": "hostname",
            "in": "path",
            "required": true,
            "type": "string"
          },
          {
            "name": "body",
            "in": "body",
            "required": true,
            "schema": {
              "$ref": "#/definitions/clustermanagerUpdateNodeStateRequest"
            }
          }
        ],
        "tags": [
          "ClusterManager"
        ]
      }
    },
    "/api/v1/apphoster/quotas": {
      "get": {
        "summary": "GetClusterResourceQuotas responsible for fetching resource quotas information",
        "operationId": "GetClusterResourceQuotas",
        "responses": {
          "200": {
            "description": "A successful response.",
            "schema": {
              "$ref": "#/definitions/clustermanagerResponse"
            }
          }
        },
        "parameters": [
          {
            "name": "namespaces",
            "in": "query",
            "required": false,
            "type": "array",
            "items": {
              "type": "string"
            }
          }
        ],
        "tags": [
          "ClusterManager"
        ]
      },
      "post": {
        "summary": "SetClusterResourceQuotas responsible for applying resource quotas to appropriate namespaces",
        "operationId": "SetClusterResourceQuotas",
        "responses": {
          "200": {
            "description": "A successful response.",
            "schema": {
              "$ref": "#/definitions/clustermanagerResponse"
            }
          }
        },
        "parameters": [
          {
            "name": "body",
            "in": "body",
            "required": true,
            "schema": {
              "$ref": "#/definitions/clustermanagerSetClusterResourceQuotasRequest"
            }
          }
        ],
        "tags": [
          "ClusterManager"
        ]
      }
    },
    "/api/v1/apphoster/quotas/{namespace}": {
      "delete": {
        "summary": "DeleteClusterResourceQuotas responsible for deleting a resource quota from appropriate namespace",
        "operationId": "DeleteClusterResourceQuotas",
        "responses": {
          "200": {
            "description": "A successful response.",
            "schema": {
              "$ref": "#/definitions/clustermanagerResponse"
            }
          }
        },
        "parameters": [
          {
            "name": "namespace",
            "in": "path",
            "required": true,
            "type": "string"
          }
        ],
        "tags": [
          "ClusterManager"
        ]
      }
    }
  },
  "definitions": {
    "clustermanagerCreateNodeRequest": {
      "type": "object",
      "properties": {
        "hostname": {
          "type": "string"
        },
        "master": {
          "type": "boolean",
          "format": "boolean"
        }
      }
    },
    "clustermanagerGetKubeConfigResponse": {
      "type": "object",
      "properties": {
        "kubeconfig": {
          "type": "string"
        }
      }
    },
    "clustermanagerQuota": {
      "type": "object",
      "properties": {
        "namespace": {
          "type": "string"
        },
        "memory": {
          "type": "integer",
          "format": "int64"
        },
        "cpu": {
          "type": "number",
          "format": "double"
        }
      }
    },
    "clustermanagerResponse": {
      "type": "object",
      "properties": {
        "timestamp": {
          "type": "string",
          "format": "date-time"
        },
        "status": {
          "$ref": "#/definitions/v1clustermanagerStatus"
        },
        "message": {
          "type": "string"
        },
        "body": {
          "$ref": "#/definitions/protobufAny"
        }
      },
      "title": "Response holds information related to a response message that is sent on appropriate request"
    },
    "clustermanagerSetClusterResourceQuotasRequest": {
      "type": "object",
      "properties": {
        "quotas": {
          "type": "array",
          "items": {
            "$ref": "#/definitions/clustermanagerQuota"
          }
        }
      }
    },
    "clustermanagerState": {
      "type": "string",
      "enum": [
        "active",
        "unschedulable",
        "maintenance"
      ],
      "default": "active",
      "description": "Node state. We use lower case properties since\nthey are used as parameters in the API path.\n\n - active: Schedulable, accepts new deployments\n - unschedulable: Node is up and serving running applications however doesn't accepts new deployments\n - maintenance: Node doesn't run any workloads and doesn't accept new deployments"
    },
    "clustermanagerUpdateNodeStateRequest": {
      "type": "object",
      "properties": {
        "hostname": {
          "type": "string"
        },
        "state": {
          "$ref": "#/definitions/clustermanagerState"
        }
      }
    },
    "protobufAny": {
      "type": "object",
      "properties": {
        "type_url": {
          "type": "string",
          "description": "A URL/resource name that uniquely identifies the type of the serialized\nprotocol buffer message. This string must contain at least\none \"/\" character. The last segment of the URL's path must represent\nthe fully qualified name of the type (as in\n\"path/google.protobuf.Duration\"). The name should be in a canonical form\n(e.g., leading \".\" is not accepted).\n\nIn practice, teams usually precompile into the binary all types that they\nexpect it to use in the context of Any. However, for URLs which use the\nscheme \"http\", \"https\", or no scheme, one can optionally set up a type\nserver that maps type URLs to message definitions as follows:\n\n* If no scheme is provided, \"https\" is assumed.\n* An HTTP GET on the URL must yield a [google.protobuf.Type][]\n  value in binary format, or produce an error.\n* Applications are allowed to cache lookup results based on the\n  URL, or have them precompiled into a binary to avoid any\n  lookup. Therefore, binary compatibility needs to be preserved\n  on changes to types. (Use versioned type names to manage\n  breaking changes.)\n\nNote: this functionality is not currently available in the official\nprotobuf release, and it is not used for type URLs beginning with\ntype.googleapis.com.\n\nSchemes other than \"http\", \"https\" (or the empty scheme) might be\nused with implementation specific semantics."
        },
        "value": {
          "type": "string",
          "format": "byte",
          "description": "Must be a valid serialized protocol buffer of the above specified type."
        }
      },
      "description": "\"Any\" contains an arbitrary serialized protocol buffer message along with a\nURL that describes the type of the serialized message.\n\nProtobuf library provides support to pack/unpack Any values in the form\nof utility functions or additional generated methods of the Any type.\n\nExample 1: Pack and unpack a message in C++.\n\n    Foo foo = ...;\n    Any any;\n    any.PackFrom(foo);\n    ...\n    if (any.UnpackTo(\u0026foo)) {\n      ...\n    }\n\nExample 2: Pack and unpack a message in Java.\n\n    Foo foo = ...;\n    Any any = Any.pack(foo);\n    ...\n    if (any.is(Foo.class)) {\n      foo = any.unpack(Foo.class);\n    }\n\n Example 3: Pack and unpack a message in Python.\n\n    foo = Foo(...)\n    any = Any()\n    any.Pack(foo)\n    ...\n    if any.Is(Foo.DESCRIPTOR):\n      any.Unpack(foo)\n      ...\n\n Example 4: Pack and unpack a message in Go\n\n     foo := \u0026pb.Foo{...}\n     any, err := ptypes.MarshalAny(foo)\n     ...\n     foo := \u0026pb.Foo{}\n     if err := ptypes.UnmarshalAny(any, foo); err != nil {\n       ...\n     }\n\nThe pack methods provided by protobuf library will by default use\n'type.googleapis.com/full.type.name' as the type URL and the unpack\nmethods only use the fully qualified type name after the last '/'\nin the type URL, for example \"foo.bar.com/x/y.z\" will yield type\nname \"y.z\".\n\n\nJSON\n====\nThe JSON representation of an \"Any\" value uses the regular\nrepresentation of the deserialized, embedded message, with an\nadditional field \"@type\" which contains the type URL. Example:\n\n    package google.profile;\n    message Person {\n      string first_name = 1;\n      string last_name = 2;\n    }\n\n    {\n      \"@type\": \"type.googleapis.com/google.profile.Person\",\n      \"firstName\": \u003cstring\u003e,\n      \"lastName\": \u003cstring\u003e\n    }\n\nIf the embedded message type is well-known and has a custom JSON\nrepresentation, that representation will be embedded adding a field\n\"value\" which holds the custom JSON in addition to the \"@type\"\nfield. Example (for message [google.protobuf.Duration][]):\n\n    {\n      \"@type\": \"type.googleapis.com/google.protobuf.Duration\",\n      \"value\": \"1.212s\"\n    }"
    },
    "v1clustermanagerStatus": {
      "type": "string",
      "enum": [
        "SUCCESS",
        "ERROR",
        "IN_PROGRESS"
      ],
      "default": "SUCCESS",
      "description": "- SUCCESS: Operation was successful\n - ERROR: Operation failed\n - IN_PROGRESS: Operation in progress",
      "title": "Status represents operation status"
    }
  }
}
`
)
