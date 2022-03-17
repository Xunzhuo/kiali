# Build Kiali image for Tencent Cloud Mesh
mkdir kiali_sources
cd kiali_sources
export KIALI_SOURCES=$(pwd)

git clone https://git.woa.com/bitliu/kiali.git -b tcm
git clone https://github.com/kiali/kiali-ui.git

# Build the back-end and run the tests
cd $KIALI_SOURCES/kiali
make build test

# Build the front-end and run the tests
cd $KIALI_SOURCES/kiali-ui
yarn && yarn build

export CLUSTER_TYPE=local
export OPERATOR_CONTAINER_NAME=ccr.ccs.tencentyun.com/kiali/kiali-operator
export OPERATOR_QUAY_NAME=ccr.ccs.tencentyun.com/kiali/kiali-operator
export CONTAINER_NAME=ccr.ccs.tencentyun.com/kiali/kiali
export QUAY_NAME=ccr.ccs.tencentyun.com/kiali/kiali

cd $KIALI_SOURCES/kiali

# Build the Kiali-server and Kiali-operator container images and push them to the cluster
make CONTAINER_VERSION=tcm-1.48 build container-build container-push