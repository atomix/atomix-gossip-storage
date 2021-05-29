// Copyright 2020-present Open Networking Foundation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v2beta1

import (
	"context"
	"encoding/json"
	"fmt"
	protocolapi "github.com/atomix/atomix-api/go/atomix/protocol"
	corev2beta1 "github.com/atomix/atomix-controller/pkg/apis/core/v2beta1"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/reference"
	"k8s.io/utils/pointer"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	storagev2beta1 "github.com/atomix/atomix-gossip-storage/pkg/apis/storage/v2beta1"
	"github.com/gogo/protobuf/jsonpb"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	apiPort               = 5678
	defaultImageEnv       = "DEFAULT_NODE_V2BETA1_IMAGE"
	defaultImage          = "atomix/atomix-gossip-storage-node:latest"
	headlessServiceSuffix = "hs"
	appLabel              = "app"
	databaseLabel         = "database"
	appAtomix             = "atomix"
)

const (
	configPath         = "/etc/atomix"
	clusterConfigFile  = "cluster.json"
	protocolConfigFile = "protocol.json"
)

const (
	configVolume = "config"
)

const clusterDomainEnv = "CLUSTER_DOMAIN"

func addGossipProtocolController(mgr manager.Manager) error {
	r := &Reconciler{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
	}

	// Create a new controller
	c, err := controller.New(mgr.GetScheme().Name(), mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to the storage resource and enqueue Clusters that reference it
	err = c.Watch(&source.Kind{Type: &storagev2beta1.GossipProtocol{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource StatefulSet
	err = c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}}, &handler.EnqueueRequestForOwner{
		OwnerType:    &storagev2beta1.GossipProtocol{},
		IsController: true,
	})
	if err != nil {
		return err
	}
	return nil
}

var _ reconcile.Reconciler = &Reconciler{}

// Reconciler reconciles a GossipProtocol object
type Reconciler struct {
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Cluster object and makes changes based on the state read
// and what is in the Cluster.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Info("Reconcile GossipProtocol")
	protocol := &storagev2beta1.GossipProtocol{}
	err := r.client.Get(context.TODO(), request.NamespacedName, protocol)
	if err != nil {
		log.Error(err, "Reconcile GossipProtocol")
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	log.Info("Reconcile Protocol")
	err = r.reconcileProtocol(protocol)
	if err != nil {
		log.Error(err, "Reconcile Protocol")
		return reconcile.Result{}, err
	}

	log.Info("Reconcile ProtocolStatus")
	if ok, err := r.reconcileProtocolStatus(protocol); err != nil {
		log.Error(err, "Reconcile ProtocolStatus")
		return reconcile.Result{}, err
	} else if ok {
		return reconcile.Result{}, nil
	}
	if ok, err := r.reconcilePodStatuses(protocol); err != nil {
		log.Error(err, "Reconcile ProtocolStatus")
		return reconcile.Result{}, err
	} else if ok {
		return reconcile.Result{}, nil
	}
	return reconcile.Result{}, nil
}

func (r *Reconciler) reconcileProtocolStatus(protocol *storagev2beta1.GossipProtocol) (bool, error) {
	replicas, err := r.getProtocolReplicas(protocol)
	if err != nil {
		return false, err
	}

	partitions, err := r.getProtocolPartitions(protocol)
	if err != nil {
		return false, err
	}

	if protocol.Status.ProtocolStatus == nil ||
		isReplicasChanged(protocol.Status.Replicas, replicas) ||
		isPartitionsChanged(protocol.Status.Partitions, partitions) {
		var revision int64
		if protocol.Status.ProtocolStatus != nil {
			revision = protocol.Status.ProtocolStatus.Revision
		}
		if protocol.Status.ProtocolStatus == nil ||
			!isReplicasSame(protocol.Status.Replicas, replicas) ||
			!isPartitionsSame(protocol.Status.Partitions, partitions) {
			revision++
		}
		protocol.Status.ProtocolStatus = &corev2beta1.ProtocolStatus{
			Revision:   revision,
			Replicas:   replicas,
			Partitions: partitions,
		}

		if err := r.client.Status().Update(context.TODO(), protocol); err != nil {
			log.Error(err)
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (r *Reconciler) reconcilePodStatuses(protocol *storagev2beta1.GossipProtocol) (bool, error) {
	var revision int64
	if protocol.Status.ProtocolStatus != nil {
		revision = protocol.Status.ProtocolStatus.Revision
	}

	pods := &corev1.PodList{}
	options := &client.ListOptions{
		Namespace:     protocol.Namespace,
		LabelSelector: labels.SelectorFromSet(newLabels(protocol)),
	}
	if err := r.client.List(context.TODO(), pods, options); err != nil {
		log.Error(err)
		return false, err
	}

	// Iterate through pods belonging to the protocol and update their configurations
	var updated bool
	var returnErr error
	for _, pod := range pods.Items {
		// Find the PodStatus for this Pod
		var podStatus *storagev2beta1.PodStatus
		var podStatusIndex int
		for i, status := range protocol.Status.Pods {
			if status.UID == pod.UID {
				podStatus = &status
				podStatusIndex = i
			}
		}

		// If no PodStatus was found, append a new PodStatus to the protocol status
		if podStatus == nil {
			podCopy := pod
			ref, err := reference.GetReference(r.scheme, &podCopy)
			if err != nil {
				log.Error(err)
				return false, err
			}
			podStatus = &storagev2beta1.PodStatus{
				ObjectReference: *ref,
			}
			podStatusIndex = len(protocol.Status.Pods)
			protocol.Status.Pods = append(protocol.Status.Pods, *podStatus)
		}

		// If the pod has already been updated with this revision, skip it
		if revision <= podStatus.Revision {
			continue
		}

		// Connect to the gossip pod and update the protocol configuration
		conn, err := grpc.Dial(fmt.Sprintf("%s:%d", pod.Status.PodIP, apiPort), grpc.WithInsecure())
		if err != nil {
			log.Error(err)
			returnErr = err
		} else {
			client := protocolapi.NewProtocolConfigServiceClient(conn)
			request := &protocolapi.UpdateConfigRequest{
				Config: r.getProtocolConfig(*protocol.Status.ProtocolStatus),
			}
			_, err = client.UpdateConfig(context.TODO(), request)
			if err != nil {
				log.Error(err)
				returnErr = err
			} else {
				podStatus.Revision = revision
				protocol.Status.Pods[podStatusIndex] = *podStatus
			}
		}
	}

	if !updated {
		return false, returnErr
	}

	if err := r.client.Status().Update(context.TODO(), protocol); err != nil {
		log.Error(err)
		return false, err
	}
	return true, returnErr
}

func isReplicasSame(a, b []corev2beta1.ReplicaStatus) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ar := a[i]
		br := b[i]
		if ar.ID != br.ID {
			return false
		}
	}
	return true
}

func isReplicasChanged(a, b []corev2beta1.ReplicaStatus) bool {
	if len(a) != len(b) {
		return true
	}
	for i := 0; i < len(a); i++ {
		ar := a[i]
		br := b[i]
		if ar.ID != br.ID {
			return true
		}
		if ar.Ready != br.Ready {
			return true
		}
	}
	return false
}

func isPartitionsSame(a, b []corev2beta1.PartitionStatus) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ap := a[i]
		bp := b[i]
		if ap.ID != bp.ID {
			return false
		}
		if len(ap.Replicas) != len(bp.Replicas) {
			return false
		}
		for j := 0; j < len(ap.Replicas); j++ {
			if ap.Replicas[j] != bp.Replicas[j] {
				return false
			}
		}
	}
	return true
}

func isPartitionsChanged(a, b []corev2beta1.PartitionStatus) bool {
	if len(a) != len(b) {
		return true
	}
	for i := 0; i < len(a); i++ {
		ap := a[i]
		bp := b[i]
		if ap.ID != bp.ID {
			return true
		}
		if len(ap.Replicas) != len(bp.Replicas) {
			return true
		}
		for j := 0; j < len(ap.Replicas); j++ {
			if ap.Replicas[j] != bp.Replicas[j] {
				return true
			}
		}
		if ap.Ready != bp.Ready {
			return true
		}
	}
	return false
}

func (r *Reconciler) getProtocolConfig(status corev2beta1.ProtocolStatus) protocolapi.ProtocolConfig {
	var config protocolapi.ProtocolConfig
	for _, replicaStatus := range status.Replicas {
		protocolReplica := protocolapi.ProtocolReplica{
			ID:     replicaStatus.ID,
			NodeID: replicaStatus.NodeID,
		}
		if replicaStatus.Host != nil {
			protocolReplica.Host = *replicaStatus.Host
		}
		if replicaStatus.Port != nil {
			protocolReplica.APIPort = *replicaStatus.Port
		}
		protocolReplica.ExtraPorts = replicaStatus.ExtraPorts
		config.Replicas = append(config.Replicas, protocolReplica)
	}
	for _, partitionStatus := range status.Partitions {
		protocolPartition := protocolapi.ProtocolPartition{
			PartitionID: partitionStatus.ID,
			Replicas:    partitionStatus.Replicas,
		}
		config.Partitions = append(config.Partitions, protocolPartition)
	}
	return config
}

func (r *Reconciler) getProtocolReplicas(protocol *storagev2beta1.GossipProtocol) ([]corev2beta1.ReplicaStatus, error) {
	numReplicas := getReplicas(protocol)
	replicas := make([]corev2beta1.ReplicaStatus, 0, numReplicas)
	for i := 0; i < numReplicas; i++ {
		replicaReady, err := r.isReplicaReady(protocol, i)
		if err != nil {
			return nil, err
		}
		replica := corev2beta1.ReplicaStatus{
			ID:    getPodName(protocol, i),
			Host:  pointer.StringPtr(getPodDNSName(protocol, i)),
			Port:  pointer.Int32Ptr(int32(apiPort)),
			Ready: replicaReady,
		}
		replicas = append(replicas, replica)
	}
	return replicas, nil
}

func (r *Reconciler) getProtocolPartitions(protocol *storagev2beta1.GossipProtocol) ([]corev2beta1.PartitionStatus, error) {
	numReplicas := getReplicas(protocol)
	partitions := make([]corev2beta1.PartitionStatus, 0, protocol.Spec.Partitions)
	for partitionID := 1; partitionID <= int(protocol.Spec.Partitions); partitionID++ {
		partitionReady := true
		replicaNames := make([]string, 0, numReplicas)
		for i := 0; i < numReplicas; i++ {
			replicaReady, err := r.isReplicaReady(protocol, i)
			if err != nil {
				return nil, err
			} else if !replicaReady {
				partitionReady = false
			}
			replicaNames = append(replicaNames, getPodName(protocol, i))
		}
		partition := corev2beta1.PartitionStatus{
			ID:       uint32(partitionID),
			Replicas: replicaNames,
			Ready:    partitionReady,
		}
		partitions = append(partitions, partition)
	}
	return partitions, nil
}

func (r *Reconciler) isReplicaReady(protocol *storagev2beta1.GossipProtocol, replica int) (bool, error) {
	podName := types.NamespacedName{
		Namespace: protocol.Namespace,
		Name:      getPodName(protocol, replica),
	}
	pod := &corev1.Pod{}
	if err := r.client.Get(context.TODO(), podName, pod); err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue, nil
		}
	}
	return false, nil
}

