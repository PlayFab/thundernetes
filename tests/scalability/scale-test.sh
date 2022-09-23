#!/bin/bash

source ./util.sh

GSB_NAME=gameserverbuild-sample-openarena

echo "test 1: scale up to 10 game servers"
kubectl apply -f ./max/10.yaml
scale_up $GSB_NAME 10
scale_clear $GSB_NAME

echo "test 2: scale up to 50 game servers"
kubectl apply -f ./max/50.yaml
scale_up $GSB_NAME 50
scale_clear $GSB_NAME
