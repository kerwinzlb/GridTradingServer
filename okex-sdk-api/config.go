package okex

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/kerwinzlb/GridTradingServer/log"
)

/*
 OKEX api config info
 @author Tony Tian
 @date 2018-03-17
 @version 1.0.0
*/

type Config struct {
	// Rest api endpoint url. eg: http://www.okex.com/
	Endpoint string

	// Rest websocket api endpoint url. eg: ws://192.168.80.113:10442/
	WSEndpoint string

	//mongo api endpoint url
	MgoEndpoint string

	WsServerAddr string //websocket server地址
	WsServerPort int    //websocket server端口

	// The user's api key provided by OKEx.
	ApiKey string
	// The user's secret key provided by OKEx. The secret key used to sign your request data.
	SecretKey string
	// The Passphrase will be provided by you to further secure your API access.
	Passphrase string
	// Http request timeout.
	TimeoutSecond int
	// Whether to print API information
	IsPrint bool
	// Internationalization @see file: constants.go
	I18n string
}

// GetConfiguration: read config from .json file
// 1) No config file, using default value, don't create new file;
// 2) has config file, error in reading config, stop and display correct info;
// 3) has config file, no error in reading config, go ahead load and run;
func GetConfiguration(configFilePath string) (*Config, error) {
	//check the file path
	filePath, _ := filepath.Abs(configFilePath)
	log.Debugf("load vnode config file from %v", filePath)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		//If no config file exists, return nil
		log.Errorf("%v not exists\nUse default settings", configFilePath)
		return nil, err
	}

	if _, err := os.Stat(configFilePath); err != nil {

		log.Errorf("Open %v error: \n%v\n", configFilePath, err)

		return nil, err
	}

	return GetUserConfig(configFilePath)
}

//GetUserConfig -- read the user configuration in json file
func GetUserConfig(filepath string) (*Config, error) {
	// Open our jsonFile
	jsonFile, err := os.Open(filepath)
	// if we os.Open returns an error then handle it
	if err != nil {
		return nil, err
	}

	log.Debugf("Successfully Opened %v", filepath)
	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()

	// read our opened xmlFile as a byte array.
	byteValue, _ := ioutil.ReadAll(jsonFile)

	// initialize the Confgiruation array
	var conf Config

	// we unmarshal our byteArray which contains our
	// jsonFile's content into 'users' which we defined above
	json.Unmarshal(byteValue, &conf)

	return &conf, nil
}
