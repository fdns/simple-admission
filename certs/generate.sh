#!/usr/bin/env bash
cfssl gencert -initca ca.json | cfssljson -bare ca
cfssl gencert -ca ca.pem -ca-key ca-key.pem admission.json | cfssljson -bare admission
