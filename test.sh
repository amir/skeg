#!/bin/bash

export TESTS_NAMESPACE=skeg-tests

TILLER_PORT=44134
TILLER_NAMESPACE=kube-system
TILLER_POD=`kubectl -n $TILLER_NAMESPACE get pods \
  --selector=app=helm,name=tiller \
  -o jsonpath="{range .items[*]}{@.metadata.name}{end}" | head -n 1`

kubectl port-forward -n $TILLER_NAMESPACE $TILLER_POD $TILLER_PORT:$TILLER_PORT > /dev/null 2>&1 &
portforwardPID=$!

./setup.sh

go test

./cleanup.sh

kill $portforwardPID
