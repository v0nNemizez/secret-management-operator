package controller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	instancev1alpha1 "github.com/v0nNemizez/secret-management-operator/api/v1alpha1"
	storagev1alpha1 "github.com/v0nNemizez/secret-management-operator/api/v1alpha1"
)

// OpenBaoReconciler reconciles a OpenBao object
type OpenBaoReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=instance.secrets.com,resources=openbaoes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=instance.secrets.com,resources=openbaoes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=instance.secrets.com,resources=openbaoes/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete

func (r *OpenBaoReconciler) ensureConfigMap(ctx context.Context, req ctrl.Request, openbao *storagev1alpha1.OpenBao) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name + "-config",
			Namespace: req.Namespace,
		},
		Data: map[string]string{
			"config.json": openbao.Spec.Config,
		},
	}

	_, err := ctrl.CreateOrUpdate(ctx, r.Client, cm, func() error {
		cm.Data["config.json"] = openbao.Spec.Config
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *OpenBaoReconciler) ensureStatefulSet(ctx context.Context, req ctrl.Request, openbao *storagev1alpha1.OpenBao) error {
	replicas := int32(openbao.Spec.Replicas)

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": req.Name},
			},
			ServiceName: req.Name,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": req.Name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "openbao",
							Image:   openbao.Spec.Image,
							Command: []string{"bao"},
							Args:    []string{"server --config /etc/openbao/config.json"},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config-volume",
									MountPath: "/etc/openbao/config.json",
									SubPath:   "config.json",
								},
								{
									Name:      "data-volume",
									MountPath: "/var/lib/openbao",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: req.Name + "-config",
									},
								},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "data-volume",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(openbao.Spec.StorageSize),
							},
						},
					},
				},
			},
		},
	}

	_, err := ctrl.CreateOrUpdate(ctx, r.Client, sts, func() error {
		sts.Spec.Replicas = &replicas
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *OpenBaoReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var openbao storagev1alpha1.OpenBao
	if err := r.Get(ctx, req.NamespacedName, &openbao); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.ensureConfigMap(ctx, req, &openbao); err != nil {
		log.Error(err, "Failed to ensure ConfigMap")
		return ctrl.Result{}, err
	}

	if err := r.ensureStatefulSet(ctx, req, &openbao); err != nil {
		log.Error(err, "Failed to ensure StatefulSet")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenBaoReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&instancev1alpha1.OpenBao{}).
		Complete(r)
}
