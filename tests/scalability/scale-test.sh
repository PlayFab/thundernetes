#!/bin/bash

source ./util.sh

GSB_NAME=gameserverbuild-sample-openarena

echo "test 1: scale up to 16 servers from 1 standby server"
kubectl apply -f ./standby/1.yaml
scale_up $GSB_NAME 16
kubectl delete gsb gameserverbuild-sample-openarena

echo "test 2: scale up to 16 servers from 4 standby server"
kubectl apply -f ./standby/4.yaml
scale_up $GSB_NAME 16
scale_clear
kubectl delete gsb gameserverbuild-sample-openarena

echo "test 3: scale up to 16 servers from 16 standby server"
kubectl apply -f ./standby/16.yaml
scale_up $GSB_NAME 16
scale_clear
kubectl delete gsb gameserverbuild-sample-openarena
