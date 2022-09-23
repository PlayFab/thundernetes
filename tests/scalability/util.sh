#!/bin/bash

function scale_up() {
    st=$(date +%s)
    buildID=$1
    replicas=$2

    IP=$(kubectl get svc -n thundernetes-system thundernetes-controller-manager -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

    echo build ID: $buildID
    for i in $(seq 1 $replicas); do
        session=$(uuidgen)
        ret=500
        while [ "$ret" -gt 400 ]; do
            ret=$(curl -s -o /dev/null -w "%{http_code}" -H 'Content-Type: application/json' -d "{\"buildID\":\"${buildID}\",\"sessionID\":\"${session}\"}" http://${IP}:5000/api/v1/allocate)
        done
        echo

        echo up $i - $session
    done
    et=$(date +%s)

    echo "Scale up time: $((et-st))s"
}
echo "Added function scale_up(buildID, replicas)"

function scale_clear(){
    kubectl get gs -o=jsonpath='{range .items[?(@.status.state=="Active")]}{.metadata.name}{"\n"}' | xargs -I {} kubectl delete gs {}
}
echo "Added function scale_clear()"
