package server

import (
	"encoding/hex"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
	"sync"
	"sync/atomic"
	"reflect"

	"github.com/kerwinzlb/GridTradingServer/common"
	"github.com/kerwinzlb/GridTradingServer/db"
	"github.com/kerwinzlb/GridTradingServer/log"
	"github.com/kerwinzlb/GridTradingServer/okex-sdk-api"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	MGO_DB_NAME                = "grid"
	MGO_COLLECTION_CONFIG_NAME = "config"
	MGO_COLLECTION_TICKET_NAME = "ticket"
	MGO_COLLECTION_ORDER_NAME  = "order"
)

type Server struct {
	conf         *okex.Config
	mgoConf      atomic.Value
	instId       string
	tickSzN      int
	lotSzN       int
	minSzN       int
	gridNum      int
	restClient   *okex.Client
	wsClient     *okex.OKWSAgent
	mgo          *db.Mongo
	status       atomic.Value
	lock         *sync.RWMutex

	stop chan struct{} // Channel to wait for termination notifications
}

func New(instId string, conf *okex.Config) (*Server, error) {
	confCopy := *conf
	s := &Server{
		conf:         &confCopy,
		instId:       instId,
		restClient:   okex.NewClient(*conf),
		mgo:          db.NewMgo(conf.MgoEndpoint),
		lock:         new(sync.RWMutex),
		stop:         make(chan struct{}),
	}

	s.wsClient = okex.NewAgent(conf, s.Stop)
	s.status.Store(0)
	pubRes, err := s.restClient.GetPublicInstruments(okex.SPOT, "", s.instId)
	if err != nil {
		return nil, err
	}
	s.tickSzN = common.FloatRoundLen(pubRes.Data[0].TickSz)
	s.lotSzN = common.FloatRoundLen(pubRes.Data[0].LotSz)
	s.minSzN = common.FloatRoundLen(pubRes.Data[0].MinSz)

	err = s.mgo.Connect()
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Server) getSz(pric float64, side string, conf Config) (string, string) {
	sz := ""
	if conf.Mode == 0 {
		if side == "buy" {
			sz = strconv.FormatFloat(conf.BuyNum, 'f', s.lotSzN, 64)
		} else if side == "sell" {
			sz = strconv.FormatFloat(conf.SellNum, 'f', s.lotSzN, 64)
		}
	} else if conf.Mode == 1 {
		if side == "buy" {
			sz = strconv.FormatFloat(conf.BuyAmt/(pric), 'f', s.lotSzN, 64)
		} else if side == "sell" {
			sz = strconv.FormatFloat(conf.SellAmt/(pric), 'f', s.lotSzN, 64)
		}
	}
	px := strconv.FormatFloat(pric, 'f', s.tickSzN, 64)
	return px, sz
}

func (s *Server) initPostOrder() error {
	ticRes, err := s.restClient.GetMarketTicker(s.instId)
	if err != nil {
		return err
	}
	// pric, _ := strconv.ParseFloat(ticRes.Data[0].Last, 64)
	// s.sz = strconv.FormatFloat(s.conf.Amount/pric, 'f', s.lotSzN, 64)
	s.InsertTicker(ticRes.Data[0])
	last, _ := strconv.ParseFloat(ticRes.Data[0].Last, 64)
	index := 0
	conf := s.mgoConf.Load().(Config)
	for i := s.gridNum; i > 0; i-- {
		pric := last/math.Pow((1+conf.BuyGridSize), float64(i))
		px, sz := s.getSz(pric, "buy", conf)
		clOrdId := hex.EncodeToString([]byte(strconv.Itoa(index - i)))
		_, err := s.PostBuyTradeOrder(clOrdId, px, sz)
		if err != nil {
			return err
		}

		pric = last*math.Pow((1+conf.SellGridSize), float64(i))
		px, sz = s.getSz(pric, "sell", conf)
		px = strconv.FormatFloat(pric, 'f', s.tickSzN, 64)
		clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index + i)))
		_, err = s.PostSellTradeOrder(clOrdId, px, sz)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) GetMgoConfig() (Config, error) {
	conf := new(Config)//TODO:need to test!!!!
	err := s.mgo.FindOne(MGO_DB_NAME, MGO_COLLECTION_CONFIG_NAME, bson.M{"instId": s.instId}, conf)
	if err != nil {
		return Config{}, err
	}
	return *conf, nil
}

