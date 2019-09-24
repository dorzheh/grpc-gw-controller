package apphcmanager 

const (
swagger = `{
  "swagger": "2.0",
  "info": {
    "title": "apphcmanager.proto",
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
    "/api/v1/apphc/version": {
      "get": {
        "summary": "Obtain Controller version",
        "operationId": "GetVersion",
        "responses": {
          "200": {
            "description": "A successful response.",
            "schema": {
              "$ref": "#/definitions/apphcmanagerGetApphcVersionResponse"
            }
          }
        },
        "tags": [
          "ApphcManager"
        ]
      }
    }
  },
  "definitions": {
    "apphcmanagerGetApphcVersionResponse": {
      "type": "object",
      "properties": {
        "version": {
          "type": "string"
        },
        "api_version": {
          "type": "string"
        },
        "git_commit": {
          "type": "string"
        },
        "git_state": {
          "type": "string"
        }
      }
    }
  }
}
`
)
