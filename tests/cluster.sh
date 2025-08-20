#!/bin/bash

set -ex

create_cluster() {
    echo "Creating Kind cluster..."
    cat <<EOF | kind create cluster --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 80
    hostPort: 12080
    protocol: TCP
  - containerPort: 443
    hostPort: 12443
    protocol: TCP
EOF
}

deploy_ingress_nginx() {
    echo "Deploying Ingress Nginx..."
    kubectl apply -f ../deploy/ingress-nginx.yaml
    
    echo "Waiting for Ingress Nginx to be ready..."
    kubectl wait --namespace ingress-nginx \
                 --for=condition=ready pod \
                 --selector=app.kubernetes.io/component=controller \
                 --timeout=90s
}

deploy_keda() {
    echo "Installing KEDA..."
    helm repo add kedacore https://kedacore.github.io/charts
    helm repo update
    helm install keda kedacore/keda --namespace keda --create-namespace
    
    # echo "Waiting for KEDA to be ready..."
    # kubectl wait --namespace keda \
    #              --for=condition=ready pod \
    #              --selector=app=keda-operator \
    #              --timeout=90s
}

deploy_external_scaler() {
    echo "Deploying external scaler..."
    kubectl apply -f ../deploy/scaler.yaml
    
    echo "Waiting for external scaler to be ready..."
    kubectl wait --namespace keda \
                 --for=condition=ready pod \
                 --selector=app=ingress-nginx-external-scaler \
                 --timeout=90s
}

load_image_from_file() {
    local file=$1
    echo "Extracting images from $file..."
    images=$(grep "image:" $file | awk '{print $2}' | tr -d '"')
    
    echo "Pulling and loading images into Kind cluster:"
    for image in $images; do
        echo "  - $image"
        # 检查本地是否已存在镜像
        if ! docker inspect "$image" >/dev/null 2>&1; then
            echo "    Image $image not found locally, pulling..."
            docker pull "$image"
        else
            echo "    Image $image already exists locally, skipping pull"
        fi
        kind load docker-image "$image"
    done
}

deploy_example() {
    echo "Deploying example application..."
    kubectl apply -f ../deploy/example.yaml
    
    echo "Waiting for example application to be ready..."
    kubectl wait --for=condition=ready pod \
                 --selector=app=test-1 \
                 --timeout=90s
}

deploy_wrk() {
    echo "Deploying wrk..."
    kubectl apply -f ../deploy/wrk.yaml
    echo "Waiting for wrk to be ready..."
    kubectl wait --for=condition=ready pod \
                 --selector=app=wrk \
                 --timeout=90s
}

load_image_in_keda() {
    kubectl get pods -n keda -o yaml | grep image: | grep -v '\- image' | \
    awk '{print $2}' | awk -F@ '{print $1}' | xargs -I {} docker pull {}
    kubectl get pods -n keda -o yaml | grep image: | grep -v '\- image' | \
    awk '{print $2}' | awk -F@ '{print $1}' | xargs -I {} kind load docker-image {}
}


setup_all() {
    create_cluster
    echo "Loading Nginx images into Kind cluster..."
    load_image_from_file ../deploy/ingress-nginx.yaml 
    deploy_ingress_nginx
    
    deploy_keda
    load_image_in_keda
    
    load_image_from_file ../deploy/scaler.yaml 
    deploy_external_scaler

    load_image_from_file ../deploy/example.yaml 
    deploy_example

    load_image_from_file ../deploy/wrk.yaml
    deploy_wrk
    echo "All components deployed successfully!"
}

# 进入脚本所在目录
cd "$(dirname "$0")"

# 如果直接运行此脚本，则执行完整设置
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    setup_all
fi
