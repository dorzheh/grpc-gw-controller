#!/usr/bin/env bash
#####################
# Upgrade controller
#####################
# dorzheho@cisco.com
#####################

set -e
set -x 
SCRIPTS_DIR=$(dirname $0)
source $SCRIPTS_DIR/environment.conf
source $SCRIPTS_DIR/images.conf

helm upgrade --set image.tag=$CONTROLLER_IMAGE_TAG cisco-apphoster-controller $SCRIPTS_DIR/../helm --reuse-values

echo "==> Waiting for the AppHoster Controller"
count=30

while (( $count != 0 ));do
      items=( $(kubectl get pods --selector=app=cisco-apphoster-controller -n cisco-apphoster-controller  -o jsonpath='{.items[*].status.phase}') )
      (( ${#items[*]} != 2 )) && break 
      [[ ${items[0]} == "Running" &&  ${items[1]} == "Running" ]] && break
      ((count--))
      sleep 1
done

if (( $count == 0 )); then
   echo "ERROR: AppHoster Controller is not ready"
   exit 1
fi

