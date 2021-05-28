package okex

/*
 OKEX websocket API agent
 @author Lingting Fu
 @date 2018-12-27
 @version 1.0.0
*/

import (
	"bytes"
	"compress/flate"
	"io/ioutil"

	"github.com/gorilla/websocket"
	"github.com/kerwinzlb/GridTradingServer/log"

	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"
	"time"
)

type OKWSAgent struct {
	baseUrl string
	config  *Config
	conn    *websocket.Conn

	wsEvtCh  chan interface{}
	wsErrCh  chan interface{}
	wsTbCh   chan interface{}
	stopCh   chan interface{}
	errCh    chan error
	signalCh chan os.Signal

	subMap         map[string][]ReceivedDataCallback
	activeChannels map[string]bool
	hotDepthsMap   map[string]*WSHotDepths

	processMut sync.Mutex
}

func (a *OKWSAgent) Start(config *Config) error {
	a.baseUrl = config.WSEndpoint
	log.Debugf("Connecting to %s", a.baseUrl)
	c, _, err := websocket.DefaultDialer.Dial(a.baseUrl, nil)

	if err != nil {
		log.Debugf("dial:%+v", err)
		return err
	} else {
		if config.IsPrint {
			log.Debugf("Connected to %s", a.baseUrl)
		}
		a.conn = c
		a.config = config

		a.wsEvtCh = make(chan interface{})
		a.wsErrCh = make(chan interface{})
		a.wsTbCh = make(chan interface{})
		a.errCh = make(chan error)
		a.stopCh = make(chan interface{}, 16)
		a.signalCh = make(chan os.Signal)
		a.activeChannels = make(map[string]bool)
		a.subMap = make(map[string][]ReceivedDataCallback)
		a.hotDepthsMap = make(map[string]*WSHotDepths)

		signal.Notify(a.signalCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

		go a.work()
		go a.receive()
		go a.finalize()
	}

	return nil
}

func (a *OKWSAgent) Subscribe(channel, instType string, cb ReceivedDataCallback) error {
	a.processMut.Lock()
	defer a.processMut.Unlock()

	// st := SubscriptionTopic{channel, filter}
	bo, err := subscribeOp(channel, instType)
	if err != nil {
		return err
	}

	msg, err := Struct2JsonString(bo)
	if a.config.IsPrint {
		log.Debugf("Send Msg: %s", msg)
	}
	if err := a.conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
		return err
	}

	cbs := a.subMap[channel]
	if cbs == nil {
		cbs = []ReceivedDataCallback{}
		a.activeChannels[channel] = false
	}

	if cb != nil {
		cbs = append(cbs, cb)
		// fullTopic, _ := st.ToString()
		a.subMap[channel] = cbs
		// a.subMap[fullTopic] = cbs
	}

	return nil
}

func (a *OKWSAgent) UnSubscribe(channel, instType string) error {
	a.processMut.Lock()
	defer a.processMut.Unlock()

	// st := SubscriptionTopic{channel, filter}
	bo, err := unsubscribeOp(channel, instType)
	if err != nil {
		return err
	}

	msg, err := Struct2JsonString(bo)
	if a.config.IsPrint {
		log.Debugf("Send Msg: %s", msg)
	}
	if err := a.conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
		return err
	}

	a.subMap[channel] = nil
	a.activeChannels[channel] = false

	return nil
}

func (a *OKWSAgent) Login(apiKey, passphrase string) error {

	timestamp := EpochTime()

	preHash := PreHashString(timestamp, GET, "/users/self/verify", "")
	if sign, err := HmacSha256Base64Signer(preHash, a.config.SecretKey); err != nil {
		return err
	} else {
		op, err := loginOp(apiKey, passphrase, timestamp, sign)
		data, err := Struct2JsonString(op)
		err = a.conn.WriteMessage(websocket.TextMessage, []byte(data))
		if err != nil {
			return err
		}
		time.Sleep(time.Millisecond * 100)
	}
	return nil
}

func (a *OKWSAgent) keepalive() {
	a.ping()
}

func (a *OKWSAgent) Stop() error {
	defer func() {
		a := recover()
		log.Debugf("Stop End. Recover msg: %+v", a)
	}()

	close(a.stopCh)
	return nil
}

func (a *OKWSAgent) finalize() error {
	defer func() {
		log.Debugf("Finalize End. Connection to WebSocket is closed.")
	}()

	select {
	case <-a.stopCh:
		if a.conn != nil {
			close(a.errCh)
			close(a.wsTbCh)
			close(a.wsEvtCh)
			close(a.wsErrCh)
			return a.conn.Close()
		}
	}

	return nil
}

