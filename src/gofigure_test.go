package gofigure

import (
	"io/ioutil"
	"os"
	"testing"
)

type config struct {
	Simple struct {
		Username string
		Password string
	}

	AwsPlaceholders struct {
		FooBar string
	} `yaml:"awsPlaceholders"`

	EnvPlaceholders struct {
		V string `yaml:"helloWorld"`
	} `yaml:"envPlaceholders"`
}

type mockFileService struct {
}

func (fileService mockFileService) GetContents(name string) ([]byte, error) {
	return ioutil.ReadFile(name)
}

func TestPublicApi_Build(t *testing.T) {
	_ = os.Setenv("HELLO_WORLD", "hello world")

	fileService := new(mockFileService)
	gofig, err := Build(fileService, "src/gofigure/samples/devoverride", "config.yml", "config.dev.yml")
	if err != nil {
		t.Error(err)
	}

	target := *gofig.Configuration()
	simple := *(target["simple"].(*map[string]interface{}))
	if "postgres" != simple["username"] {
		t.Fail()
	}
	if "password" != simple["password"] {
		t.Fail()
	}

	aws := *(target["awsPlaceholders"]).(*map[string]interface{})
	if "hello world" != aws["foobar"] {
		t.Fail()
	}

	env := *(target["envPlaceholders"]).(*map[string]interface{})
	if "hello world" != env["helloWorld"] {
		t.Fail()
	}
}

func TestPublicApi_Load(t *testing.T) {
	_ = os.Setenv("HELLO_WORLD", "hello world")

	fileService := new(mockFileService)
	target := &config{}
	err := Load(fileService, target, "src/gofigure/samples/devoverride", "config.yml", "config.dev.yml")
	if err != nil {
		t.Error(err)
	}

	simple := target.Simple
	if "postgres" != simple.Username {
		t.Fail()
	}
	if "password" != simple.Password {
		t.Fail()
	}

	aws := target.AwsPlaceholders
	if "hello world" != aws.FooBar {
		t.Fail()
	}

	env := target.EnvPlaceholders
	if "hello world" != env.V {
		t.Fail()
	}
}

func TestParseBlob(t *testing.T) {
	var data = `
db:
    username: ssm:/dev/foo/bar/db_username
    password: ssm:/dev/foo/bar/db_password
app:
    env: env:APP_ENVIRONMENT
`
	blobs := map[string][]byte{
		"config.yaml": []byte(data),
	}
	configs, err := parseBlobs(blobs)
	if err != nil {
		t.Error(err)
	}

	if len(configs) != 1 {
		t.Fail()
	}
}

func TestParseBlob_MultiBlobs(t *testing.T) {
	var config = `
db:
    username: ssm:/local/foo/bar/db_username
    password: ssm:/local/foo/bar/db_password
`
	var configDev = `
db:
    username: ssm:/dev/foo/bar/db_username
    password: ssm:/dev/foo/bar/db_password
`
	blobs := map[string][]byte{
		"config.dev.yml": []byte(configDev),
		"config.yml":     []byte(config),
	}
	configs, err := parseBlobs(blobs)
	if err != nil {
		t.Error(err)
	}

	if len(configs) != 2 {
		t.Fail()
	}

	index := 0
	for key := range configs {
		if key == "config.dev.yml" {
			break
		}

		index++
	}

	if index == 0 {
		t.Fail()
	}
}

func TestMergeConfigs_DevConfigIntoBase(t *testing.T) {
	var config = `
db:
    username: ssm:/local/foo/bar/db_username
    password: ssm:/local/foo/bar/db_password
`
	var configDev = `
db:
    username: ssm:/dev/foo/bar/db_username
    password: ssm:/dev/foo/bar/db_password
`
	blobs := map[string][]byte{
		"config.yml":     []byte(config),
		"config.dev.yml": []byte(configDev),
	}
	parsed, _ := parseBlobs(blobs)

	expected := map[string]interface{}{
		"db": map[string]string{
			"username": "ssm:/dev/foo/bar/db_username",
			"password": "ssm:/dev/foo/bar/db_password",
		},
	}
	merged := *mergeConfigs(parsed)

	targetDb := merged["db"].(map[interface{}]interface{})

	targetUsername := targetDb["username"]
	expectedUsername := expected["db"].(map[string]string)["username"]
	if expectedUsername != targetUsername {
		t.Fail()
	}

	targetPassword := targetDb["password"]
	expectedPassword := expected["db"].(map[string]string)["password"]
	if expectedPassword != targetPassword {
		t.Fail()
	}
}

func TestReplacePlaceholders_SSMParameters(t *testing.T) {
	config := map[interface{}]interface{}{
		"foo": map[interface{}]interface{}{
			"bar": "ssm:/dev/unittesting/foo/bar",
		},
	}

	targetPrt, err := replacePlaceholders(&config)
	if err != nil {
		t.Fail()
	}
	target := *targetPrt
	targetFooPtr := target["foo"].(*map[string]interface{})
	targetFoo := *targetFooPtr
	targetBar := targetFoo["bar"].(string)

	expected := "hello world"
	if expected != targetBar {
		t.Fail()
	}
}

func TestReplacePlaceholders_EnvParameter(t *testing.T) {
	config := map[interface{}]interface{}{
		"foo": map[interface{}]interface{}{
			"bar": "env:GOFIG_UNITEST",
		},
	}

	expected := "hello world"
	_ = os.Setenv("GOFIG_UNITEST", expected)
	targetPtr, err := replacePlaceholders(&config)
	if err != nil {
		t.Fail()
	}
	target := *targetPtr
	targetFooPtr := target["foo"].(*map[string]interface{})
	targetFoo := *targetFooPtr
	targetBar := targetFoo["bar"].(string)

	if expected != targetBar {
		t.Fail()
	}
}