func (s *Server) Monitor() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			conf, err := s.GetMgoConfig()
			if err != nil {
				log.Error("Monitor", "GetMgoConfig err", err)
				continue
			}
			status := s.status.Load()
			if status == 0 {
				if conf.Status == 1 {
					s.Start()
				}
			} else if status == 1 {
				if conf.Status == 0 {
					s.Stop()
				} else if conf.Status == 1 {
					mgoConf := s.mgoConf.Load().(Config)
					if !reflect.DeepEqual(mgoConf, conf) {
						s.mgoConf.Store(conf)
					}
				}
			}
		case <-s.stop:
			return
		}
	}
}

func (s *Server) Start() error {
	s.wsClient.Start()
	err := s.wsClient.Login(s.conf.ApiKey, s.conf.Passphrase)
	if err != nil {
		return err
	}
	log.Info("websocket Login success")
	err = s.wsClient.Subscribe(okex.CHNL_OEDERS, okex.MARGIN, s.ReceivedOrdersDataCallback)
	if err != nil {
		return err
	}
	log.Info("websocket Subscribe success")

	conf, err := s.GetMgoConfig()
	if err != nil {
		return err
	}
	s.mgoConf.Store(conf)
	s.gridNum = conf.GridNum
	err = s.initPostOrder()
	if err != nil {
		return err
	}
	s.status.Store(1)
	return nil
}

func (s *Server) InsertTicker(ticket okex.Ticket) error {
	last, _ := strconv.ParseFloat(ticket.Last, 64)
	ts, _ := strconv.ParseInt(ticket.Ts, 10, 64)
	err := s.mgo.Insert(MGO_DB_NAME, MGO_COLLECTION_TICKET_NAME, bson.M{"instId": ticket.InstId, "last": last, "ts": ts})
	if err != nil {
		log.Error("InsertTicker", "error", err)
		return err
	}
	return nil
}

func (s *Server) InsertOrder(index int, order okex.DataOrder) error {
	px, _ := strconv.ParseFloat(order.Px, 64)
	sz, _ := strconv.ParseFloat(order.Sz, 64)
	avgPx, _ := strconv.ParseFloat(order.AvgPx, 64)
	fee, _ := strconv.ParseFloat(order.Fee, 64)
	fillTime, _ := strconv.ParseInt(order.FillTime, 10, 64)
	cTime, _ := strconv.ParseInt(order.CTime, 10, 64)
	err := s.mgo.Insert(MGO_DB_NAME, MGO_COLLECTION_TICKET_NAME, bson.M{"index": index, "instId": order.InstId, "side": order.Side, "px": px, "sz": sz, "avgPx": avgPx, "fee": fee, "fillTime": fillTime, "cTime": cTime})
	if err != nil {
		log.Error("InsertOrder", "error", err)
		return err
	}
	return nil
}

func (s *Server) ReceivedOrdersDataCallback(obj interface{}) error {
	res := obj.(*okex.WSOrdersResponse)
	if len(res.Data) == 0 {
		return nil
	}
	if res.Data[0].Code == "0" && res.Data[0].InstType == okex.MARGIN && res.Data[0].InstId == s.instId && res.Data[0].OrdType == "post_only" {
		if res.Data[0].State == "filled" {
			conf := s.mgoConf.Load().(Config)
			buyGridSize := 1+conf.BuyGridSize
			sellGridSize := 1+conf.SellGridSize
			clOrdId, _ := hex.DecodeString(res.Data[0].ClOrdId)
			index, _ := strconv.Atoi(string(clOrdId))
			go s.InsertOrder(index, res.Data[0])
			pri, _ := strconv.ParseFloat(res.Data[0].Px, 64)
			if res.Data[0].Side == "buy" {
				log.Debug("买单成交")
				pric := pri*sellGridSize
				px, sz := s.getSz(pric, "sell", conf)
				clOrdId := hex.EncodeToString([]byte(strconv.Itoa(index + 1)))
				_, err := s.PostSellTradeOrder(clOrdId, px, sz)
				if err != nil {
					log.Error("买单成交，挂卖单失败", "error", err)
					return err
				}
				log.Debug("买单成交，挂卖单成功")

				pric = pri/math.Pow(buyGridSize, float64(s.gridNum))
				px, sz = s.getSz(pric, "buy", conf)
				clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index - s.gridNum)))
				_, err = s.PostBuyTradeOrder(clOrdId, px, sz)
				if err != nil {
					log.Error("买单成交，挂买单失败", "error", err)
					return err
				}
				log.Debug("买单成交，挂买单成功")
				clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index + s.gridNum + 1)))
				_, err = s.restClient.PostTradeCancelOrder(s.instId, "", clOrdId)
				if err != nil {
					log.Error("买单成交，撤销卖单失败", "error", err)
					return err
				}
				log.Debug("买单成交，撤销卖单成功")
			} else if res.Data[0].Side == "sell" {
				log.Debug("卖单成交")
				pric := pri/buyGridSize
				px, sz := s.getSz(pric, "buy", conf)
				clOrdId := hex.EncodeToString([]byte(strconv.Itoa(index - 1)))
				_, err := s.PostBuyTradeOrder(clOrdId, px, sz)
				if err != nil {
					log.Error("卖单成交，挂买单失败", "error", err)
					return err
				}
				log.Debug("卖单成交，挂买单成功")
				pric = pri*math.Pow(sellGridSize, float64(s.gridNum))
				px, sz = s.getSz(pric, "sell", conf)
				clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index + s.gridNum)))
				_, err = s.PostSellTradeOrder(clOrdId, px, sz)
				if err != nil {
					log.Error("卖单成交，挂卖单失败", "error", err)
					return err
				}
				log.Debug("卖单成交，挂卖单成功")
				clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index - s.gridNum - 1)))
				_, err = s.restClient.PostTradeCancelOrder(s.instId, "", clOrdId)
				if err != nil {
					log.Error("卖单成交，撤销买单失败", "error", err)
					return err
				}
				log.Debug("卖单成交，撤销买单成功")
			}
		}
	}
	return nil
}