func (r *Reconciler) reconcileProtocol(protocol *storagev2beta1.GossipProtocol) error {
	err := r.reconcileConfigMap(protocol)
	if err != nil {
		return err
	}

	err = r.reconcileStatefulSet(protocol)
	if err != nil {
		return err
	}

	err = r.reconcileService(protocol)
	if err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) reconcileConfigMap(protocol *storagev2beta1.GossipProtocol) error {
	log.Info("Reconcile gossip protocol config map")
	cm := &corev1.ConfigMap{}
	name := types.NamespacedName{
		Namespace: protocol.Namespace,
		Name:      protocol.Name,
	}
	err := r.client.Get(context.TODO(), name, cm)
	if err != nil && k8serrors.IsNotFound(err) {
		err = r.addConfigMap(protocol)
	}
	return err
}

func (r *Reconciler) addConfigMap(protocol *storagev2beta1.GossipProtocol) error {
	log.Info("Creating gossip ConfigMap", "Name", protocol.Name, "Namespace", protocol.Namespace)
	var config interface{}

	clusterConfig, err := newNodeConfigString(protocol)
	if err != nil {
		return err
	}

	protocolConfig, err := newProtocolConfigString(config)
	if err != nil {
		return err
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      protocol.Name,
			Namespace: protocol.Namespace,
			Labels:    newLabels(protocol),
		},
		Data: map[string]string{
			clusterConfigFile:  clusterConfig,
			protocolConfigFile: protocolConfig,
		},
	}

	if err := controllerutil.SetControllerReference(protocol, cm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), cm)
}

