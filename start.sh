#!/bin/sh

if [ -f /var/run/secrets/kubernetes.io/serviceaccount/ca.crt ]
then
   cat  /var/run/secrets/kubernetes.io/serviceaccount/ca.crt >> /etc/ssl/certs/ca-certificates.crt
fi
./region-api
