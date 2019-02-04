# CRD Controller using kubebuilder

### Prerequisites

* `go get -u github.com/masudur-rahman/CRD-Controller-kubebuilder`
* `cd /home/$USER/go/src/github.com/masudur-rahman/CRD-Controller-kubebuilder`


### Run the Controller

* `make install`
* `make run`


#### In another terminal

* `kubectl apply -f config/samples/workloads_v1beta1_containerset.yaml`