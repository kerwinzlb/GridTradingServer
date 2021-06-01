package server

import (
	"encoding/hex"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/kerwinzlb/GridTradingServer/common"
	"github.com/kerwinzlb/GridTradingServer/db"
	"github.com/kerwinzlb/GridTradingServer/log"
	"github.com/kerwinzlb/GridTradingServer/okex-sdk-api"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	MGO_DB_NAME                = "grid"
	MGO_COLLECTION_TICKET_NAME = "ticket"
	MGO_COLLECTION_ORDER_NAME  = "order"
)

type Server struct {
	conf         *okex.Config
	instId       string
	sz           string
	tickSzN      int
	lotSzN       int
	minSzN       int
	gridSize     float64
	gridNum      int
	restEndpoint string
	restClient   *okex.Client
	wsEndpoint   string
	wsClient     *okex.OKWSAgent
	mgo          *db.Mongo

	stop chan struct{} // Channel to wait for termination notifications
}

func New(instId string, conf *okex.Config) (*Server, error) {
	confCopy := *conf
	s := &Server{
		conf:         &confCopy,
		instId:       instId,
		gridSize:     1 + conf.GridSize,
		gridNum:      conf.GridNum,
		restEndpoint: conf.Endpoint,
		restClient:   okex.NewClient(*conf),
		wsEndpoint:   conf.WSEndpoint,
		wsClient:     new(okex.OKWSAgent),
		mgo:          db.NewMgo(conf.MgoEndpoint),
		stop:         make(chan struct{}),
	}

	pubRes, err := s.restClient.GetPublicInstruments("SPOT", "", s.instId)
	if err != nil {
		return nil, err
	}
	s.tickSzN = common.FloatRoundLen(pubRes.Data[0].TickSz)
	s.lotSzN = common.FloatRoundLen(pubRes.Data[0].LotSz)
	s.minSzN = common.FloatRoundLen(pubRes.Data[0].MinSz)
	return s, nil
}

