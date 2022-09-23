#!/bin/bash

source ./util.sh

GSID=85ffe8da-c82f-4035-86c5-9d2b5f42d6f7

echo "test 1: scale up to 16 servers from 1 standby server"
kubectl apply -f ./standby/1.yaml
scale_up $GSID 16
kubectl delete gsb gameserverbuild-sample-openarena

echo "test 2: scale up to 16 servers from 4 standby server"
kubectl apply -f ./standby/4.yaml
scale_up $GSID 16
scale_clear
kubectl delete gsb gameserverbuild-sample-openarena

echo "test 3: scale up to 16 servers from 16 standby server"
kubectl apply -f ./standby/16.yaml
scale_up $GSID 16
scale_clear
kubectl delete gsb gameserverbuild-sample-openarena
