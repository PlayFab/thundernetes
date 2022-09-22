---
layout: default
title: Useful scripts
parent: Development
nav_order: 5
---

# Useful scripts

## Generate cert for testing

{% include code-block-start.md %}
openssl genrsa 2048 > private.pem
openssl req -x509 -days 1000 -new -key private.pem -out public.pem
kubectl create namespace thundernetes-system
kubectl create secret tls tls-secret -n thundernetes-system --cert=public.pem --key=private.pem
{% include code-block-end.md %}

## Allocate a game server

### With TLS auth

{% include code-block-start.md %}
IP=$(kubectl get svc -n thundernetes-system thundernetes-controller-manager -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
curl --key ~/private.pem --cert ~/public.pem --insecure -H 'Content-Type: application/json' -d '{"buildID":"85ffe8da-c82f-4035-86c5-9d2b5f42d6f5","sessionID":"85ffe8da-c82f-4035-86c5-9d2b5f42d6f5"}' https://${IP}:5000/api/v1/allocate
{% include code-block-end.md %}

### Without TLS auth

{% include code-block-start.md %}
IP=$(kubectl get svc -n thundernetes-system thundernetes-controller-manager -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
curl -H 'Content-Type: application/json' -d '{"buildID":"85ffe8da-c82f-4035-86c5-9d2b5f42d6f5","sessionID":"85ffe8da-c82f-4035-86c5-9d2b5f42d6f5"}' http://${IP}:5000/api/v1/allocate
{% include code-block-end.md %}

## Do 50 allocations

### Without TLS auth

{% include code-block-start.md %}
IP=$(kubectl get svc -n thundernetes-system thundernetes-controller-manager -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
for i in {1..50}; do SESSION_ID=$(uuidgen); curl -H 'Content-Type: application/json' -d '{"buildID":"85ffe8da-c82f-4035-86c5-9d2b5f42d6f6","sessionID":"'${SESSION_ID}'"}' http://${IP}:5000/api/v1/allocate; done
{% include code-block-end.md %}

### With TLS auth

{% include code-block-start.md %}
IP=$(kubectl get svc -n thundernetes-system thundernetes-controller-manager -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
for i in {1..50}; do SESSION_ID=$(uuidgen); curl --key ~/private.pem --cert ~/public.pem --insecure -H 'Content-Type: application/json' -d '{"buildID":"85ffe8da-c82f-4035-86c5-9d2b5f42d6f6","sessionID":"'${SESSION_ID}'"}' https://${IP}:5000/api/v1/allocate; done
{% include code-block-end.md %}
