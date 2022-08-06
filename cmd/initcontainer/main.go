package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// GsdkConfig is the configuration for the GSDK
// it will be written to the file that will be read by the GSDK running alongside the GameServer
type GsdkConfig struct {
	HeartbeatEndpoint        string                   `json:"heartbeatEndpoint"`
	SessionHostId            string                   `json:"sessionHostId"`
	VmId                     string                   `json:"vmId"`
	LogFolder                string                   `json:"logFolder"`
	CertificateFolder        string                   `json:"certificateFolder"`
	SharedContentFolder      string                   `json:"sharedContentFolder"`
	BuildMetadata            map[string]string        `json:"buildMetadata"`
	GamePorts                map[string]string        `json:"gamePorts"`
	PublicIpV4Address        string                   `json:"publicIpV4Address"`
	GameServerConnectionInfo GameServerConnectionInfo `json:"gameServerConnectionInfo"`
	ServerInstanceNumber     int                      `json:"serverInstanceNumber"` // Not used
	FullyQualifiedDomainName string                   `json:"fullyQualifiedDomainName"`
}

type GameServerConnectionInfo struct {
	PublicIpV4Address      string     `json:"publicIpV4Address"`
	GamePortsConfiguration []GamePort `json:"gamePortsConfiguration"`
}

type GamePort struct {
	Name                 string `json:"name"`
	ServerListeningPort  int    `json:"serverListeningPort"`
	ClientConnectionPort int    `json:"clientConnectionPort"`
}

var (
	heartbeatEndpointPort   string
	gsdkConfigFilePath      string
	sharedContentFolderPath string
	certificateFolderPath   string
	serverLogPath           string
	vmId                    string
	gamePortsString         string
	sessionHostId           string
	crdNamespace            string
	nodeInternalIP          string
	logger                  *log.Entry
)

func main() {
	getGameServerNameNamespaceFromEnv()
	logger = log.WithFields(log.Fields{"GameServerName": sessionHostId, "GameServerNamespace": crdNamespace})

	setLogLevel()

	getGsdkEnvVariables()

	err := createGsdkFolders()
	handleError(err)

	gamePorts, gamePortConfiguration, err := parsePorts()
	if err != nil {
		logger.Fatalf("Could not parse game ports %s", err)
	}
	logger.Debugf("Parsed ports %v", gamePorts)

	buildMetadata := parseBuildMetadata()
	logger.Debugf("Parsed build metadata %v", buildMetadata)

	config := &GsdkConfig{
		HeartbeatEndpoint:   fmt.Sprintf("%s:%s", nodeInternalIP, heartbeatEndpointPort),
		SessionHostId:       sessionHostId,
		VmId:                vmId,
		LogFolder:           serverLogPath,
		CertificateFolder:   certificateFolderPath,
		SharedContentFolder: sharedContentFolderPath,
		BuildMetadata:       buildMetadata,
		GamePorts:           gamePorts,
		PublicIpV4Address:   nodeInternalIP, // this is the internal IP of the node
		GameServerConnectionInfo: GameServerConnectionInfo{
			PublicIpV4Address:      nodeInternalIP,
			GamePortsConfiguration: gamePortConfiguration,
		},
		FullyQualifiedDomainName: "NOT_APPLICABLE",
	}

	logger.Info("Marshalling to JSON")
	configJson, err := json.Marshal(config)
	logger.Debugf("Marshalled JSON: %s", configJson)
	handleError(err)

	logger.Info("Getting and creating folder(s)")
	folderPath := filepath.Dir(gsdkConfigFilePath)
	err = os.MkdirAll(folderPath, os.ModePerm)
	handleError(err)

	logger.Infof("Creating empty GSDK JSON file %s", gsdkConfigFilePath)
	f, err := os.Create(gsdkConfigFilePath)
	handleError(err)
	logger.Debugf("Created empty GSDK JSON file %s", gsdkConfigFilePath)

	logger.Infof("Saving GSDK JSON to file %s", gsdkConfigFilePath)
	_, err = f.Write(configJson)
	handleError(err)
	logger.Debugf("Saved GSDK JSON to file %s", gsdkConfigFilePath)
}

// parseBuildMetadata parses the build metadata from the corresponding environment variable
func parseBuildMetadata() map[string]string {
	buildMetadata := make(map[string]string)
	envMetadata := os.Getenv("PF_GAMESERVER_BUILD_METADATA")
	if envMetadata != "" {
		s := strings.Split(envMetadata, "?")
		for _, s2 := range s {
			if s2 == "" {
				continue
			}
			s3 := strings.Split(s2, ",")
			buildMetadata[s3[0]] = s3[1]
		}
	}
	return buildMetadata
}

