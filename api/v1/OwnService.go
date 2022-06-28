package v1

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"net"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"
)

// ServicePort svc的端口映射关系
type ServicePort struct {
	Name       string             `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	Protocol   string             `json:"protocol,omitempty" protobuf:"bytes,2,opt,name=protocol,casttype=Protocol"`
	Port       int32              `json:"port" protobuf:"varint,3,opt,name=port"`
	TargetPort intstr.IntOrString `json:"targetPort,omitempty" protobuf:"bytes,4,opt,name=targetPort"`
	NodePort   int32              `json:"nodePort,omitempty" protobuf:"varint,5,opt,name=nodePort"`
}

type OwnService struct {
	Ports     []v1.ServicePort `json:"ports,omitempty" patchStrategy:"merge" patchMergeKey:"port" protobuf:"bytes,1,rep,name=ports"`
	ClusterIP string           `json:"clusterIP,omitempty" protobuf:"bytes,3,opt,name=clusterIP"`
}

type ServicePortStatus struct {
	v1.ServicePort `json:"servicePort,omitempty"`
	// 检查此端口连通性
	Health bool `json:"health,omitempty"`
}

type UnitRelationServiceStatus struct {
	Type            v1.ServiceType      `json:"type,omitempty"`
	ClusterIP       string              `json:"clusterIP,omitempty"`
	Ports           []ServicePortStatus `json:"ports,omitempty"`
	SessionAffinity v1.ServiceAffinity  `json:"sessionAffinity,omitempty"`
}

type UnitRelationEndpointStatus struct {
	PodName  string `json:"podName"`
	PodIP    string `json:"podIP"`
	NodeName string `json:"nodeName"`
}

func (ownService *OwnService) MakeOwnResource(instance *Unit, logger logr.Logger, schema *runtime.Scheme) (interface{}, error) {
	// new a service object
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels:    instance.Labels,
		},
		Spec: v1.ServiceSpec{
			Ports: ownService.Ports,
			Type:  v1.ServiceTypeClusterIP,
		},
	}
	if ownService.ClusterIP != "" {
		svc.Spec.ClusterIP = ownService.ClusterIP
	}

	// add selector
	svc.Spec.Selector = instance.Spec.Selector.MatchLabels

	if err := controllerutil.SetControllerReference(instance, svc, schema); err != nil {
		msg := fmt.Sprintf("set controllerreference for service %s/%s", instance.Name, instance.Namespace)
		logger.Error(err, msg)
		return nil, err
	}
	return svc, nil
}

func (ownService *OwnService) OwnResourceExist(instance *Unit, client client.Client, logger logr.Logger) (bool, interface{}, error) {
	found := &v1.Service{}

	err := client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, found)

	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil, nil
		}
		msg := fmt.Sprintf("Service %s/%s found, but with error: %s  \n", instance.Namespace, instance.Name, err)
		logger.Error(err, msg)
		return true, found, err
	}
	return true, found, nil
}

func (ownService *OwnService) ApplyOwnResource(instance *Unit, client client.Client, logger logr.Logger, schema *runtime.Scheme) error {
	exist, found, err := ownService.OwnResourceExist(instance, client, logger)
	if err != nil {
		return err
	}

	svc, err := ownService.MakeOwnResource(instance, logger, schema)
	if err != nil {
		return nil
	}

	newService := svc.(*v1.Service)
	if !exist {
		msg := fmt.Sprintf("service %s/%s not found , create it!", instance.Name, instance.Namespace)
		logger.Info(msg)
		if err := client.Create(context.TODO(), newService); err != nil {
			return err
		}
		return nil
	} else {
		foundService := found.(*v1.Service)

		//这里有个问题 巨坑，新建的service每次的ClusterIP都不知道，所以要和创建好的service保持一直
		newService.Spec.ClusterIP = foundService.Spec.ClusterIP
		newService.Spec.SessionAffinity = foundService.Spec.SessionAffinity
		newService.ObjectMeta.ResourceVersion = foundService.ObjectMeta.ResourceVersion

		if !reflect.DeepEqual(newService.Spec, foundService.Spec) {
			msg := fmt.Sprintf("Updating Service %s/%s", newService.Namespace, newService.Name)
			logger.Info(msg)
			return client.Update(context.TODO(), newService)
		}
		return nil
	}
}

func (ownService *OwnService) UpdateOwnResourceStatus(instance *Unit, client client.Client, logger logr.Logger) (*Unit, error) {
	found := &v1.Service{}
	err := client.Get(context.TODO(), types.NamespacedName{
		Name:      instance.Name,
		Namespace: instance.Namespace,
	}, found)
	if err != nil {
		return instance, err
	}

	var portsStatus []ServicePortStatus
	for _, port := range found.Spec.Ports {
		//对service每个port进行健康检查，并且在status字段上添加健康状态
		health := true
		checkPort := port.Port
		addr := found.Spec.ClusterIP
		sock := fmt.Sprintf("%s:%d", addr, checkPort)
		proto := string(port.Protocol)
		_, err := net.DialTimeout(proto, sock, time.Duration(100)*time.Millisecond)
		if err != nil {
			health = false
		}
		portStatus := ServicePortStatus{
			ServicePort: port,
			Health:      health,
		}
		portsStatus = append(portsStatus, portStatus)
	}

	serviceStatus := UnitRelationServiceStatus{
		Type:      found.Spec.Type,
		ClusterIP: found.Spec.ClusterIP,
		Ports:     portsStatus,
	}

	instance.Status.RelationResourceStatus.Service = serviceStatus

	//更新Endpoint status
	foundEndpoints := &v1.Endpoints{}
	err = client.Get(context.TODO(), types.NamespacedName{
		Name:      instance.Name,
		Namespace: instance.Namespace,
	}, foundEndpoints)
	if err != nil {
		return instance, err
	}

	if foundEndpoints.Subsets != nil && foundEndpoints.Subsets[0].Addresses != nil {
		var endpointStatuses []UnitRelationEndpointStatus
		for _, endpoint := range foundEndpoints.Subsets[0].Addresses {
			endpointStatus := UnitRelationEndpointStatus{
				PodName:  endpoint.Hostname,
				PodIP:    endpoint.IP,
				NodeName: *endpoint.NodeName,
			}
			endpointStatuses = append(endpointStatuses, endpointStatus)
		}
		instance.Status.RelationResourceStatus.Endpoint = endpointStatuses
	}
	instance.Status.LastUpdateTime = metav1.Now()

	return instance, nil
}
