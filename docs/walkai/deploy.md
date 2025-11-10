## Steps

### Enable MIG
```bash
sudo nvidia-smi -i 0 -mig 1
```
Check with this
```bash
nvidia-smi -i 0 -q | grep -A3 "MIG Mode"
```

If it didn't work, try rebooting
```bash
sudo reboot
```

### Generate CDI spec and enable CDI mode

```bash
sudo nvidia-ctk cdi generate --output=/etc/cdi/nvidia.yaml
sudo nvidia-ctk config --in-place --set nvidia-container-runtime.mode=cdi
sudo systemctl restart docker
```

### Create the kubernetes cluster

```bash
minikube start -p walkai-dev \
--apiserver-ips=192.168.76.2,127.0.0.1 \
--apiserver-names=localhost,k8s.walkai.internal \
--driver=docker \
--container-runtime=docker  \
--cpus=22 \
--memory=200g  \
--disk-size=50g \
--gpus nvidia.com  \
--force
```

### Disable the default addon

```bash
minikube addons disable nvidia-device-plugin
kubectl -n kube-system delete ds nvidia-device-plugin-daemonset --ignore-not-found
```

### Install GPU Operator using helm

```bash
helm repo add nvidia https://helm.ngc.nvidia.com/nvidia && helm repo update
helm install --wait --generate-name \
     -n gpu-operator --create-namespace \
     nvidia/gpu-operator --version v22.9.0 \
     --set driver.enabled=false \
     --set migManager.enabled=false \
     --set mig.strategy=mixed \
     --set toolkit.enabled=true
```

### Install cert-manager
```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.18.2/cert-manager.yaml
```

```bash
git clone https://github.com/walkai-org/walkai-nos.git
```
```bash
cd walkai-nos
```

```bash
kubectl label node walkai-dev nos.nebuly.com/gpu-partitioning=mig --overwrite
```

Edit the API endpoint at cluster_info_exporter_config.yaml. If you are running a local cluster you can use:
```bash
minikube -p <node name> ssh -- "ip route | awk '/default/ {print \$3; exit}'"
```
To get the IP of the gateway. Or you can use ngrok for a public ip to your api

Create secret with the API Token

```bash
kubectl create namespace nos-system
kubectl create secret generic cluster-info-exporter-secrets -n nos-system --from-literal=apiToken='<token>'
```

### API client RBAC and token
The clusterinfoexporter kustomization now also provisions the `walkai` namespace together with an `api-client` service account, the `discovery-minimal` and `admin` ClusterRoleBindings, and the long-lived `api-client-permanent-token` Secret. After the manifests are applied you can retrieve the token that your API needs with:
```bash
kubectl get secret api-client-permanent-token -n walkai   -o jsonpath='{.data.token}' | base64 -d; echo
```

```bash
kubectl apply -k config/migagent/default
kubectl apply -k config/gpupartitioner/default
kubectl apply -k config/clusterinfoexporter/default
```

```bash
kubectl get pods -n nos-system -w
```
