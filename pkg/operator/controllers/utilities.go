package controllers

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	InitContainerName = "initcontainer"

	GameServerKind      = "GameServer"
	GameServerBuildKind = "GameServerBuild"

	DataVolumeName      = "gsdkdata"
	DataVolumeMountPath = "/gsdkdata"

	// MinPort is minimum Port Number
	MinPort int32 = 10000
	// MaxPort is maximum Port Number
	MaxPort int32 = 50000

	RandStringSize = 5

	LabelBuildID          = "BuildID"
	LabelBuildName        = "BuildName"
	LabelOwningGameServer = "OwningGameServer"
	LabelOwningOperator   = "OwningOperator"
	LabelNodeName         = "NodeName"

	GsdkConfigFile             = DataVolumeMountPath + "/Config/gsdkConfig.json"
	LogDirectory               = DataVolumeMountPath + "/GameLogs/"
	CertificatesDirectory      = DataVolumeMountPath + "/GameCertificates"
	GameSharedContentDirectory = DataVolumeMountPath + "/GameSharedContent"

	DaemonSetPort int32 = 56001
)

var InitContainerImage string

func init() {
	rand.Seed(time.Now().UTC().UnixNano()) //randomize name creation

	InitContainerImage = os.Getenv("THUNDERNETES_INIT_CONTAINER_IMAGE")
	if InitContainerImage == "" {
		panic("THUNDERNETES_INIT_CONTAINER_IMAGE cannot be empty")
	}
}

// generateName generates a random string concatenated with prefix and a dash
func generateName(prefix string) string {
	return prefix + "-" + randString(RandStringSize)
}

// randString creates a random string with lowercase characters
func randString(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyz")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// GetPublicIPForNode returns the Public IP of the node
// if the Node does not have a Public IP, method returns the internal one
func GetPublicIPForNode(ctx context.Context, r client.Reader, nodeName string) (string, error) {
	log := log.FromContext(ctx)
	var node corev1.Node
	if err := r.Get(ctx, client.ObjectKey{Name: nodeName}, &node); err != nil {
		return "", err
	}

	for _, x := range node.Status.Addresses {
		if x.Type == corev1.NodeExternalIP {
			return x.Address, nil
		}
	}
	log.Info(fmt.Sprintf("Node with name %s does not have a Public IP, will try to return the internal IP", nodeName))
	// externalIP not found, try InternalIP
	for _, x := range node.Status.Addresses {
		if x.Type == corev1.NodeInternalIP {
			return x.Address, nil
		}
	}

	return "", fmt.Errorf("node %s does not have a Public or Internal IP", nodeName)
}

// NewGameServerForGameServerBuild creates a GameServer for a GameServerBuild
func NewGameServerForGameServerBuild(gsb *mpsv1alpha1.GameServerBuild, portRegistry *PortRegistry) (*mpsv1alpha1.GameServer, error) {
	gs := &mpsv1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateName(gsb.Name),
			Namespace: gsb.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(gsb, schema.GroupVersionKind{
					Group:   mpsv1alpha1.GroupVersion.Group,
					Version: mpsv1alpha1.GroupVersion.Version,
					Kind:    GameServerBuildKind,
				}),
			},
			Labels: map[string]string{LabelBuildID: gsb.Spec.BuildID, LabelBuildName: gsb.Name},
		},
		Spec: mpsv1alpha1.GameServerSpec{
			Template:      gsb.Spec.Template,
			BuildID:       gsb.Spec.BuildID,
			TitleID:       gsb.Spec.TitleID,
			PortsToExpose: gsb.Spec.PortsToExpose,
			BuildMetadata: gsb.Spec.BuildMetadata,
		},
		// we don't create any status since we have the .Status subresource enabled
	}
	// assigning host ports for all the containers in the Template.Spec
	for i := 0; i < len(gsb.Spec.Template.Spec.Containers); i++ {
		container := gsb.Spec.Template.Spec.Containers[i]
		for i := 0; i < len(container.Ports); i++ {
			if sliceContainsPortToExpose(gsb.Spec.PortsToExpose, container.Name, container.Ports[i].Name) {
				port, err := portRegistry.GetNewPort()
				if err != nil {
					return nil, err
				}
				container.Ports[i].HostPort = port

				// if the user has specified that they want to use the host's network, we override the container port
				if gsb.Spec.Template.Spec.HostNetwork {
					container.Ports[i].ContainerPort = port
				}
			}
		}
	}

	return gs, nil
}

// NewPodForGameServer returns a Kubernetes Pod struct for a specified GameServer
// Pod has the same name as the GameServer
// It also sets a label called "GameServer" with the value of the corresponding GameServer resource
func NewPodForGameServer(gs *mpsv1alpha1.GameServer) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gs.Name, // same Name as the GameServer
			Namespace: gs.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(gs, schema.GroupVersionKind{
					Group:   mpsv1alpha1.GroupVersion.Group,
					Version: mpsv1alpha1.GroupVersion.Version,
					Kind:    GameServerKind,
				}),
			},
		},
		Spec: gs.Spec.Template.Spec,
	}

	// copy Labels and Annotations from Pod Template
	pod.ObjectMeta.Annotations = gs.Spec.Template.Annotations
	pod.ObjectMeta.Labels = gs.Spec.Template.Labels

	// initialize the Labels map if it's nil, so we can add thundernetes Labels
	if pod.ObjectMeta.Labels == nil {
		pod.ObjectMeta.Labels = make(map[string]string)
	}

	// add thundernetes Labels
	pod.ObjectMeta.Labels[LabelBuildID] = gs.Spec.BuildID
	pod.ObjectMeta.Labels[LabelBuildName] = gs.Labels[LabelBuildName]
	pod.ObjectMeta.Labels[LabelOwningGameServer] = gs.Name
	pod.ObjectMeta.Labels[LabelOwningOperator] = "thundernetes"

	// following methods should be called in this exact order
	modifyRestartPolicy(pod)
	createDataVolumeOnPod(pod)
	// attach data volume and env for all containers in the Pod
	for i := 0; i < len(pod.Spec.Containers); i++ {
		attachDataVolumeOnContainer(&pod.Spec.Containers[i])
		pod.Spec.Containers[i].Env = append(pod.Spec.Containers[i].Env, getGameServerEnvVariables(gs)...)
	}
	attachInitContainer(gs, pod)

	return pod
}