func (a *OKWSAgent) ping() {
	msg := "ping"
	// log.Debugf("Send Msg: %s", msg)
	a.conn.WriteMessage(websocket.TextMessage, []byte(msg))
}

func (a *OKWSAgent) GzipDecode(in []byte) ([]byte, error) {
	reader := flate.NewReader(bytes.NewReader(in))
	defer reader.Close()

	return ioutil.ReadAll(reader)
}

func (a *OKWSAgent) handleErrResponse(r interface{}) error {
	log.Debugf("handleErrResponse %+v \n", r)
	return nil
}

func (a *OKWSAgent) handleEventResponse(r interface{}) error {
	er := r.(*WSEventResponse)
	a.activeChannels[er.Channel] = (er.Event == CHNL_EVENT_SUBSCRIBE)
	return nil
}

func (a *OKWSAgent) handleTableResponse(r interface{}) error {
	channel := ""
	switch r.(type) {
	case *WSOrdersResponse:
		channel = r.(*WSOrdersResponse).Arg.Channel
	case *WSTableResponse:
		channel = r.(*WSTableResponse).Table
	case *WSDepthTableResponse:
		channel = r.(*WSDepthTableResponse).Table
	}

	cbs := a.subMap[channel]
	if cbs != nil {
		for i := 0; i < len(cbs); i++ {
			cb := cbs[i]
			go cb(r)
			// if err := cb(r); err != nil {
			// 	return err
			// }
		}
	}
	return nil
}

func (a *OKWSAgent) work() {
	defer func() {
		a := recover()
		log.Debugf("Work End. Recover msg: %+v", a)
		debug.PrintStack()
	}()

	defer a.Stop()

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.keepalive()
		case errR := <-a.wsErrCh:
			a.handleErrResponse(errR)
		case evtR := <-a.wsEvtCh:
			a.handleEventResponse(evtR)
		case tb := <-a.wsTbCh:
			a.handleTableResponse(tb)
		case <-a.signalCh:
			break
		case err := <-a.errCh:
			log.Debug("work() a.errCh", err)
			c, _, err := websocket.DefaultDialer.Dial(a.baseUrl, nil)
			if err != nil {
				log.Debugf("dial:%+v", err)
			} else {
				a.conn = c
				a.Login(a.config.ApiKey, a.config.Passphrase)
				a.Subscribe(CHNL_OEDERS, "SPOT", nil)
			}
		case <-a.stopCh:
			return

		}
	}
}

func (a *OKWSAgent) receive() {
	defer func() {
		a := recover()
		if a != nil {
			log.Debugf("Receive End. Recover msg: %+v", a)
			debug.PrintStack()
		}
	}()

	for {
		select {
		case <-a.stopCh:
			return
		}
		messageType, message, err := a.conn.ReadMessage()
		if err != nil {
			a.errCh <- err
		}
		txtMsg := message
		switch messageType {
		case websocket.TextMessage:
		case websocket.BinaryMessage:
			txtMsg, err = a.GzipDecode(message)
		}

		rsp, err := loadResponse(txtMsg)
		if rsp != nil {
			if a.config.IsPrint {
				log.Debugf("LoadedRep: %+v, err: %+v", rsp, err)
			}
		}

		if err != nil {
			continue
		}

		switch rsp.(type) {
		case *WSErrorResponse:
			a.wsErrCh <- rsp
		case *WSEventResponse:
			er := rsp.(*WSEventResponse)
			a.wsEvtCh <- er
		case *WSDepthTableResponse:
			var err error
			dtr := rsp.(*WSDepthTableResponse)
			hotDepths := a.hotDepthsMap[dtr.Table]
			if hotDepths == nil {
				hotDepths = NewWSHotDepths(dtr.Table)
				err = hotDepths.loadWSDepthTableResponse(dtr)
				if err == nil {
					a.hotDepthsMap[dtr.Table] = hotDepths
				}
			} else {
				err = hotDepths.loadWSDepthTableResponse(dtr)
			}

			if err == nil {
				a.wsTbCh <- dtr
			} else {
				log.Debugf("Failed to loadWSDepthTableResponse, dtr: %+v, err: %+v", dtr, err)
			}

		case *WSTableResponse:
			tb := rsp.(*WSTableResponse)
			a.wsTbCh <- tb
		case *WSOrdersResponse:
			ord := rsp.(*WSOrdersResponse)
			a.wsTbCh <- ord
		default:
			//log.Println(rsp)
		}
	}
}
