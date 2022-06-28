# unit
创建一个unitCRD，使得我们创建一个unit的时候，对应的deployment/statefulSet，service，pvc等内容都可以一起创建，这样就不需要每个build-in资源都自己创建，方便统一化管理和设置。
## Description
目前的第一个版本是完全由reconcile去创建所有的资源，后续的版本可以用webhook等资源添加，总的来说我们需要构建的yaml文件如下
```yaml
apiVersion: custom.hmlss.ml/v1
kind: Unit
metadata:
  name: unit-sample
spec:
  # TODO(user): Add fields here
  category: "Deployment"
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    spec:
      containers:
        - image: nginx
          imagePullPolicy: IfNotPresent
          name: unit-sample
          resources:
            limits:
              cpu: 110m
              memory: 256Mi
            requests:
              cpu: 100m
              memory: 128Mi
  relationResource:
    serviceInfo:
      ports:
        - name: http
          port: 80
          protocol: TCP
          targetPort: 80
    pvcInfo:
      spec:
        accessModes:
          - ReadWriteMany
        resources:
          requests:
            storage: 10Gi
        storageClassName: standard
```
其中第一部分主要包括deployment/statefulSet的资源字段，用于表示我们需要创建的deploy  
第二部分则是我们的service资源字段，主要是port字段信息，如果要支持指定ClusterIP也可以  
第三部分则是PVC的资源，表示对应的PVC。

这样我们就可以根据设计出来的yaml文件，配置对应的实体信息
首先就是unitSpec对象
```go
type unitSpec struct {
	category string `json:category`
	relicas int32 `json:relicas`
	selector corev1.Selector `json:selector`
	template v1.PodTemplateSpec `json:template`
	relationResource UnitRelationResource `json:relationResource`
}
```
其中UnitRelationResource就是我们上面pvc和service的结合体，对应的数据结构如下：
```go
type unitRelationResource struct{
	ownService `json:serviceInfo`
	ownPVC `json:pvcInfo`
}
```
ownService和ownPVC就是我们上面定义好的资源对象
```go
type ownService struct{
	port v1.ServicePortSpec 'json:ports'
	ClusterIP string 'json:clusterIP'
}

type ownPVC struct{
	Spec v1.persistenVolumeClaimSpec 'json:spec'
}
```

这样我们的unit对象就可以找到所有build-in资源对象，结下来需要涉及到对应资源的创建。也就是reconcile的逻辑实现了。但是在开始之前，我们首先要考虑清楚对应的操作流程是什么样子的。

首先unit对象提交进来，我们可以通过client获取到对应的结构，根据结构将所需要创建的资源给拆解出来，比如说一个category为deployment的unit对象我们可以拆解成ownDeployment，ownService和ownPVC三个对象数组。
这样做的好处是什么呢？因为这三个对象有相同的操作流程，或者说接口方式。他们都需要先判断对应的资源是否存在，如果不存在的话需要创建新的资源，来保证real和expect保持一致。如果存在的话，则需要比较这几者的Spec是否和expect的Spec保持一致，如果不一致的话，同样需要update资源，满足reconcile。
所以这样我们就可以抽象出一个接口，叫ownResource，他需要实现以下4个方法  

```go
ownResourceExist (判断资源是否已经存在,返回对应的real实体和bool和error)
ownResourceMake (创建对应的build-in实体的yaml文件，返回expect实体和bool)
ownResourceApply (根据上面两个对象返回的信息，选择更新对应资源或者创建新的资源)
ownResourceStatusUpdate(获取状态信息一栏，使得每次获取unit的时候可以附带上额外的资源信息)
```





## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:
	
```sh
make docker-build docker-push IMG=<some-registry>/unit:tag
```
	
3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/unit:tag
```

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller to the cluster:

```sh
make undeploy
```

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/) 
which provides a reconcile function responsible for synchronizing resources untile the desired state is reached on the cluster 

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