func (s *Server) initPostOrder() error {
	ticRes, err := s.restClient.GetMarketTicker(s.instId)
	if err != nil {
		return err
	}
	pric, _ := strconv.ParseFloat(ticRes.Data[0].Last, 64)
	s.sz = strconv.FormatFloat(s.conf.Amount/pric, 'f', s.lotSzN, 64)
	err = s.mgo.Connect()
	if err != nil {
		return err
	}
	s.InsertTicker(ticRes.Data[0])
	last, _ := strconv.ParseFloat(ticRes.Data[0].Last, 64)
	index := 0
	for i := s.gridNum; i > 0; i-- {
		px := strconv.FormatFloat(last/math.Pow(s.gridSize, float64(i)), 'f', s.tickSzN, 64)
		clOrdId := hex.EncodeToString([]byte(strconv.Itoa(index - i)))
		_, px, err := s.PostBuyTradeOrder(clOrdId, px, s.sz)
		if err != nil {
			return err
		}

		px = strconv.FormatFloat(last*math.Pow(s.gridSize, float64(i)), 'f', s.tickSzN, 64)
		clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index + i)))
		_, px, err = s.PostSellTradeOrder(clOrdId, px, s.sz)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) Start() error {
	s.wsClient.Start(s.conf)
	err := s.wsClient.Login(s.conf.ApiKey, s.conf.Passphrase)
	if err != nil {
		return err
	}
	log.Info("websocket Login success")
	err = s.wsClient.Subscribe(okex.CHNL_OEDERS, "SPOT", s.ReceivedOrdersDataCallback)
	if err != nil {
		return err
	}
	log.Info("websocket Subscribe success")

	err = s.initPostOrder()
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) InsertTicker(ticket okex.Ticket) error {
	last, _ := strconv.ParseFloat(ticket.Last, 64)
	ts, _ := strconv.ParseInt(ticket.Ts, 10, 64)
	err := s.mgo.Insert(MGO_DB_NAME, MGO_COLLECTION_TICKET_NAME, bson.M{"instid": ticket.InstId, "last": last, "ts": ts})
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
	err := s.mgo.Insert(MGO_DB_NAME, MGO_COLLECTION_TICKET_NAME, bson.M{"index": index, "instid": order.InstId, "side": order.Side, "px": px, "sz": sz, "avgPx": avgPx, "fee": fee, "fillTime": fillTime, "cTime": cTime})
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
	if res.Data[0].Code == "0" && res.Data[0].InstType == "SPOT" && res.Data[0].InstId == s.instId && res.Data[0].OrdType == "post_only" {
		if res.Data[0].State == "filled" {
			clOrdId, _ := hex.DecodeString(res.Data[0].ClOrdId)
			index, _ := strconv.Atoi(string(clOrdId))
			go s.InsertOrder(index, res.Data[0])
			pri, _ := strconv.ParseFloat(res.Data[0].Px, 64)
			if res.Data[0].Side == "buy" {
				log.Debug("买单成交")
				px := strconv.FormatFloat(pri*s.gridSize, 'f', s.tickSzN, 64)
				clOrdId := hex.EncodeToString([]byte(strconv.Itoa(index + 1)))
				_, px, err := s.PostSellTradeOrder(clOrdId, px, s.sz)
				if err != nil {
					log.Error("买单成交，挂卖单失败", "error", err)
					return err
				}
				log.Debug("买单成交，挂卖单成功")

				px = strconv.FormatFloat(pri/math.Pow(s.gridSize, float64(s.gridNum)), 'f', s.tickSzN, 64)
				clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index - s.gridNum)))
				_, px, err = s.PostBuyTradeOrder(clOrdId, px, s.sz)
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
				px := strconv.FormatFloat(pri/s.gridSize, 'f', s.tickSzN, 64)
				clOrdId := hex.EncodeToString([]byte(strconv.Itoa(index - 1)))
				_, px, err := s.PostBuyTradeOrder(clOrdId, px, s.sz)
				if err != nil {
					log.Error("卖单成交，挂买单失败", "error", err)
					return err
				}
				log.Debug("卖单成交，挂买单成功")
				px = strconv.FormatFloat(pri*math.Pow(s.gridSize, float64(s.gridSize)), 'f', s.tickSzN, 64)
				clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index + s.gridNum)))
				_, px, err = s.PostSellTradeOrder(clOrdId, px, s.sz)
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

func (s *Server) PostBuyTradeOrder(clOrdId, px, sz string) (string, string, error) {
	req := okex.NewParams()
	req["instId"] = s.instId
	req["tdMode"] = "cash"
	req["side"] = "buy"
	req["ordType"] = "post_only"
	req["px"] = strings.TrimRight(strings.TrimRight(px, "0"), ".")
	req["sz"] = sz
	res, err := s.restClient.PostTradeOrder(&req)
	if err != nil {
		return "", "", err
	}
	return res.Data[0].OrdId, px, nil
}

func (s *Server) PostSellTradeOrder(clOrdId, px, sz string) (string, string, error) {
	req := okex.NewParams()
	req["instId"] = s.instId
	req["tdMode"] = "cash"
	req["side"] = "sell"
	req["ordType"] = "post_only"
	req["px"] = strings.TrimRight(strings.TrimRight(px, "0"), ".")
	req["sz"] = sz
	res, err := s.restClient.PostTradeOrder(&req)
	if err != nil {
		return "", "", err
	}
	return res.Data[0].OrdId, px, nil
}

func (s *Server) Stop() error {
	s.wsClient.UnSubscribe(okex.CHNL_OEDERS, "SPOT")
	s.wsClient.Stop()

	req := okex.NewReqParams()
	req.AddParam("instType", "SPOT")
	req.AddParam("instId", s.instId)
	req.AddParam("ordType", "post_only")
	trdp, err := s.restClient.GetTradeOrdersPending(req)
	if err != nil {
		log.Error("Server stop", "GetTradeOrdersPending err", err)
	}
	for key, val := range *trdp {
		if key == "data" {
			orders := val.([]map[string]string)
			for _, order := range orders {
				s.restClient.PostTradeCancelOrder(s.instId, order["ordId"], "")
			}
			break
		}
	}
	s.mgo.DisConnect()
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
