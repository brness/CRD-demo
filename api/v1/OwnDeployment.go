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

type OwnDeployment struct {
	Spec appsv1.DeploymentSpec `json:"spec"`
}

func (ownDeployment *OwnDeployment) MakeOwnResource(instance *Unit, logger logr.Logger, schema *runtime.Scheme) (interface{}, error) {
	// new a deployment object

	dlp := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: instance.Namespace,
			Name:      instance.Name,
			Labels:    instance.Labels,
		},
		Spec: ownDeployment.Spec,
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
	templateEnvs := dlp.Spec.Template.Spec.Containers[0].Env
	for index := range templateEnvs {
		if templateEnvs[index].Name != "POD_NAME" && templateEnvs[index].Name != "APPNAME" {
			specEnvs = append(specEnvs, templateEnvs[index])
		}
	}
	dlp.Spec.Template.Spec.Containers[0].Env = append(specEnvs, customizeEnv...)
	//设置label保持一致
	dlp.Spec.Template.ObjectMeta.Labels = instance.Spec.Selector.MatchLabels

	if err := controllerutil.SetControllerReference(instance, dlp, schema); err != nil {
		msg := fmt.Sprintf("set controllerreference for deployment %s/%s", instance.Name, instance.Namespace)
		logger.Error(err, msg)
		return nil, err
	}
	return dlp, nil
}

func (ownDeployment *OwnDeployment) OwnResourceExist(instance *Unit, client client.Client, logger logr.Logger) (bool, interface{}, error) {
	found := &appsv1.Deployment{}

	err := client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, found)

	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil, nil
		}
		msg := fmt.Sprintf("Deployment %s/%s found, but with error: %s  \n", instance.Namespace, instance.Name, err)
		logger.Error(err, msg)
		return true, found, err
	}
	return true, found, nil
}

func (ownDeployment *OwnDeployment) ApplyOwnResource(instance *Unit, client client.Client, logger logr.Logger, schema *runtime.Scheme) error {
	//先判断对应的资源是否存在
	exist, found, err := ownDeployment.OwnResourceExist(instance, client, logger)
	if err != nil {
		return err
	}
	//然后在创建对应的资源
	dlp, err := ownDeployment.MakeOwnResource(instance, logger, schema)
	if err != nil {
		return err
	}

	newDeployment := dlp.(*appsv1.Deployment)
	if !exist {
		msg := fmt.Sprintf("deployment %s/%s not found , create it!", instance.Name, instance.Namespace)
		logger.Info(msg)
		if err := client.Create(context.TODO(), newDeployment); err != nil {
			return err
		}
		return nil
	} else {
		foundDeployment := found.(*appsv1.Deployment)

		if !reflect.DeepEqual(newDeployment.Spec, foundDeployment.Spec) {
			msg := fmt.Sprintf("Updating Deployment %s/%s", newDeployment.Namespace, newDeployment.Name)
			logger.Info(msg)
			return client.Update(context.TODO(), newDeployment)
		}
		return nil
	}

}

func (ownDeployment *OwnDeployment) UpdateOwnResourceStatus(instance *Unit, client client.Client, logger logr.Logger) (*Unit, error) {
	found := &appsv1.Deployment{}
	err := client.Get(context.TODO(), types.NamespacedName{
		Name:      instance.Name,
		Namespace: instance.Namespace,
	}, found)
	if err != nil {
		return instance, err
	}

	instance.Status.BaseDeployment = found.Status
	instance.Status.LastUpdateTime = metav1.Now()
	return instance, nil
}
