#!/bin/env bash

# 1
kind delete cluster && kind create cluster --config ${1}

# 2 
kubectl create secret docker-registry ${2} --docker-server=https://www.cliureever.com --docker-password=19930825abcD! --docker-username=admin
# 3
helm install rl . --set "imagePullSecrets\[0\]=${2}"
# 4
# kubectl apply -f ../../controllers/fixtures/fixtures_cr_v1beta2.yaml