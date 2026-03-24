package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"

	logrus "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const (
	testGsdkConfigFile        = "/tmp/GsdkConfig.json"
	testHeartbeatEndpointPort = "56001"
	testSharedContentFolder   = "testSharedContentFolder"
	testCertificateFolder     = "testCertificateFolder"
	testLogDirectory          = "testLogDirectory"
	testVmId                  = "testVmId"
	testGameServerName        = "testGameServerName"
	testGameServerNamespace   = "testGameServerNamespace"
	testGameServerPorts       = "portName,80,10000?portName2,443,10001"
	testBuildMetadata         = "key1,value1?key2,value2"
	testNodeInternalIP        = "127.0.0.1"
)

type initContainerTestSuite struct {
	suite.Suite
}

func (suite *initContainerTestSuite) SetupSuite() {
	log.Println("Setting env variables")
	setEnvVariables()
}

func (suite *initContainerTestSuite) TearDownSuite() {
	log.Println("Unsetting env variables")
	unsetEnvVariables()
	err := os.Remove(testGsdkConfigFile)
	assert.NoError(suite.T(), err)
}

// TestInitContainer tests the core functionality of the init container
// which is to create a proper GSDK file
func (suite *initContainerTestSuite) TestInitContainer() {
	main()
	assert.FileExists(suite.T(), testGsdkConfigFile)
	jsonFile, err := os.Open(testGsdkConfigFile)
	assert.NoError(suite.T(), err)
	defer jsonFile.Close()
	byteValue, err := io.ReadAll(jsonFile)
	assert.NoError(suite.T(), err)
	var gsdkConfig *GsdkConfig
	err = json.Unmarshal(byteValue, &gsdkConfig)
	assert.NoError(suite.T(), err)

	assert.Equal(suite.T(), fmt.Sprintf("%s:%s", nodeInternalIP, heartbeatEndpointPort), gsdkConfig.HeartbeatEndpoint)
	assert.Equal(suite.T(), testSharedContentFolder, gsdkConfig.SharedContentFolder)
	assert.Equal(suite.T(), testCertificateFolder, gsdkConfig.CertificateFolder)
	assert.Equal(suite.T(), testLogDirectory, gsdkConfig.LogFolder)
	assert.Equal(suite.T(), testVmId, gsdkConfig.VmId)
	assert.Equal(suite.T(), testGameServerName, gsdkConfig.SessionHostId)
	assert.Equal(suite.T(), testNodeInternalIP, gsdkConfig.PublicIpV4Address)
	assert.Equal(suite.T(), testNodeInternalIP, gsdkConfig.GameServerConnectionInfo.PublicIpV4Address)

	portsMap, ports, err := parsePorts()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), portsMap, map[string]string{"portName": "80", "portName2": "443"})
	assert.Equal(suite.T(), ports, []GamePort{{Name: "portName", ServerListeningPort: 80, ClientConnectionPort: 10000}, {Name: "portName2", ServerListeningPort: 443, ClientConnectionPort: 10001}})

	metadata := parseBuildMetadata()
	assert.Equal(suite.T(), metadata, map[string]string{"key1": "value1", "key2": "value2"})
}

// TestInitContainerSuite launches the test suite
func TestInitContainerSuite(t *testing.T) {
	suite.Run(t, new(initContainerTestSuite))
}

func setEnvVariables() {
	os.Setenv("GSDK_CONFIG_FILE", testGsdkConfigFile)
	os.Setenv("HEARTBEAT_ENDPOINT_PORT", testHeartbeatEndpointPort)
	os.Setenv("PF_SHARED_CONTENT_FOLDER", testSharedContentFolder)
	os.Setenv("CERTIFICATE_FOLDER", testCertificateFolder)
	os.Setenv("PF_SERVER_LOG_DIRECTORY", testLogDirectory)
	os.Setenv("PF_VM_ID", testVmId)
	os.Setenv("PF_GAMESERVER_NAME", testGameServerName)
	os.Setenv("PF_GAMESERVER_NAMESPACE", testGameServerNamespace)
	os.Setenv("PF_GAMESERVER_PORTS", testGameServerPorts)
	os.Setenv("PF_GAMESERVER_BUILD_METADATA", testBuildMetadata)
	os.Setenv("PF_NODE_INTERNAL_IP", testNodeInternalIP)
}

func unsetEnvVariables() {
	os.Unsetenv("GSDK_CONFIG_FILE")
	os.Unsetenv("HEARTBEAT_ENDPOINT_PORT")
	os.Unsetenv("PF_SHARED_CONTENT_FOLDER")
	os.Unsetenv("CERTIFICATE_FOLDER")
	os.Unsetenv("PF_SERVER_LOG_DIRECTORY")
	os.Unsetenv("PF_VM_ID")
	os.Unsetenv("PF_GAMESERVER_NAME")
	os.Unsetenv("PF_GAMESERVER_NAMESPACE")
	os.Unsetenv("PF_GAMESERVER_PORTS")
	os.Unsetenv("PF_GAMESERVER_BUILD_METADATA")
	os.Unsetenv("PF_NODE_INTERNAL_IP")
}