func modifyRestartPolicy(pod *corev1.Pod) {
	pod.Spec.RestartPolicy = corev1.RestartPolicyNever
}

// attachInitContainer attaches the init container to the GameServer Pod
func attachInitContainer(gs *mpsv1alpha1.GameServer, pod *corev1.Pod) {
	initcontainer := corev1.Container{
		Name:            InitContainerName,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Image:           InitContainerImage,
		Env:             getInitContainerEnvVariables(gs),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      DataVolumeName,
				MountPath: DataVolumeMountPath,
			},
		},
	}
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, initcontainer)
}

// createDataVolumeOnPod creates a Volume that will be mounted to the GameServer Pod
// The init container writes to this volume whereas the GameServer container reads from it (the GSDK methods)
func createDataVolumeOnPod(pod *corev1.Pod) {
	dataDir := corev1.Volume{
		Name: DataVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	pod.Spec.Volumes = append(pod.Spec.Volumes, dataDir)
}

// attachDataVolumeOnContainer attaches the data volume to the specified container
func attachDataVolumeOnContainer(container *corev1.Container) {
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      DataVolumeName,
		MountPath: DataVolumeMountPath,
	})
}

// getInitContainerEnvVariables returns the environment variables for the init container
func getInitContainerEnvVariables(gs *mpsv1alpha1.GameServer) []corev1.EnvVar {
	envList := []corev1.EnvVar{
		{
			Name:  "HEARTBEAT_ENDPOINT_PORT",
			Value: fmt.Sprintf("%d", DaemonSetPort),
		},
		{
			Name:  "GSDK_CONFIG_FILE",
			Value: GsdkConfigFile,
		},
		{
			Name:  "PF_SHARED_CONTENT_FOLDER",
			Value: GameSharedContentDirectory,
		},
		{
			Name:  "CERTIFICATE_FOLDER",
			Value: CertificatesDirectory,
		},
		{
			Name:  "PF_SERVER_LOG_DIRECTORY",
			Value: LogDirectory,
		},
		{
			Name: "PF_VM_ID",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		},
		{
			Name: "PF_NODE_INTERNAL_IP",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "status.hostIP",
				},
			},
		},
		{
			Name:  "PF_GAMESERVER_NAME", // this becomes SessionHostId in gsdkConfig.json file
			Value: gs.Name,              // GameServer.Name is the same as Pod.Name
		},
		{
			Name:  "PF_GAMESERVER_NAMESPACE",
			Value: gs.Namespace,
		},
	}

	var b bytes.Buffer
	// get game ports
	for _, container := range gs.Spec.Template.Spec.Containers {
		for _, port := range container.Ports {
			containerPort := strconv.Itoa(int(port.ContainerPort))
			hostPort := strconv.Itoa(int(port.HostPort))
			if sliceContainsPortToExpose(gs.Spec.PortsToExpose, container.Name, port.Name) {
				b.WriteString(port.Name + "," + containerPort + "," + hostPort + "?")
			}
		}
	}

	envList = append(envList, corev1.EnvVar{
		Name:  "PF_GAMESERVER_PORTS",
		Value: strings.TrimSuffix(b.String(), "?"),
	})

	var buildMetada string
	for _, metadataItem := range gs.Spec.BuildMetadata {
		buildMetada += metadataItem.Key + "," + metadataItem.Value + "?"
	}
	envList = append(envList, corev1.EnvVar{
		Name:  "PF_GAMESERVER_BUILD_METADATA",
		Value: strings.TrimSuffix(buildMetada, "?"),
	})

	return envList
}

// ger getGameServerEnvVariables returns the environment variables for the GameServer container
func getGameServerEnvVariables(gs *mpsv1alpha1.GameServer) []corev1.EnvVar {
	envList := []corev1.EnvVar{
		{
			Name:  "PF_GAMESERVER_NAME",
			Value: gs.Name,
		},
		{
			Name:  "GSDK_CONFIG_FILE",
			Value: GsdkConfigFile,
		},
		{
			Name:  "PF_GAMESERVER_NAMESPACE",
			Value: gs.Namespace,
		},
		{
			Name:  "PF_BUILD_ID",
			Value: gs.Spec.BuildID,
		},
		{
			Name:  "PF_TITLE_ID",
			Value: gs.Spec.TitleID,
		},
	}

	return envList
}

// sliceContainsPortToExpose returns true if the specific containerName/tuple value is contained in the slice
func sliceContainsPortToExpose(slice []mpsv1alpha1.PortToExpose, containerName, portName string) bool {
	for _, item := range slice {
		if item.ContainerName == containerName && item.PortName == portName {
			return true
		}
	}
	return false
}

// containsString returns true if the specific string value is contained in the slice
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// getContainerHostPortTuples returns a concatenated of hostPort:containerPort tuples
func getContainerHostPortTuples(pod *corev1.Pod) string {
	var ports strings.Builder
	for _, container := range pod.Spec.Containers {
		for _, portInfo := range container.Ports {
			ports.WriteString(fmt.Sprintf("%d:%d,", portInfo.ContainerPort, portInfo.HostPort))
		}
	}
	return strings.TrimSuffix(ports.String(), ",")
}
