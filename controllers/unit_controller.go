/*
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
*/

package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	customv1 "Unit/api/v1"
)

// UnitReconciler reconciles a Unit object
type UnitReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

type OwnResource interface {
	// MakeOwnResource 根据指定的Unit，生成对应build-in资源对象
	MakeOwnResource(instance *customv1.Unit, logger logr.Logger, schema *runtime.Scheme) (interface{}, error)

	// OwnResourceExist 判断该资源是否存在
	OwnResourceExist(instance *customv1.Unit, client client.Client, logger logr.Logger) (bool, interface{}, error)

	// UpdateOwnResourceStatus 更新Unit所包含的build-in资源，用来填充status字段
	UpdateOwnResourceStatus(instance *customv1.Unit, client client.Client, logger logr.Logger) (*customv1.Unit, error)

	// ApplyOwnResource 创建/更新Unit对应的build-in资源
	ApplyOwnResource(instance *customv1.Unit, client client.Client, logger logr.Logger, schema *runtime.Scheme) error
}

func (r *UnitReconciler) getOwnResources(instance *customv1.Unit) ([]OwnResource, error) {
	var ownResource []OwnResource

	if instance.Spec.Category == "Deployment" {
		ownDeployment := customv1.OwnDeployment{
			Spec: appsv1.DeploymentSpec{
				Replicas: instance.Spec.Replicas,
				Selector: instance.Spec.Selector,
				Template: instance.Spec.Template,
			},
		}
		//ownDeployment.Spec.Template.Labels = instance.Spec.Selector.MatchLabels
		ownResource = append(ownResource, &ownDeployment)
	} else {
		ownStatefulSet := &customv1.OwnStatefulSet{
			Spec: appsv1.StatefulSetSpec{
				Replicas:    instance.Spec.Replicas,
				Selector:    instance.Spec.Selector,
				Template:    instance.Spec.Template,
				ServiceName: instance.Name,
			},
		}
		ownResource = append(ownResource, ownStatefulSet)
	}

	if instance.Spec.RelationResource.Service != nil {
		ownResource = append(ownResource, instance.Spec.RelationResource.Service)
	}
	if instance.Spec.RelationResource.PVC != nil {
		ownResource = append(ownResource, instance.Spec.RelationResource.PVC)
	}
	//if instance.Spec.RelationResource.Ingress != nil {
	//	ownResource = append(ownResource, instance.Spec.RelationResource.Ingress)
	//}
	return ownResource, nil
}

//+kubebuilder:rbac:groups=custom.hmlss.ml,resources=units,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=custom.hmlss.ml,resources=units/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=custom.hmlss.ml,resources=units/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Unit object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *UnitReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	_ = r.Log.WithValues("unit", req.NamespacedName)

	// TODO(user): your logic here

	//出现问题需要回溯
	defer func() {
		if rec := recover(); r != nil {
			switch x := rec.(type) {
			case error:
				r.Log.Error(x, "Reconcile error")
			}
		}
	}()

	instance := &customv1.Unit{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found Create objects are automatically garbage collected.
			return reconcile.Result{}, nil
		}
	}

	//从yaml文件中读取有那些build-in类型的数据，并且转化程对应的ownResources
	OwnResources, err := r.getOwnResources(instance)
	if err != nil {
		msg := fmt.Sprintf("%s %s Reconciler.getOwnResource() function error", instance.Namespace, instance.Name)
		r.Log.Error(err, msg)
		return ctrl.Result{}, err
	}

	//从上面获取到的ownResource，判断对应的资源有没有变化，如果有的话需要更新对应的资源
	success := true
	for _, ownResource := range OwnResources {
		if err = ownResource.ApplyOwnResource(instance, r.Client, r.Log, r.Scheme); err != nil {
			success = false
		}
	}

	//更新Unit.status字段
	updateInstance := instance.DeepCopy()
	for _, ownResource := range OwnResources {
		updateInstance, err = ownResource.UpdateOwnResourceStatus(updateInstance, r.Client, r.Log)
		if err != nil {
			success = false
		}
	}

	//apply update to api-server if status change
	if updateInstance != nil && !reflect.DeepEqual(updateInstance.Status, instance.Status) {
		if err := r.Status().Update(context.Background(), updateInstance); err != nil {
			r.Log.Error(err, "unable to update the Unit status")
		}
	}

	if !success {
		msg := fmt.Sprintf("Reconcile Unit %s/%s failed", instance.Namespace, instance.Name)
		r.Log.Error(err, msg)
		return ctrl.Result{}, err
	} else {
		msg := fmt.Sprintf("Reconcile Unit %s/%s success", instance.Namespace, instance.Name)
		r.Log.Info(msg)
		return ctrl.Result{}, nil
	}

}

// SetupWithManager sets up the controller with the Manager.
func (r *UnitReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&customv1.Unit{}).
		Complete(r)
}
