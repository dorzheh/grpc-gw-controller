#!/usr/bin/env bash 
##############################
# Install Apphoster controller
##############################
# dorzheho@cisco.com
##############################

DOCKER_REGISTRY=${1?"FATAL: Docker registry is not provided"}
RANCHER_SERVER_ENDPOINT=${2?"FATAL: Rancher endpoint is not provided"}
SVCS_URL_EXTERNAL_IP=$3

_fullpath()
{
  [[ $1 = /* ]] && echo "$1" || echo "$PWD/${1#./}"
}

SCRIPTS_DIR=$(dirname $(_fullpath $0))
ROOT_DIR=$(dirname $SCRIPTS_DIR)

source $SCRIPTS_DIR/environment.conf
source $SCRIPTS_DIR/images.conf

deploy()
{
  echo "==> Obtaining Rancher token"
  local login_token=$(curl -s "$RANCHER_SERVER_ENDPOINT/v3-public/localProviders/local?action=login" -H 'content-type: application/json' --data-binary '{"username":"admin","password":"intucell"}' --insecure | jq -r .token)

  echo "==> Deploying AppHoster Controller"
  local cmd="--namespace cisco-apphoster-controller --name cisco-apphoster-controller \
     --set image.registry=$DOCKER_REGISTRY \
     --set image.tag=$CONTROLLER_IMAGE_TAG \
     --set env.APPHC_ADAPTERS_RANCHER_SERVER_ENDPOINT=$RANCHER_SERVER_ENDPOINT \
     --set apphc.rancher_server_creds_token=$login_token \
     --set apphc.rancher_catalog_password=$ENV_APPHC_ADAPTERS_RANCHER_CATALOG_PASSWORD \
     --set apphc.bearer_token=$ENV_APPHC_BEARER_TOKEN"
 
  [[ "x$SVCS_URL_EXTERNAL_IP" != "x" ]] && cmd="$cmd --set env.APPHC_SVCS_URL_EXTERNAL_IP=$SVCS_URL_EXTERNAL_IP"

  helm install $ROOT_DIR/helm --namespace cisco-apphoster-controller --name cisco-apphoster-controller $cmd
}

readiness()
{
  echo "==> Waiting for the AppHoster Controller"
  count=30

  while (( $count != 0 ));do 
      ready=$(kubectl get pods --selector=app=cisco-apphoster-controller -n cisco-apphoster-controller  -o jsonpath='{.items[*].status.containerStatuses[*].ready}')
      [ "X$ready" == "Xtrue" ] && break 
      ((count--))
      sleep 2
  done

  if (( $count == 0 )); then
    echo "  ! ERROR: AppHoster Controller not ready"
    return 1
  fi
}

deployment=$(helm list --namespace cisco-apphoster-controller)
[[ "x$deployment" != "x" ]] || deploy
readiness || exit 1


