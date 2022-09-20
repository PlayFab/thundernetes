#!/usr/bin/env bash
CURR_PATH="$(cd "$(dirname "$BASH_SOURCE")"; pwd)"
go build -o $CURR_PATH $CURR_PATH/main.go
mv $CURR_PATH/command-line-arguments $CURR_PATH/kubectl-gameserver
BASH_RC="$(cat ~/.bashrc)"
if ! [[ $BASH_RC =~ "$CURR_PATH" ]]; then
  echo "export PATH=\"\$PATH:"$CURR_PATH"\"" >> ~/.bashrc
fi
kubectl plugin list