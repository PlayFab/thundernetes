package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"

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
	byteValue, err := ioutil.ReadAll(jsonFile)
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
