package controller

import (
	"context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	instancev1alpha1 "github.com/v0nNemizez/secret-management-operator/api/v1alpha1"
	"github.com/v0nNemizez/secret-management-operator/pkg/lib/certificates"
)

// ClusterReconciler reconciles a Cluster object
type ClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=instance.secrets.com,resources=openbaoes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=instance.secrets.com,resources=openbaoes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=instance.secrets.com,resources=openbaoes/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

func (r *ClusterReconciler) ensureStatefulSet(ctx context.Context, req ctrl.Request, cluster *instancev1alpha1.Cluster) error {
	replicas := int32(cluster.Spec.ClusterSize)

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
							Image:   cluster.Spec.Image,
							Command: []string{"bao", "server", "--config=/etc/openbao/config.json"},
							Env:     getEnvVars(cluster.Spec.Envs),
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
								{
									Name:      "cert-volume",
									MountPath: "/etc/openbao/cert",
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
						{
							Name: "cert-volume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: req.Name + "-cert",
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
								corev1.ResourceStorage: resource.MustParse(cluster.Spec.StorageSize),
							},
						},
					},
				},
			},
		},
	}

	// Use the sts variable to create the StatefulSet in the cluster
	if err := r.Client.Create(ctx, sts); err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
		log.FromContext(ctx).Info("StatefulSet already exists, updating it")
		if err := r.Client.Update(ctx, sts); err != nil {
			return err
		}
	}

	return nil
}

func (r *ClusterReconciler) ensureConfigMap(ctx context.Context, req ctrl.Request, cluster *instancev1alpha1.Cluster) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name + "-config",
			Namespace: req.Namespace,
		},
		Data: map[string]string{
			"config.json": cluster.Spec.Config,
		},
	}

	found := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{Name: configMap.Name, Namespace: configMap.Namespace}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.Create(ctx, configMap)
		}
		return err
	}

	found.Data = configMap.Data
	return r.Update(ctx, found)
}

func getEnvVars(envs []instancev1alpha1.EnvOptions) []corev1.EnvVar {
	var envVars []corev1.EnvVar
	for _, env := range envs {
		envVars = append(envVars, corev1.EnvVar{
			Name:  env.Name,
			Value: env.Value,
		})
	}
	return envVars
}

func (r *ClusterReconciler) ensureCertificateGeneration(ctx context.Context, req ctrl.Request, cluster *instancev1alpha1.Cluster) error {
	cert, key, err := certificates.GenerateCertificate()
	if err != nil {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name + "-cert",
			Namespace: req.Namespace,
		},
		Data: map[string][]byte{
			"cert.crt": cert,
			"key.pem":  key,
		},
	}

	found := &corev1.Secret{}
	err = r.Get(ctx, client.ObjectKey{Name: secret.Name, Namespace: secret.Namespace}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.Create(ctx, secret)
		}
		return err
	}

	found.Data = secret.Data
	return r.Update(ctx, found)
}

func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	cluster := &instancev1alpha1.Cluster{}
	err := r.Get(ctx, req.NamespacedName, cluster)
	if err != nil {
		if errors.IsNotFound(err) {
			// Resource is already deleted, no further action needed
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to fetch Cluster resource")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !cluster.DeletionTimestamp.IsZero() {
		// Perform cleanup, but don’t fail the reconcile if cleanup encounters ignorable errors
		if err := r.cleanupResources(ctx, req); err != nil {
			log.Error(err, "Error during resource cleanup, proceeding with finalizer removal")
			// Log the error but don’t return it—allow finalizer removal to proceed
		}

		// Remove finalizer if present
		if controllerutil.ContainsFinalizer(cluster, "finalizer.instance.secrets.com") {
			log.Info("Removing finalizer from Cluster")
			controllerutil.RemoveFinalizer(cluster, "finalizer.instance.secrets.com")
			if err := r.Update(ctx, cluster); err != nil {
				log.Error(err, "Failed to remove finalizer")
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	// Add finalizer if not present (creation/update case)
	if !controllerutil.ContainsFinalizer(cluster, "finalizer.instance.secrets.com") {
		log.Info("Adding finalizer to Cluster")
		controllerutil.AddFinalizer(cluster, "finalizer.instance.secrets.com")
		if err := r.Update(ctx, cluster); err != nil {
			log.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
	}

	// Normal reconciliation logic
	err = r.ensureStatefulSet(ctx, req, cluster)
	if err != nil {
		log.Error(err, "Failed to ensure StatefulSet")
		return ctrl.Result{}, err
	}

	err = r.ensureConfigMap(ctx, req, cluster)
	if err != nil {
		log.Error(err, "Failed to ensure ConfigMap")
		return ctrl.Result{}, err
	}

	err = r.ensureCertificateGeneration(ctx, req, cluster)
	if err != nil {
		log.Error(err, "Failed to ensure certificate generation")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// cleanupResources sletter StatefulSet og alle tilhørende pods
func (r *ClusterReconciler) cleanupResources(ctx context.Context, req ctrl.Request) error {
	logger := log.FromContext(ctx)

	// Delete StatefulSet if it exists
	sts := &appsv1.StatefulSet{}
	err := r.Get(ctx, req.NamespacedName, sts)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("StatefulSet not found, skipping deletion")
		} else {
			logger.Error(err, "Failed to fetch StatefulSet")
			return err
		}
	} else {
		if err := r.Delete(ctx, sts); err != nil {
			if !errors.IsNotFound(err) {
				logger.Error(err, "Failed to delete StatefulSet")
				return err
			}
			logger.Info("StatefulSet already deleted")
		} else {
			logger.Info("StatefulSet deleted successfully")
		}
	}

	// Delete associated pods
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, client.InNamespace(req.Namespace), client.MatchingLabels{"app": req.Name}); err != nil {
		logger.Error(err, "Failed to list pods")
		return err
	}

	for _, pod := range podList.Items {
		if err := r.Delete(ctx, &pod); err != nil {
			if !errors.IsNotFound(err) {
				logger.Error(err, "Failed to delete pod", "Pod", pod.Name)
				return err
			}
			logger.Info("Pod already deleted", "Pod", pod.Name)
		} else {
			logger.Info("Pod deleted successfully", "Pod", pod.Name)
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&instancev1alpha1.Cluster{}).
		Complete(r)
}