// newNodeConfigString creates a node configuration string for the given cluster
func newNodeConfigString(protocol *storagev2beta1.GossipProtocol) (string, error) {
	replicas := make([]protocolapi.ProtocolReplica, protocol.Spec.Replicas)
	replicaNames := make([]string, protocol.Spec.Replicas)
	for i := 0; i < getReplicas(protocol); i++ {
		replicas[i] = protocolapi.ProtocolReplica{
			ID:      getPodName(protocol, i),
			Host:    getPodDNSName(protocol, i),
			APIPort: apiPort,
		}
		replicaNames[i] = getPodName(protocol, i)
	}

	partitions := make([]protocolapi.ProtocolPartition, 0, protocol.Spec.Partitions)
	for partitionID := 1; partitionID <= int(protocol.Spec.Partitions); partitionID++ {
		partitionReplicas := make([]string, len(replicaNames))
		for i := 0; i < len(replicaNames); i++ {
			partitionReplicas[i] = replicaNames[(partitionID+i)%len(replicaNames)]
		}
		partition := protocolapi.ProtocolPartition{
			PartitionID: uint32(partitionID),
			Replicas:    partitionReplicas,
		}
		partitions = append(partitions, partition)
	}

	config := &protocolapi.ProtocolConfig{
		Replicas:   replicas,
		Partitions: partitions,
	}

	marshaller := jsonpb.Marshaler{}
	return marshaller.MarshalToString(config)
}