func TestParsePorts(t *testing.T) {
	originalGamePortsString := gamePortsString
	t.Cleanup(func() { gamePortsString = originalGamePortsString })

	t.Run("valid ports", func(t *testing.T) {
		gamePortsString = "portName,80,10000?portName2,443,10001"
		ports, portConfig, err := parsePorts()
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{"portName": "80", "portName2": "443"}, ports)
		assert.Equal(t, []GamePort{
			{Name: "portName", ServerListeningPort: 80, ClientConnectionPort: 10000},
			{Name: "portName2", ServerListeningPort: 443, ClientConnectionPort: 10001},
		}, portConfig)
	})

	t.Run("invalid container port", func(t *testing.T) {
		gamePortsString = "portName,abc,10000"
		_, _, err := parsePorts()
		assert.Error(t, err)
	})

	t.Run("invalid host port", func(t *testing.T) {
		gamePortsString = "portName,80,abc"
		_, _, err := parsePorts()
		assert.Error(t, err)
	})

	t.Run("trailing separator", func(t *testing.T) {
		gamePortsString = "portName,80,10000?"
		ports, portConfig, err := parsePorts()
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{"portName": "80"}, ports)
		assert.Equal(t, []GamePort{
			{Name: "portName", ServerListeningPort: 80, ClientConnectionPort: 10000},
		}, portConfig)
	})

	t.Run("single port", func(t *testing.T) {
		gamePortsString = "gamePort,8080,20000"
		ports, portConfig, err := parsePorts()
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{"gamePort": "8080"}, ports)
		assert.Equal(t, []GamePort{
			{Name: "gamePort", ServerListeningPort: 8080, ClientConnectionPort: 20000},
		}, portConfig)
	})
}

func TestParseBuildMetadata(t *testing.T) {
	t.Run("valid metadata", func(t *testing.T) {
		t.Setenv("PF_GAMESERVER_BUILD_METADATA", "key1,value1?key2,value2")
		metadata := parseBuildMetadata()
		assert.Equal(t, map[string]string{"key1": "value1", "key2": "value2"}, metadata)
	})

	t.Run("empty metadata", func(t *testing.T) {
		t.Setenv("PF_GAMESERVER_BUILD_METADATA", "")
		metadata := parseBuildMetadata()
		assert.Equal(t, map[string]string{}, metadata)
	})

	t.Run("single entry", func(t *testing.T) {
		t.Setenv("PF_GAMESERVER_BUILD_METADATA", "key1,value1")
		metadata := parseBuildMetadata()
		assert.Equal(t, map[string]string{"key1": "value1"}, metadata)
	})

	t.Run("trailing separator", func(t *testing.T) {
		t.Setenv("PF_GAMESERVER_BUILD_METADATA", "key1,value1?")
		metadata := parseBuildMetadata()
		assert.Equal(t, map[string]string{"key1": "value1"}, metadata)
	})
}

func TestCreateFolderIfNotExists(t *testing.T) {
	t.Run("creates new folder", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "newFolder")
		err := createFolderIfNotExists(dir)
		assert.NoError(t, err)
		assert.DirExists(t, dir)
	})

	t.Run("existing folder no error", func(t *testing.T) {
		dir := t.TempDir()
		err := createFolderIfNotExists(dir)
		assert.NoError(t, err)
		assert.DirExists(t, dir)
	})
}

func TestSetLogLevel(t *testing.T) {
	originalLevel := logrus.GetLevel()
	t.Cleanup(func() { logrus.SetLevel(originalLevel) })

	t.Run("debug level", func(t *testing.T) {
		t.Setenv("LOG_LEVEL", "debug")
		setLogLevel()
		assert.Equal(t, logrus.DebugLevel, logrus.GetLevel())
	})

	t.Run("empty defaults to info", func(t *testing.T) {
		t.Setenv("LOG_LEVEL", "")
		setLogLevel()
		assert.Equal(t, logrus.InfoLevel, logrus.GetLevel())
	})

	t.Run("invalid defaults to info", func(t *testing.T) {
		t.Setenv("LOG_LEVEL", "invalid")
		setLogLevel()
		assert.Equal(t, logrus.InfoLevel, logrus.GetLevel())
	})
}

func TestCreateGsdkFolders(t *testing.T) {
	originalServerLogPath := serverLogPath
	t.Cleanup(func() { serverLogPath = originalServerLogPath })

	t.Run("creates server log directory", func(t *testing.T) {
		serverLogPath = filepath.Join(t.TempDir(), "serverLogs")
		err := createGsdkFolders()
		assert.NoError(t, err)
		assert.DirExists(t, serverLogPath)
	})

	t.Run("no error when directory already exists", func(t *testing.T) {
		serverLogPath = t.TempDir() // already exists
		err := createGsdkFolders()
		assert.NoError(t, err)
		assert.DirExists(t, serverLogPath)
	})
}

