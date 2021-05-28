package server

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kerwinzlb/GridTradingServer/common"
	"github.com/kerwinzlb/GridTradingServer/log"
	"github.com/kerwinzlb/GridTradingServer/okex-sdk-api"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Server struct {
	conf         *okex.Config
	instId       string
	last         string
	sz           string
	tickSzN      int
	lotSzN       int
	minSzN       int
	gridSize     float64
	gridNum      int
	gridMap      map[string]string
	restEndpoint string
	restClient   *okex.Client
	wsEndpoint   string
	wsClient     *okex.OKWSAgent
	mgoEndpoint  string
	mgoClient    *mongo.Client
	mgoCancel    context.CancelFunc
	ctx          context.Context

	stop chan struct{} // Channel to wait for termination notifications
	lock *sync.RWMutex
}

func New(instId string, conf *okex.Config) (*Server, error) {
	confCopy := *conf
	s := &Server{
		conf:         &confCopy,
		instId:       instId,
		gridSize:     1 + conf.GridSize,
		gridNum:      conf.GridNum,
		gridMap:      make(map[string]string),
		restEndpoint: conf.Endpoint,
		restClient:   okex.NewClient(*conf),
		wsEndpoint:   conf.WSEndpoint,
		wsClient:     new(okex.OKWSAgent),
		mgoEndpoint:  conf.MgoEndpoint,
		mgoClient:    new(mongo.Client),
		stop:         make(chan struct{}),
		lock:         new(sync.RWMutex),
	}
	err := s.newMgo()
	if err != nil {
		return nil, err
	}
	ticRes, err := s.restClient.GetMarketTicker(s.instId)
	if err != nil {
		return nil, err
	}
	s.last = ticRes.Data[0].Last
	s.insertTicker(ticRes.Data[0].InstId, ticRes.Data[0].Last, ticRes.Data[0].Ts)
	pubRes, err := s.restClient.GetPublicInstruments("SPOT", "", s.instId)
	if err != nil {
		return nil, err
	}
	s.tickSzN = common.FloatRoundLen(pubRes.Data[0].TickSz)
	s.lotSzN = common.FloatRoundLen(pubRes.Data[0].LotSz)
	s.minSzN = common.FloatRoundLen(pubRes.Data[0].MinSz)
	pric, _ := strconv.ParseFloat(ticRes.Data[0].Last, 64)
	s.sz = strconv.FormatFloat(s.conf.Amount/pric, 'f', s.lotSzN, 64)
	return s, nil
}

func (s *Server) newMgo() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(s.mgoEndpoint))
	if err != nil {
		return err
	}
	s.mgoClient = client
	s.mgoCancel = cancel
	s.ctx = ctx
	return nil
}

func (s *Server) insertTicker(instid, last, ts string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := s.mgoClient.Database(s.conf.MgoDBName).Collection(s.conf.MgoCollectionName[1])
	collection.InsertOne(ctx, bson.M{"instid": instid, "last": last, "ts": ts})
	return nil
}

func (s *Server) insertOrders(instid, side, px, sz, avgPx, fee, fillTime, cTime string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := s.mgoClient.Database(s.conf.MgoDBName).Collection(s.conf.MgoCollectionName[0])
	collection.InsertOne(ctx, bson.M{"instid": instid, "side": side, "px": px, "sz": sz, "avgPx": avgPx, "fee": fee, "fillTime": fillTime, "cTime": cTime})
	return nil
}

func (s *Server) Start() error {
	buyPri, _ := strconv.ParseFloat(s.last, 64)
	sellPri, _ := strconv.ParseFloat(s.last, 64)
	for i := 0; i < s.gridNum; i++ {
		buyPri = common.FloatRound(buyPri/s.gridSize, s.tickSzN)
		px := strconv.FormatFloat(buyPri, 'f', s.tickSzN, 64)
		ordId, px, err := s.PostBuyTradeOrder(px, s.sz)
		if err != nil {
			return err
		}
		s.setGridMap(px, ordId)

		sellPri = common.FloatRound(sellPri*s.gridSize, s.tickSzN)
		px = strconv.FormatFloat(sellPri, 'f', s.tickSzN, 64)
		ordId, px, err = s.PostSellTradeOrder(px, s.sz)
		if err != nil {
			return err
		}
		s.setGridMap(px, ordId)
	}

	s.wsClient.Start(s.conf)
	err := s.wsClient.Login(s.conf.ApiKey, s.conf.Passphrase)
	if err != nil {
		return err
	}
	s.Subscribe()

	return nil
}

func (s *Server) setGridMap(px, OrdId string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.gridMap[px] = OrdId
	return
}

func (s *Server) getGridMap(px string) string {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.gridMap[px]
}

func (s *Server) deleteGridMap(px string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.gridMap, px)
	return
}

func (s *Server) Subscribe() error {
	err := s.wsClient.Subscribe(okex.CHNL_OEDERS, "SPOT", s.ReceivedOrdersDataCallback)
	if err != nil {
		return err
	}
	time.Sleep(time.Second)
	return nil
}