// newProtocolConfigString creates a protocol configuration string for the given cluster and protocol
func newProtocolConfigString(config interface{}) (string, error) {
	bytes, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (r *Reconciler) reconcileStatefulSet(protocol *storagev2beta1.GossipProtocol) error {
	log.Info("Reconcile gossip protocol stateful set")
	statefulSet := &appsv1.StatefulSet{}
	name := types.NamespacedName{
		Namespace: protocol.Namespace,
		Name:      protocol.Name,
	}
	err := r.client.Get(context.TODO(), name, statefulSet)
	if err != nil && k8serrors.IsNotFound(err) {
		err = r.addStatefulSet(protocol)
	}
	return err
}

func (r *Reconciler) addStatefulSet(protocol *storagev2beta1.GossipProtocol) error {
	log.Info("Creating gossip replicas", "Name", protocol.Name, "Namespace", protocol.Namespace)

	image := getImage(protocol)
	pullPolicy := protocol.Spec.ImagePullPolicy
	if pullPolicy == "" {
		pullPolicy = corev1.PullIfNotPresent
	}

	volumes := []corev1.Volume{
		{
			Name: configVolume,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: protocol.Name,
					},
				},
			},
		},
	}

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      protocol.Name,
			Namespace: protocol.Namespace,
			Labels:    newLabels(protocol),
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: getHeadlessServiceName(protocol),
			Replicas:    &protocol.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: newLabels(protocol),
			},
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: newLabels(protocol),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "gossip",
							Image:           image,
							ImagePullPolicy: pullPolicy,
							Env: []corev1.EnvVar{
								{
									Name: "NODE_ID",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "api",
									ContainerPort: apiPort,
								},
							},
							Args: []string{
								"$(NODE_ID)",
								fmt.Sprintf("%s/%s", configPath, clusterConfigFile),
								fmt.Sprintf("%s/%s", configPath, protocolConfigFile),
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{"stat", "/tmp/atomix-ready"},
									},
								},
								InitialDelaySeconds: 5,
								TimeoutSeconds:      10,
								FailureThreshold:    12,
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.IntOrString{Type: intstr.Int, IntVal: apiPort},
									},
								},
								InitialDelaySeconds: 60,
								TimeoutSeconds:      10,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      configVolume,
									MountPath: configPath,
								},
							},
						},
					},
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
								{
									Weight: 1,
									PodAffinityTerm: corev1.PodAffinityTerm{
										LabelSelector: &metav1.LabelSelector{
											MatchLabels: newLabels(protocol),
										},
										Namespaces:  []string{protocol.Namespace},
										TopologyKey: "kubernetes.io/hostname",
									},
								},
							},
						},
					},
					ImagePullSecrets: protocol.Spec.ImagePullSecrets,
					Volumes:          volumes,
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(protocol, statefulSet, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), statefulSet)
}