func TestParseBuildMetadataMultipleSeparators(t *testing.T) {
	t.Run("three metadata entries", func(t *testing.T) {
		t.Setenv("PF_GAMESERVER_BUILD_METADATA", "k1,v1?k2,v2?k3,v3")
		metadata := parseBuildMetadata()
		assert.Equal(t, map[string]string{"k1": "v1", "k2": "v2", "k3": "v3"}, metadata)
	})

	t.Run("multiple consecutive separators", func(t *testing.T) {
		t.Setenv("PF_GAMESERVER_BUILD_METADATA", "k1,v1??k2,v2")
		metadata := parseBuildMetadata()
		assert.Equal(t, map[string]string{"k1": "v1", "k2": "v2"}, metadata)
	})

	t.Run("leading separator", func(t *testing.T) {
		t.Setenv("PF_GAMESERVER_BUILD_METADATA", "?k1,v1")
		metadata := parseBuildMetadata()
		assert.Equal(t, map[string]string{"k1": "v1"}, metadata)
	})

	t.Run("leading and trailing separators", func(t *testing.T) {
		t.Setenv("PF_GAMESERVER_BUILD_METADATA", "?k1,v1?k2,v2?")
		metadata := parseBuildMetadata()
		assert.Equal(t, map[string]string{"k1": "v1", "k2": "v2"}, metadata)
	})

	t.Run("value containing comma", func(t *testing.T) {
		t.Setenv("PF_GAMESERVER_BUILD_METADATA", "key,val1,val2")
		metadata := parseBuildMetadata()
		// splits on comma: index 0 is key, index 1 is val1; extra elements are ignored
		assert.Equal(t, "val1", metadata["key"])
		assert.Len(t, metadata, 1)
	})
}

func TestParsePortsMultiplePorts(t *testing.T) {
	originalGamePortsString := gamePortsString
	t.Cleanup(func() { gamePortsString = originalGamePortsString })

	t.Run("three ports", func(t *testing.T) {
		gamePortsString = "game,8080,30000?query,8081,30001?rcon,27015,30002"
		ports, portConfig, err := parsePorts()
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{"game": "8080", "query": "8081", "rcon": "27015"}, ports)
		assert.Equal(t, []GamePort{
			{Name: "game", ServerListeningPort: 8080, ClientConnectionPort: 30000},
			{Name: "query", ServerListeningPort: 8081, ClientConnectionPort: 30001},
			{Name: "rcon", ServerListeningPort: 27015, ClientConnectionPort: 30002},
		}, portConfig)
	})

	t.Run("four ports", func(t *testing.T) {
		gamePortsString = "a,1,100?b,2,200?c,3,300?d,4,400"
		ports, portConfig, err := parsePorts()
		assert.NoError(t, err)
		assert.Len(t, ports, 4)
		assert.Len(t, portConfig, 4)
		assert.Equal(t, "1", ports["a"])
		assert.Equal(t, "4", ports["d"])
		assert.Equal(t, 300, portConfig[2].ClientConnectionPort)
	})
}

func TestCreateFolderIfNotExistsNestedPath(t *testing.T) {
	t.Run("fails for deeply nested non-existent parents", func(t *testing.T) {
		base := t.TempDir()
		deepPath := filepath.Join(base, "a", "b", "c", "d")
		err := createFolderIfNotExists(deepPath)
		assert.Error(t, err)
	})
}

func TestGsdkConfigJsonStructure(t *testing.T) {
	config := &GsdkConfig{
		HeartbeatEndpoint:   "127.0.0.1:56001",
		SessionHostId:       "testHost",
		VmId:                "vm1",
		LogFolder:           "/logs",
		CertificateFolder:   "/certs",
		SharedContentFolder: "/shared",
		BuildMetadata:       map[string]string{"key": "val"},
		GamePorts:           map[string]string{"game": "8080"},
		PublicIpV4Address:   "10.0.0.1",
		GameServerConnectionInfo: GameServerConnectionInfo{
			PublicIpV4Address: "10.0.0.1",
			GamePortsConfiguration: []GamePort{
				{Name: "game", ServerListeningPort: 8080, ClientConnectionPort: 30000},
			},
		},
		ServerInstanceNumber:     0,
		FullyQualifiedDomainName: "NOT_APPLICABLE",
	}

	data, err := json.Marshal(config)
	assert.NoError(t, err)

	var raw map[string]json.RawMessage
	err = json.Unmarshal(data, &raw)
	assert.NoError(t, err)

	expectedKeys := []string{
		"heartbeatEndpoint",
		"sessionHostId",
		"vmId",
		"logFolder",
		"certificateFolder",
		"sharedContentFolder",
		"buildMetadata",
		"gamePorts",
		"publicIpV4Address",
		"gameServerConnectionInfo",
		"serverInstanceNumber",
		"fullyQualifiedDomainName",
	}
	for _, key := range expectedKeys {
		assert.Contains(t, raw, key, "JSON should contain key %q", key)
	}
	assert.Len(t, raw, len(expectedKeys))

	// Verify nested gameServerConnectionInfo structure
	var connInfo map[string]json.RawMessage
	err = json.Unmarshal(raw["gameServerConnectionInfo"], &connInfo)
	assert.NoError(t, err)
	assert.Contains(t, connInfo, "publicIpV4Address")
	assert.Contains(t, connInfo, "gamePortsConfiguration")
}
