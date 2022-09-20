#!/usr/bin/env bash
CURR_PATH="$(dirname "$BASH_SOURCE")"
go build -o $CURR_PATH $CURR_PATH/main.go
mv $CURR_PATH/command-line-arguments $CURR_PATH/kubectl-gameserver-allocate
export PATH=$PATH:"$(cd $CURR_PATH; pwd)"
kubectl plugin list