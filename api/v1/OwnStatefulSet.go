package v1

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type OwnStatefulSet struct {
	Spec appsv1.StatefulSetSpec
}

func (ownStatefulSet *OwnStatefulSet) MakeOwnResource(instance *Unit, logger logr.Logger, schema *runtime.Scheme) (interface{}, error) {
	// new a statefulSet object

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels:    instance.Labels,
		},
		Spec: ownStatefulSet.Spec,
	}

	//add some custom envs, ignore this step if you don't need it
	customizeEnv := []v1.EnvVar{
		{
			Name: "POD_NAME",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  "metadata.name",
				},
			},
		},
		{
			Name:  "APPNAME",
			Value: instance.Name,
		},
	}

	var specEnvs []v1.EnvVar
	templateEnvs := sts.Spec.Template.Spec.Containers[0].Env
	for index := range templateEnvs {
		if templateEnvs[index].Name != "POD_NAME" && templateEnvs[index].Name != "APPNAME" {
			specEnvs = append(specEnvs, templateEnvs[index])
		}
	}
	sts.Spec.Template.Spec.Containers[0].Env = append(specEnvs, customizeEnv...)

	if err := controllerutil.SetControllerReference(instance, sts, schema); err != nil {
		msg := fmt.Sprintf("set controllerreference for statefulSet %s/%s", instance.Name, instance.Namespace)
		logger.Error(err, msg)
		return nil, err
	}
	return sts, nil
}

func (ownStatefulSet *OwnStatefulSet) OwnResourceExist(instance *Unit, client client.Client, logger logr.Logger) (bool, interface{}, error) {
	found := &appsv1.StatefulSet{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil, nil
		}
		msg := fmt.Sprintf("StatefulSet %s/%s found, but with error: %s  \n", instance.Namespace, instance.Name, err)
		logger.Error(err, msg)
		return true, found, err
	}
	return true, found, nil
}

func (ownStatefulSet *OwnStatefulSet) ApplyOwnResource(instance *Unit, client client.Client, logger logr.Logger, schema *runtime.Scheme) error {
	//先判断对应的资源是否存在
	exist, found, err := ownStatefulSet.OwnResourceExist(instance, client, logger)
	if err != nil {
		return err
	}
	//然后在创建对应的资源
	sts, err := ownStatefulSet.MakeOwnResource(instance, logger, schema)
	if err != nil {
		return err
	}

	newStatefulSet := sts.(*appsv1.StatefulSet)
	if !exist {
		msg := fmt.Sprintf("statefulSet %s/%s not found , create it!", instance.Name, instance.Namespace)
		logger.Info(msg)
		if err := client.Create(context.TODO(), newStatefulSet); err != nil {
			return err
		}
		return nil
	} else {
		foundStatefulSet := found.(*appsv1.StatefulSet)

		if !reflect.DeepEqual(newStatefulSet.Spec, foundStatefulSet.Spec) {
			msg := fmt.Sprintf("Updating SatefulSeet %s/%s", newStatefulSet.Namespace, newStatefulSet.Name)
			logger.Info(msg)
			return client.Update(context.TODO(), newStatefulSet)
		}
		return nil
	}

}

func (ownStatefulSet *OwnStatefulSet) UpdateOwnResourceStatus(instance *Unit, client client.Client, logger logr.Logger) (*Unit, error) {
	found := &appsv1.StatefulSet{}
	err := client.Get(context.TODO(), types.NamespacedName{
		Name:      instance.Name,
		Namespace: instance.Namespace,
	}, found)
	if err != nil {
		return instance, err
	}

	instance.Status.BaseStatefulSet = found.Status
	instance.Status.LastUpdateTime = metav1.Now()
	return instance, nil
}