// parsePorts parses the portName/containerPort/hostPort from the gamePortsString
func parsePorts() (map[string]string, []GamePort, error) {
	// format is port.Name + "," + containerPort + "," + hostPort + "?" + ...
	// similar to how docker run -p works https://docs.docker.com/config/containers/container-networking/
	s := strings.Split(gamePortsString, "?")
	gamePortConfiguration := make([]GamePort, 0)
	gamePorts := make(map[string]string)
	for _, s2 := range s {
		if s2 == "" {
			continue
		}
		s3 := strings.Split(s2, ",")
		containerPort, err := strconv.Atoi(s3[1])
		if err != nil {
			return nil, nil, errors.Wrapf(err, "could not parse port with portName %s, containerPort %s", s3[0], s3[2])
		}
		hostPort, err := strconv.Atoi(s3[2])
		if err != nil {
			return nil, nil, errors.Wrapf(err, "could not parse port with portName %s, hostPort %s", s3[0], s3[2])
		}

		gamePortConfiguration = append(gamePortConfiguration, GamePort{
			Name:                 s3[0],
			ServerListeningPort:  containerPort,
			ClientConnectionPort: hostPort,
		})
		gamePorts[s3[0]] = strconv.Itoa(containerPort)
	}
	return gamePorts, gamePortConfiguration, nil
}

// handleError panics if error != nil
func handleError(e error) {
	if e != nil {
		logger.Fatalf("panic because error: %s", e)
	}
}

// checkEnvOrFatal panics if the environment variable is not set
func checkEnvOrFatal(envName string, envValue string) {
	if envValue == "" {
		logger.Fatalf("Env %s is empty", envName)
	}
}

// getGameServerNameNamespaceFromEnv gets the game server name and namespace from the environment variables
// we get these variables first so we can initialize the logger
func getGameServerNameNamespaceFromEnv() {
	sessionHostId = os.Getenv("PF_GAMESERVER_NAME")
	if sessionHostId == "" {
		handleError(errors.New("PF_GAMESERVER_NAME is empty"))
	}

	crdNamespace = os.Getenv("PF_GAMESERVER_NAMESPACE")
	if crdNamespace == "" {
		handleError(errors.New("PF_GAMESERVER_NAMESPACE is empty"))
	}
}

// getGsdkEnvVariables gets the GSDK environment variables
func getGsdkEnvVariables() {
	heartbeatEndpointPort = os.Getenv("HEARTBEAT_ENDPOINT_PORT")
	checkEnvOrFatal("HEARTBEAT_ENDPOINT_PORT", heartbeatEndpointPort)

	gsdkConfigFilePath = os.Getenv("GSDK_CONFIG_FILE")
	checkEnvOrFatal("GSDK_CONFIG_FILE", gsdkConfigFilePath)

	sharedContentFolderPath = os.Getenv("PF_SHARED_CONTENT_FOLDER")
	checkEnvOrFatal("PF_SHARED_CONTENT_FOLDER", sharedContentFolderPath)

	certificateFolderPath = os.Getenv("CERTIFICATE_FOLDER")
	checkEnvOrFatal("CERTIFICATE_FOLDER", certificateFolderPath)

	serverLogPath = os.Getenv("PF_SERVER_LOG_DIRECTORY")
	checkEnvOrFatal("PF_SERVER_LOG_DIRECTORY", serverLogPath)

	vmId = os.Getenv("PF_VM_ID")
	checkEnvOrFatal("PF_VM_ID", vmId)

	gamePortsString = os.Getenv("PF_GAMESERVER_PORTS")
	checkEnvOrFatal("PF_GAMESERVER_PORTS", gamePortsString)

	nodeInternalIP = os.Getenv("PF_NODE_INTERNAL_IP")
	checkEnvOrFatal("PF_NODE_INTERNAL_IP", nodeInternalIP)
}

// setLogLevel sets the log level based on the LOG_LEVEL environment variable
func setLogLevel() {
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.TextFormatter{})

	logLevel, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		logLevel = log.InfoLevel
	}

	log.SetLevel(logLevel)
}

// createFolderIfNotExists creates the folder if it does not exist
func createFolderIfNotExists(path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}

// createGsdkFolders creates the folders for the GSDK related methods
func createGsdkFolders() error {
	// for the time being, we create only the server log folder
	err := createFolderIfNotExists(serverLogPath)
	if err != nil {
		return err
	}
	return nil
}