func (s *Server) UnSubscribe() error {
	err := s.wsClient.UnSubscribe(okex.CHNL_OEDERS, "SPOT")
	if err != nil {
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
			s.deleteGridMap(res.Data[0].Px)
			s.insertOrders(res.Data[0].InstId, res.Data[0].Side, res.Data[0].Px, res.Data[0].Sz, res.Data[0].AvgPx, res.Data[0].Fee, res.Data[0].FillTime, res.Data[0].CTime)
			sellPri, _ := strconv.ParseFloat(res.Data[0].Px, 64)
			buyPri, _ := strconv.ParseFloat(res.Data[0].Px, 64)
			if res.Data[0].Side == "buy" {
				log.Debug("买单成交")
				//增加一个卖单
				sellPri = common.FloatRound(sellPri*s.gridSize, s.tickSzN)
				px := strconv.FormatFloat(sellPri, 'f', s.tickSzN, 64)
				ordId, px, err := s.PostSellTradeOrder(px, s.sz)
				if err != nil {
					log.Debug("买单成交，挂卖单失败", err)
					return err
				}
				s.setGridMap(px, ordId)
				log.Debug("买单成交，挂卖单成功")

				for i := 0; i < s.gridNum; i++ {
					buyPri = common.FloatRound(buyPri/s.gridSize, s.tickSzN)
					sellPri = common.FloatRound(sellPri*s.gridSize, s.tickSzN)
				}
				//增加一个买单
				px = strconv.FormatFloat(buyPri, 'f', s.tickSzN, 64)
				ordId, px, err = s.PostBuyTradeOrder(px, s.sz)
				if err != nil {
					log.Debug("买单成交，挂买单失败", err)
					return err
				}
				s.setGridMap(px, ordId)
				log.Debug("买单成交，挂买单成功")
				//撤销一个卖单
				px = strings.TrimRight(strings.TrimRight(strconv.FormatFloat(sellPri, 'f', s.tickSzN, 64), "0"), ".")
				OrdId := s.getGridMap(px)
				err = s.PostTradeCancelOrder(OrdId, "")
				if err != nil {
					log.Debug("买单成交，撤销卖单失败", err)
					return err
				}
				s.deleteGridMap(px)
				log.Debug("买单成交，撤销卖单成功")
			} else if res.Data[0].Side == "sell" {
				log.Debug("卖单成交")
				//增加一个买单
				buyPri = common.FloatRound(buyPri/s.gridSize, s.tickSzN)
				px := strconv.FormatFloat(buyPri, 'f', s.tickSzN, 64)
				ordId, px, err := s.PostBuyTradeOrder(px, s.sz)
				if err != nil {
					log.Debug("卖单成交，挂买单失败", err)
					return err
				}
				s.setGridMap(px, ordId)
				log.Debug("卖单成交，挂买单成功")
				for i := 0; i < s.gridNum; i++ {
					buyPri = common.FloatRound(buyPri/s.gridSize, s.tickSzN)
					sellPri = common.FloatRound(sellPri*s.gridSize, s.tickSzN)
				}
				//增加一个卖单
				px = strconv.FormatFloat(sellPri, 'f', s.tickSzN, 64)
				ordId, px, err = s.PostSellTradeOrder(px, s.sz)
				if err != nil {
					log.Debug("卖单成交，挂卖单失败", err)
					return err
				}
				s.setGridMap(px, ordId)
				log.Debug("卖单成交，挂卖单成功")
				//撤销一个买单
				px = strings.TrimRight(strings.TrimRight(strconv.FormatFloat(buyPri, 'f', s.tickSzN, 64), "0"), ".")
				OrdId := s.getGridMap(px)
				err = s.PostTradeCancelOrder(OrdId, "")
				if err != nil {
					log.Debug("卖单成交，撤销买单失败", err)
					return err
				}
				s.deleteGridMap(px)
				log.Debug("卖单成交，撤销买单成功")
			}
		}

		// if res.Data[0].State == "live" {
		// 	s.setGridMap(res.Data[0].Px, res.Data[0].OrdId)
		// 	// log.Debug("px sz OrdId", res.Data[0].Px, res.Data[0].Sz, res.Data[0].OrdId)
		// }

		// if res.Data[0].State == "canceled" {
		// 	s.deleteGridMap(res.Data[0].Px)
		// }

	}
	return nil
}

func (s *Server) PostTradeCancelOrder(ordId, clOrdId string) error {
	_, err := s.restClient.PostTradeCancelOrder(s.instId, ordId, clOrdId)
	if err != nil {
		return err
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

func (s *Server) PostBuyTradeOrder(px, sz string) (string, string, error) {
	// log.Debug("PostBuyTradeOrder px sz", px, sz)
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

func (s *Server) PostSellTradeOrder(px, sz string) (string, string, error) {
	// log.Debug("PostSellTradeOrder px sz", px, sz)
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
	s.lock.Lock()
	defer s.lock.Unlock()

	s.UnSubscribe()
	s.wsClient.Stop()
	for _, val := range s.gridMap {
		s.PostTradeCancelOrder(val, "")
	}

	s.mgoClient.Disconnect(s.ctx)
	s.mgoCancel()
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
