# A simple example of an admission controller for Kubernetes
This admission controller is an example for the blogpost at [https://fdns.github.io/2021-04-11/Kubernetes-Admission-Controller](https://fdns.github.io/2021-04-11/Kubernetes-Admission-Controller).

This admission controller will filter all jobs created in the default namespace, and only allow the creation of the jobs that setup all the required fields so that the job can be run as secure as possible (Using gvisor, no root user, etc).

## Usage
All documentation to run it can be found at the makefile
```
> make help
Use make NAMESPACE=override to change the target namespace

build                          Build docker image with tag fdns/simple-admission:latest
certificates                   Generate certs/manifest.yaml with the ValidatingWebhookConfiguration and a randomly generated cert
kind                           Build image and upload to king
skaffold                       Generate certificates and start skaffold
```
