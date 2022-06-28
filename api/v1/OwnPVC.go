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
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// pvc声明信息
type OwnPVC struct {
	Spec v1.PersistentVolumeClaimSpec ` json:"spec"`
}

func (ownPVC *OwnPVC) MakeOwnResource(instance *Unit, logger logr.Logger, schema *runtime.Scheme) (interface{}, error) {
	//new a PVC object
	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels:    instance.Labels,
		},
		Spec: ownPVC.Spec,
	}

	if err := controllerutil.SetControllerReference(instance, pvc, schema); err != nil {
		msg := fmt.Sprintf("set controllerreference for PVC %s/%s", instance.Name, instance.Namespace)
		logger.Error(err, msg)
		return nil, err
	}
	return pvc, nil
}

func (ownPVC *OwnPVC) OwnResourceExist(instance *Unit, client client.Client, logger logr.Logger) (bool, interface{}, error) {
	pvc := &v1.PersistentVolumeClaim{}
	err := client.Get(context.TODO(), types.NamespacedName{Namespace: instance.Namespace, Name: instance.Name}, pvc)

	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil, nil
		}
		msg := fmt.Sprintf("PVC %s/%s found, but has err %s", instance.Namespace, instance.Name, err)
		logger.Info(msg)
		return true, pvc, err
	}
	return true, pvc, nil
}

func (ownPVC *OwnPVC) ApplyOwnResource(instance *Unit, client client.Client, logger logr.Logger, schema *runtime.Scheme) error {
	exist, found, err := ownPVC.OwnResourceExist(instance, client, logger)
	if err != nil {
		return err
	}
	pvc, err := ownPVC.MakeOwnResource(instance, logger, schema)
	if err != nil {
		return nil
	}

	newPVC := pvc.(*v1.PersistentVolumeClaim)
	if !exist {
		msg := fmt.Sprintf("creating PVC")
		logger.Info(msg)
		return client.Create(context.TODO(), newPVC)
	} else {
		foundPVC := found.(*v1.PersistentVolumeClaim)
		//这里需要手动赋值，要不然创建的PVC一直和原来的不一样
		newPVC.Spec.VolumeMode = foundPVC.Spec.VolumeMode
		if !reflect.DeepEqual(newPVC.Spec, foundPVC.Spec) {
			msg := fmt.Sprintf("updating PVC")
			logger.Info(msg)
			return client.Update(context.TODO(), newPVC)
		}
		return nil
	}
}

func (ownPVC *OwnPVC) UpdateOwnResourceStatus(instance *Unit, client client.Client, logger logr.Logger) (*Unit, error) {
	pvc := &v1.PersistentVolumeClaim{}
	if err := client.Get(context.TODO(), types.NamespacedName{Namespace: instance.Namespace, Name: instance.Name}, pvc); err != nil {
		return instance, err
	}

	instance.Status.RelationResourceStatus.PVC = pvc.Status
	instance.Status.LastUpdateTime = metav1.Now()
	return instance, nil
}