func (s *Server) PostTradeCancelBatchOrders(ordIds, clOrdIds []string) error {
	reqs := make([]map[string]string, 0)
	for i := 0; i < len(ordIds); i++ {
		req := okex.NewParams()
		req["instId"] = s.instId
		req["ordId"] = ordIds[i]
		req["clOrdId"] = clOrdIds[i]
		reqs = append(reqs, req)
	}
	_, err := s.restClient.PostTradeCancelBatchOrders(&reqs)
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) PostBuyTradeOrder(clOrdId, px, sz string) (string, error) {
	req := okex.NewParams()
	req["instId"] = s.instId
	req["tdMode"] = "cross"
	req["ccy"] = "USDT"
	req["side"] = "buy"
	req["ordType"] = "post_only"
	req["px"] = strings.TrimRight(strings.TrimRight(px, "0"), ".")
	req["sz"] = sz
	res, err := s.restClient.PostTradeOrder(&req)
	if err != nil {
		return "", err
	}
	return res.Data[0].OrdId, nil
}

func (s *Server) PostSellTradeOrder(clOrdId, px, sz string) (string, error) {
	req := okex.NewParams()
	req["instId"] = s.instId
	req["tdMode"] = "cross"
	req["ccy"] = "USDT"
	req["side"] = "sell"
	req["ordType"] = "post_only"
	req["px"] = strings.TrimRight(strings.TrimRight(px, "0"), ".")
	req["sz"] = sz
	res, err := s.restClient.PostTradeOrder(&req)
	if err != nil {
		return "", err
	}
	return res.Data[0].OrdId, nil
}

func (s *Server) CancelAllOrders() {
	req := okex.NewReqParams()
	req.AddParam("instType", okex.MARGIN)
	req.AddParam("instId", s.instId)
	req.AddParam("ordType", "post_only")
	trdp, err := s.restClient.GetTradeOrdersPending(req)
	if err != nil {
		log.Error("CancelAllOrders", "GetTradeOrdersPending err", err)
	}
	for key, val := range *trdp {
		if key == "data" {
			orders := val.([]map[string]string)
			for _, order := range orders {
				_, err := s.restClient.PostTradeCancelOrder(s.instId, order["ordId"], "")
				if err != nil {
					log.Error("CancelAllOrders", "PostTradeCancelOrder err", err)
				}
			}
			break
		}
	}
}

func (s *Server) Stop() error {
	s.wsClient.UnSubscribe(okex.CHNL_OEDERS, okex.MARGIN)
	s.wsClient.Stop()

	s.CancelAllOrders()
	s.mgo.DisConnect()
	s.status.Store(0)
	return nil
}

func (s *Server) Exit() error {
	s.Stop()
	close(s.stop)
	return nil
}

// Wait blocks the thread until the node is stopped. If the node is not running
// at the time of invocation, the method immediately returns.
func (s *Server) Wait() {
	stop := s.stop

	<-stop
	os.Exit(1)
}