func (r *Reconciler) reconcileService(protocol *storagev2beta1.GossipProtocol) error {
	log.Info("Reconcile gossip protocol headless service")
	service := &corev1.Service{}
	name := types.NamespacedName{
		Namespace: protocol.Namespace,
		Name:      getHeadlessServiceName(protocol),
	}
	err := r.client.Get(context.TODO(), name, service)
	if err != nil && k8serrors.IsNotFound(err) {
		err = r.addService(protocol)
	}
	return err
}

func (r *Reconciler) addService(protocol *storagev2beta1.GossipProtocol) error {
	log.Info("Creating headless gossip service", "Name", protocol.Name, "Namespace", protocol.Namespace)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getHeadlessServiceName(protocol),
			Namespace: protocol.Namespace,
			Labels:    newLabels(protocol),
			Annotations: map[string]string{
				"service.alpha.kubernetes.io/tolerate-unready-endpoints": "true",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: "api",
					Port: apiPort,
				},
			},
			PublishNotReadyAddresses: true,
			ClusterIP:                "None",
			Selector:                 newLabels(protocol),
		},
	}

	if err := controllerutil.SetControllerReference(protocol, service, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), service)
}

// getReplicas returns the number of replicas in the given database
func getReplicas(storage *storagev2beta1.GossipProtocol) int {
	if storage.Spec.Replicas == 0 {
		return 1
	}
	return int(storage.Spec.Replicas)
}

// getResourceName returns the given resource name for the given cluster
func getResourceName(protocol *storagev2beta1.GossipProtocol, resource string) string {
	return fmt.Sprintf("%s-%s", protocol.Name, resource)
}

// getHeadlessServiceName returns the headless service name for the given cluster
func getHeadlessServiceName(protocol *storagev2beta1.GossipProtocol) string {
	return getResourceName(protocol, headlessServiceSuffix)
}

// getPodName returns the name of the pod for the given pod ID
func getPodName(protocol *storagev2beta1.GossipProtocol, pod int) string {
	return fmt.Sprintf("%s-%d", protocol.Name, pod)
}

// getPodDNSName returns the fully qualified DNS name for the given pod ID
func getPodDNSName(protocol *storagev2beta1.GossipProtocol, pod int) string {
	domain := os.Getenv(clusterDomainEnv)
	if domain == "" {
		domain = "cluster.local"
	}
	return fmt.Sprintf("%s-%d.%s.%s.svc.%s", protocol.Name, pod, getHeadlessServiceName(protocol), protocol.Namespace, domain)
}

// newLabels returns the labels for the given cluster
func newLabels(protocol *storagev2beta1.GossipProtocol) map[string]string {
	labels := make(map[string]string)
	for key, value := range protocol.Labels {
		labels[key] = value
	}
	labels[appLabel] = appAtomix
	labels[databaseLabel] = fmt.Sprintf("%s.%s", protocol.Name, protocol.Namespace)
	return labels
}

func getImage(storage *storagev2beta1.GossipProtocol) string {
	if storage.Spec.Image != "" {
		return storage.Spec.Image
	}
	return getDefaultImage()
}

func getDefaultImage() string {
	image := os.Getenv(defaultImageEnv)
	if image == "" {
		image = defaultImage
	}
	return image
}
