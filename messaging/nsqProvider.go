package messaging

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/fairway-corp/swagchat-api/utils"
)

type NsqProvider struct{}

func (provider NsqProvider) Init() error {
	return nil
}

func (provider NsqProvider) PublishMessage(mi *MessagingInfo) error {
	if utils.Cfg.RealtimeServer.Endpoint != "" {
		input, err := json.Marshal(mi.Message)
		resp, err := http.Post(utils.AppendStrings(utils.Cfg.Messaging.RealtimeQue.Endpoint, "/pub?topic=", utils.Cfg.Messaging.RealtimeQue.Topic), "application/json", bytes.NewBuffer(input))
		if err != nil {
			return err
		}
		_, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return errors.New(utils.AppendStrings("http status code[", strconv.Itoa(resp.StatusCode), "]"))
		}
	}
	return nil
}