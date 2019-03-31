package gofigure

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"gopkg.in/yaml.v2"
	"os"
	"regexp"
	"sort"
	"strings"
)

type Gofigure struct {
	configuration *map[string]interface{}
}

type GofigureFileService interface {
	GetContents(path string) ([]byte, error)
}

func (g *Gofigure) Configuration() *map[string]interface{} {
	return g.configuration
}

func Load(fileService GofigureFileService, out interface{}, dirname string, configFiles ...string) error {
	gofigure, err := Build(fileService, dirname, configFiles...)
	if err != nil {
		return err
	}

	err = gofigure.convertToDestination(out)
	return err
}

func Build(fileService GofigureFileService, dirname string, configFiles ...string) (gofigure *Gofigure, err error) {
	blobs, err := readFiles(fileService, dirname, configFiles)
	if err != nil {
		return nil, err
	}

	configs, err := parseBlobs(blobs)
	if err != nil {
		return nil, err
	}

	configWithPlaceholders := mergeConfigs(configs)
	config, err := replacePlaceholders(configWithPlaceholders)
	if err != nil {
		return nil, err
	}

	gofigure = &Gofigure{configuration: config}
	return gofigure, nil
}

func readFiles(fileService GofigureFileService, dirname string, configFiles []string) (blobs map[string][]byte, err error) {
	blobs = make(map[string][]byte)
	for _, filename := range configFiles {

		file, err := fileService.GetContents(fmt.Sprintf("%v/%v", dirname, filename))
		if err != nil {
			return nil, err
		}

		blobs[strings.ToLower(filename)] = file
	}

	return blobs, nil
}

func parseBlobs(blobs map[string][]byte) (configs map[string]map[string]interface{}, err error) {
	configs = make(map[string]map[string]interface{})
	keys := []string{}
	for key := range blobs {
		keys = append(keys, key)
	}

	sort.Sort(sort.Reverse(sort.StringSlice(keys)))

	for _, key := range keys {
		contents := make(map[string]interface{})
		bytes := blobs[key]
		err := yaml.Unmarshal(bytes, &contents)
		if err != nil {
			return nil, err
		}

		configs[key] = contents
	}

	return configs, nil
}

func mergeConfigs(maps map[string]map[string]interface{}) *map[interface{}]interface{} {
	config := make(map[interface{}]interface{})
	for _, configFile := range maps {
		for key, value := range configFile {
			config[key] = value
		}
	}
	return &config
}

func replacePlaceholders(config *map[interface{}]interface{}) (*map[string]interface{}, error) {
	ssmPattern, _ := regexp.Compile("(^|\\A)?(ssm:)")
	envPattern, _ := regexp.Compile("(^|\\A)?(env:)")
	mapped := make(map[string]interface{})
	for keyInterface, value := range *config {
		key := keyInterface.(string)
		unboxedMap, ok := value.(map[interface{}]interface{})
		if ok {
			inner, err := replacePlaceholders(&unboxedMap)
			if err != nil {
				return nil, err
			}
			mapped[key] = inner
			continue
		}

		param := value.(string)

		if index := ssmPattern.FindStringIndex(param); index != nil {
			ssmParamName := param[index[1]:]
			ssmParameter, err := loadSSMParameter(&ssmParamName)
			if err != nil {
				return nil, err
			}
			mapped[key] = *ssmParameter
			continue
		}

		if index := envPattern.FindStringIndex(param); index != nil {
			envParamName := param[index[1]:]
			envParam := os.Getenv(envParamName)
			mapped[key] = envParam
			continue
		}

		mapped[key] = param
	}

	return &mapped, nil
}

func loadSSMParameter(paramName *string) (secret *string, err error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		Config:            aws.Config{Region: aws.String("ap-southeast-2")},
		SharedConfigState: session.SharedConfigEnable,
	})

	ssmsvc := ssm.New(sess, aws.NewConfig().WithRegion("ap-southeast-2"))
	withDecryption := true
	param, err := ssmsvc.GetParameter(&ssm.GetParameterInput{
		Name:           paramName,
		WithDecryption: &withDecryption,
	})
	if err != nil {
		return nil, err
	}

	return param.Parameter.Value, nil
}

func (g *Gofigure) convertToDestination(out interface{}) error {
	bytes, err := yaml.Marshal(g.Configuration())
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(bytes, out)
	if err != nil {
		return err
	}
	return nil
}
